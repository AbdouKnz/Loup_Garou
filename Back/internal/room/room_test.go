package room

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/protocol"
)

// fakeClient enregistre les trames envoyées à un joueur, comme le ferait une
// vraie connexion réseau.
type fakeClient struct {
	mu     sync.Mutex
	frames []protocol.Envelope
	kicked bool
}

func newFakeClient() *fakeClient { return &fakeClient{} }

func (f *fakeClient) Send(env protocol.Envelope) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.frames = append(f.frames, env)
}

// Kick enregistre la trame finale puis marque la connexion comme refermée.
func (f *fakeClient) Kick(env protocol.Envelope) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.frames = append(f.frames, env)
	f.kicked = true
}

// wasKicked indique si la salle a refermé cette connexion.
func (f *fakeClient) wasKicked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.kicked
}

// all renvoie une copie des trames reçues, dans l'ordre.
func (f *fakeClient) all() []protocol.Envelope {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]protocol.Envelope(nil), f.frames...)
}

func (f *fakeClient) count(kind protocol.MessageType) int {
	total := 0
	for _, env := range f.all() {
		if env.Type == kind {
			total++
		}
	}
	return total
}

// lastState renvoie le dernier instantané reçu par ce joueur.
func (f *fakeClient) lastState(t *testing.T) protocol.RoomState {
	t.Helper()

	var state protocol.RoomState
	found := false

	for _, env := range f.all() {
		if env.Type != protocol.TypeRoomState {
			continue
		}

		// Une valeur neuve à chaque trame : décoder par-dessus la précédente
		// laisserait survivre les champs absents du JSON (game, par exemple).
		var current protocol.RoomState
		if err := env.DecodeData(&current); err != nil {
			t.Fatalf("instantané illisible : %v", err)
		}

		state, found = current, true
	}

	if !found {
		t.Fatal("aucun instantané reçu")
	}

	return state
}

// lastError renvoie la dernière erreur reçue par ce joueur.
func (f *fakeClient) lastError(t *testing.T) protocol.ErrorPayload {
	t.Helper()

	var payload protocol.ErrorPayload
	found := false

	for _, env := range f.all() {
		if env.Type != protocol.TypeError {
			continue
		}

		var current protocol.ErrorPayload
		if err := env.DecodeData(&current); err != nil {
			t.Fatalf("erreur illisible : %v", err)
		}

		payload, found = current, true
	}

	if !found {
		t.Fatal("aucune erreur reçue")
	}

	return payload
}

// chats renvoie les messages de chat reçus, dans l'ordre.
func (f *fakeClient) chats(t *testing.T) []protocol.ChatMessage {
	t.Helper()

	var out []protocol.ChatMessage
	for _, env := range f.all() {
		if env.Type != protocol.TypeChatMessage {
			continue
		}

		var message protocol.ChatMessage
		if err := env.DecodeData(&message); err != nil {
			t.Fatalf("message illisible : %v", err)
		}
		out = append(out, message)
	}

	return out
}

// accountOf fabrique l'identifiant de compte d'un joueur de test : les tests
// qui ne parlent pas de comptes ont ainsi des identités distinctes sans avoir à
// les inventer.
func accountOf(pseudo string) string { return "compte-" + pseudo }

// newRunningRoom démarre une salle dans sa goroutine et l'arrête au nettoyage.
func newRunningRoom(t *testing.T, code, hostPseudo string, opts Options) (*Room, PlayerID, string) {
	t.Helper()

	salle, hostID, token := New(code, accountOf(hostPseudo), hostPseudo, opts, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go salle.Run(ctx)

	return salle, hostID, token
}

// findPlayer retrouve la vue d'un joueur par son pseudo.
func findPlayer(t *testing.T, state protocol.RoomState, pseudo string) protocol.PlayerView {
	t.Helper()

	for _, player := range state.Players {
		if player.Pseudo == pseudo {
			return player
		}
	}

	t.Fatalf("joueur %q absent de l'instantané", pseudo)

	return protocol.PlayerView{}
}

func TestSalleAccueilleLesJoueurs(t *testing.T) {
	salle, hostID, hostToken := newRunningRoom(t, "ABC234", "Alice", DefaultOptions())

	hostClient := newFakeClient()
	authenticated, err := salle.Authenticate(hostToken, hostClient)
	if err != nil {
		t.Fatalf("authentification de l'hôte impossible : %v", err)
	}
	if authenticated != hostID {
		t.Fatalf("identifiant = %q, attendu %q", authenticated, hostID)
	}

	bobID, bobToken, err := salle.Join(accountOf("Bob"), "  Bob  ")
	if err != nil {
		t.Fatalf("entrée de Bob impossible : %v", err)
	}

	bobClient := newFakeClient()
	if _, err := salle.Authenticate(bobToken, bobClient); err != nil {
		t.Fatalf("authentification de Bob impossible : %v", err)
	}

	// L'hôte reçoit l'état complet, vu par lui, deux joueurs en ligne.
	state := hostClient.lastState(t)

	if state.You != string(hostID) {
		t.Errorf("you = %q, attendu %q", state.You, hostID)
	}
	if state.HostID != string(hostID) {
		t.Errorf("hostId = %q, attendu %q", state.HostID, hostID)
	}
	if state.Phase != protocol.PhaseLobby {
		t.Errorf("phase = %q, attendu %q", state.Phase, protocol.PhaseLobby)
	}
	if state.MaxPlayers != protocol.MaxPlayers {
		t.Errorf("maxPlayers = %d, attendu %d", state.MaxPlayers, protocol.MaxPlayers)
	}
	if len(state.Players) != 2 {
		t.Fatalf("joueurs = %d, attendu 2", len(state.Players))
	}

	alice := findPlayer(t, state, "Alice")
	if !alice.Host || !alice.Online || alice.ID != string(hostID) {
		t.Errorf("vue d'Alice inattendue : %+v", alice)
	}

	// Le pseudo a été nettoyé et Bob voit qu'il est bien lui-même.
	bob := findPlayer(t, state, "Bob")
	if bob.ID != string(bobID) || !bob.Online {
		t.Errorf("vue de Bob inattendue : %+v", bob)
	}
	if bobState := bobClient.lastState(t); bobState.You != string(bobID) {
		t.Errorf("you de Bob = %q, attendu %q", bobState.You, bobID)
	}

	// La trame d'authentification arrive toujours en premier.
	if frames := hostClient.all(); frames[0].Type != protocol.TypeAuthOK {
		t.Errorf("première trame = %q, attendue %q", frames[0].Type, protocol.TypeAuthOK)
	}

	// Tout le monde est prévenu de l'arrivée.
	found := false
	for _, message := range hostClient.chats(t) {
		if message.System && strings.Contains(message.Text, "Bob a rejoint la partie") {
			found = true
		}
	}
	if !found {
		t.Error("aucune annonce d'arrivée reçue par l'hôte")
	}
}

func TestSalleRefusePseudoInvalideEtJoueursEnTrop(t *testing.T) {
	opts := DefaultOptions()
	opts.MaxPlayers = 2

	salle, _, _ := newRunningRoom(t, "ABC234", "Alice", opts)

	if _, _, err := salle.Join(accountOf("B"), "B"); protocol.CodeOf(err) != protocol.CodeInvalidPseudo {
		t.Fatalf("pseudo trop court : code = %q", protocol.CodeOf(err))
	}

	if _, _, err := salle.Join(accountOf("Bob"), "Bob"); err != nil {
		t.Fatalf("Bob devrait pouvoir entrer : %v", err)
	}

	if _, _, err := salle.Join(accountOf("Chloé"), "Chloé"); protocol.CodeOf(err) != protocol.CodeRoomFull {
		t.Fatalf("salle pleine : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeRoomFull)
	}
}

func TestChatEtLimiteDeDebit(t *testing.T) {
	salle, hostID, hostToken := newRunningRoom(t, "ABC234", "Alice", DefaultOptions())

	hostClient := newFakeClient()
	if _, err := salle.Authenticate(hostToken, hostClient); err != nil {
		t.Fatalf("authentification impossible : %v", err)
	}

	_, bobToken, err := salle.Join(accountOf("Bob"), "Bob")
	if err != nil {
		t.Fatalf("entrée de Bob impossible : %v", err)
	}

	bobID, err := salle.Authenticate(bobToken, newFakeClient())
	if err != nil {
		t.Fatalf("authentification de Bob impossible : %v", err)
	}

	if err := salle.Chat(bobID, "Bonjour tout le monde"); err != nil {
		t.Fatalf("chat refusé : %v", err)
	}

	received := hostClient.chats(t)
	last := received[len(received)-1]
	if last.Text != "Bonjour tout le monde" || last.Pseudo != "Bob" {
		t.Errorf("message reçu inattendu : %+v", last)
	}

	// Le message trop long est refusé avant même d'atteindre l'acteur.
	if err := salle.Chat(hostID, strings.Repeat("a", protocol.MaxChatLength+1)); protocol.CodeOf(err) != protocol.CodeInvalidMessage {
		t.Errorf("message trop long : code = %q", protocol.CodeOf(err))
	}

	// La limite de débit laisse passer ChatBurst messages au total, pas un de plus
	// (le premier message comptait déjà).
	for i := 0; i < protocol.ChatBurst-1; i++ {
		if err := salle.Chat(bobID, "spam"); err != nil {
			t.Fatalf("message %d refusé : %v", i+2, err)
		}
	}

	if err := salle.Chat(bobID, "spam"); protocol.CodeOf(err) != protocol.CodeRateLimited {
		t.Errorf("limite de débit : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeRateLimited)
	}
}

func TestAuthentificationRefuseUnJetOnInconnu(t *testing.T) {
	salle, _, _ := newRunningRoom(t, "ABC234", "Alice", DefaultOptions())

	if _, err := salle.Authenticate("jeton-inventé", newFakeClient()); protocol.CodeOf(err) != protocol.CodeNotAuthenticated {
		t.Fatalf("code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeNotAuthenticated)
	}

	if _, err := salle.Authenticate("", newFakeClient()); protocol.CodeOf(err) != protocol.CodeNotAuthenticated {
		t.Fatalf("jeton vide : code = %q", protocol.CodeOf(err))
	}
}

func TestNouvelleConnexionRemplaceLAncienne(t *testing.T) {
	salle, hostID, hostToken := newRunningRoom(t, "ABC234", "Alice", DefaultOptions())

	ancienne := newFakeClient()
	if _, err := salle.Authenticate(hostToken, ancienne); err != nil {
		t.Fatalf("première connexion impossible : %v", err)
	}

	nouvelle := newFakeClient()
	if _, err := salle.Authenticate(hostToken, nouvelle); err != nil {
		t.Fatalf("reprise impossible : %v", err)
	}

	if payload := ancienne.lastError(t); payload.Code != protocol.CodeUnauthorized {
		t.Errorf("code = %q, attendu %q", payload.Code, protocol.CodeUnauthorized)
	}

	if !ancienne.wasKicked() {
		t.Error("l'ancienne connexion doit être refermée après la reprise de la place")
	}

	if nouvelle.wasKicked() {
		t.Error("la nouvelle connexion ne doit pas être refermée")
	}

	// La fermeture de l'ancienne connexion ne doit pas déloger la nouvelle.
	salle.Disconnect(ancienne)

	_, bobToken, err := salle.Join(accountOf("Bob"), "Bob")
	if err != nil {
		t.Fatalf("entrée de Bob impossible : %v", err)
	}

	bobClient := newFakeClient()
	bobID, err := salle.Authenticate(bobToken, bobClient)
	if err != nil {
		t.Fatalf("authentification de Bob impossible : %v", err)
	}

	if err := salle.SetReady(bobID, true); err != nil {
		t.Fatalf("mise en prêt impossible : %v", err)
	}

	state := bobClient.lastState(t)
	if alice := findPlayer(t, state, "Alice"); !alice.Online || alice.ID != string(hostID) {
		t.Errorf("la reconnexion a été perdue : %+v", alice)
	}
	if bob := findPlayer(t, state, "Bob"); !bob.Ready {
		t.Errorf("Bob devrait être prêt : %+v", bob)
	}
}

func TestDeconnexionMarqueHorsLigneEtPermetLaReprise(t *testing.T) {
	salle, _, hostToken := newRunningRoom(t, "ABC234", "Alice", DefaultOptions())

	hostClient := newFakeClient()
	if _, err := salle.Authenticate(hostToken, hostClient); err != nil {
		t.Fatalf("authentification de l'hôte impossible : %v", err)
	}

	_, bobToken, err := salle.Join(accountOf("Bob"), "Bob")
	if err != nil {
		t.Fatalf("entrée de Bob impossible : %v", err)
	}

	bobClient := newFakeClient()
	if _, err := salle.Authenticate(bobToken, bobClient); err != nil {
		t.Fatalf("authentification de Bob impossible : %v", err)
	}

	salle.Disconnect(bobClient)

	if bob := findPlayer(t, hostClient.lastState(t), "Bob"); bob.Online {
		t.Error("Bob devrait apparaître hors ligne")
	}

	// Il retrouve sa place (même jeton) après un rafraîchissement de page.
	reprise := newFakeClient()
	if _, err := salle.Authenticate(bobToken, reprise); err != nil {
		t.Fatalf("reprise impossible : %v", err)
	}

	if bob := findPlayer(t, hostClient.lastState(t), "Bob"); !bob.Online {
		t.Error("Bob devrait de nouveau être en ligne")
	}
	if state := reprise.lastState(t); state.You == "" {
		t.Error("l'instantané de reprise n'indique pas le joueur")
	}
}

func TestDepartDeLHoteTransmetLeRole(t *testing.T) {
	salle, hostID, hostToken := newRunningRoom(t, "ABC234", "Alice", DefaultOptions())

	hostClient := newFakeClient()
	if _, err := salle.Authenticate(hostToken, hostClient); err != nil {
		t.Fatalf("authentification de l'hôte impossible : %v", err)
	}

	bobID, bobToken, err := salle.Join(accountOf("Bob"), "Bob")
	if err != nil {
		t.Fatalf("entrée de Bob impossible : %v", err)
	}

	bobClient := newFakeClient()
	if _, err := salle.Authenticate(bobToken, bobClient); err != nil {
		t.Fatalf("authentification de Bob impossible : %v", err)
	}

	salle.Leave(hostID)

	state := bobClient.lastState(t)
	if state.HostID != string(bobID) {
		t.Errorf("hostId = %q, attendu %q", state.HostID, bobID)
	}
	if bob := findPlayer(t, state, "Bob"); !bob.Host {
		t.Error("Bob devrait être le nouvel hôte")
	}
	if len(state.Players) != 1 {
		t.Errorf("joueurs = %d, attendu 1 (Alice est partie)", len(state.Players))
	}
}

func TestSweepRetireLesJoueursHorsLigneEtFermeLaSalle(t *testing.T) {
	var closed string

	opts := DefaultOptions()
	opts.OfflineTimeout = time.Minute
	opts.EmptyRoomTimeout = time.Hour

	salle, hostID, _ := New("ABC234", accountOf("Alice"), "Alice", opts, func(code string) { closed = code })

	now := time.Now()

	// Un joueur jamais connecté n'est pas « hors ligne » : rien ne bouge.
	salle.sweep(now)
	if len(salle.players) != 1 {
		t.Fatalf("joueurs = %d, attendu 1", len(salle.players))
	}

	// Hors ligne depuis plus que le délai : il est retiré, la salle se vide.
	salle.players[hostID].offlineSince = now.Add(-2 * time.Minute)
	salle.sweep(now)

	if len(salle.players) != 0 {
		t.Fatalf("joueurs = %d, attendu 0", len(salle.players))
	}
	if closed != "" {
		t.Fatalf("la salle s'est fermée trop tôt : %q", closed)
	}

	// Vipère vide depuis assez longtemps : elle se ferme et se fait oublier.
	salle.sweep(now.Add(2 * time.Hour))

	if closed != "ABC234" {
		t.Errorf("salle fermée = %q, attendu %q", closed, "ABC234")
	}
	if _, _, err := salle.Join(accountOf("Bob"), "Bob"); err != ErrClosed {
		t.Errorf("erreur = %v, attendu %v", err, ErrClosed)
	}
}

func TestSweepFermeUneSalleJamaisRejointe(t *testing.T) {
	var closed string

	opts := DefaultOptions()
	opts.EmptyRoomTimeout = time.Minute
	opts.OfflineTimeout = time.Hour

	salle, _, _ := New("ABC234", accountOf("Alice"), "Alice", opts, func(code string) { closed = code })

	// Personne ne s'est jamais connecté : la salle ne doit pas rester ouverte.
	salle.sweep(time.Now().Add(2 * time.Minute))

	if closed != "ABC234" {
		t.Errorf("salle fermée = %q, attendu %q", closed, "ABC234")
	}
}

func TestRegistreCreeEtRetrouveLesSalles(t *testing.T) {
	registre := NewRegistry(DefaultOptions())
	t.Cleanup(registre.Close)

	code, hostID, token, err := registre.Create(accountOf("Alice"), "  Alice  ")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	if _, err := protocol.NormalizeRoomCode(code); err != nil {
		t.Errorf("code généré invalide (%q) : %v", code, err)
	}
	if hostID == "" || token == "" {
		t.Error("identifiant ou jeton manquant")
	}
	if registre.Len() != 1 {
		t.Errorf("salles = %d, attendu 1", registre.Len())
	}

	// La recherche ignore la casse.
	if _, ok := registre.Get(strings.ToLower(code)); !ok {
		t.Error("la salle devrait être retrouvée en minuscules")
	}
	if _, ok := registre.Get("ABC234X"); ok {
		t.Error("un code mal formé ne doit pas être trouvé")
	}

	bobID, bobToken, err := registre.Join(code, accountOf("Bob"), "Bob")
	if err != nil {
		t.Fatalf("entrée impossible : %v", err)
	}
	if bobID == "" || bobToken == "" || bobToken == token {
		t.Error("identifiants de Bob inattendus")
	}

	if _, _, err := registre.Join("ZZZZZZ", accountOf("Bob"), "Bob"); protocol.CodeOf(err) != protocol.CodeRoomNotFound {
		t.Errorf("salle inconnue : code = %q", protocol.CodeOf(err))
	}
	if _, _, _, err := registre.Create(accountOf("A"), "A"); protocol.CodeOf(err) != protocol.CodeInvalidPseudo {
		t.Errorf("pseudo invalide : code = %q", protocol.CodeOf(err))
	}
}

func TestRegistreFermeSesSalles(t *testing.T) {
	registre := NewRegistry(DefaultOptions())

	code, _, _, err := registre.Create(accountOf("Alice"), "Alice")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	salle, ok := registre.Get(code)
	if !ok {
		t.Fatal("salle introuvable")
	}

	registre.Close()

	if registre.Len() != 0 {
		t.Errorf("salles = %d, attendu 0", registre.Len())
	}
	if _, err := salle.Summary(); err != ErrClosed {
		t.Errorf("erreur = %v, attendu %v", err, ErrClosed)
	}
}
