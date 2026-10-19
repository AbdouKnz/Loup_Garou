// Package game contient le moteur du Loup-Garou : les règles, et rien d'autre.
//
// Le moteur est pur : il ne connaît ni l'horloge, ni le réseau, ni les
// goroutines. On lui présente l'heure et des commandes ; il renvoie les
// événements à raconter. C'est la salle (package room) qui décide quand
// appeler Apply, quand diffuser, et qui garde le secret des rôles : la seule
// vue autorisée sort de Snapshot.
package game

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// Role est le rôle d'un joueur. Le MVP n'en connaît que deux.
type Role string

// Rôles disponibles.
const (
	Villageois Role = "villageois"
	LoupGarou  Role = "loup-garou"
	// Voyante observe un joueur chaque nuit et découvre son rôle.
	Voyante Role = "voyante"
	// Sorciere dispose d'une potion de vie et d'une potion de mort, à usage
	// unique, dont une seule par nuit.
	Sorciere Role = "sorciere"
)

// Phase est l'étape d'une partie.
type Phase string

// Phases d'une partie, dans l'ordre d'un cycle.
//
// La distribution des rôles ouvre la partie : c'est une pause, sans action, où
// chacun découvre ce qu'il est avant que la nuit ne tombe.
const (
	// PhaseRoles est la distribution des rôles, avant la première nuit.
	PhaseRoles   Phase = "roles"
	PhaseNight   Phase = "nuit"
	PhaseDawn    Phase = "aube"
	PhaseDebate  Phase = "debat"
	PhaseVote    Phase = "vote"
	PhaseVerdict Phase = "verdict"
	PhaseOver    Phase = "fin"
)

// stepKind nomme une veillée de la nuit.
//
// La nuit n'est pas un bloc : elle se déroule en veillées successives, une pour
// chaque rôle qui se réveille. Seuls les loups s'y prêtent pour l'instant ; les
// rôles qui viendront s'y ajouteront dans l'ordre du scénario.
type stepKind string

// Veillées d'une nuit.
const (
	// stepSeer est la veillée de la voyante : elle observe un joueur.
	stepSeer stepKind = "voyante"
	// stepWolves est la veillée des loups : ils désignent leur proie.
	stepWolves stepKind = "loups"
	// stepWitch est la veillée de la sorcière : elle décide de ses potions, en
	// connaissant la proie des loups.
	stepWitch stepKind = "sorciere"
)

// Winner désigne le camp vainqueur.
type Winner string

// Camps victorieux.
const (
	WinnerVillage Winner = "village"
	WinnerWolves  Winner = "loups"
)

// Player est un participant, tel que la salle le présente au moteur.
type Player struct {
	ID   string
	Name string
}

// CommandKind distingue les intentions acceptées par le moteur.
type CommandKind int

// Intentions acceptées par le moteur.
const (
	// CommandAdvance clôt la phase en cours, échéance atteinte.
	CommandAdvance CommandKind = iota
	// CommandClose clôt la phase en cours sans attendre l'échéance : plus
	// personne n'a rien à y faire.
	CommandClose
	// CommandWolfVote désigne la victime de la nuit (loups vivants).
	CommandWolfVote
	// CommandVillageVote désigne le joueur éliminé par le village.
	CommandVillageVote
	// CommandNightAction est l'action d'un rôle pendant sa veillée : la voyante
	// observe, la sorcière se sert de ses potions.
	CommandNightAction
	// CommandSpeak clôt la prise de parole du joueur dont c'est le tour : le
	// débat passe au suivant sans attendre l'échéance.
	CommandSpeak
)

// ActionKind est ce qu'un joueur fait pendant une veillée.
//
// Le moteur s'en sert pour dire à chaque joueur ce qu'on attend de lui
// (« devorer », « voir », « potions ») ; le client s'en sert pour répondre
// (« voir », « soigner », « empoisonner », « passer »).
type ActionKind string

// Actions d'une veillée.
const (
	// ActionDevour invite les loups à désigner leur proie.
	ActionDevour ActionKind = "devorer"
	// ActionSee invite la voyante à observer un joueur ; c'est aussi sa réponse.
	ActionSee ActionKind = "voir"
	// ActionPotions invite la sorcière à se servir de ses potions, ou non.
	ActionPotions ActionKind = "potions"
	// ActionHeal est la réponse de la sorcière : elle sauve la proie de la nuit.
	ActionHeal ActionKind = "soigner"
	// ActionPoison est la réponse de la sorcière : elle empoisonne un joueur.
	ActionPoison ActionKind = "empoisonner"
	// ActionPass est la réponse du joueur : il renonce à agir cette nuit.
	ActionPass ActionKind = "passer"
)

// Command est une intention adressée au moteur.
type Command struct {
	Kind CommandKind
	// Actor est le joueur à l'origine de l'action (vide pour CommandAdvance).
	Actor string
	// Action précise l'action d'une veillée (CommandNightAction).
	Action ActionKind
	// Target est la cible choisie : vide signifie « vote blanc », ou « pas de
	// cible » pour une action qui n'en demande pas.
	Target string
}

// EventKind identifie ce qui vient de se produire.
type EventKind string

// Types d'événements.
const (
	EventPhaseStarted EventKind = "phase"
	// EventSpeak annonce une prise de parole pendant le débat : le joueur dont
	// c'est le tour est visé par Target.
	EventSpeak         EventKind = "parole"
	EventTie           EventKind = "egalite"
	EventVictim        EventKind = "victime"
	EventSaved         EventKind = "sauve"
	EventNoVictim      EventKind = "aucune-victime"
	EventEliminated    EventKind = "elimine"
	EventNoElimination EventKind = "aucune-elimination"
	EventGameOver      EventKind = "fin"
)

// Event décrit un fait de jeu.
//
// Text est la phrase du narrateur : la salle la publie telle quelle dans le fil
// de discussion, en message système. Le texte porte donc la voix du jeu, jamais
// celle d'un joueur.
type Event struct {
	Kind    EventKind
	Day     int
	Round   int
	Phase   Phase
	Actor   string
	Target  string
	Cause   DeathCause
	Role    Role
	Winners Winner
	Text    string
}

// DeathCause dit de quoi un joueur est mort pendant la nuit.
type DeathCause string

// Causes de mort nocturne.
const (
	// DeathWolves : la proie des loups.
	DeathWolves DeathCause = "loups"
	// DeathPoison : la potion de mort de la sorcière.
	DeathPoison DeathCause = "poison"
)

// Death est un mort de la nuit ; le rôle n'en sort jamais.
type Death struct {
	Target string
	Cause  DeathCause
}

// Observation est ce que la voyante a découvert d'un joueur, une nuit donnée.
type Observation struct {
	Day    int
	Target string
	Role   Role
}

// Potions dit ce qu'il reste à la sorcière : chaque potion ne sert qu'une fois.
type Potions struct {
	Heal   bool
	Poison bool
}

// Reason explique pourquoi une commande a été refusée.
type Reason string

// Motifs de refus.
const (
	ReasonBadConfig      Reason = "bad_config"
	ReasonTooFewPlayers  Reason = "too_few_players"
	ReasonTooManyPlayers Reason = "too_many_players"
	ReasonGameOver       Reason = "game_over"
	ReasonWrongPhase     Reason = "wrong_phase"
	ReasonNotWolf        Reason = "not_a_wolf"
	ReasonNotSeer        Reason = "not_the_seer"
	ReasonNotWitch       Reason = "not_the_witch"
	ReasonWrongAction    Reason = "wrong_action"
	ReasonNoPotion       Reason = "no_potion"
	ReasonDead           Reason = "player_dead"
	ReasonBadTarget      Reason = "bad_target"
	ReasonUnknownPlayer  Reason = "unknown_player"
)

// Error est un refus de règle, porteur d'un message destiné au joueur.
type Error struct {
	Reason  Reason
	Message string
}

// Error implémente error.
func (e *Error) Error() string { return string(e.Reason) + " : " + e.Message }

// ReasonOf extrait le motif d'une erreur du moteur ; vide si l'erreur vient
// d'ailleurs.
func ReasonOf(err error) Reason {
	var ruleErr *Error
	if errors.As(err, &ruleErr) {
		return ruleErr.Reason
	}
	return ""
}

// MessageOf extrait le message destiné au joueur ; les erreurs techniques sont
// masquées pour ne rien divulguer de l'état du moteur.
func MessageOf(err error) string {
	var ruleErr *Error
	if errors.As(err, &ruleErr) {
		return ruleErr.Message
	}
	return "Règle refusée par le moteur."
}

// refuse construit un refus de règle.
func refuse(reason Reason, format string, args ...any) *Error {
	return &Error{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

// player est un participant vu par le moteur.
type player struct {
	id    string
	name  string
	role  Role
	alive bool
}

// Game est une partie en cours.
//
// Tous les champs sont privés : on interroge la partie par les accesseurs et
// par Snapshot, jamais directement. Les rôles, en particulier, ne peuvent
// sortir que par un filtre.
type Game struct {
	cfg     Config
	rng     *rand.Rand
	players []*player
	index   map[string]*player

	phase      Phase
	day        int
	round      int
	candidates []string
	// votes associe un votant à sa cible ; une cible vide est un vote blanc.
	votes map[string]string
	// acted retient qui s'est déjà prononcé pendant l'étape en cours, quelle
	// que soit la façon de le faire (voix, observation, potion).
	acted    map[string]bool
	deadline time.Time
	winner   Winner

	// speakers est l'ordre de parole du débat en cours — les vivants, dans
	// l'ordre de la table — et speakerIndex la prise de parole en cours. Hors
	// débat, les deux sont sans objet.
	speakers     []string
	speakerIndex int

	// steps est la suite des veillées de la nuit en cours, et step celle qui se
	// déroule. Hors nuit, les deux sont sans objet.
	steps []stepKind
	step  int

	// deaths sont les morts de la dernière nuit, avec leur cause.
	deaths     []Death
	eliminated string
	// wolfTarget est la proie désignée par les loups, retenue jusqu'à la fin de
	// la nuit : les veillées suivantes peuvent encore intervenir (soins,
	// poison). Vide, la nuit n'aura pas de victime.
	wolfTarget string
	// nightNote est la phrase qui annoncera, à l'aube, une nuit sans victime.
	nightNote string
	// healed dit si la sorcière a sauvé la proie de la nuit ; poisoned retient
	// la victime de sa potion de mort. Les deux sont remis à zéro chaque nuit.
	healed   bool
	poisoned string
	// healUsed et poisonUsed disent si les potions ont été bues : elles ne se
	// remplissent pas.
	healUsed   bool
	poisonUsed bool
	// vision retient ce que la voyante a découvert, nuit par nuit.
	vision map[string][]Observation
	// dayBallot retient le scrutin du jour une fois le vote clos, pour que le
	// verdict puisse l'afficher à côté de l'éliminé. La nuit, les loups votent
	// en secret : aucun bulletin n'en sort.
	dayBallot map[string]string
}

// New distribue les rôles et ouvre la distribution.
//
// now sert à dater la première échéance : le moteur ne lit jamais l'horloge.
// Les événements renvoyés sont ceux du narrateur (les rôles sont distribués).
//
// La partie ne commence pas tout de suite : la distribution est une pause, et
// l'échéance de la première nuit n'est armée qu'à sa clôture. Personne ne perd
// donc une seconde de jeu à lire son rôle.
func New(players []Player, cfg Config, rng *rand.Rand, now time.Time) (*Game, []Event, error) {
	cfg = cfg.WithDefaults()

	if rng == nil {
		return nil, nil, refuse(ReasonBadConfig, "Aucune source aléatoire fournie.")
	}
	if len(players) < cfg.MinPlayers {
		return nil, nil, refuse(ReasonTooFewPlayers,
			"Il faut au moins %d joueurs pour lancer une partie.", cfg.MinPlayers)
	}
	if len(players) > cfg.MaxPlayers {
		return nil, nil, refuse(ReasonTooManyPlayers,
			"Une partie ne peut pas dépasser %d joueurs.", cfg.MaxPlayers)
	}

	g := &Game{
		cfg:     cfg,
		rng:     rng,
		index:   make(map[string]*player, len(players)),
		votes:   make(map[string]string, len(players)),
		acted:   make(map[string]bool, len(players)),
		vision:  make(map[string][]Observation),
		players: make([]*player, 0, len(players)),
	}

	for _, incoming := range players {
		if incoming.ID == "" {
			return nil, nil, refuse(ReasonBadConfig, "Un joueur sans identifiant ne peut pas entrer en jeu.")
		}
		if _, exists := g.index[incoming.ID]; exists {
			return nil, nil, refuse(ReasonBadConfig, "Deux joueurs partagent le même identifiant.")
		}

		p := &player{id: incoming.ID, name: incoming.Name, alive: true}
		g.index[incoming.ID] = p
		g.players = append(g.players, p)
	}

	g.dealRoles()
	g.day = 1

	event := g.startRoles(now)

	return g, []Event{event}, nil
}

// startRoles ouvre la distribution des rôles : une pause sans action ni vote,
// où chacun découvre ce qu'il est, avant que la première nuit ne tombe.
func (g *Game) startRoles(now time.Time) Event {
	return g.startPhase(PhaseRoles, now)
}

// dealRoles distribue les rôles tirés au sort.
//
// La composition est celle annoncée à la table : seules les places sont
// mélangées, jamais les rôles, pour que le nombre de loups ne dépende pas du
// hasard.
func (g *Game) dealRoles() {
	roles := g.cfg.CompositionFor(len(g.players))

	for seat, index := range g.rng.Perm(len(g.players)) {
		g.players[index].role = roles[seat]
	}
}

// Phase renvoie la phase en cours.
func (g *Game) Phase() Phase { return g.phase }

// Day renvoie le numéro du jour, à partir de 1.
func (g *Game) Day() int { return g.day }

// Deadline renvoie l'échéance de la phase en cours (zéro si la partie est finie).
func (g *Game) Deadline() time.Time { return g.deadline }

// Over indique si la partie est terminée.
func (g *Game) Over() bool { return g.winner != "" }

// Winner renvoie le camp vainqueur, vide tant que la partie dure.
func (g *Game) Winner() Winner { return g.winner }

// Dead indique si un joueur a quitté le jeu. Un identifiant inconnu est
// simplement considéré comme mort : il n'est pas en jeu.
func (g *Game) Dead(id string) bool {
	player, ok := g.index[id]

	return !ok || !player.alive
}

// Apply applique une commande et renvoie les événements à raconter.
func (g *Game) Apply(cmd Command, now time.Time) ([]Event, error) {
	if g.Over() {
		return nil, refuse(ReasonGameOver, "La partie est terminée.")
	}

	switch cmd.Kind {
	case CommandAdvance:
		return g.advance(now, false)
	case CommandClose:
		return g.advance(now, true)
	case CommandWolfVote:
		return g.recordWolfVote(cmd)
	case CommandVillageVote:
		return g.recordVote(cmd, PhaseVote, "")
	case CommandNightAction:
		return g.recordAction(cmd)
	case CommandSpeak:
		return g.endSpeak(cmd, now)
	default:
		return nil, refuse(ReasonBadConfig, "Commande inconnue.")
	}
}

// Pending renvoie les joueurs qui doivent encore se prononcer pendant la phase
// en cours ; la liste est vide quand la phase n'attend personne — soit que tout
// le monde a voté, soit qu'elle n'appelle pas de vote.
//
// C'est ce qui permet à la salle de clore la phase sans faire patienter la
// table : une fois Pending vide, il n'y a plus rien à attendre.
func (g *Game) Pending() []string {
	var voters []*player

	switch g.phase {
	case PhaseNight:
		voters = g.nightActors()
	case PhaseVote:
		voters = g.alive()
	case PhaseDebate:
		// Le débat attend celui dont c'est le tour, et lui seul : personne ne
		// peut voter avant que le dernier n'ait parlé.
		if speaker := g.currentSpeaker(); speaker != "" {
			return []string{speaker}
		}

		return nil
	default:
		return nil
	}

	out := []string{}
	for _, voter := range voters {
		if !g.acted[voter.id] {
			out = append(out, voter.id)
		}
	}

	return out
}

// recordWolfVote enregistre la voix d'un loup, pendant sa veillée seulement.
func (g *Game) recordWolfVote(cmd Command) ([]Event, error) {
	if g.phase != PhaseNight || g.currentStep() != stepWolves {
		return nil, refuse(ReasonWrongPhase, "Les loups ne se réveillent pas à cet instant.")
	}

	return g.recordVote(cmd, PhaseNight, LoupGarou)
}

// recordVote enregistre la voix d'un joueur.
//
// role attendu vide signifie « n'importe quel joueur vivant » : c'est le cas du
// vote du village, où les loups votent comme tout le monde.
func (g *Game) recordVote(cmd Command, phase Phase, role Role) ([]Event, error) {
	if g.phase != phase {
		return nil, refuse(ReasonWrongPhase, "Ce n'est pas le moment de voter.")
	}

	actor, ok := g.index[cmd.Actor]
	if !ok {
		return nil, refuse(ReasonUnknownPlayer, "Ce joueur ne fait pas partie de la partie.")
	}
	if role != "" && actor.role != role {
		return nil, refuse(ReasonNotWolf, "Seuls les loups dévorent la nuit.")
	}
	if !actor.alive {
		// Un mort regarde : il ne vote plus.
		return nil, refuse(ReasonDead, "Les morts ne votent plus.")
	}

	target, err := g.checkTarget(cmd.Target)
	if err != nil {
		return nil, err
	}

	g.votes[actor.id] = target
	g.acted[actor.id] = true

	return nil, nil
}

// recordAction enregistre l'action d'un rôle pendant sa veillée.
func (g *Game) recordAction(cmd Command) ([]Event, error) {
	if g.phase != PhaseNight {
		return nil, refuse(ReasonWrongPhase, "Ce n'est pas le moment d'agir.")
	}

	actor, ok := g.index[cmd.Actor]
	if !ok {
		return nil, refuse(ReasonUnknownPlayer, "Ce joueur ne fait pas partie de la partie.")
	}
	if !actor.alive {
		return nil, refuse(ReasonDead, "Les morts ne jouent plus.")
	}
	if g.acted[actor.id] {
		// Une veillée ne vaut qu'une décision : la sorcière ne se sert pas de
		// ses deux potions la même nuit.
		return nil, refuse(ReasonWrongPhase, "Vous avez déjà agi pendant cette veillée.")
	}

	switch g.currentStep() {
	case stepSeer:
		return g.seerAction(actor, cmd)
	case stepWitch:
		return g.witchAction(actor, cmd)
	default:
		return nil, refuse(ReasonWrongPhase, "Personne n'est réveillé à cet instant.")
	}
}

// seerAction fait observer un joueur à la voyante : elle en découvre le rôle,
// qu'elle retient pour elle seule.
func (g *Game) seerAction(actor *player, cmd Command) ([]Event, error) {
	if actor.role != Voyante {
		return nil, refuse(ReasonNotSeer, "Seule la voyante observe pendant cette veillée.")
	}
	if cmd.Action != ActionSee {
		return nil, refuse(ReasonWrongAction, "La voyante doit désigner qui elle observe.")
	}

	target, err := g.checkTarget(cmd.Target)
	if err != nil {
		return nil, err
	}
	if target == "" || target == actor.id {
		return nil, refuse(ReasonBadTarget, "La voyante doit observer un autre joueur.")
	}

	g.vision[actor.id] = append(g.vision[actor.id], Observation{
		Day:    g.day,
		Target: target,
		Role:   g.index[target].role,
	})
	g.acted[actor.id] = true

	return nil, nil
}

// witchAction enregistre le choix de la sorcière : sauver la proie de la nuit,
// empoisonner un joueur, ou ne rien faire.
//
// Une seule potion par nuit, et chacune ne sert qu'une fois. Seule la potion de
// vie est datée : la potion de mort attend l'aube pour agir.
func (g *Game) witchAction(actor *player, cmd Command) ([]Event, error) {
	if actor.role != Sorciere {
		return nil, refuse(ReasonNotWitch, "Seule la sorcière se lève pendant cette veillée.")
	}

	switch cmd.Action {
	case ActionPass:
		g.acted[actor.id] = true
		return nil, nil

	case ActionHeal:
		if g.healUsed {
			return nil, refuse(ReasonNoPotion, "La potion de vie a déjà été bue.")
		}
		if g.wolfTarget == "" {
			return nil, refuse(ReasonBadTarget, "Les loups n'ont désigné personne à sauver.")
		}

		g.healUsed = true
		g.healed = true
		g.acted[actor.id] = true

		return nil, nil

	case ActionPoison:
		if g.poisonUsed {
			return nil, refuse(ReasonNoPotion, "La potion de mort a déjà été bue.")
		}

		target, err := g.checkTarget(cmd.Target)
		if err != nil {
			return nil, err
		}
		if target == "" || target == actor.id {
			return nil, refuse(ReasonBadTarget, "La sorcière doit empoisonner un autre joueur.")
		}

		g.poisonUsed = true
		g.poisoned = target
		g.acted[actor.id] = true

		return nil, nil

	default:
		return nil, refuse(ReasonWrongAction, "La sorcière doit soigner, empoisonner ou passer.")
	}
}

// checkTarget vérifie qu'une cible est vivante et autorisée au tour courant.
func (g *Game) checkTarget(raw string) (string, error) {
	if raw == "" {
		// Vote blanc : autorisé, et compté comme tel.
		return "", nil
	}

	target, ok := g.index[raw]
	if !ok {
		return "", refuse(ReasonBadTarget, "Ce joueur ne fait pas partie de la partie.")
	}
	if !target.alive {
		return "", refuse(ReasonBadTarget, "Ce joueur n'est plus en jeu.")
	}
	if len(g.candidates) > 0 && !contains(g.candidates, raw) {
		return "", refuse(ReasonBadTarget, "Seuls les joueurs à égalité sont en ballotage.")
	}

	return raw, nil
}

// advance clôt la phase en cours.
//
// L'échéance doit être atteinte — sauf pour CommandClose, que la salle n'émet
// que lorsque plus personne ne doit se prononcer. Une minuterie périmée ne peut
// donc pas faire avancer une phase qui vient de commencer.
func (g *Game) advance(now time.Time, force bool) ([]Event, error) {
	if !force && now.Before(g.deadline) {
		return nil, refuse(ReasonWrongPhase, "La phase en cours n'est pas terminée.")
	}

	switch g.phase {
	case PhaseRoles:
		// La distribution ne se referme qu'à son échéance : le moteur décide
		// seul du temps laissé à la table pour lire son rôle.
		return []Event{g.startNight(now)}, nil
	case PhaseNight:
		return g.closeStep(now)
	case PhaseDawn:
		return g.startDebate(now), nil
	case PhaseDebate:
		return g.nextSpeaker(now)
	case PhaseVote:
		return g.closeVote(now)
	case PhaseVerdict:
		return g.nextNight(now)
	default:
		return nil, nil
	}
}

// startDebate ouvre le débat : les vivants prendront la parole l'un après
// l'autre, dans l'ordre de la table.
func (g *Game) startDebate(now time.Time) []Event {
	g.speakers = make([]string, 0, len(g.players))
	for _, p := range g.alive() {
		g.speakers = append(g.speakers, p.id)
	}
	g.speakerIndex = 0

	return []Event{g.startPhase(PhaseDebate, now), g.speakEvent()}
}

// nextSpeaker clôt la prise de parole en cours : le débat passe au vivant
// suivant, ou se referme sur le vote quand le dernier a parlé.
func (g *Game) nextSpeaker(now time.Time) ([]Event, error) {
	if g.speakerIndex+1 < len(g.speakers) {
		g.speakerIndex++
		g.deadline = now.Add(g.cfg.Duration(PhaseDebate))

		return []Event{g.speakEvent()}, nil
	}

	return []Event{g.startPhase(PhaseVote, now)}, nil
}

// endSpeak clôt la prise de parole sur la demande de son acteur.
//
// Seul le joueur dont c'est le tour peut le faire : un autre n'abrège pas un
// débat qu'il n'a pas la parole.
func (g *Game) endSpeak(cmd Command, now time.Time) ([]Event, error) {
	if g.phase != PhaseDebate {
		return nil, refuse(ReasonWrongPhase, "Ce n'est pas le moment de prendre la parole.")
	}
	if cmd.Actor != g.currentSpeaker() {
		return nil, refuse(ReasonWrongPhase, "Ce n'est pas votre tour de parole.")
	}

	return g.advance(now, true)
}

// currentSpeaker renvoie le joueur dont c'est le tour de parole, vide sinon.
func (g *Game) currentSpeaker() string {
	if g.phase != PhaseDebate || g.speakerIndex < 0 || g.speakerIndex >= len(g.speakers) {
		return ""
	}

	return g.speakers[g.speakerIndex]
}

// CurrentSpeaker renvoie le joueur dont c'est le tour de parole, vide sinon.
func (g *Game) CurrentSpeaker() string { return g.currentSpeaker() }

// speakEvent annonce la prise de parole du joueur dont c'est le tour.
func (g *Game) speakEvent() Event {
	speaker := g.currentSpeaker()

	return Event{
		Kind:   EventSpeak,
		Phase:  PhaseDebate,
		Day:    g.day,
		Round:  g.round,
		Target: speaker,
		Text: fmt.Sprintf("🎤 %s prend la parole (%d s).",
			g.index[speaker].name, int(g.cfg.Duration(PhaseDebate).Seconds())),
	}
}

// startNight ouvre la nuit : ses veillées se succéderont jusqu'à l'aube.
func (g *Game) startNight(now time.Time) Event {
	g.steps = g.nightPlan()
	g.step = 0
	g.spare("")
	g.deaths = nil
	g.healed = false
	g.poisoned = ""

	return g.startPhase(PhaseNight, now)
}

// nightPlan liste les veillées d'une nuit, dans l'ordre du scénario : chaque
// rôle encore en vie se lève à son tour, et les loups toujours.
//
// La sorcière ne se lève que s'il lui reste une potion : sans rien à décider,
// sa veillée ne serait qu'une attente de plus.
func (g *Game) nightPlan() []stepKind {
	plan := make([]stepKind, 0, 3)

	if g.awake(Voyante) {
		plan = append(plan, stepSeer)
	}

	plan = append(plan, stepWolves)

	if g.awake(Sorciere) && (!g.healUsed || !g.poisonUsed) {
		plan = append(plan, stepWitch)
	}

	return plan
}

// awake dit si un rôle compte encore un joueur vivant pour se lever.
func (g *Game) awake(role Role) bool {
	return len(g.livingWithRole(role)) > 0
}

// livingWithRole renvoie les joueurs vivants d'un rôle, dans l'ordre de la table.
func (g *Game) livingWithRole(role Role) []*player {
	out := make([]*player, 0, 1)

	for _, p := range g.players {
		if p.alive && p.role == role {
			out = append(out, p)
		}
	}

	return out
}

// currentStep renvoie la veillée en cours, vide s'il n'y en a pas.
func (g *Game) currentStep() stepKind {
	if g.step < 0 || g.step >= len(g.steps) {
		return ""
	}

	return g.steps[g.step]
}

// nightActors renvoie les joueurs que la veillée en cours attend.
func (g *Game) nightActors() []*player {
	switch g.currentStep() {
	case stepSeer:
		return g.livingWithRole(Voyante)
	case stepWolves:
		return g.wolves()
	case stepWitch:
		return g.livingWithRole(Sorciere)
	default:
		return nil
	}
}

// yourAction dit ce que le moteur attend d'un joueur pendant la veillée en
// cours : vide, il n'a rien à y faire.
func (g *Game) yourAction(me *player) ActionKind {
	switch g.currentStep() {
	case stepSeer:
		if me.role == Voyante {
			return ActionSee
		}
	case stepWolves:
		if me.role == LoupGarou {
			return ActionDevour
		}
	case stepWitch:
		if me.role == Sorciere {
			return ActionPotions
		}
	}

	return ""
}

// closeStep clôt la veillée en cours : ses acteurs ont parlé, la nuit passe à
// la veillée suivante — ou se referme si c'était la dernière.
func (g *Game) closeStep(now time.Time) ([]Event, error) {
	if g.currentStep() == stepWolves {
		return g.closeWolves(now)
	}

	// Les veillées de la voyante et de la sorcière n'ont rien à dépouiller :
	// leur effet a été retenu à l'instant de l'action.
	return g.nextStep(now)
}

// closeWolves dépouille le vote des loups et retient la proie de la nuit.
//
// La mort n'est appliquée qu'à la fin de la nuit : les veillées suivantes
// peuvent encore intervenir (soins, poison). Si les loups se partagent, un
// second tour s'ouvre ; s'ils se partagent encore, la nuit n'aura pas de proie.
func (g *Game) closeWolves(now time.Time) ([]Event, error) {
	outcome := g.tally(g.wolves())

	if outcome.tie {
		return g.runoff(PhaseNight, now, outcome, "Les loups ne sont pas d'accord")
	}

	if outcome.top == "" {
		// Aucun vote utile : la nuit passera sans victime.
		g.spare("🌙 Les loups n'ont désigné personne : la nuit fut tranquille.")
	} else {
		g.wolfTarget = outcome.top
	}

	return g.nextStep(now)
}

// nextStep ouvre la veillée suivante ; la dernière terminée, la nuit se referme.
func (g *Game) nextStep(now time.Time) ([]Event, error) {
	g.step++
	if g.step >= len(g.steps) {
		return g.finishNight(now)
	}

	// Nouvelle veillée : ses acteurs repartent d'un tour vierge.
	g.resetRound(now)

	return nil, nil
}

// finishNight applique les morts de la nuit et lève l'aube.
//
// Les causes se cumulent : la proie des loups — sauf si la sorcière l'a sauvée
// — puis la potion de mort.
func (g *Game) finishNight(now time.Time) ([]Event, error) {
	var events []Event

	saved := g.wolfTarget != "" && g.healed

	switch {
	case saved:
		// La proie est épargnée : le village apprend qu'une attaque a été
		// déjouée, jamais contre qui.
		events = append(events, Event{
			Kind: EventSaved, Phase: PhaseNight, Day: g.day,
			Text: "🌿 Les loups ont frappé, mais la victime a été sauvée.",
		})
	case g.wolfTarget != "":
		g.deaths = append(g.deaths, Death{Target: g.wolfTarget, Cause: DeathWolves})
	}

	if g.poisoned != "" && g.poisoned != g.wolfTarget {
		g.deaths = append(g.deaths, Death{Target: g.poisoned, Cause: DeathPoison})
	}

	// Une nuit sans aucun mort se raconte : le texte retenu en tête de nuit dit
	// pourquoi (personne de désigné, ou second tour sans majorité).
	if len(g.deaths) == 0 && !saved {
		events = append(events, Event{
			Kind: EventNoVictim, Phase: PhaseNight, Day: g.day,
			Text: g.nightNote,
		})
	}

	for _, death := range g.deaths {
		g.kill(death.Target)

		events = append(events, Event{
			Kind: EventVictim, Phase: PhaseNight, Day: g.day,
			Target: death.Target, Cause: death.Cause, Text: g.deathText(death),
		})
	}

	if g.checkVictory() {
		return append(events, g.gameOverEvent()), nil
	}

	return append(events, g.startPhase(PhaseDawn, now)), nil
}

// spare retient qu'aucune proie ne sera désignée, et la phrase qui l'annoncera
// à l'aube. Le texte vide remet la note à zéro, en tête de nuit.
func (g *Game) spare(text string) {
	g.wolfTarget = ""
	g.nightNote = text
}

// closeVote dépouille le vote du village.
func (g *Game) closeVote(now time.Time) ([]Event, error) {
	// Le scrutin est retenu pour que le verdict l'affiche à côté du résultat.
	g.dayBallot = copyBallot(g.votes)

	outcome := g.tally(g.alive())

	if outcome.tie {
		return g.runoff(PhaseVote, now, outcome, "Le village est partagé")
	}

	if outcome.top == "" {
		return []Event{
			{
				Kind: EventNoElimination, Phase: PhaseVote, Day: g.day, Round: g.round,
				Text: "⚖️ Le village n'a désigné personne : personne n'est éliminé.",
			},
			g.startPhase(PhaseVerdict, now),
		}, nil
	}

	g.kill(outcome.top)
	g.eliminated = outcome.top
	role := g.index[outcome.top].role

	events := []Event{{
		Kind: EventEliminated, Phase: PhaseVote, Day: g.day, Round: g.round,
		Target: outcome.top, Role: role, Text: g.eliminatedText(outcome.top, role),
	}}

	if g.checkVictory() {
		return append(events, g.gameOverEvent()), nil
	}

	return append(events, g.startPhase(PhaseVerdict, now)), nil
}

// runoff ouvre un second tour : seuls les joueurs à égalité restent en
// ballotage. Si le second tour se termine encore à égalité, personne n'est
// touché — la partie ne peut donc pas s'enliser.
func (g *Game) runoff(phase Phase, now time.Time, outcome outcome, reason string) ([]Event, error) {
	if g.round >= 2 {
		return g.giveUp(phase, now)
	}

	// Second tour : on ne garde que les joueurs à égalité.
	g.round = 2
	g.candidates = outcome.candidates
	g.resetBallot(now)

	return []Event{{
		Kind: EventTie, Phase: phase, Day: g.day, Round: g.round,
		Text: fmt.Sprintf("%s : second tour entre %s.", reason, g.listNames(outcome.candidates)),
	}}, nil
}

// giveUp clôt un second tour resté sans majorité : la nuit n'aura pas de proie,
// le village n'élimine personne.
//
// L'épargne de la nuit n'est annoncée qu'à la fin de la nuit, avec les autres
// nouvelles de l'aube : tant qu'elle dure, le village n'apprend rien.
func (g *Game) giveUp(phase Phase, now time.Time) ([]Event, error) {
	if phase == PhaseVote {
		return []Event{
			{
				Kind: EventNoElimination, Phase: phase, Day: g.day, Round: g.round,
				Text: "⚖️ Second tour sans majorité : personne n'est éliminé.",
			},
			g.startPhase(PhaseVerdict, now),
		}, nil
	}

	g.spare("🌙 Second tour sans majorité : personne n'est mort cette nuit.")

	return g.nextStep(now)
}

// nextNight enchaîne sur la nuit suivante, ou termine la partie.
func (g *Game) nextNight(now time.Time) ([]Event, error) {
	if g.checkVictory() {
		return []Event{g.gameOverEvent()}, nil
	}

	g.day++

	return []Event{g.startNight(now)}, nil
}

// startPhase ouvre une phase : nouvelle échéance, tour et voix remis à zéro.
func (g *Game) startPhase(phase Phase, now time.Time) Event {
	g.phase = phase
	g.resetRound(now)

	return Event{
		Kind:  EventPhaseStarted,
		Phase: phase,
		Day:   g.day,
		Round: g.round,
		Text:  phaseText(phase, g.day, g.cfg),
	}
}

// resetRound remet le tour en cours à zéro et arme l'échéance de l'étape qui
// s'ouvre.
func (g *Game) resetRound(now time.Time) {
	g.round = 1
	g.candidates = nil
	g.resetBallot(now)
}

// resetBallot efface les voix et les actions de l'étape en cours et repousse
// l'échéance.
func (g *Game) resetBallot(now time.Time) {
	g.votes = make(map[string]string, len(g.players))
	g.acted = make(map[string]bool, len(g.players))
	g.deadline = now.Add(g.stepDuration())
}

// stepDuration est la durée de l'étape en cours : celle de la veillée pendant
// la nuit, celle de la phase sinon.
func (g *Game) stepDuration() time.Duration {
	if g.phase != PhaseNight {
		return g.cfg.Duration(g.phase)
	}

	if g.currentStep() == stepWolves {
		return g.cfg.Duration(PhaseNight)
	}

	return g.cfg.NightStepDuration()
}

// checkVictory met à jour le vainqueur si la partie est terminée.
//
// Le village gagne quand plus aucun loup n'est vivant ; les loups gagnent dès
// qu'ils sont aussi nombreux que les villageois : ils ne peuvent plus être
// mis en minorité.
func (g *Game) checkVictory() bool {
	wolves, villagers := 0, 0

	for _, p := range g.players {
		if !p.alive {
			continue
		}
		if p.role == LoupGarou {
			wolves++
		} else {
			villagers++
		}
	}

	switch {
	case wolves == 0:
		g.winner = WinnerVillage
	case wolves >= villagers:
		g.winner = WinnerWolves
	default:
		return false
	}

	g.phase = PhaseOver
	g.deadline = time.Time{}

	return true
}

// kill retire un joueur du jeu.
func (g *Game) kill(id string) {
	if p, ok := g.index[id]; ok {
		p.alive = false
	}
}

// wolves renvoie les loups vivants, dans l'ordre de la table.
func (g *Game) wolves() []*player {
	out := make([]*player, 0, 3)
	for _, p := range g.players {
		if p.alive && p.role == LoupGarou {
			out = append(out, p)
		}
	}

	return out
}

// alive renvoie les joueurs vivants, dans l'ordre de la table.
func (g *Game) alive() []*player {
	out := make([]*player, 0, len(g.players))
	for _, p := range g.players {
		if p.alive {
			out = append(out, p)
		}
	}

	return out
}

// outcome est l'issue d'un tour de vote.
type outcome struct {
	// top est la cible la plus désignée, vide si aucun vote utile.
	top string
	// candidates liste les cibles à égalité en tête, dans l'ordre de la table.
	candidates []string
	// tie est vrai quand plusieurs cibles se partagent la première place.
	tie bool
	// count est le nombre de voix exprimées sur une personne.
	count int
	// blank est le nombre de votes blancs.
	blank int
}

// tally compte les voix du tour courant.
//
// Seuls les votes déjà enregistrés comptent : un joueur qui n'a pas voté à
// l'échéance est simplement absent du décompte.
func (g *Game) tally(voters []*player) outcome {
	counts := make(map[string]int, len(voters))
	var result outcome

	for _, voter := range voters {
		target, voted := g.votes[voter.id]
		if !voted {
			continue
		}
		if target == "" {
			result.blank++
			continue
		}

		counts[target]++
	}

	best := 0
	for _, p := range g.players {
		count := counts[p.id]
		if count == 0 {
			continue
		}

		result.count += count

		switch {
		case count > best:
			best = count
			result.top = p.id
			result.candidates = []string{p.id}
		case count == best:
			result.candidates = append(result.candidates, p.id)
		}
	}

	result.tie = len(result.candidates) > 1

	return result
}

// listNames met des pseudos en phrase : « Alice et Bob ».
func (g *Game) listNames(ids []string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if p, ok := g.index[id]; ok {
			names = append(names, p.name)
		}
	}

	switch len(names) {
	case 0:
		return "personne"
	case 1:
		return names[0]
	}

	joined := names[0]
	for _, name := range names[1 : len(names)-1] {
		joined += ", " + name
	}

	return joined + " et " + names[len(names)-1]
}

// deathText raconte une mort de la nuit — sans révéler son rôle.
func (g *Game) deathText(death Death) string {
	name := g.index[death.Target].name

	if death.Cause == DeathPoison {
		return fmt.Sprintf("☠️ %s est mort dans d'étranges circonstances.", name)
	}

	return fmt.Sprintf("🐺 %s a été dévoré pendant la nuit.", name)
}

// eliminatedText raconte l'élimination du jour, rôle compris : c'est le verdict
// du village, il est public.
func (g *Game) eliminatedText(id string, role Role) string {
	return fmt.Sprintf("⚖️ Le village a éliminé %s : c'était un %s.", g.index[id].name, role)
}

// Phases liste toutes les étapes d'une partie, dans l'ordre du cycle. C'est la
// référence que la salle et les tests comparent au contrat public.
func Phases() []Phase {
	return []Phase{PhaseRoles, PhaseNight, PhaseDawn, PhaseDebate, PhaseVote, PhaseVerdict, PhaseOver}
}

// gameOverEvent annonce la fin de la partie et révèle tous les rôles.
func (g *Game) gameOverEvent() Event {
	var text string
	if g.winner == WinnerWolves {
		text = "🐺 Les loups ont gagné : ils sont maintenant aussi nombreux que le village."
	} else {
		text = "🏆 Le village est sauvé : plus aucun loup n'est en vie."
	}

	return Event{
		Kind:    EventGameOver,
		Phase:   PhaseOver,
		Day:     g.day,
		Winners: g.winner,
		Text:    text + " " + g.revealText(),
	}
}

// revealText liste les rôles de tous les joueurs, dans l'ordre de la table.
func (g *Game) revealText() string {
	text := "Rôles :"
	for i, p := range g.players {
		if i > 0 {
			text += ","
		}
		text += fmt.Sprintf(" %s (%s)", p.name, p.role)
	}

	return text + "."
}

// phaseText écrit la phrase d'ouverture d'une phase.
func phaseText(phase Phase, day int, cfg Config) string {
	switch phase {
	case PhaseRoles:
		return "🎭 Les rôles sont distribués : chacun découvre le sien."
	case PhaseNight:
		return fmt.Sprintf("🌙 Nuit %d : le village s'endort, les loups se réveillent.", day)
	case PhaseDawn:
		return "🌅 Le jour se lève sur le village."
	case PhaseDebate:
		return fmt.Sprintf("☀️ Le débat s'ouvre : chacun prend la parole à son tour (%d s).", int(cfg.Speak.Seconds()))
	case PhaseVote:
		return "🗳️ Aux urnes : chacun désigne un suspect."
	case PhaseVerdict:
		return "⚖️ Le village rend son verdict."
	default:
		return ""
	}
}

// contains indique si liste contient value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}

	return false
}
