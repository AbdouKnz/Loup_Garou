// Package room gère les salles d'attente (lobby) et leurs joueurs.
//
// Chaque salle est un acteur : une goroutine unique détient son état et ne le
// modifie qu'en réponse à des commandes reçues par canal. Aucun verrou n'est
// donc nécessaire, et l'ordre des événements est celui des commandes.
//
// La salle ne connaît ni HTTP ni WebSocket : elle ne parle que par l'interface
// Client, ce qui la rend testable sans réseau.
package room

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/protocol"
)

// ErrClosed signale qu'une salle n'accepte plus de commandes.
var ErrClosed = errors.New("salle fermée")

// PlayerID identifie un joueur au sein d'une salle.
type PlayerID string

// Client est la connexion d'un joueur, fournie par la couche réseau.
//
// Send ne doit jamais bloquer : la couche réseau met les trames en file
// d'attente et ferme la connexion si le client ne consomme pas assez vite.
// L'acteur de la salle appelle Send depuis sa propre goroutine.
type Client interface {
	Send(env protocol.Envelope)
	// Kick dépose une dernière trame puis ferme la connexion. La couche réseau
	// garantit que la trame part avant la fermeture.
	Kick(env protocol.Envelope)
}

// Player est l'état interne d'un joueur ; il n'est jamais transmis tel quel.
type Player struct {
	ID     PlayerID
	Pseudo string
	// Account est l'identifiant du compte qui tient cette place : c'est lui qui
	// rend l'entrée idempotente (un même compte ne prend qu'une place). Vide
	// pour un bot.
	Account string
	// Token est le secret de session : il ne quitte jamais le serveur.
	Token        string
	Ready        bool
	IsHost       bool
	joinedAt     time.Time
	offlineSince time.Time
	chatTimes    []time.Time
	// IsBot signale un joueur automatique : pas de connexion, il est toujours
	// considéré comme présent et prêt.
	IsBot bool
	// botAction retient le dernier fait d'arme d'un bot, pour qu'il n'agisse
	// qu'une fois par phase : la clé est « phase-jour-tour ».
	botAction string
}

// Options règle le comportement d'une salle (surchargé dans les tests).
type Options struct {
	// MaxPlayers borne le nombre de joueurs.
	MaxPlayers int
	// OfflineTimeout est le délai avant qu'un joueur hors ligne soit retiré.
	OfflineTimeout time.Duration
	// EmptyRoomTimeout est le délai avant qu'une salle vide soit fermée.
	EmptyRoomTimeout time.Duration
	// SweepInterval est la fréquence du balayage d'entretien.
	SweepInterval time.Duration
	// Game règle la partie elle-même (durées des phases, table minimale).
	Game game.Config
	// BotDelay est le temps de réflexion laissé aux bots avant chaque vague
	// d'actions : les phases restent lisibles pour les humains.
	BotDelay time.Duration
	// GameSeed fixe la graine du tirage des rôles. Zéro demande une graine
	// aléatoire : c'est le réglage de production, les tests la fixent pour être
	// reproductibles.
	GameSeed int64
}

// DefaultOptions renvoie les réglages de production.
func DefaultOptions() Options {
	return Options{
		MaxPlayers:       protocol.MaxPlayers,
		OfflineTimeout:   5 * time.Minute,
		EmptyRoomTimeout: 30 * time.Minute,
		SweepInterval:    30 * time.Second,
		Game:             game.DefaultConfig(),
		BotDelay:         2 * time.Second,
	}
}

// withDefaults complète les champs laissés à zéro.
func (o Options) withDefaults() Options {
	def := DefaultOptions()

	if o.MaxPlayers > 0 {
		def.MaxPlayers = o.MaxPlayers
	}
	if o.OfflineTimeout > 0 {
		def.OfflineTimeout = o.OfflineTimeout
	}
	if o.EmptyRoomTimeout > 0 {
		def.EmptyRoomTimeout = o.EmptyRoomTimeout
	}
	if o.SweepInterval > 0 {
		def.SweepInterval = o.SweepInterval
	}
	def.Game = o.Game.WithDefaults()
	def.GameSeed = o.GameSeed
	// Une valeur négative désactive la vague de bots (tests déterministes).
	if o.BotDelay != 0 {
		def.BotDelay = o.BotDelay
	}

	return def
}

// cmdKind énumère les commandes acceptées par l'acteur.
type cmdKind int

const (
	cmdAuth cmdKind = iota
	cmdJoin
	cmdDisconnect
	cmdLeave
	cmdChat
	cmdReady
	cmdStart
	cmdAgain
	cmdVote
	cmdNightVote
	cmdNightAction
	// cmdSpeak clôt la prise de parole de son auteur (débat).
	cmdSpeak
	cmdAddBots
	cmdRemoveBots
	cmdBotTurn
	cmdTick
	cmdSummary
)

// command est une intention adressée à l'acteur de la salle.
type command struct {
	kind    cmdKind
	account string
	pseudo  string
	token   string
	text    string
	action  string
	target  string
	ready   bool
	count   int
	client  Client
	player  PlayerID
	at      time.Time
	reply   chan result
}

// result est la réponse de l'acteur à une commande.
type result struct {
	playerID PlayerID
	token    string
	summary  protocol.RoomSummary
	err      error
}

// Room est une salle : un acteur qui sérialise toutes les mutations d'état.
//
// Toutes les méthodes publiques sont sûres depuis n'importe quelle goroutine :
// elles déposent une commande sur un canal et attendent la réponse. L'état
// n'est lu et modifié que par la goroutine qui exécute Run.
type Room struct {
	code     string
	opts     Options
	onClose  func(code string)
	commands chan command
	done     chan struct{}

	closeOnce sync.Once

	// --- état réservé à la goroutine de l'acteur ---
	createdAt     time.Time
	everConnected bool
	emptySince    time.Time
	hostID        PlayerID
	players       map[PlayerID]*Player
	order         []PlayerID
	clients       map[PlayerID]Client
	messages      []protocol.ChatMessage
	rng           *mathrand.Rand
	game          *game.Game
	timer         *time.Timer
	// botTimer programme la prochaine vague d'actions des bots.
	botTimer *time.Timer
}

// New crée une salle dont l'hôte est déjà inscrit.
//
// La salle ne traite les commandes qu'à partir de l'appel à Run ; onClose est
// appelée si la salle décide de se fermer (elle est alors retirée du registre).
func New(code, hostAccount, hostPseudo string, opts Options, onClose func(code string)) (*Room, PlayerID, string) {
	now := time.Now()
	opts = opts.withDefaults()

	seed := opts.GameSeed
	if seed == 0 {
		seed = now.UnixNano()
	}

	r := &Room{
		code:      code,
		opts:      opts,
		onClose:   onClose,
		commands:  make(chan command),
		done:      make(chan struct{}),
		createdAt: now,
		players:   make(map[PlayerID]*Player),
		clients:   make(map[PlayerID]Client),
		rng:       mathrand.New(mathrand.NewSource(seed)),
	}

	hostID, token := r.addPlayer(hostAccount, hostPseudo, true, now)
	r.hostID = hostID

	return r, hostID, token
}

// Run exécute la boucle de l'acteur jusqu'à l'arrêt de la salle.
func (r *Room) Run(ctx context.Context) {
	ticker := time.NewTicker(r.opts.SweepInterval)
	defer ticker.Stop()
	defer r.shutdown()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.done:
			return
		case cmd := <-r.commands:
			r.execute(cmd)
		case now := <-ticker.C:
			r.sweep(now)
		}
	}
}

// Close arrête la salle (arrêt du serveur ou fermeture explicite).
func (r *Room) Close() { r.shutdown() }

// Code renvoie le code de la partie.
func (r *Room) Code() string { return r.code }

// Authenticate rattache une connexion au joueur détenteur du jeton.
//
// La salle confirme l'identité puis diffuse l'état complet à tous les joueurs :
// l'arrivée (ou le retour) du joueur est ainsi vue par tout le monde.
func (r *Room) Authenticate(token string, client Client) (PlayerID, error) {
	res, err := r.call(command{kind: cmdAuth, token: token, client: client})
	if err != nil {
		return "", err
	}
	return res.playerID, res.err
}

// Join inscrit un nouveau joueur dans le salon d'attente, au nom de son compte.
//
// Un compte qui tient déjà une place la retrouve : un rafraîchissement du
// navigateur ou une seconde tentative ne duplique personne.
func (r *Room) Join(account, pseudo string) (PlayerID, string, error) {
	res, err := r.call(command{kind: cmdJoin, account: account, pseudo: pseudo})
	if err != nil {
		return "", "", err
	}
	return res.playerID, res.token, res.err
}

// Disconnect détache la connexion d'un joueur : il devient « hors ligne » et
// garde sa place le temps de se reconnecter.
func (r *Room) Disconnect(client Client) {
	_, _ = r.call(command{kind: cmdDisconnect, client: client})
}

// Leave retire définitivement un joueur de la partie.
func (r *Room) Leave(player PlayerID) {
	_, _ = r.call(command{kind: cmdLeave, player: player})
}

// Chat publie un message dans le fil de la salle.
func (r *Room) Chat(player PlayerID, text string) error {
	clean, err := protocol.NormalizeChat(text)
	if err != nil {
		return err
	}

	res, err := r.call(command{kind: cmdChat, player: player, text: clean})
	if err != nil {
		return err
	}
	return res.err
}

// SetReady change l'état « prêt » d'un joueur.
func (r *Room) SetReady(player PlayerID, ready bool) error {
	res, err := r.call(command{kind: cmdReady, player: player, ready: ready})
	if err != nil {
		return err
	}
	return res.err
}

// StartGame lance la partie : l'hôte distribue les rôles et le narrateur prend
// la main.
func (r *Room) StartGame(player PlayerID) error {
	res, err := r.call(command{kind: cmdStart, player: player})
	if err != nil {
		return err
	}
	return res.err
}

// Again renvoie la table en salle d'attente après une partie : l'hôte décide
// d'en lancer une nouvelle.
func (r *Room) Again(player PlayerID) error {
	res, err := r.call(command{kind: cmdAgain, player: player})
	if err != nil {
		return err
	}
	return res.err
}

// Vote désigne le suspect du village (phase de vote).
func (r *Room) Vote(player PlayerID, target string) error {
	res, err := r.call(command{kind: cmdVote, player: player, target: target})
	if err != nil {
		return err
	}
	return res.err
}

// NightVote désigne la victime des loups (phase de nuit).
func (r *Room) NightVote(player PlayerID, target string) error {
	res, err := r.call(command{kind: cmdNightVote, player: player, target: target})
	if err != nil {
		return err
	}
	return res.err
}

// NightAction est l'action d'un rôle pendant sa veillée : la voyante observe un
// joueur, la sorcière se sert de ses potions. Le moteur refuse ce qui n'est pas
// attendu de ce joueur à cet instant.
func (r *Room) NightAction(player PlayerID, action, target string) error {
	res, err := r.call(command{kind: cmdNightAction, player: player, action: action, target: target})
	if err != nil {
		return err
	}
	return res.err
}

// SpeakDone clôt la prise de parole de son auteur : le débat passe au vivant
// suivant. Le moteur refuse quiconque n'a pas la parole.
func (r *Room) SpeakDone(player PlayerID) error {
	res, err := r.call(command{kind: cmdSpeak, player: player})
	if err != nil {
		return err
	}
	return res.err
}

// AddBots inscrit des joueurs automatiques : réservé à l'hôte, en salle
// d'attente. Le nombre demandé est plafonné aux places restantes.
func (r *Room) AddBots(player PlayerID, count int) error {
	res, err := r.call(command{kind: cmdAddBots, player: player, count: count})
	if err != nil {
		return err
	}
	return res.err
}

// RemoveBots retire un bot, ou tous les bots si target est vide : réservé à
// l'hôte, en salle d'attente.
func (r *Room) RemoveBots(player PlayerID, target string) error {
	res, err := r.call(command{kind: cmdRemoveBots, player: player, target: target})
	if err != nil {
		return err
	}
	return res.err
}

// botTurn fait jouer les bots qui ont quelque chose à faire à cet instant :
// c'est la vague programmée par armBotWave. Sans effet hors partie.
func (r *Room) botTurn(now time.Time) error {
	res, err := r.call(command{kind: cmdBotTurn, at: now})
	if err != nil {
		return err
	}
	return res.err
}

// expire fait avancer la partie : appelée par la minuterie de la salle à
// l'échéance de la phase en cours. Une échéance périmée est sans effet.
func (r *Room) expire(now time.Time) error {
	res, err := r.call(command{kind: cmdTick, at: now})
	if err != nil {
		return err
	}
	return res.err
}

// Summary renvoie un résumé de la salle (écran « rejoindre »).
func (r *Room) Summary() (protocol.RoomSummary, error) {
	res, err := r.call(command{kind: cmdSummary})
	if err != nil {
		return protocol.RoomSummary{}, err
	}
	return res.summary, res.err
}

// call dépose une commande et attend son résultat.
func (r *Room) call(cmd command) (result, error) {
	cmd.reply = make(chan result, 1)

	select {
	case r.commands <- cmd:
	case <-r.done:
		return result{}, ErrClosed
	}

	select {
	case res := <-cmd.reply:
		return res, nil
	case <-r.done:
		return result{}, ErrClosed
	}
}

// shutdown ferme la salle une seule fois : les commandes en attente échouent
// immédiatement au lieu de bloquer.
func (r *Room) shutdown() {
	r.closeOnce.Do(func() {
		r.stopTimer()
		r.stopBotTimer()
		close(r.done)
	})
}

// execute traite une commande. Le résultat est toujours transmis, même en cas
// d'erreur, pour ne jamais laisser l'appelant bloqué.
func (r *Room) execute(cmd command) {
	switch cmd.kind {
	case cmdAuth:
		r.doAuthenticate(cmd)
	case cmdJoin:
		r.doJoin(cmd)
	case cmdDisconnect:
		r.doDisconnect(cmd)
	case cmdLeave:
		r.doLeave(cmd)
	case cmdChat:
		r.doChat(cmd)
	case cmdReady:
		r.doReady(cmd)
	case cmdStart:
		r.doStart(cmd)
	case cmdAgain:
		r.doAgain(cmd)
	case cmdVote:
		r.doVote(cmd, game.CommandVillageVote)
	case cmdNightVote:
		r.doVote(cmd, game.CommandWolfVote)
	case cmdNightAction:
		r.doNightAction(cmd)
	case cmdSpeak:
		r.doSpeak(cmd)
	case cmdAddBots:
		r.doAddBots(cmd)
	case cmdRemoveBots:
		r.doRemoveBots(cmd)
	case cmdBotTurn:
		r.doBotTurn(cmd)
	case cmdTick:
		r.doTick(cmd)
	case cmdSummary:
		r.doSummary(cmd)
	default:
		cmd.reply <- result{err: protocol.NewError(protocol.CodeInternal, "Commande inconnue.")}
	}
}

func (r *Room) doAuthenticate(cmd command) {
	player := r.playerByToken(cmd.token)
	if player == nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeNotAuthenticated,
			"Cette place n'existe plus dans la partie. Rejoignez-la à nouveau.")}
		return
	}

	// Une seule connexion active par joueur : la nouvelle remplace l'ancienne.
	//
	// L'ancienne reçoit l'explication puis est refermée, sans quoi un onglet
	// abandonné pourrait continuer à jouer dans le dos de celui qui vient de
	// reprendre la place.
	if previous, ok := r.clients[player.ID]; ok && previous != cmd.client {
		previous.Kick(protocol.MustEncode(protocol.TypeError, protocol.ErrorPayload{
			Code:    protocol.CodeUnauthorized,
			Message: "Cette place a été reprise depuis un autre onglet.",
		}))
	}

	r.clients[player.ID] = cmd.client
	player.offlineSince = time.Time{}
	r.everConnected = true

	cmd.client.Send(protocol.MustEncode(protocol.TypeAuthOK, protocol.AuthOKPayload{
		PlayerID: string(player.ID),
	}))
	r.broadcastState()

	cmd.reply <- result{playerID: player.ID}
}

func (r *Room) doJoin(cmd command) {
	// Un compte ne tient qu'une place. S'il en a déjà une, on la lui rend : la
	// table ne bouge pas, personne n'est annoncé deux fois.
	if existing := r.playerByAccount(cmd.account); existing != nil {
		cmd.reply <- result{playerID: existing.ID, token: existing.Token}
		return
	}

	pseudo, err := protocol.NormalizePseudo(cmd.pseudo)
	if err != nil {
		cmd.reply <- result{err: err}
		return
	}

	// Une partie en cours n'accepte plus personne : les rôles sont distribués,
	// un arrivant serait spectateur malgré lui.
	if r.game != nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeGameInProgress,
			"La partie a déjà commencé. Attendez qu'elle se termine pour entrer.")}
		return
	}

	if len(r.players) >= r.opts.MaxPlayers {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeRoomFull,
			"La partie est complète (%d joueurs).", r.opts.MaxPlayers)}
		return
	}

	playerID, token := r.addPlayer(cmd.account, pseudo, false, time.Now())
	r.addSystemMessagef("%s a rejoint la partie.", pseudo)
	r.broadcastState()

	cmd.reply <- result{playerID: playerID, token: token}
}

func (r *Room) doDisconnect(cmd command) {
	for id, client := range r.clients {
		if client != cmd.client {
			continue
		}

		delete(r.clients, id)
		r.players[id].offlineSince = time.Now()
		r.broadcastState()
		break
	}

	cmd.reply <- result{}
}

func (r *Room) doLeave(cmd command) {
	// En pleine partie, quitter n'est pas une option : la table est fixée, et
	// un loup qui s'en va fausserait la partie. La place est conservée et le
	// joueur peut revenir avec son jeton.
	if r.game != nil {
		if player, ok := r.players[cmd.player]; ok {
			delete(r.clients, cmd.player)
			player.offlineSince = time.Now()
			r.broadcastState()
		}

		cmd.reply <- result{}
		return
	}

	r.removePlayer(cmd.player, "a quitté la partie")
	cmd.reply <- result{}
}

func (r *Room) doChat(cmd command) {
	player, ok := r.players[cmd.player]
	if !ok {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeNotAuthenticated,
			"Vous ne faites pas partie de cette partie.")}
		return
	}

	// Le silence de la nuit et des morts : vérifié avant la limite de débit,
	// pour qu'un message refusé ne consomme pas de jeton.
	if err := r.canChat(player); err != nil {
		cmd.reply <- result{err: err}
		return
	}

	now := time.Now()
	if !r.allowChat(player, now) {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeRateLimited,
			"Doucement ! Patientez quelques secondes avant d'envoyer un nouveau message.")}
		return
	}

	message := protocol.ChatMessage{
		ID:       newID("M"),
		PlayerID: string(player.ID),
		Pseudo:   player.Pseudo,
		Text:     cmd.text,
		At:       now.UTC(),
	}

	r.appendMessage(message)
	r.broadcastChat(message)

	cmd.reply <- result{}
}

func (r *Room) doReady(cmd command) {
	player, ok := r.players[cmd.player]
	if !ok {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeNotAuthenticated,
			"Vous ne faites pas partie de cette partie.")}
		return
	}

	player.Ready = cmd.ready
	r.broadcastState()

	cmd.reply <- result{}
}

func (r *Room) doStart(cmd command) {
	player, ok := r.players[cmd.player]
	if !ok || !player.IsHost {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeUnauthorized,
			"Seul l'hôte peut lancer la partie.")}
		return
	}

	if r.game != nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeGameInProgress,
			"La partie a déjà commencé.")}
		return
	}

	if min := r.opts.Game.MinPlayers; len(r.order) < min {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeNotEnoughPlayers,
			"Il faut au moins %d joueurs pour lancer la partie (%d pour le moment).", min, len(r.order))}
		return
	}

	players := make([]game.Player, 0, len(r.order))
	for _, id := range r.order {
		players = append(players, game.Player{ID: string(id), Name: r.players[id].Pseudo})
	}

	salle, events, err := game.New(players, r.opts.Game, r.rng, time.Now())
	if err != nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeInternal,
			"Impossible de distribuer les rôles : %s", game.MessageOf(err))}
		return
	}

	r.game = salle
	r.narrate(events...)
	r.armTimer(salle.Deadline())
	r.armBotWave()
	r.broadcastState()

	cmd.reply <- result{}
}

// doAgain renvoie la table en salle d'attente après une partie terminée.
func (r *Room) doAgain(cmd command) {
	player, ok := r.players[cmd.player]
	if !ok || !player.IsHost {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeUnauthorized,
			"Seul l'hôte peut relancer une partie.")}
		return
	}

	if r.game == nil || !r.game.Over() {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeWrongPhase,
			"La partie n'est pas terminée.")}
		return
	}

	r.game = nil
	r.stopTimer()
	r.stopBotTimer()

	for _, p := range r.players {
		// Un bot reste toujours prêt : la table peut relancer aussitôt.
		p.Ready = p.IsBot
		p.botAction = ""
	}

	r.addSystemMessagef("Une nouvelle partie se prépare : chacun se déclare prêt.")
	r.broadcastState()

	cmd.reply <- result{}
}

// doVote enregistre l'action d'un joueur et rediffuse la table : les autres
// doivent voir qui a déjà parlé.
//
// Le moteur est seul juge : la salle ne vérifie ni la phase, ni le droit de
// voter, ni la cible. Elle se contente de traduire le refus.
func (r *Room) doVote(cmd command, kind game.CommandKind) {
	if r.game == nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeWrongPhase,
			"Aucune partie n'est en cours.")}
		return
	}

	err := r.play(game.Command{
		Kind:   kind,
		Actor:  string(cmd.player),
		Target: cmd.target,
	}, time.Now())
	if err != nil {
		cmd.reply <- result{err: gameRefusal(err)}
		return
	}

	// Plus personne à attendre ? La phase se clôt sans faire patienter la table.
	r.closeIfEveryoneVoted()
	r.broadcastState()

	cmd.reply <- result{}
}

// doNightAction fait jouer un rôle pendant sa veillée : la voyante observe, la
// sorcière se sert de ses potions. Une veillée n'attend qu'un acteur ; une fois
// sa décision prise, la nuit peut passer à la suivante.
func (r *Room) doNightAction(cmd command) {
	if r.game == nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeWrongPhase,
			"Aucune partie n'est en cours.")}
		return
	}

	err := r.play(game.Command{
		Kind:   game.CommandNightAction,
		Actor:  string(cmd.player),
		Action: game.ActionKind(cmd.action),
		Target: cmd.target,
	}, time.Now())
	if err != nil {
		cmd.reply <- result{err: gameRefusal(err)}
		return
	}

	r.closeIfEveryoneVoted()
	r.broadcastState()

	cmd.reply <- result{}
}

// doSpeak clôt la prise de parole d'un joueur : le débat passe au suivant sans
// attendre l'échéance. Le moteur est seul juge du tour de parole.
func (r *Room) doSpeak(cmd command) {
	if r.game == nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeWrongPhase,
			"Aucune partie n'est en cours.")}
		return
	}

	err := r.play(game.Command{Kind: game.CommandSpeak, Actor: string(cmd.player)}, time.Now())
	if err != nil {
		cmd.reply <- result{err: gameRefusal(err)}
		return
	}

	r.broadcastState()

	cmd.reply <- result{}
}

// doAddBots ajoute des joueurs automatiques : hôte uniquement, en salle
// d'attente. Les bots se déclarent prêts d'eux-mêmes.
func (r *Room) doAddBots(cmd command) {
	if player, ok := r.players[cmd.player]; !ok || !player.IsHost {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeUnauthorized,
			"Seul l'hôte peut ajouter des bots.")}
		return
	}

	if r.game != nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeGameInProgress,
			"La partie a commencé : on n'ajoute plus de bots.")}
		return
	}

	if cmd.count < 1 {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeInvalidMessage,
			"Ajoutez au moins un bot.")}
		return
	}

	if seats := r.opts.MaxPlayers - len(r.players); seats <= 0 {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeRoomFull,
			"La partie est complète (%d joueurs).", r.opts.MaxPlayers)}
		return
	} else if cmd.count > seats {
		cmd.count = seats
	}

	names := make([]string, 0, cmd.count)
	for i := 0; i < cmd.count; i++ {
		name := r.nextBotName()
		r.addBot(name, time.Now())
		names = append(names, name)
	}

	r.addSystemMessagef("%s", botsArrivedText(names))
	r.broadcastState()

	cmd.reply <- result{}
}

// doRemoveBots retire un bot, ou tous les bots : hôte uniquement, en salle
// d'attente.
func (r *Room) doRemoveBots(cmd command) {
	if player, ok := r.players[cmd.player]; !ok || !player.IsHost {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeUnauthorized,
			"Seul l'hôte peut retirer les bots.")}
		return
	}

	if r.game != nil {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeGameInProgress,
			"La partie a commencé : les places sont fixées.")}
		return
	}

	var removed []string

	if cmd.target == "" {
		for _, id := range append([]PlayerID(nil), r.order...) {
			if player := r.players[id]; player != nil && player.IsBot {
				removed = append(removed, player.Pseudo)
				r.removeBot(id)
			}
		}
	} else {
		player, ok := r.players[PlayerID(cmd.target)]
		if !ok || !player.IsBot {
			cmd.reply <- result{err: protocol.NewError(protocol.CodeBadTarget,
				"Ce joueur n'est pas un bot.")}
			return
		}
		removed = append(removed, player.Pseudo)
		r.removeBot(player.ID)
	}

	if len(removed) == 0 {
		cmd.reply <- result{err: protocol.NewError(protocol.CodeBadTarget,
			"Aucun bot à retirer.")}
		return
	}

	r.addSystemMessagef("%s", botsLeftText(removed))
	r.broadcastState()

	cmd.reply <- result{}
}

// doBotTurn fait jouer la vague de bots à l'instant prévu : chacun agit au plus
// une fois par phase, puis la salle rediffuse la table.
func (r *Room) doBotTurn(cmd command) {
	if r.game != nil && !r.game.Over() {
		r.runBots(cmd.at)
	}

	cmd.reply <- result{}
}

// doTick clôt la phase en cours à son échéance.
//
// Une minuterie périmée (la phase a changé entre-temps) est simplement ignorée :
// le moteur refuse une avance prématurée, ce n'est pas une erreur.
func (r *Room) doTick(cmd command) {
	reply := cmd.reply
	defer func() {
		if reply != nil {
			reply <- result{}
		}
	}()

	if r.game == nil || r.game.Over() {
		return
	}

	if err := r.play(game.Command{Kind: game.CommandAdvance}, cmd.at); err != nil {
		r.armTimer(r.game.Deadline())
		return
	}

	r.broadcastState()
}

// play applique une commande au moteur, raconte ce qu'il en sort et reprend le
// chronomètre de la nouvelle phase.
func (r *Room) play(cmd game.Command, at time.Time) error {
	events, err := r.game.Apply(cmd, at)
	if err != nil {
		return err
	}

	r.narrate(events...)
	r.armTimer(r.game.Deadline())
	r.armBotWave()

	return nil
}

// closeIfEveryoneVoted clôt la phase en cours dès que tous ceux qui devaient
// se prononcer l'ont fait.
//
// La distribution des rôles en est exclue : elle n'attend personne, mais c'est
// le moteur — et lui seul — qui décide du temps laissé à la table pour lire son
// rôle. Sans cette exception, une vague de bots la refermerait aussitôt.
func (r *Room) closeIfEveryoneVoted() {
	if r.game == nil || r.game.Over() || r.game.Phase() == game.PhaseRoles {
		return
	}

	if len(r.game.Pending()) > 0 {
		return
	}

	_ = r.play(game.Command{Kind: game.CommandClose}, time.Now())
}

// narrate publie les événements du narrateur : une ligne dans le fil, et une
// trame « game.event » pour que l'interface réagisse sans attendre.
func (r *Room) narrate(events ...game.Event) {
	if len(events) == 0 {
		return
	}

	now := time.Now().UTC()
	for _, event := range events {
		message := protocol.ChatMessage{
			ID:     newID("S"),
			Pseudo: "Narrateur",
			Text:   event.Text,
			At:     now,
			System: true,
		}
		r.appendMessage(message)
		r.broadcastChat(message)

		r.broadcast(protocol.MustEncode(protocol.TypeGameEvent, protocol.GameEventPayload{
			Kind:    string(event.Kind),
			Text:    event.Text,
			Phase:   protocol.Phase(event.Phase),
			Day:     event.Day,
			Round:   event.Round,
			Target:  event.Target,
			Cause:   string(event.Cause),
			Role:    string(event.Role),
			Winners: string(event.Winners),
		}))
	}
}

// armTimer programme l'avance automatique de la phase en cours. Une échéance
// nulle (partie finie ou pas de partie) ne programme rien.
func (r *Room) armTimer(deadline time.Time) {
	r.stopTimer()

	if deadline.IsZero() {
		return
	}

	delay := time.Until(deadline)
	if delay < 0 {
		delay = 0
	}

	r.timer = time.AfterFunc(delay, func() {
		if err := r.expire(time.Now()); err != nil && !errors.Is(err, ErrClosed) {
			// La salle est arrêtée : il n'y a plus rien à faire avancer.
			return
		}
	})
}

// stopTimer désarme la minuterie en cours, s'il y en a une.
func (r *Room) stopTimer() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

// armBotWave programme la prochaine vague d'actions des bots.
//
// La vague est reprogrammée à chaque changement d'état de la partie : elle
// laisse aux bots un temps de réflexion (BotDelay) pour que les humains voient
// la phase. Quand il n'y a rien à faire, la vague suivante est simplement
// inutile — un bot n'agit qu'une fois par phase.
func (r *Room) armBotWave() {
	r.stopBotTimer()

	if r.opts.BotDelay <= 0 || !r.hasBot() || r.game == nil || r.game.Over() {
		return
	}

	r.botTimer = time.AfterFunc(r.opts.BotDelay, func() {
		// Une vague périmée (salle fermée) échoue en silence.
		_ = r.botTurn(time.Now())
	})
}

// stopBotTimer désarme la vague de bots en cours, s'il y en a une.
func (r *Room) stopBotTimer() {
	if r.botTimer != nil {
		r.botTimer.Stop()
		r.botTimer = nil
	}
}

// hasBot dit si la table compte au moins un joueur automatique.
func (r *Room) hasBot() bool {
	for _, id := range r.order {
		if player := r.players[id]; player != nil && player.IsBot {
			return true
		}
	}

	return false
}

// canChat dit si un joueur a le droit de parler : la nuit, le village dort ; et
// un mort, une fois pour toutes, ne parle plus.
func (r *Room) canChat(player *Player) error {
	if r.game == nil || r.game.Over() {
		return nil
	}

	if r.game.Dead(string(player.ID)) {
		return protocol.NewError(protocol.CodePlayerDead,
			"Les morts ne parlent plus : ils regardent.")
	}

	if r.game.Phase() == game.PhaseNight {
		return protocol.NewError(protocol.CodeWrongPhase,
			"Le village dort : personne ne parle pendant la nuit.")
	}

	if r.game.Phase() == game.PhaseRoles {
		return protocol.NewError(protocol.CodeWrongPhase,
			"La partie commence : chacun découvre son rôle.")
	}

	// Le débat est un tour de parole : seul celui qui l'a peut écrire, les
	// autres écoutent. Le serveur décide, pas l'interface.
	if r.game.Phase() == game.PhaseDebate && r.game.CurrentSpeaker() != string(player.ID) {
		return protocol.NewError(protocol.CodeWrongPhase,
			"Ce n'est pas votre tour : écoutez, puis ce sera à vous.")
	}

	return nil
}

// gameRefusal traduit un refus du moteur en erreur de protocole.
func gameRefusal(err error) error {
	reason := game.ReasonOf(err)

	code, ok := map[game.Reason]protocol.ErrorCode{
		game.ReasonBadConfig:      protocol.CodeInternal,
		game.ReasonTooFewPlayers:  protocol.CodeNotEnoughPlayers,
		game.ReasonTooManyPlayers: protocol.CodeRoomFull,
		game.ReasonGameOver:       protocol.CodeGameOver,
		game.ReasonWrongPhase:     protocol.CodeWrongPhase,
		game.ReasonNotWolf:        protocol.CodeNotWolf,
		game.ReasonNotSeer:        protocol.CodeWrongAction,
		game.ReasonNotWitch:       protocol.CodeWrongAction,
		game.ReasonWrongAction:    protocol.CodeWrongAction,
		game.ReasonNoPotion:       protocol.CodeWrongAction,
		game.ReasonDead:           protocol.CodePlayerDead,
		game.ReasonBadTarget:      protocol.CodeBadTarget,
		game.ReasonUnknownPlayer:  protocol.CodeUnauthorized,
	}[reason]
	if !ok {
		code = protocol.CodeInternal
	}

	return protocol.NewError(code, "%s", game.MessageOf(err))
}

func (r *Room) doSummary(cmd command) {
	cmd.reply <- result{summary: protocol.RoomSummary{
		Code:       r.code,
		Phase:      r.phase(),
		Players:    len(r.players),
		MaxPlayers: r.opts.MaxPlayers,
	}}
}

// phase annonce l'étape publique de la salle : la salle d'attente, ou la phase
// de la partie en cours.
func (r *Room) phase() protocol.Phase {
	if r.game == nil {
		return protocol.PhaseLobby
	}

	return protocol.Phase(r.game.Phase())
}

// sweep entretient la salle : il retire les joueurs partis depuis trop
// longtemps et ferme les salles devenues inutiles.
//
// Pendant une partie, personne n'est retiré : une absence ne doit pas coûter sa
// place à un joueur, ni fausser le compte des vivants. La salle se ferme en
// revanche si plus personne n'est connecté pour la voir.
func (r *Room) sweep(now time.Time) {
	if r.game == nil {
		// copy : removePlayer modifie r.order en cours d'itération.
		for _, id := range append([]PlayerID(nil), r.order...) {
			player := r.players[id]
			if player == nil {
				continue
			}
			if _, online := r.clients[id]; online {
				continue
			}
			if player.offlineSince.IsZero() {
				continue
			}
			if now.Sub(player.offlineSince) >= r.opts.OfflineTimeout {
				r.removePlayer(id, "s'est déconnecté")
			}
		}
	}

	// Une salle que personne n'a jamais rejointe ne doit pas rester ouverte.
	if !r.everConnected && now.Sub(r.createdAt) >= r.opts.EmptyRoomTimeout {
		r.close("personne ne l'a rejointe")
		return
	}

	if r.online() > 0 {
		r.emptySince = time.Time{}
		return
	}

	if r.emptySince.IsZero() {
		r.emptySince = now
		return
	}

	if now.Sub(r.emptySince) >= r.opts.EmptyRoomTimeout {
		r.close("salle vide")
	}
}

// online compte les joueurs connectés.
func (r *Room) online() int {
	connected := 0
	for _, id := range r.order {
		if _, ok := r.clients[id]; ok {
			connected++
		}
	}

	return connected
}

// close prévient les joueurs encore connectés, retire la salle du registre et
// arrête l'acteur.
func (r *Room) close(reason string) {
	for _, client := range r.clients {
		client.Send(protocol.MustEncode(protocol.TypeError, protocol.ErrorPayload{
			Code:    protocol.CodeRoomNotFound,
			Message: "La partie a été fermée (" + reason + ").",
		}))
	}

	if r.onClose != nil {
		r.onClose(r.code)
	}

	r.shutdown()
}

// addPlayer inscrit un joueur et renvoie son identifiant et son jeton.
func (r *Room) addPlayer(account, pseudo string, host bool, now time.Time) (PlayerID, string) {
	player := &Player{
		ID:       PlayerID(newID("P")),
		Pseudo:   pseudo,
		Account:  account,
		Token:    newToken(),
		IsHost:   host,
		joinedAt: now,
	}

	r.players[player.ID] = player
	r.order = append(r.order, player.ID)

	return player.ID, player.Token
}

// playerByAccount retrouve la place d'un compte, s'il en tient une.
//
// Les bots n'ont pas de compte (chaîne vide) : ils ne sont jamais retrouvés
// ici.
func (r *Room) playerByAccount(account string) *Player {
	if account == "" {
		return nil
	}

	for _, id := range r.order {
		if player := r.players[id]; player != nil && player.Account == account {
			return player
		}
	}

	return nil
}

// removePlayer retire définitivement un joueur, transmet l'hôte si nécessaire
// et prévient les joueurs restants.
func (r *Room) removePlayer(id PlayerID, reason string) {
	player, ok := r.players[id]
	if !ok {
		return
	}

	delete(r.clients, id)
	delete(r.players, id)

	for i, candidate := range r.order {
		if candidate == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}

	r.addSystemMessagef("%s %s.", player.Pseudo, reason)

	if player.IsHost && len(r.order) > 0 {
		newHost := r.pickHost()
		r.players[newHost].IsHost = true
		r.hostID = newHost
		r.addSystemMessagef("%s est le nouvel hôte.", r.players[newHost].Pseudo)
	}

	if len(r.players) == 0 {
		r.emptySince = time.Now()
	}

	r.broadcastState()
}

// addBot inscrit un joueur automatique : sans connexion, toujours prêt.
func (r *Room) addBot(pseudo string, now time.Time) PlayerID {
	player := &Player{
		ID:       PlayerID(newID("B")),
		Pseudo:   pseudo,
		IsBot:    true,
		Ready:    true,
		joinedAt: now,
	}

	r.players[player.ID] = player
	r.order = append(r.order, player.ID)

	return player.ID
}

// removeBot retire un bot de la table. Les humains ne passent pas par ici : la
// salle ne supprime une place qu'à la demande de son détenteur.
func (r *Room) removeBot(id PlayerID) {
	player, ok := r.players[id]
	if !ok || !player.IsBot {
		return
	}

	r.removePlayer(id, "a été retiré")
}

// nextBotName propose un pseudo de bot encore libre, puis « Bot N » en secours.
func (r *Room) nextBotName() string {
	taken := make(map[string]bool, len(r.players))
	for _, player := range r.players {
		taken[player.Pseudo] = true
	}

	for _, name := range botNames {
		if !taken[name] {
			return name
		}
	}

	for i := 1; ; i++ {
		name := fmt.Sprintf("Bot %d", i)
		if !taken[name] {
			return name
		}
	}
}

// pickHost choisit le prochain hôte : un humain, en ligne de préférence.
//
// Les bots n'hébergent jamais : personne ne pourrait lancer la partie depuis
// leur place.
func (r *Room) pickHost() PlayerID {
	for _, id := range r.order {
		if player := r.players[id]; player != nil && !player.IsBot {
			if _, online := r.clients[id]; online {
				return id
			}
		}
	}

	for _, id := range r.order {
		if player := r.players[id]; player != nil && !player.IsBot {
			return id
		}
	}

	if len(r.order) > 0 {
		return r.order[0]
	}

	return ""
}

// playerByToken retrouve un joueur par son jeton de session.
func (r *Room) playerByToken(token string) *Player {
	if token == "" {
		return nil
	}

	for _, player := range r.players {
		if player.Token == token {
			return player
		}
	}

	return nil
}

// allowChat applique une limite de débit simple : ChatBurst messages au plus
// par fenêtre de ChatWindow.
func (r *Room) allowChat(player *Player, now time.Time) bool {
	cutoff := now.Add(-protocol.ChatWindow)

	kept := player.chatTimes[:0]
	for _, at := range player.chatTimes {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}

	player.chatTimes = append(kept, now)

	return len(player.chatTimes) <= protocol.ChatBurst
}

// appendMessage ajoute un message à l'historique en bornant sa taille.
func (r *Room) appendMessage(message protocol.ChatMessage) {
	r.messages = append(r.messages, message)

	if overflow := len(r.messages) - protocol.MaxMessages; overflow > 0 {
		r.messages = append(r.messages[:0], r.messages[overflow:]...)
	}
}

// addSystemMessagef ajoute une nouvelle automatique et la diffuse.
func (r *Room) addSystemMessagef(format string, args ...any) {
	message := protocol.ChatMessage{
		ID:     newID("S"),
		Pseudo: "Système",
		Text:   fmt.Sprintf(format, args...),
		At:     time.Now().UTC(),
		System: true,
	}

	r.appendMessage(message)
	r.broadcastChat(message)
}

// broadcastState envoie à chaque joueur l'instantané de la salle tel qu'il le voit.
func (r *Room) broadcastState() {
	now := time.Now()

	for id, client := range r.clients {
		client.Send(r.stateFor(id, now))
	}
}

// broadcast envoie la même trame à tous les joueurs connectés.
func (r *Room) broadcast(env protocol.Envelope) {
	for _, client := range r.clients {
		client.Send(env)
	}
}

// broadcastChat diffuse un nouveau message aux joueurs connectés.
func (r *Room) broadcastChat(message protocol.ChatMessage) {
	r.broadcast(protocol.MustEncode(protocol.TypeChatMessage, message))
}

// newToken tire un secret de session (32 octets aléatoires).
func newToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("génération de jeton impossible : " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// newID tire un identifiant public court et lisible (préfixé pour le débogage).
func newID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		panic("génération d'identifiant impossible : " + err.Error())
	}
	return prefix + hex.EncodeToString(buf)
}
