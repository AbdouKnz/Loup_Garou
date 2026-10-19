package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/abdou/loupgarou/internal/protocol"
)

// wsTimeout borne chaque attente du test : un échec doit être un échec, pas un
// blocage.
const wsTimeout = 5 * time.Second

// dialRoom ouvre une session WebSocket sur une partie.
func dialRoom(t *testing.T, baseURL, code string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()

	url := "ws" + strings.TrimPrefix(baseURL, "http") + "/api/rooms/" + code + "/ws"

	socket, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("connexion WebSocket impossible : %v", err)
	}

	t.Cleanup(func() { _ = socket.CloseNow() })

	return socket
}

// sendFrame écrit une trame vers le serveur.
func sendFrame(t *testing.T, socket *websocket.Conn, env protocol.Envelope) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()

	if err := wsjson.Write(ctx, socket, env); err != nil {
		t.Fatalf("envoi de %q impossible : %v", env.Type, err)
	}
}

// readFrame lit les trames jusqu'à en trouver une du type voulu.
func readFrame(t *testing.T, socket *websocket.Conn, want protocol.MessageType) protocol.Envelope {
	t.Helper()

	deadline := time.Now().Add(wsTimeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("trame %q non reçue à temps", want)
		}

		ctx, cancel := context.WithTimeout(context.Background(), remaining)
		var env protocol.Envelope
		err := wsjson.Read(ctx, socket, &env)
		cancel()

		if err != nil {
			t.Fatalf("lecture impossible en attendant %q : %v", want, err)
		}
		if env.Type == want {
			return env
		}
	}
}

// decode décode la charge utile d'une trame.
func decode[T any](t *testing.T, env protocol.Envelope) T {
	t.Helper()

	var out T
	if err := env.DecodeData(&out); err != nil {
		t.Fatalf("trame %q illisible : %v", env.Type, err)
	}

	return out
}

// readChat attend le message de chat contenant text.
func readChat(t *testing.T, socket *websocket.Conn, text string) protocol.ChatMessage {
	t.Helper()

	deadline := time.Now().Add(wsTimeout)
	for time.Now().Before(deadline) {
		message := decode[protocol.ChatMessage](t, readFrame(t, socket, protocol.TypeChatMessage))
		if strings.Contains(message.Text, text) {
			return message
		}
	}

	t.Fatalf("message contenant %q non reçu", text)

	return protocol.ChatMessage{}
}

// findView retrouve la vue d'un joueur par son pseudo.
func findView(state protocol.RoomState, pseudo string) (protocol.PlayerView, bool) {
	for _, player := range state.Players {
		if player.Pseudo == pseudo {
			return player, true
		}
	}

	return protocol.PlayerView{}, false
}

// waitForState lit les instantanés jusqu'à en trouver un qui satisfasse match.
//
// Le serveur diffuse un instantané à chaque changement : les trames arrivent
// dans l'ordre, et un test doit donc attendre celui qui l'intéresse.
func waitForState(t *testing.T, socket *websocket.Conn, what string, match func(protocol.RoomState) bool) protocol.RoomState {
	t.Helper()

	deadline := time.Now().Add(wsTimeout)

	for {
		if time.Now().After(deadline) {
			t.Fatalf("aucun instantané ne correspond à %q", what)
		}

		state := decode[protocol.RoomState](t, readFrame(t, socket, protocol.TypeRoomState))
		if match(state) {
			return state
		}
	}
}

// authentifie ouvre la session WebSocket d'un joueur.
func authentifie(t *testing.T, baseURL, code, token string) *websocket.Conn {
	t.Helper()

	socket := dialRoom(t, baseURL, code)
	sendFrame(t, socket, protocol.MustEncode(protocol.TypeAuth, protocol.AuthPayload{Token: token}))

	return socket
}

func TestWebSocketParcoursComplet(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	// --- l'hôte ouvre la partie ---
	hote := createRoom(t, srv.URL, "Alice")
	alice := authentifie(t, srv.URL, hote.Code, hote.Token)

	confirmation := decode[protocol.AuthOKPayload](t, readFrame(t, alice, protocol.TypeAuthOK))
	if confirmation.PlayerID != hote.PlayerID {
		t.Errorf("playerId = %q, attendu %q", confirmation.PlayerID, hote.PlayerID)
	}

	state := waitForState(t, alice, "l'hôte seul dans la partie", func(state protocol.RoomState) bool {
		return state.You == hote.PlayerID && len(state.Players) == 1
	})
	if state.HostID != hote.PlayerID {
		t.Errorf("hostId = %q, attendu %q", state.HostID, hote.PlayerID)
	}
	if state.CanStart {
		t.Error("l'hôte ne devrait pas pouvoir lancer la partie tout seul")
	}

	// --- un deuxième joueur arrive par HTTP puis se connecte ---
	bob := joinRoom(t, srv.URL, hote.Code, "Bob")
	bobSocket := authentifie(t, srv.URL, hote.Code, bob.Token)
	readFrame(t, bobSocket, protocol.TypeAuthOK)

	state = waitForState(t, alice, "deux joueurs en ligne", func(state protocol.RoomState) bool {
		bob, ok := findView(state, "Bob")
		return len(state.Players) == 2 && ok && bob.Online
	})
	if state.CanStart {
		t.Error("deux joueurs ne suffisent pas à lancer la partie (six minimum)")
	}

	// --- Bob se signale prêt ---
	sendFrame(t, bobSocket, protocol.MustEncode(protocol.TypePlayerReady, protocol.PlayerReadyPayload{Ready: true}))

	waitForState(t, alice, "Bob prêt", func(state protocol.RoomState) bool {
		bob, ok := findView(state, "Bob")
		return ok && bob.Ready
	})

	// --- le chat circule ---
	sendFrame(t, bobSocket, protocol.MustEncode(protocol.TypeChatSend, protocol.ChatSendPayload{Text: "Coucou !"}))

	message := readChat(t, alice, "Coucou !")
	if message.Pseudo != "Bob" {
		t.Errorf("pseudo = %q, attendu %q", message.Pseudo, "Bob")
	}

	// --- lancer la partie exige une table complète ---
	sendFrame(t, alice, protocol.MustEncode(protocol.TypeGameStart, nil))

	refus := decode[protocol.ErrorPayload](t, readFrame(t, alice, protocol.TypeError))
	if refus.Code != protocol.CodeNotEnoughPlayers {
		t.Errorf("code = %q, attendu %q", refus.Code, protocol.CodeNotEnoughPlayers)
	}

	// --- ping/pong maintient la connexion en vie ---
	sendFrame(t, alice, protocol.MustEncode(protocol.TypePing, nil))
	readFrame(t, alice, protocol.TypePong)

	// --- Bob ferme son navigateur : Alice le voit hors ligne ---
	if err := bobSocket.Close(websocket.StatusNormalClosure, "fin du test"); err != nil {
		t.Fatalf("fermeture impossible : %v", err)
	}

	// --- Bob garde sa place : il est seulement hors ligne ---
	state = waitForState(t, alice, "Bob hors ligne", func(state protocol.RoomState) bool {
		bob, ok := findView(state, "Bob")
		return ok && !bob.Online
	})
	if len(state.Players) != 2 {
		t.Errorf("joueurs = %d, attendu 2 (place conservée)", len(state.Players))
	}

	// L'instantané transporte l'historique : un joueur qui arrive en cours de
	// partie retrouve la discussion.
	history := false
	for _, past := range state.Messages {
		if past.Text == "Coucou !" {
			history = true
		}
	}
	if !history {
		t.Error("le chat devrait être inclus dans l'instantané")
	}

	// Une violation de débit de chat est refusée sans couper la connexion.
	for i := 0; i < protocol.ChatBurst+1; i++ {
		sendFrame(t, alice, protocol.MustEncode(protocol.TypeChatSend, protocol.ChatSendPayload{Text: "flood"}))
	}

	limite := decode[protocol.ErrorPayload](t, readFrame(t, alice, protocol.TypeError))
	if limite.Code != protocol.CodeRateLimited {
		t.Errorf("code = %q, attendu %q", limite.Code, protocol.CodeRateLimited)
	}
}

func TestWebSocketRepriseDePlaceFermeLAncienneConnexion(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	hote := createRoom(t, srv.URL, "Alice")

	ancienne := authentifie(t, srv.URL, hote.Code, hote.Token)
	readFrame(t, ancienne, protocol.TypeAuthOK)

	// Le même joueur ouvre une seconde session : c'est elle qui fait autorité.
	nouvelle := authentifie(t, srv.URL, hote.Code, hote.Token)
	readFrame(t, nouvelle, protocol.TypeAuthOK)

	refus := decode[protocol.ErrorPayload](t, readFrame(t, ancienne, protocol.TypeError))
	if refus.Code != protocol.CodeUnauthorized {
		t.Errorf("code = %q, attendu %q", refus.Code, protocol.CodeUnauthorized)
	}

	// L'ancienne connexion est refermée juste après le message : un onglet
	// abandonné ne peut plus jouer dans le dos du joueur.
	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()

	var env protocol.Envelope
	if err := wsjson.Read(ctx, ancienne, &env); err == nil {
		t.Errorf("l'ancienne connexion devrait être fermée, trame reçue : %+v", env)
	}

	// La nouvelle session, elle, reste pleinement opérationnelle.
	sendFrame(t, nouvelle, protocol.MustEncode(protocol.TypeChatSend, protocol.ChatSendPayload{Text: "je suis toujours là"}))

	if message := readChat(t, nouvelle, "je suis toujours là"); message.Pseudo != "Alice" {
		t.Errorf("pseudo = %q, attendu %q", message.Pseudo, "Alice")
	}
}

func TestWebSocketExigeLAuthentificationDAbord(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	hote := createRoom(t, srv.URL, "Alice")

	socket := dialRoom(t, srv.URL, hote.Code)
	sendFrame(t, socket, protocol.MustEncode(protocol.TypeChatSend, protocol.ChatSendPayload{Text: "sans identité"}))

	refus := decode[protocol.ErrorPayload](t, readFrame(t, socket, protocol.TypeError))
	if refus.Code != protocol.CodeNotAuthenticated {
		t.Errorf("code = %q, attendu %q", refus.Code, protocol.CodeNotAuthenticated)
	}

	// Le serveur ferme ensuite la connexion.
	ctx, cancel := context.WithTimeout(context.Background(), wsTimeout)
	defer cancel()

	var env protocol.Envelope
	if err := wsjson.Read(ctx, socket, &env); err == nil {
		t.Errorf("la connexion devrait être fermée, trame reçue : %+v", env)
	}
}

func TestWebSocketRefuseUnJetOnInvalide(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	hote := createRoom(t, srv.URL, "Alice")

	socket := authentifie(t, srv.URL, hote.Code, "jeton-inventé")

	refus := decode[protocol.ErrorPayload](t, readFrame(t, socket, protocol.TypeError))
	if refus.Code != protocol.CodeNotAuthenticated {
		t.Errorf("code = %q, attendu %q", refus.Code, protocol.CodeNotAuthenticated)
	}
}

func TestWebSocketSalleInconnue(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	response := doJSON(t, http.MethodGet, srv.URL+"/api/rooms/ZZZZZZ/ws", "")
	if response.status != http.StatusNotFound {
		t.Fatalf("statut = %d, attendu %d", response.status, http.StatusNotFound)
	}

	var payload protocol.ErrorPayload
	if err := json.Unmarshal(response.body, &payload); err != nil {
		t.Fatalf("erreur illisible (%s) : %v", response.body, err)
	}
	if payload.Code != protocol.CodeRoomNotFound {
		t.Errorf("code = %q, attendu %q", payload.Code, protocol.CodeRoomNotFound)
	}
}

func TestWebSocketRefuseLesTramesInconnues(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	hote := createRoom(t, srv.URL, "Alice")

	socket := authentifie(t, srv.URL, hote.Code, hote.Token)
	readFrame(t, socket, protocol.TypeAuthOK)

	// Le client ne peut pas envoyer une trame réservée au serveur.
	sendFrame(t, socket, protocol.Envelope{Type: protocol.TypeRoomState, Data: json.RawMessage(`{}`)})

	refus := decode[protocol.ErrorPayload](t, readFrame(t, socket, protocol.TypeError))
	if refus.Code != protocol.CodeInvalidMessage {
		t.Errorf("code = %q, attendu %q", refus.Code, protocol.CodeInvalidMessage)
	}
}
