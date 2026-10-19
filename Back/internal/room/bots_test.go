package room

import (
	mathrand "math/rand"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/protocol"
)

// botOptions règle une salle où le test garde la main : la vague de bots est
// désactivée (délai négatif) et pilotée explicitement par botTurn.
func botOptions() Options {
	opts := gameOptions()
	opts.BotDelay = -1

	return opts
}

// botTable monte une salle avec un seul humain connecté : l'hôte.
func botTable(t *testing.T) *table {
	t.Helper()

	salle, hostID, token := newRunningRoom(t, "BOT234", "Hôte", botOptions())

	tbl := &table{salle: salle, host: hostID}
	tbl.seats = append(tbl.seats, connect(t, salle, "Hôte", hostID, token))

	return tbl
}

func TestLHoteAjouteDesBots(t *testing.T) {
	tbl := botTable(t)

	if err := tbl.salle.AddBots(tbl.host, 5); err != nil {
		t.Fatalf("ajout de bots impossible : %v", err)
	}

	state := tbl.seats[0].client.lastState(t)
	if len(state.Players) != 6 {
		t.Fatalf("joueurs = %d, attendu 6", len(state.Players))
	}

	bots := 0
	for _, player := range state.Players {
		if !player.Bot {
			continue
		}
		bots++
		if !player.Ready {
			t.Errorf("le bot %s n'est pas prêt", player.Pseudo)
		}
		if !player.Online {
			t.Errorf("le bot %s est annoncé hors ligne", player.Pseudo)
		}
	}
	if bots != 5 {
		t.Errorf("bots = %d, attendu 5", bots)
	}
	if !state.CanStart {
		t.Error("l'hôte devrait pouvoir lancer la partie à six joueurs")
	}
}

func TestLesBotsNeDebordentPasLaTable(t *testing.T) {
	tbl := botTable(t)

	// Beaucoup plus que de places : la salle plafonne au maximum.
	if err := tbl.salle.AddBots(tbl.host, 20); err != nil {
		t.Fatalf("ajout impossible : %v", err)
	}

	state := tbl.seats[0].client.lastState(t)
	if len(state.Players) != protocol.MaxPlayers {
		t.Errorf("joueurs = %d, attendu %d", len(state.Players), protocol.MaxPlayers)
	}

	if err := tbl.salle.AddBots(tbl.host, 1); protocol.CodeOf(err) != protocol.CodeRoomFull {
		t.Errorf("salle pleine : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeRoomFull)
	}
}

func TestLesBotsRestentReservesALHote(t *testing.T) {
	tbl := botTable(t)

	otherID, otherToken, err := tbl.salle.Join(accountOf("Bob"), "Bob")
	if err != nil {
		t.Fatalf("entrée impossible : %v", err)
	}
	other := connect(t, tbl.salle, "Bob", otherID, otherToken)

	if err := tbl.salle.AddBots(other.id, 3); protocol.CodeOf(err) != protocol.CodeUnauthorized {
		t.Errorf("ajout par un non-hôte : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeUnauthorized)
	}
	if err := tbl.salle.RemoveBots(other.id, ""); protocol.CodeOf(err) != protocol.CodeUnauthorized {
		t.Errorf("retrait par un non-hôte : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeUnauthorized)
	}
	if err := tbl.salle.AddBots(tbl.host, 0); protocol.CodeOf(err) != protocol.CodeInvalidMessage {
		t.Errorf("ajout de zéro bot : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeInvalidMessage)
	}
}

func TestOnNeTouchePlusAuxBotsEnPartie(t *testing.T) {
	tbl := botTable(t)

	if err := tbl.salle.AddBots(tbl.host, 5); err != nil {
		t.Fatalf("ajout impossible : %v", err)
	}
	tbl.start(t)

	if err := tbl.salle.AddBots(tbl.host, 1); protocol.CodeOf(err) != protocol.CodeGameInProgress {
		t.Errorf("ajout en partie : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeGameInProgress)
	}
	if err := tbl.salle.RemoveBots(tbl.host, ""); protocol.CodeOf(err) != protocol.CodeGameInProgress {
		t.Errorf("retrait en partie : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeGameInProgress)
	}
}

func TestLHoteRetireLesBots(t *testing.T) {
	tbl := botTable(t)

	if err := tbl.salle.AddBots(tbl.host, 5); err != nil {
		t.Fatalf("ajout impossible : %v", err)
	}

	var first protocol.PlayerView
	for _, player := range tbl.seats[0].client.lastState(t).Players {
		if player.Bot {
			first = player
			break
		}
	}
	if first.ID == "" {
		t.Fatal("aucun bot dans la table")
	}

	if err := tbl.salle.RemoveBots(tbl.host, first.ID); err != nil {
		t.Fatalf("retrait d'un bot impossible : %v", err)
	}
	if got := len(tbl.seats[0].client.lastState(t).Players); got != 5 {
		t.Errorf("joueurs = %d, attendu 5", got)
	}

	// Un humain ne se retire pas par ce chemin.
	if err := tbl.salle.RemoveBots(tbl.host, string(tbl.host)); protocol.CodeOf(err) != protocol.CodeBadTarget {
		t.Errorf("retrait d'un humain : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeBadTarget)
	}

	if err := tbl.salle.RemoveBots(tbl.host, ""); err != nil {
		t.Fatalf("retrait de tous les bots impossible : %v", err)
	}

	state := tbl.seats[0].client.lastState(t)
	if len(state.Players) != 1 {
		t.Errorf("joueurs = %d, attendu 1", len(state.Players))
	}
	for _, player := range state.Players {
		if player.Bot {
			t.Errorf("le bot %s est encore là", player.Pseudo)
		}
	}
}

func TestBotChoiceEcarteEtRestreint(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(1))
	view := game.View{Alive: []string{"a", "b", "c"}}

	// Jamais soi-même, jamais un joueur écarté : il ne reste que c.
	for i := 0; i < 50; i++ {
		if got := botChoice(rng, view, "a", []string{"b"}); got != "c" {
			t.Fatalf("choix = %q, attendu \"c\"", got)
		}
	}

	// Plus personne de votable : le bot s'abstient.
	if got := botChoice(rng, view, "a", []string{"b", "c"}); got != "" {
		t.Errorf("choix = %q, attendu un vote blanc", got)
	}

	// Un second tour ne laisse voter que les candidats.
	runoff := game.View{Alive: []string{"a", "b", "c"}, Candidates: []string{"b"}}
	for i := 0; i < 50; i++ {
		if got := botChoice(rng, runoff, "a", nil); got != "b" {
			t.Fatalf("ballotage : choix = %q, attendu \"b\"", got)
		}
	}
}

func TestUnePartieAvecDesBotsSeJoueJusquAuBout(t *testing.T) {
	tbl := botTable(t)

	if err := tbl.salle.AddBots(tbl.host, 5); err != nil {
		t.Fatalf("ajout impossible : %v", err)
	}
	tbl.start(t)

	// On déroule la partie comme le ferait la salle : une vague de bots, puis
	// l'échéance — mais l'échéance seulement quand la vague n'a rien fait
	// avancer, sans quoi la table sauterait des phases où les bots n'auraient pas
	// encore joué (le vote du jour, notamment).
	finished := false
	for step := 0; step < 400 && !finished; step++ {
		before := tbl.seats[0].client.lastState(t)
		if before.Phase == protocol.PhaseOver {
			finished = true
			break
		}

		if err := tbl.salle.botTurn(time.Now()); err != nil {
			t.Fatalf("vague de bots impossible : %v", err)
		}

		if after := tbl.seats[0].client.lastState(t); after.Game.Deadline.Equal(before.Game.Deadline) {
			expireNow(t, tbl)
		}
	}

	if !finished {
		t.Fatal("la partie ne s'est jamais terminée")
	}

	state := tbl.seats[0].client.lastState(t)
	if state.Game == nil || state.Game.Winner == "" {
		t.Fatalf("aucun vainqueur : %+v", state.Game)
	}
	if len(state.Game.Roles) != 6 {
		t.Errorf("rôles révélés à la fin : %v", state.Game.Roles)
	}

	// Les bots ont bel et bien voté : un scrutin public est apparu en cours de
	// jeu — les bulletins du jour, pendant le vote ou au verdict qui le dépouille.
	if !someState(t, tbl.seats[0].client, func(s protocol.RoomState) bool {
		return s.Game != nil && (len(s.Game.Voted) > 0 || len(s.Game.Ballot) > 0)
	}) {
		t.Error("aucun scrutin n'a été observé : les bots n'ont pas voté")
	}
}

// someState dit si au moins un instantané reçu vérifie match.
func someState(t *testing.T, client *fakeClient, match func(protocol.RoomState) bool) bool {
	t.Helper()

	for _, env := range client.all() {
		if env.Type != protocol.TypeRoomState {
			continue
		}

		var state protocol.RoomState
		if err := env.DecodeData(&state); err != nil {
			t.Fatalf("instantané illisible : %v", err)
		}
		if match(state) {
			return true
		}
	}

	return false
}

func TestAuDebatLesBotsNeParlentQuALeurTour(t *testing.T) {
	tbl := botTable(t)

	if err := tbl.salle.AddBots(tbl.host, 5); err != nil {
		t.Fatalf("ajout impossible : %v", err)
	}
	tbl.start(t)

	advanceTo(t, tbl, protocol.PhaseDebate)

	// L'hôte ouvre le débat : tant que la parole est à lui, les bots se taisent
	// et ne la prennent pas.
	if got := tbl.seats[0].client.lastState(t).Game.Speaker; got != string(tbl.host) {
		t.Fatalf("premier orateur = %q, attendu l'hôte %q", got, tbl.host)
	}
	if err := tbl.salle.botTurn(time.Now()); err != nil {
		t.Fatalf("vague de bots impossible : %v", err)
	}
	if state := tbl.seats[0].client.lastState(t); state.Game.Speaker != string(tbl.host) {
		t.Fatalf("un bot a parlé hors de son tour : orateur = %q", state.Game.Speaker)
	}

	// L'hôte rend la parole : chaque bot la prend à son tour, puis la rend —
	// la vague referme donc le débat sur le vote sans attendre personne.
	if err := tbl.salle.SpeakDone(tbl.host); err != nil {
		t.Fatalf("la parole n'a pas pu être rendue : %v", err)
	}
	if err := tbl.salle.botTurn(time.Now()); err != nil {
		t.Fatalf("vague de bots impossible : %v", err)
	}

	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseVote {
		t.Fatalf("phase = %q, attendu %q : les bots n'ont pas rendu la parole", state.Phase, protocol.PhaseVote)
	}
}
