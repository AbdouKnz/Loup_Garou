package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/protocol"
)

// partieRapide règle des phases courtes mais jouables : le test doit avoir le
// temps d'agir entre deux bascules de la minuterie.
func partieRapide() game.Config {
	return game.Config{
		MinPlayers: 6,
		MaxPlayers: 12,
		Night:      3 * time.Second,
		NightStep:  3 * time.Second,
		Reveal:     400 * time.Millisecond,
		Speak:      2 * time.Second,
		Vote:       3 * time.Second,
	}
}

// send envoie une trame sur une connexion.
func send(t *testing.T, socket *websocket.Conn, messageType protocol.MessageType, payload any) {
	t.Helper()

	sendFrame(t, socket, protocol.MustEncode(messageType, payload))
}

// readError attend une trame d'erreur et renvoie son code.
func readError(t *testing.T, socket *websocket.Conn) protocol.ErrorCode {
	t.Helper()

	return decode[protocol.ErrorPayload](t, readFrame(t, socket, protocol.TypeError)).Code
}

// TestWebSocketPartieComplete joue une partie entière à six, par de vraies
// connexions WebSocket : lancement, nuit, vote, fin, revanche.
func TestWebSocketPartieComplete(t *testing.T) {
	srv := newTestServerWithGame(t, t.TempDir(), partieRapide())

	// --- six joueurs s'attablent ---
	hote := createRoom(t, srv.URL, "Joueur 1")
	sessions := []sessionResponse{hote}

	for i := 2; i <= 6; i++ {
		sessions = append(sessions, joinRoom(t, srv.URL, hote.Code, fmt.Sprintf("Joueur %d", i)))
	}

	sockets := make([]*websocket.Conn, len(sessions))
	for i, session := range sessions {
		sockets[i] = authentifie(t, srv.URL, hote.Code, session.Token)

		confirmation := decode[protocol.AuthOKPayload](t, readFrame(t, sockets[i], protocol.TypeAuthOK))
		if confirmation.PlayerID != session.PlayerID {
			t.Fatalf("%s : identité %q, attendue %q", session.Pseudo, confirmation.PlayerID, session.PlayerID)
		}
	}

	state := waitForState(t, sockets[0], "six joueurs en ligne", func(state protocol.RoomState) bool {
		return len(state.Players) == 6 && state.CanStart
	})
	if state.Phase != protocol.PhaseLobby {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseLobby)
	}

	// --- la partie démarre : chacun découvre son rôle ---
	send(t, sockets[0], protocol.TypeGameStart, nil)

	roles := make([]string, len(sockets))
	ids := make([]string, len(sockets))

	for i, socket := range sockets {
		session := sessions[i]

		nuit := waitForState(t, socket, "la première nuit", func(state protocol.RoomState) bool {
			return state.Phase == protocol.PhaseNight && state.Game != nil
		})

		roles[i] = nuit.Game.YourRole
		ids[i] = nuit.You

		if nuit.Game.Day != 1 || len(nuit.Game.Alive) != 6 {
			t.Errorf("%s : nuit %+v", session.Pseudo, nuit.Game)
		}
		if nuit.CanStart {
			t.Errorf("%s : l'hôte peut relancer une partie en cours", session.Pseudo)
		}
		if len(nuit.Game.Roles) != 0 {
			t.Errorf("%s : les rôles sont révélés avant la fin", session.Pseudo)
		}
	}

	wolf, seer, witch := -1, -1, -1
	villagers := []int{}
	for i, role := range roles {
		switch role {
		case protocol.RoleLoupGarou:
			wolf = i
		case protocol.RoleVoyante:
			seer = i
		case protocol.RoleSorciere:
			witch = i
		default:
			villagers = append(villagers, i)
		}
	}
	if wolf < 0 || seer < 0 || witch < 0 || len(villagers) == 0 {
		t.Fatalf("rôles inattendus : %v", roles)
	}
	victim := villagers[0]

	// Personne n'entre en cours de partie.
	tardif := signup(t, srv.URL, "Tardif")
	response := doJSONAs(t, http.MethodPost, srv.URL+"/api/rooms/"+hote.Code+"/join", "", tardif)
	if response.status != http.StatusConflict {
		t.Errorf("entrée en cours de partie : statut %d, corps %s", response.status, response.body)
	}
	if code := response.errorCode(t); code != protocol.CodeGameInProgress {
		t.Errorf("code = %q, attendu %q", code, protocol.CodeGameInProgress)
	}

	// --- la nuit : chaque veillée appelle son rôle ---
	//
	// La voyante se lève la première : elle observe le loup, et son carnet ne
	// regarde qu'elle.
	send(t, sockets[seer], protocol.TypeGameNightAction, protocol.NightActionPayload{
		Action: protocol.ActionSee,
		Target: ids[wolf],
	})

	carnet := waitForState(t, sockets[seer], "le carnet de la voyante", func(state protocol.RoomState) bool {
		return state.Game != nil && len(state.Game.YourVision) == 1
	})
	if carnet.Game.YourVision[0].Role != protocol.RoleLoupGarou {
		t.Errorf("la voyante a vu %q, attendu un loup", carnet.Game.YourVision[0].Role)
	}

	// Le loup, lui, n'a rien vu venir.
	send(t, sockets[wolf], protocol.TypeGameNightVote, protocol.VotePayload{Target: ids[victim]})

	// La sorcière est appelée à son tour : elle reçoit la proie des loups, et
	// les laisse faire.
	sorciere := waitForState(t, sockets[witch], "l'invitation de la sorcière", func(state protocol.RoomState) bool {
		return state.Game != nil && state.Game.YourAction == protocol.ActionPotions
	})
	if sorciere.Game.WolfTarget != ids[victim] {
		t.Errorf("proie annoncée %q, attendue %q", sorciere.Game.WolfTarget, ids[victim])
	}
	if sorciere.Game.YourPotions == nil || !sorciere.Game.YourPotions.Heal {
		t.Errorf("potions de la sorcière : %+v", sorciere.Game.YourPotions)
	}

	send(t, sockets[witch], protocol.TypeGameNightAction, protocol.NightActionPayload{Action: protocol.ActionPass})

	for i, socket := range sockets {
		aube := waitForState(t, socket, "l'aube", func(state protocol.RoomState) bool {
			return state.Phase == protocol.PhaseDawn
		})

		if len(aube.Game.Deaths) != 1 || aube.Game.Deaths[0].Target != ids[victim] {
			t.Errorf("%s : morts %+v, attendu la victime %q", sessions[i].Pseudo, aube.Game.Deaths, ids[victim])
		}
		if len(aube.Game.Deaths) == 1 && aube.Game.Deaths[0].Cause != protocol.CauseWolves {
			t.Errorf("%s : cause %q, attendue %q", sessions[i].Pseudo, aube.Game.Deaths[0].Cause, protocol.CauseWolves)
		}
		if aube.Game.EliminatedRole != "" {
			t.Errorf("%s : le rôle de la victime est révélé (%q)", sessions[i].Pseudo, aube.Game.EliminatedRole)
		}

		morte, ok := findView(aube, sessions[victim].Pseudo)
		if !ok || !morte.Dead {
			t.Errorf("%s : la victime n'apparaît pas morte dans la table", sessions[i].Pseudo)
		}
	}

	// --- la victime est un spectateur muet ---
	send(t, sockets[victim], protocol.TypeChatSend, protocol.ChatSendPayload{Text: "je suis mort"})
	if code := readError(t, sockets[victim]); code != protocol.CodePlayerDead {
		t.Errorf("chat d'un mort : code = %q, attendu %q", code, protocol.CodePlayerDead)
	}

	// --- le débat : un tour de parole, l'orateur seul a la plume ---
	debate := waitForState(t, sockets[0], "le débat", func(state protocol.RoomState) bool {
		return state.Phase == protocol.PhaseDebate && state.Game.Speaker != ""
	})

	indexOf := func(playerID string) int {
		for i := range ids {
			if ids[i] == playerID {
				return i
			}
		}
		return -1
	}

	speaker := indexOf(debate.Game.Speaker)
	if speaker < 0 {
		t.Fatalf("l'orateur %q n'est pas à la table", debate.Game.Speaker)
	}

	// L'orateur a la parole, et lui seul : sa voix part dans le fil.
	send(t, sockets[speaker], protocol.TypeChatSend, protocol.ChatSendPayload{Text: "je soupçonne quelqu'un"})
	if message := readChat(t, sockets[0], "je soupçonne quelqu'un"); message.Pseudo != sessions[speaker].Pseudo {
		t.Errorf("pseudo = %q, attendu %q", message.Pseudo, sessions[speaker].Pseudo)
	}

	// Chaque orateur rend ensuite la parole : le débat court jusqu'au vote sans
	// attendre les secondes de chacun, et dans l'ordre annoncé par le serveur.
	for _, playerID := range debate.Game.Speakers {
		who := indexOf(playerID)
		send(t, sockets[who], protocol.TypeGameSpeakDone, nil)

		waitForState(t, sockets[who], "la parole rendue", func(state protocol.RoomState) bool {
			if state.Phase != protocol.PhaseDebate {
				return true
			}
			for _, spoken := range state.Game.Spoken {
				if spoken == playerID {
					return true
				}
			}
			return false
		})
	}

	// --- le vote : la victime ne vote plus, le village condamne le loup ---
	waitForState(t, sockets[0], "le vote", func(state protocol.RoomState) bool {
		return state.Phase == protocol.PhaseVote
	})

	send(t, sockets[victim], protocol.TypeGameVote, protocol.VotePayload{Target: ids[wolf]})
	if code := readError(t, sockets[victim]); code != protocol.CodePlayerDead {
		t.Errorf("vote d'un mort : code = %q, attendu %q", code, protocol.CodePlayerDead)
	}

	for i, socket := range sockets {
		if i == victim {
			continue
		}
		send(t, socket, protocol.TypeGameVote, protocol.VotePayload{Target: ids[wolf]})
	}

	state = waitForState(t, sockets[0], "la fin de la partie", func(state protocol.RoomState) bool {
		return state.Phase == protocol.PhaseOver
	})
	if state.Game.Winner != "village" {
		t.Errorf("vainqueur = %q, attendu village", state.Game.Winner)
	}
	if len(state.Game.Roles) != 6 {
		t.Fatalf("rôles révélés à la fin : %v", state.Game.Roles)
	}
	if state.Game.Roles[ids[wolf]] != protocol.RoleLoupGarou {
		t.Errorf("le loup n'est pas démasqué : %v", state.Game.Roles)
	}
	if state.Game.Roles[ids[victim]] != protocol.RoleVillageois {
		t.Errorf("la victime n'est pas innocentée : %v", state.Game.Roles)
	}

	// --- la revanche : l'hôte renvoie tout le monde en salle d'attente ---
	send(t, sockets[0], protocol.TypeGameAgain, nil)

	for i, socket := range sockets {
		salle := waitForState(t, socket, "le retour en salle d'attente", func(state protocol.RoomState) bool {
			return state.Phase == protocol.PhaseLobby && state.Game == nil
		})

		if want := i == 0; salle.CanStart != want {
			t.Errorf("%s : canStart = %v, attendu %v", sessions[i].Pseudo, salle.CanStart, want)
		}
	}

	// La salle accepte de nouveau du monde : la partie est bien terminée.
	thence := joinRoom(t, srv.URL, hote.Code, "Revanche")
	if thence.PlayerID == "" {
		t.Error("un joueur devrait pouvoir entrer dans la salle d'attente")
	}
}
