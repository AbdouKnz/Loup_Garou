package room

import (
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/protocol"
)

// TestLaDistributionMetLaPartieEnPause vérifie le temps d'arrêt du début de
// partie : la table découvre son rôle, personne n'a rien à jouer, et le moteur
// seul décide quand la nuit tombe.
func TestLaDistributionMetLaPartieEnPause(t *testing.T) {
	opts := gameOptions()
	opts.Game.Roles = time.Second

	tbl := newTable(t, 6, opts)

	if err := tbl.salle.StartGame(tbl.host); err != nil {
		t.Fatalf("lancement impossible : %v", err)
	}

	state := tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseRoles {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseRoles)
	}
	if state.Game == nil {
		t.Fatal("aucune partie dans l'instantané")
	}
	if state.Game.Day != 1 || state.Game.Round != 1 {
		t.Errorf("jour %d tour %d, attendu 1 et 1", state.Game.Day, state.Game.Round)
	}

	// Chacun connaît déjà son rôle, et n'a rien à jouer.
	for _, s := range tbl.seats {
		vue := s.client.lastState(t).Game
		if vue == nil || vue.YourRole == "" {
			t.Fatalf("%s ne connaît pas son rôle pendant la distribution", s.pseudo)
		}
		if vue.YourAction != "" {
			t.Errorf("%s a une action à jouer pendant la distribution : %q", s.pseudo, vue.YourAction)
		}
	}

	// Le narrateur a annoncé la distribution.
	if phrases := narrations(t, tbl.seats[0].client); len(phrases) == 0 {
		t.Error("le narrateur n'a rien dit à la distribution")
	}

	// Ni vote, ni parole : la partie n'a pas commencé.
	if err := tbl.salle.NightVote(tbl.host, string(tbl.seats[1].id)); protocol.CodeOf(err) != protocol.CodeWrongPhase {
		t.Errorf("vote pendant la distribution : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongPhase)
	}
	if err := tbl.salle.Chat(tbl.host, "coucou"); protocol.CodeOf(err) != protocol.CodeWrongPhase {
		t.Errorf("chat pendant la distribution : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongPhase)
	}

	// Rien ne se referme avant l'échéance : la distribution va jusqu'au bout.
	if phase := tbl.seats[0].client.lastState(t).Phase; phase != protocol.PhaseRoles {
		t.Fatalf("la distribution s'est refermée trop tôt : %q", phase)
	}

	tbl.crossDistribution(t)

	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseNight {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseNight)
	}
}

// TestLaDistributionNEstPasEcourteeParLesBots : la vague des bots ne doit pas
// refermer la distribution, même si celle-ci n'attend personne.
func TestLaDistributionNEstPasEcourteeParLesBots(t *testing.T) {
	tbl := botTable(t)

	if err := tbl.salle.AddBots(tbl.host, 5); err != nil {
		t.Fatalf("ajout de bots impossible : %v", err)
	}
	if err := tbl.salle.StartGame(tbl.host); err != nil {
		t.Fatalf("lancement impossible : %v", err)
	}

	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseRoles {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseRoles)
	}

	// La vague passe pendant la distribution : elle y a un rôle à connaître — le
	// sien — mais rien à jouer. Elle ne doit pas la refermer pour autant.
	if err := tbl.salle.botTurn(time.Now()); err != nil {
		t.Fatalf("vague de bots impossible : %v", err)
	}

	if state := tbl.seats[0].client.lastState(t); state.Phase != protocol.PhaseRoles {
		t.Fatalf("la vague a refermé la distribution : %q", state.Phase)
	}
}
