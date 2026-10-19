package room

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/protocol"
)

// gameOptions règle les durées assez longues pour que seuls les appels
// explicites à expire fassent avancer la partie : les tests restent
// déterministes. La composition reste celle du jeu : les veillées des rôles se
// déroulent donc pour de vrai, ce que la salle doit relayer.
func gameOptions() Options {
	opts := DefaultOptions()
	opts.Game = game.Config{
		MinPlayers: 6,
		MaxPlayers: 12,
		Night:      30 * time.Second,
		NightStep:  5 * time.Second,
		Reveal:     10 * time.Second,
		Speak:      30 * time.Second,
		Vote:       30 * time.Second,
	}
	opts.GameSeed = 424242

	return opts
}

// seat est un joueur attablé : son identité et ce que sa connexion a reçu.
type seat struct {
	pseudo string
	id     PlayerID
	client *fakeClient
}

// table est une salle peuplée de joueurs connectés, prête à jouer.
type table struct {
	salle *Room
	host  PlayerID
	seats []*seat
}

// newTable monte une salle de n joueurs, tous connectés.
func newTable(t *testing.T, n int, opts Options) *table {
	t.Helper()

	salle, hostID, token := newRunningRoom(t, "ABC234", "Joueur1", opts)

	tbl := &table{salle: salle, host: hostID}
	tbl.seats = append(tbl.seats, connect(t, salle, "Joueur1", hostID, token))

	for i := 2; i <= n; i++ {
		pseudo := fmt.Sprintf("Joueur%d", i)

		id, playerToken, err := salle.Join(accountOf(pseudo), pseudo)
		if err != nil {
			t.Fatalf("entrée de %s impossible : %v", pseudo, err)
		}

		tbl.seats = append(tbl.seats, connect(t, salle, pseudo, id, playerToken))
	}

	return tbl
}

// connect attache une fausse connexion au joueur et vérifie son identité.
func connect(t *testing.T, salle *Room, pseudo string, id PlayerID, token string) *seat {
	t.Helper()

	client := newFakeClient()
	authenticated, err := salle.Authenticate(token, client)
	if err != nil {
		t.Fatalf("authentification de %s impossible : %v", pseudo, err)
	}
	if authenticated != id {
		t.Fatalf("%s : identifiant %q, attendu %q", pseudo, authenticated, id)
	}

	return &seat{pseudo: pseudo, id: id, client: client}
}

// start lance la partie depuis l'hôte, puis traverse la distribution des rôles :
// la salle attend alors sa première nuit.
func (tbl *table) start(t *testing.T) {
	t.Helper()

	if err := tbl.salle.StartGame(tbl.host); err != nil {
		t.Fatalf("lancement impossible : %v", err)
	}

	tbl.crossDistribution(t)
}

// crossDistribution fait expirer la distribution des rôles. C'est un temps
// d'arrêt voulu par le moteur, que la salle ne peut pas écourter.
func (tbl *table) crossDistribution(t *testing.T) {
	t.Helper()

	state := tbl.seats[0].client.lastState(t)
	if state.Game == nil || state.Phase != protocol.PhaseRoles {
		t.Fatalf("la partie ne s'ouvre pas sur la distribution : phase %q", state.Phase)
	}

	if err := tbl.salle.expire(state.Game.Deadline.Add(time.Millisecond)); err != nil {
		t.Fatalf("clôture de la distribution impossible : %v", err)
	}
}

// wolves liste les joueurs dont la vue dit qu'ils sont loups.
func (tbl *table) wolves(t *testing.T) []*seat {
	t.Helper()

	out := []*seat{}
	for _, s := range tbl.seats {
		if s.role(t) == protocol.RoleLoupGarou {
			out = append(out, s)
		}
	}

	return out
}

// role renvoie le rôle que ce joueur se voit attribuer.
func (s *seat) role(t *testing.T) string {
	t.Helper()

	state := s.client.lastState(t)
	if state.Game == nil {
		t.Fatalf("%s : aucune partie dans l'instantané", s.pseudo)
	}

	return state.Game.YourRole
}

// seatByPseudo retrouve un joueur par son pseudo.
func (tbl *table) seatByPseudo(t *testing.T, pseudo string) *seat {
	t.Helper()

	for _, s := range tbl.seats {
		if s.pseudo == pseudo {
			return s
		}
	}

	t.Fatalf("joueur %q absent de la table", pseudo)

	return nil
}

// alive liste les joueurs vivants selon le premier siège.
func (tbl *table) alive(t *testing.T) []string {
	t.Helper()

	state := tbl.seats[0].client.lastState(t)

	return state.Game.Alive
}

// lastMatching renvoie le dernier instantané reçu qui satisfasse match.
func lastMatching(t *testing.T, client *fakeClient, match func(protocol.RoomState) bool) (protocol.RoomState, bool) {
	t.Helper()

	var found protocol.RoomState
	ok := false

	for _, env := range client.all() {
		if env.Type != protocol.TypeRoomState {
			continue
		}

		var state protocol.RoomState
		if err := env.DecodeData(&state); err != nil {
			t.Fatalf("instantané illisible : %v", err)
		}
		if match(state) {
			found, ok = state, true
		}
	}

	return found, ok
}

// waitState attend qu'un joueur reçoive un instantané correspondant à match.
func waitState(t *testing.T, client *fakeClient, what string, match func(protocol.RoomState) bool) protocol.RoomState {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state, ok := lastMatching(t, client, match); ok {
			return state
		}
		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("aucun instantané ne correspond à %q", what)

	return protocol.RoomState{}
}

// expireNow clôt la phase en cours comme le ferait la minuterie : à un
// millième de seconde près après l'échéance, pour que les phases s'enchaînent
// sans dériver.
func expireNow(t *testing.T, tbl *table) {
	t.Helper()

	state := tbl.seats[0].client.lastState(t)
	if state.Game == nil {
		t.Fatal("aucune partie en cours")
	}

	if err := tbl.salle.expire(state.Game.Deadline.Add(time.Millisecond)); err != nil {
		t.Fatalf("expiration impossible : %v", err)
	}
}

// advanceTo fait avancer la partie jusqu'à la phase voulue.
func advanceTo(t *testing.T, tbl *table, want protocol.Phase) {
	t.Helper()

	for i := 0; i < 12; i++ {
		state := tbl.seats[0].client.lastState(t)
		if state.Phase == want {
			return
		}
		if state.Phase == protocol.PhaseOver {
			t.Fatalf("la partie s'est terminée avant d'atteindre %q", want)
		}

		expireNow(t, tbl)
	}

	t.Fatalf("phase %q jamais atteinte", want)
}

// narrations renvoie les phrases du narrateur reçues par un joueur.
func narrations(t *testing.T, client *fakeClient) []string {
	t.Helper()

	out := []string{}
	for _, env := range client.all() {
		if env.Type != protocol.TypeGameEvent {
			continue
		}

		var payload protocol.GameEventPayload
		if err := env.DecodeData(&payload); err != nil {
			t.Fatalf("événement illisible : %v", err)
		}
		out = append(out, payload.Text)
	}

	return out
}

func TestLaPartieDistribueLesRoles(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	state := tbl.seats[0].client.lastState(t)

	if state.Phase != protocol.PhaseNight {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseNight)
	}
	if state.Game == nil {
		t.Fatal("aucune partie dans l'instantané")
	}
	if state.Game.Day != 1 || state.Game.Round != 1 {
		t.Errorf("jour %d tour %d, attendu 1 et 1", state.Game.Day, state.Game.Round)
	}
	if len(state.Game.Alive) != 6 || len(state.Game.Dead) != 0 {
		t.Errorf("vivants %v, morts %v", state.Game.Alive, state.Game.Dead)
	}
	if state.CanStart {
		t.Error("on ne relance pas une partie en cours")
	}
	if len(tbl.wolves(t)) != 1 {
		t.Errorf("%d loups, attendu 1 pour six joueurs", len(tbl.wolves(t)))
	}

	// Le narrateur a annoncé la nuit, dans le fil et par un événement de jeu.
	if phrases := narrations(t, tbl.seats[0].client); len(phrases) == 0 {
		t.Error("le narrateur n'a rien dit au lancement")
	}
	if count := tbl.seats[0].client.count(protocol.TypeGameEvent); count == 0 {
		t.Error("aucun événement de jeu reçu")
	}
}

func TestAucunRoleNeFuitVersLesVillageois(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	wolf := tbl.wolves(t)[0]

	for _, s := range tbl.seats {
		state := s.client.lastState(t)

		// La composition est publique, le jeu ne l'est pas : c'est donc la vue
		// de la partie qu'on fouille, pas l'instantané entier.
		raw, err := json.Marshal(state.Game)
		if err != nil {
			t.Fatalf("encodage de l'instantané impossible : %v", err)
		}

		if s != wolf && strings.Contains(string(raw), protocol.RoleLoupGarou) {
			t.Errorf("la vue de %s contient le rôle des loups : %s", s.pseudo, raw)
		}
		if state.Game.Roles != nil {
			t.Errorf("%s voit tous les rôles en pleine partie", s.pseudo)
		}

		if s == wolf {
			if len(state.Game.Wolves) != 1 || state.Game.Wolves[0] != string(wolf.id) {
				t.Errorf("le loup ne voit pas l'antre : %v", state.Game.Wolves)
			}
			continue
		}

		if state.Game.Wolves != nil || state.Game.WolfVotes != nil {
			t.Errorf("%s voit l'antre des loups : %v", s.pseudo, state.Game.Wolves)
		}
		// Le village a ses rôles : chacun ne voit que le sien.
		if state.Game.YourRole == protocol.RoleLoupGarou || state.Game.YourRole == "" {
			t.Errorf("%s : rôle %q", s.pseudo, state.Game.YourRole)
		}
		// Et les carnets privés restent chez leurs porteurs.
		if state.Game.YourRole != protocol.RoleVoyante && state.Game.YourVision != nil {
			t.Errorf("%s lit le carnet de la voyante : %+v", s.pseudo, state.Game.YourVision)
		}
		if state.Game.YourRole != protocol.RoleSorciere && (state.Game.YourPotions != nil || state.Game.WolfTarget != "") {
			t.Errorf("%s voit les potions de la sorcière : %+v", s.pseudo, state.Game.YourPotions)
		}
	}
}

func TestLancerLaPartieRefuseUneTableIncomplete(t *testing.T) {
	tbl := newTable(t, 3, gameOptions())

	if err := tbl.salle.StartGame(tbl.host); protocol.CodeOf(err) != protocol.CodeNotEnoughPlayers {
		t.Errorf("trois joueurs : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeNotEnoughPlayers)
	}
	if err := tbl.salle.StartGame(tbl.seats[1].id); protocol.CodeOf(err) != protocol.CodeUnauthorized {
		t.Errorf("joueur non hôte : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeUnauthorized)
	}
	if state := tbl.seats[0].client.lastState(t); state.CanStart || state.Game != nil {
		t.Error("rien ne devrait pouvoir être lancé à trois joueurs")
	}
}

func TestLaMinuterieFaitAvancerLaPartie(t *testing.T) {
	opts := gameOptions()
	opts.Game.Night = 40 * time.Millisecond
	// Les veillées des rôles se succèdent : elles doivent être aussi brèves que
	// la nuit pour que l'aube arrive dans les temps.
	opts.Game.NightStep = 40 * time.Millisecond

	tbl := newTable(t, 6, opts)
	tbl.start(t)

	// Personne ne demande rien : c'est la minuterie de la salle qui clôt la nuit.
	state := waitState(t, tbl.seats[0].client, "l'aube", func(state protocol.RoomState) bool {
		return state.Phase == protocol.PhaseDawn
	})

	if state.Game == nil || state.Game.Day != 1 {
		t.Errorf("aube inattendue : %+v", state.Game)
	}
}

func TestLaNuitEstMuetteEtLesLoupsSeVoient(t *testing.T) {
	tbl := newTable(t, 7, gameOptions())
	tbl.start(t)

	wolves := tbl.wolves(t)
	if len(wolves) != 2 {
		t.Fatalf("%d loups, attendu 2 pour sept joueurs", len(wolves))
	}

	villagers := tbl.livingVillagers(t)
	victim, liveVillager := villagers[0], villagers[1]

	// La nuit s'ouvre sur la veillée de la voyante : on la laisse expirer, sans
	// que son acteur se prononce.
	expireNow(t, tbl)

	// Pendant la nuit, un villageois ne vote pas et personne ne parle.
	if err := tbl.salle.NightVote(liveVillager.id, string(wolves[0].id)); protocol.CodeOf(err) != protocol.CodeNotWolf {
		t.Errorf("vote de nuit d'un villageois : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeNotWolf)
	}
	if err := tbl.salle.Chat(liveVillager.id, "coucou"); protocol.CodeOf(err) != protocol.CodeWrongPhase {
		t.Errorf("chat de nuit d'un villageois : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongPhase)
	}
	if err := tbl.salle.Chat(wolves[0].id, "on mange qui ?"); protocol.CodeOf(err) != protocol.CodeWrongPhase {
		t.Errorf("chat de nuit d'un loup : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongPhase)
	}

	// Un premier loup se prononce : la nuit reste ouverte, et les deux loups
	// voient la voix de l'autre.
	if err := tbl.salle.NightVote(wolves[0].id, string(victim.id)); err != nil {
		t.Fatalf("vote du premier loup impossible : %v", err)
	}

	state := wolves[0].client.lastState(t)
	if state.Phase != protocol.PhaseNight {
		t.Fatalf("la nuit s'est refermée trop tôt : %q", state.Phase)
	}
	if len(state.Game.Wolves) != 2 {
		t.Errorf("le loup ne voit pas ses semblables : %v", state.Game.Wolves)
	}
	if state.Game.WolfVotes[string(wolves[0].id)] != string(victim.id) {
		t.Errorf("le loup ne voit pas sa propre voix : %v", state.Game.WolfVotes)
	}
	if state := wolves[1].client.lastState(t); state.Game.WolfVotes[string(wolves[0].id)] != string(victim.id) {
		t.Errorf("l'autre loup ne voit pas la voix de son compère : %v", state.Game.WolfVotes)
	}

	if state := liveVillager.client.lastState(t); len(state.Game.Voted) != 0 {
		t.Errorf("un villageois voit les votes de la nuit : %v", state.Game.Voted)
	}

	// Le second loup confirme : la veillée des loups se referme sans attendre
	// son échéance, mais la nuit continue — la sorcière doit encore décider.
	if err := tbl.salle.NightVote(wolves[1].id, string(victim.id)); err != nil {
		t.Fatalf("vote du second loup impossible : %v", err)
	}

	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseNight {
		t.Fatalf("la nuit s'est refermée avant la veillée de la sorcière : %q", state.Phase)
	}

	// La sorcière passe son tour : la nuit se referme sur l'aube.
	expireNow(t, tbl)

	state = tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseDawn {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseDawn)
	}
	if len(state.Game.Deaths) != 1 || state.Game.Deaths[0].Target != string(victim.id) {
		t.Fatalf("morts %+v, attendu la victime %q", state.Game.Deaths, victim.id)
	}
	if state.Game.Deaths[0].Cause != protocol.CauseWolves {
		t.Errorf("cause %q, attendue %q", state.Game.Deaths[0].Cause, protocol.CauseWolves)
	}
	if state.Game.EliminatedRole != "" {
		t.Errorf("le rôle de la victime est révélé : %q", state.Game.EliminatedRole)
	}
	if dead, found := findPlayerDead(t, state, victim.pseudo); !found || !dead {
		t.Errorf("la victime devrait apparaître morte dans la table (%s)", victim.pseudo)
	}

	// Le village débat : c'est un tour de parole. Seul l'orateur peut écrire à
	// cet instant ; la victime, elle, ne parle plus du tout.
	advanceTo(t, tbl, protocol.PhaseDebate)

	debate := tbl.seats[0].client.lastState(t)
	speaker := debate.Game.Speaker
	if speaker == "" {
		t.Fatal("le débat n'a désigné aucun orateur")
	}
	if err := tbl.salle.Chat(PlayerID(speaker), "je soupçonne un loup"); err != nil {
		t.Fatalf("chat de l'orateur refusé : %v", err)
	}
	for _, s := range tbl.seats {
		if string(s.id) == speaker || s.id == victim.id {
			continue
		}
		if err := tbl.salle.Chat(s.id, "moi aussi"); protocol.CodeOf(err) != protocol.CodeWrongPhase {
			t.Errorf("chat hors tour : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongPhase)
		}
		break
	}
	if err := tbl.salle.Chat(victim.id, "bonjour"); protocol.CodeOf(err) != protocol.CodePlayerDead {
		t.Errorf("chat d'un mort : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodePlayerDead)
	}

	advanceTo(t, tbl, protocol.PhaseVote)
	if err := tbl.salle.Vote(victim.id, string(wolves[0].id)); protocol.CodeOf(err) != protocol.CodePlayerDead {
		t.Errorf("vote d'un mort : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodePlayerDead)
	}
}

func TestLaParoleSeRendAuSuivant(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	advanceTo(t, tbl, protocol.PhaseDebate)

	first := tbl.seats[0].client.lastState(t)
	speaker, next := first.Game.Speaker, ""
	if len(first.Game.Speakers) < 2 {
		t.Fatalf("l'ordre de parole est trop court : %v", first.Game.Speakers)
	}
	next = first.Game.Speakers[1]

	// Rendre une parole qu'on n'a pas est refusé : le tour ne se vole pas.
	if err := tbl.salle.SpeakDone(PlayerID(next)); protocol.CodeOf(err) != protocol.CodeWrongPhase {
		t.Errorf("parole rendue hors tour : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongPhase)
	}

	// L'orateur rend la parole : le suivant la prend, avec une échéance neuve.
	if err := tbl.salle.SpeakDone(PlayerID(speaker)); err != nil {
		t.Fatalf("la parole n'a pas pu être rendue : %v", err)
	}

	after := tbl.seats[0].client.lastState(t)
	if after.Game.Speaker != next {
		t.Errorf("orateur = %q, attendu %q", after.Game.Speaker, next)
	}
	if after.Phase != protocol.PhaseDebate {
		t.Errorf("phase = %q, attendu le débat", after.Phase)
	}
}

func TestLeVillageCondamneLeLoupEtGagne(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	wolf := tbl.wolves(t)[0]

	// La nuit passe sans victime : le loup ne se prononce pas.
	advanceTo(t, tbl, protocol.PhaseVote)

	for _, id := range tbl.alive(t) {
		if err := tbl.salle.Vote(PlayerID(id), string(wolf.id)); err != nil {
			t.Fatalf("vote de %s impossible : %v", id, err)
		}
	}

	// La dernière voix referme le vote : le village l'emporte sans attendre.
	state := tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseOver {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseOver)
	}
	if state.Game.Winner != "village" {
		t.Errorf("vainqueur = %q, attendu village", state.Game.Winner)
	}
	if len(state.Game.Roles) != 6 {
		t.Fatalf("rôles révélés à la fin : %v", state.Game.Roles)
	}
	if state.Game.Eliminated != string(wolf.id) {
		t.Errorf("éliminé = %q, attendu %q", state.Game.Eliminated, wolf.id)
	}
	if state.Game.EliminatedRole != protocol.RoleLoupGarou {
		t.Errorf("rôle de l'éliminé = %q, attendu %q", state.Game.EliminatedRole, protocol.RoleLoupGarou)
	}

	// Tous les joueurs reçoivent la fin.
	for _, s := range tbl.seats {
		if got := s.client.lastState(t).Phase; got != protocol.PhaseOver {
			t.Errorf("%s voit %q au lieu de la fin", s.pseudo, got)
		}
	}
}

func TestLeSecondTourEcarteLesAutresSuspects(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	// La nuit s'ouvre sur la veillée de la voyante : on la laisse expirer.
	expireNow(t, tbl)

	// Puis les loups passent leur tour : la nuit se refermera sans victime.
	wolf := tbl.wolves(t)[0]
	if err := tbl.salle.NightVote(wolf.id, ""); err != nil {
		t.Fatalf("vote blanc impossible : %v", err)
	}

	advanceTo(t, tbl, protocol.PhaseVote)

	alive := tbl.alive(t)
	if len(alive) != 6 {
		t.Fatalf("vivants = %d, attendu 6", len(alive))
	}
	first, second := PlayerID(alive[0]), PlayerID(alive[1])

	// Trois voix contre trois : égalité parfaite.
	votes := []PlayerID{first, first, first, second, second, second}
	for i, id := range alive {
		if err := tbl.salle.Vote(PlayerID(id), string(votes[i])); err != nil {
			t.Fatalf("vote de %s impossible : %v", id, err)
		}
	}

	state := tbl.seats[0].client.lastState(t)
	if state.Game.Round != 2 || !state.Game.Tie {
		t.Fatalf("second tour attendu : tour %d, égalité %v", state.Game.Round, state.Game.Tie)
	}
	if len(state.Game.Candidates) != 2 {
		t.Fatalf("candidats = %v, attendu deux", state.Game.Candidates)
	}

	// Un joueur hors ballotage ne peut plus être visé.
	outsider := PlayerID(alive[2])
	if err := tbl.salle.Vote(PlayerID(alive[3]), string(outsider)); protocol.CodeOf(err) != protocol.CodeBadTarget {
		t.Errorf("voix hors ballotage : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeBadTarget)
	}

	// Le second tour, lui, tranche.
	votes = []PlayerID{first, first, first, first, second, second}
	for i, id := range alive {
		if err := tbl.salle.Vote(PlayerID(id), string(votes[i])); err != nil {
			t.Fatalf("second tour impossible : %v", err)
		}
	}

	state = tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseVerdict {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseVerdict)
	}
	if state.Game.Eliminated != string(first) {
		t.Errorf("éliminé = %q, attendu %q", state.Game.Eliminated, first)
	}
}

func TestOnNEntrePasDansUnePartieEnCours(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	if _, _, err := tbl.salle.Join(accountOf("Tardif"), "Tardif"); protocol.CodeOf(err) != protocol.CodeGameInProgress {
		t.Errorf("entrée en cours de partie : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeGameInProgress)
	}

	// Le résumé l'annonce aussi.
	summary, err := tbl.salle.Summary()
	if err != nil {
		t.Fatalf("résumé impossible : %v", err)
	}
	if summary.Phase != protocol.PhaseNight {
		t.Errorf("phase du résumé = %q, attendu %q", summary.Phase, protocol.PhaseNight)
	}
}

func TestUnJoueurHorsLigneGardeSaPlaceEnPartie(t *testing.T) {
	opts := gameOptions()
	opts.EmptyRoomTimeout = time.Minute

	var closed string
	salle, hostID, hostToken := newRunningRoomClosed(t, opts, func(code string) { closed = code })

	host := connect(t, salle, "Joueur1", hostID, hostToken)
	seats := []*seat{host}

	for i := 2; i <= 6; i++ {
		pseudo := fmt.Sprintf("Joueur%d", i)
		id, token, err := salle.Join(accountOf(pseudo), pseudo)
		if err != nil {
			t.Fatalf("entrée de %s impossible : %v", pseudo, err)
		}
		seats = append(seats, connect(t, salle, pseudo, id, token))
	}

	if err := salle.StartGame(hostID); err != nil {
		t.Fatalf("lancement impossible : %v", err)
	}

	// Tout le monde ferme son navigateur. Le balayage ne doit retirer personne.
	for _, s := range seats {
		salle.Disconnect(s.client)
	}

	now := time.Now()
	salle.sweep(now.Add(2 * time.Hour))

	if len(salle.players) != 6 {
		t.Errorf("joueurs = %d, attendu 6 (places conservées)", len(salle.players))
	}
	if closed != "" {
		t.Fatalf("la salle s'est fermée alors que la partie est en cours : %q", closed)
	}

	// Une salle où plus personne ne regarde finit tout de même par se fermer.
	salle.sweep(now.Add(4 * time.Hour))

	if closed != "ABC234" {
		t.Errorf("salle fermée = %q, attendu ABC234", closed)
	}
}

func TestLHostRejoueApresLaFin(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	// La voie la plus courte : le village condamne le seul loup dès le premier jour.
	wolf := tbl.wolves(t)[0]
	advanceTo(t, tbl, protocol.PhaseVote)

	for _, id := range tbl.alive(t) {
		if err := tbl.salle.Vote(PlayerID(id), string(wolf.id)); err != nil {
			t.Fatalf("vote impossible : %v", err)
		}
	}

	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseOver {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseOver)
	}

	// Un joueur qui n'est pas l'hôte n'a pas la main.
	if err := tbl.salle.Again(tbl.seats[1].id); protocol.CodeOf(err) != protocol.CodeUnauthorized {
		t.Errorf("revanche d'un non-hôte : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeUnauthorized)
	}

	if err := tbl.salle.Again(tbl.host); err != nil {
		t.Fatalf("revanche impossible : %v", err)
	}

	state := tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseLobby || state.Game != nil {
		t.Fatalf("retour en salle d'attente attendu : phase %q, partie %+v", state.Phase, state.Game)
	}
	if !state.CanStart {
		t.Error("l'hôte devrait pouvoir relancer la partie")
	}
	for _, p := range state.Players {
		if p.Ready {
			t.Errorf("%s est encore marqué prêt", p.Pseudo)
		}
	}

	// Une nouvelle partie redistribue les rôles.
	tbl.start(t)
	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseNight {
		t.Errorf("phase = %q, attendu %q", state.Phase, protocol.PhaseNight)
	}
}

func TestLesPhasesDuMoteurSontCellesDuFil(t *testing.T) {
	wire := map[game.Phase]protocol.Phase{
		game.PhaseRoles:   protocol.PhaseRoles,
		game.PhaseNight:   protocol.PhaseNight,
		game.PhaseDawn:    protocol.PhaseDawn,
		game.PhaseDebate:  protocol.PhaseDebate,
		game.PhaseVote:    protocol.PhaseVote,
		game.PhaseVerdict: protocol.PhaseVerdict,
		game.PhaseOver:    protocol.PhaseOver,
	}

	phases := game.Phases()
	if len(phases) != len(wire) {
		t.Fatalf("le moteur annonce %d phases, le contrat en connaît %d", len(phases), len(wire))
	}

	for _, phase := range phases {
		want, ok := wire[phase]
		if !ok {
			t.Fatalf("la phase %q du moteur n'est pas dans le contrat", phase)
		}
		if string(phase) != string(want) {
			t.Errorf("phase %q traduite en %q", phase, want)
		}
	}

	if string(game.Villageois) != protocol.RoleVillageois {
		t.Errorf("rôle %q traduit en %q", game.Villageois, protocol.RoleVillageois)
	}
	if string(game.LoupGarou) != protocol.RoleLoupGarou {
		t.Errorf("rôle %q traduit en %q", game.LoupGarou, protocol.RoleLoupGarou)
	}
	if string(game.WinnerVillage) != "village" || string(game.WinnerWolves) != "loups" {
		t.Errorf("camps traduits en %q et %q", game.WinnerVillage, game.WinnerWolves)
	}
}

// newRunningRoomClosed démarre une salle avec une fonction de fermeture.
func newRunningRoomClosed(t *testing.T, opts Options, onClose func(code string)) (*Room, PlayerID, string) {
	t.Helper()

	salle, hostID, token := New("ABC234", accountOf("Joueur1"), "Joueur1", opts, onClose)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go salle.Run(ctx)

	return salle, hostID, token
}

// livingVillagers liste les villageois encore en vie, dans l'ordre de la table.
func (tbl *table) livingVillagers(t *testing.T) []*seat {
	t.Helper()

	alive := map[string]bool{}
	for _, id := range tbl.alive(t) {
		alive[id] = true
	}

	out := []*seat{}
	for _, s := range tbl.seats {
		if alive[string(s.id)] && s.role(t) == protocol.RoleVillageois {
			out = append(out, s)
		}
	}

	if len(out) == 0 {
		t.Fatal("aucun villageois vivant")
	}

	return out
}

// findPlayerDead dit si un pseudo est marqué mort dans l'instantané.
func findPlayerDead(t *testing.T, state protocol.RoomState, pseudo string) (bool, bool) {
	t.Helper()

	for _, player := range state.Players {
		if player.Pseudo == pseudo {
			return player.Dead, true
		}
	}

	return false, false
}
