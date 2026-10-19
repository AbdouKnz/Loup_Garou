package room

import (
	"slices"
	"strings"
	"testing"

	"github.com/abdou/loupgarou/internal/protocol"
)

// seatByRole retrouve le joueur qui se voit dans ce rôle.
func (tbl *table) seatByRole(t *testing.T, role string) *seat {
	t.Helper()

	seats := tbl.seatsByRole(t, role)

	return seats[0]
}

// seatsByRole liste les joueurs qui se voient dans ce rôle, dans l'ordre de la
// table.
func (tbl *table) seatsByRole(t *testing.T, role string) []*seat {
	t.Helper()

	out := []*seat{}
	for _, s := range tbl.seats {
		if s.role(t) == role {
			out = append(out, s)
		}
	}

	if len(out) == 0 {
		t.Fatalf("aucun joueur %q dans la table", role)
	}

	return out
}

func TestLaSalleAnnonceLaComposition(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())

	state := tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseLobby {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseLobby)
	}

	want := []string{
		protocol.RoleLoupGarou, protocol.RoleVoyante, protocol.RoleSorciere,
		protocol.RoleVillageois, protocol.RoleVillageois, protocol.RoleVillageois,
	}
	if !slices.Equal(state.Composition, want) {
		t.Fatalf("composition %v, attendue %v", state.Composition, want)
	}

	// Elle suit la taille de la table : un septième joueur amène un second loup.
	id, token, err := tbl.salle.Join(accountOf("Tardif"), "Tardif")
	if err != nil {
		t.Fatalf("entrée impossible : %v", err)
	}
	tbl.seats = append(tbl.seats, connect(t, tbl.salle, "Tardif", id, token))

	want = []string{
		protocol.RoleLoupGarou, protocol.RoleLoupGarou, protocol.RoleVoyante, protocol.RoleSorciere,
		protocol.RoleVillageois, protocol.RoleVillageois, protocol.RoleVillageois,
	}
	if got := tbl.seats[0].client.lastState(t).Composition; !slices.Equal(got, want) {
		t.Errorf("composition à sept %v, attendue %v", got, want)
	}
}

// TestLaVoyanteObserveParLaSalle suit la veillée de la voyante de bout en bout :
// la salle l'invite, recueille son observation, et ne la laisse pas sortir.
func TestLaVoyanteObserveParLaSalle(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	seer := tbl.seatByRole(t, protocol.RoleVoyante)
	wolf := tbl.wolves(t)[0]
	villager := tbl.seatByRole(t, protocol.RoleVillageois)

	// La nuit s'ouvre sur la voyante, et sur elle seule.
	if state := seer.client.lastState(t); state.Game.YourAction != protocol.ActionSee {
		t.Fatalf("invitation de la voyante : %q", state.Game.YourAction)
	}
	if state := villager.client.lastState(t); state.Game.YourAction != "" || state.Game.YourVision != nil {
		t.Errorf("un villageois est invité à observer : %+v", state.Game)
	}

	// Un autre rôle ne peut pas se faire passer pour elle.
	err := tbl.salle.NightAction(villager.id, protocol.ActionSee, string(wolf.id))
	if protocol.CodeOf(err) != protocol.CodeWrongAction {
		t.Errorf("observation d'un villageois : code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeWrongAction)
	}

	// Elle observe un loup : elle l'apprend, la table non.
	if err := tbl.salle.NightAction(seer.id, protocol.ActionSee, string(wolf.id)); err != nil {
		t.Fatalf("observation impossible : %v", err)
	}

	state := seer.client.lastState(t)
	if len(state.Game.YourVision) != 1 ||
		state.Game.YourVision[0].Role != protocol.RoleLoupGarou ||
		state.Game.YourVision[0].Target != string(wolf.id) {
		t.Fatalf("carnet de la voyante : %+v", state.Game.YourVision)
	}
	if state.Game.YourAction != "" {
		t.Errorf("la voyante est encore invitée à agir : %q", state.Game.YourAction)
	}

	for _, s := range tbl.seats {
		if s == seer {
			continue
		}
		if got := s.client.lastState(t).Game.YourVision; got != nil {
			t.Errorf("%s lit le carnet de la voyante : %+v", s.pseudo, got)
		}
	}
}

// TestLaSorciereSauveParLaSalle suit la veillée de la sorcière : elle reçoit la
// proie des loups, la sauve, et l'aube n'annonce aucune victime.
func TestLaSorciereSauveParLaSalle(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	witch := tbl.seatByRole(t, protocol.RoleSorciere)
	wolf := tbl.wolves(t)[0]
	prey := tbl.seatByRole(t, protocol.RoleVillageois)

	// La voyante passe son tour, les loups désignent leur proie.
	expireNow(t, tbl)
	if err := tbl.salle.NightVote(wolf.id, string(prey.id)); err != nil {
		t.Fatalf("vote du loup impossible : %v", err)
	}

	// La sorcière est invitée, et elle connaît la proie à juger.
	state := witch.client.lastState(t)
	if state.Game.YourAction != protocol.ActionPotions || state.Game.WolfTarget != string(prey.id) {
		t.Fatalf("invitation de la sorcière : %q, proie %q", state.Game.YourAction, state.Game.WolfTarget)
	}
	if state.Game.YourPotions == nil || !state.Game.YourPotions.Heal || !state.Game.YourPotions.Poison {
		t.Fatalf("potions de la sorcière : %+v", state.Game.YourPotions)
	}

	// La proie elle-même ignore qu'elle était visée, et personne d'autre ne voit
	// les potions.
	if got := prey.client.lastState(t).Game.WolfTarget; got != "" {
		t.Errorf("la proie connaît le choix des loups : %q", got)
	}
	for _, s := range tbl.seats {
		if s == witch {
			continue
		}
		if got := s.client.lastState(t).Game.YourPotions; got != nil {
			t.Errorf("%s voit les potions de la sorcière : %+v", s.pseudo, got)
		}
	}

	// Elle la sauve : la table apprend la sauvegarde, sans le nom de la proie.
	if err := tbl.salle.NightAction(witch.id, protocol.ActionHeal, ""); err != nil {
		t.Fatalf("potion de vie impossible : %v", err)
	}

	state = tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseDawn {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseDawn)
	}
	if len(state.Game.Deaths) != 0 || len(state.Game.Dead) != 0 {
		t.Fatalf("la proie sauvée est morte : %+v %v", state.Game.Deaths, state.Game.Dead)
	}

	phrase := "🌿 Les loups ont frappé, mais la victime a été sauvée."
	if !slices.Contains(narrations(t, tbl.seats[0].client), phrase) {
		t.Errorf("l'aube ne raconte pas la sauvegarde : %v", narrations(t, tbl.seats[0].client))
	}
}

// TestLaSorciereEmpoisonneParLaSalle : la potion de mort s'ajoute à la morsure,
// et l'aube annonce les deux morts avec leur cause.
func TestLaSorciereEmpoisonneParLaSalle(t *testing.T) {
	tbl := newTable(t, 6, gameOptions())
	tbl.start(t)

	witch := tbl.seatByRole(t, protocol.RoleSorciere)
	wolf := tbl.wolves(t)[0]
	villagers := tbl.seatsByRole(t, protocol.RoleVillageois)
	prey, poisoned := villagers[0], villagers[1]

	expireNow(t, tbl)
	if err := tbl.salle.NightVote(wolf.id, string(prey.id)); err != nil {
		t.Fatalf("vote du loup impossible : %v", err)
	}
	if err := tbl.salle.NightAction(witch.id, protocol.ActionPoison, string(poisoned.id)); err != nil {
		t.Fatalf("potion de mort impossible : %v", err)
	}

	state := tbl.seats[0].client.lastState(t)
	if state.Phase != protocol.PhaseDawn {
		t.Fatalf("phase = %q, attendu %q", state.Phase, protocol.PhaseDawn)
	}

	want := []protocol.DeathView{
		{Target: string(prey.id), Cause: protocol.CauseWolves},
		{Target: string(poisoned.id), Cause: protocol.CausePoison},
	}
	if !slices.Equal(state.Game.Deaths, want) {
		t.Fatalf("morts %+v, attendu %+v", state.Game.Deaths, want)
	}

	// Chaque mort a sa phrase, et aucune ne dit son rôle.
	phrases := narrations(t, tbl.seats[0].client)
	for _, phrase := range phrases {
		if strings.Contains(phrase, "c'était") {
			t.Errorf("une mort révèle son rôle : %q", phrase)
		}
	}
	if !containsPhrase(phrases, prey.pseudo, "dévoré") {
		t.Errorf("la morsure n'est pas racontée : %v", phrases)
	}
	if !containsPhrase(phrases, poisoned.pseudo, "étranges circonstances") {
		t.Errorf("le poison n'est pas raconté : %v", phrases)
	}
}

// containsPhrase dit si une des phrases contient tous les fragments.
func containsPhrase(phrases []string, fragments ...string) bool {
	for _, phrase := range phrases {
		ok := true
		for _, fragment := range fragments {
			if !strings.Contains(phrase, fragment) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}

	return false
}
