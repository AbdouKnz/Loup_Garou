package game

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

// TestUnePartieSeTermineToujours joue des centaines de parties complètes avec
// des joueurs tirés au sort. C'est le garde-fou du moteur : quelle que soit la
// table, la partie doit finir, et jamais en laissant fuir un rôle.
func TestUnePartieSeTermineToujours(t *testing.T) {
	for seed := int64(1); seed <= 300; seed++ {
		n := 6 + int(seed%7) // de 6 à 12 joueurs
		c := newClock()
		// Le tirage des rôles et les décisions sont indépendants : deux sources
		// distinctes, toutes deux déterministes.
		roles := rand.New(rand.NewSource(seed))
		decisions := rand.New(rand.NewSource(seed * 7919))

		g, _, err := New(table(n), rolesConfig(), roles, c.now)
		if err != nil {
			t.Fatalf("graine %d : New : %v", seed, err)
		}

		for steps := 0; !g.Over(); steps++ {
			if steps > 400 {
				t.Fatalf("graine %d : la partie ne finit pas (jour %d, phase %s)", seed, g.Day(), g.Phase())
			}

			play(t, g, decisions)
			assertNoLeak(t, g, c.now)

			c.advance(g.cfg.Duration(g.Phase()))

			events, err := g.Apply(Command{Kind: CommandAdvance}, c.now)
			if err != nil {
				t.Fatalf("graine %d : avance en %s : %v", seed, g.Phase(), err)
			}
			for _, event := range events {
				if strings.TrimSpace(event.Text) == "" {
					t.Fatalf("graine %d : événement muet %+v", seed, event)
				}
			}
		}

		if g.Winner() != WinnerVillage && g.Winner() != WinnerWolves {
			t.Fatalf("graine %d : vainqueur inattendu %q", seed, g.Winner())
		}
	}
}

// play fait jouer la table au hasard, en respectant les limites du second tour
// et la veillée en cours.
func play(t *testing.T, g *Game, rng *rand.Rand) {
	t.Helper()

	switch g.phase {
	case PhaseNight:
		playNight(t, g, rng)

	case PhaseVote:
		alive := g.alive()
		for _, voter := range alive {
			target := pick(rng, g, g.preys(voter.id))
			if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter.id, Target: target}, time.Time{}); err != nil {
				t.Fatalf("vote de %s : %v", voter.id, err)
			}
		}
	}
}

// playNight fait jouer la veillée en cours, quelle qu'elle soit : les loups
// désignent leur proie, la voyante observe, la sorcière décide de ses potions.
func playNight(t *testing.T, g *Game, rng *rand.Rand) {
	t.Helper()

	switch g.currentStep() {
	case stepSeer:
		seer := g.livingWithRole(Voyante)[0]
		if target := pick(rng, g, g.preys(seer.id)); target != "" {
			act(t, g, seer.id, ActionSee, target)
		}

	case stepWolves:
		for _, wolf := range g.wolves() {
			if target := pick(rng, g, g.preys(wolf.id)); target != "" {
				if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf.id, Target: target}, time.Time{}); err != nil {
					t.Fatalf("vote de nuit de %s : %v", wolf.id, err)
				}
			}
		}

	case stepWitch:
		playWitch(t, g, rng)
	}
}

// playWitch décide à la place de la sorcière : elle sauve la proie d'un côté,
// empoisonne de l'autre, ou garde ses potions.
func playWitch(t *testing.T, g *Game, rng *rand.Rand) {
	t.Helper()

	witch := g.livingWithRole(Sorciere)[0]

	if rng.Intn(2) == 0 && !g.healUsed && g.wolfTarget != "" {
		act(t, g, witch.id, ActionHeal, "")
		return
	}

	if !g.poisonUsed {
		if target := pick(rng, g, g.preys(witch.id)); target != "" {
			act(t, g, witch.id, ActionPoison, target)
		}
	}
}

// act soumet une action de veillée et échoue si le moteur la refuse.
func act(t *testing.T, g *Game, actor string, action ActionKind, target string) {
	t.Helper()

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: actor, Action: action, Target: target}, time.Time{}); err != nil {
		t.Fatalf("action de %s : %v", actor, err)
	}
}

// preys liste les cibles possibles pour un votant : les joueurs vivants autres
// que lui, restreints aux candidats quand un second tour est en cours.
func (g *Game) preys(voter string) []string {
	allowed := func(id string) bool {
		if id == voter {
			return false
		}
		return len(g.candidates) == 0 || contains(g.candidates, id)
	}

	out := []string{}
	for _, p := range g.players {
		if p.alive && allowed(p.id) {
			out = append(out, p.id)
		}
	}

	return out
}

// pick choisit une cible au hasard, ou renonce si la liste est vide.
func pick(rng *rand.Rand, g *Game, targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	// Un joueur sur neuf s'abstient : les tours sans voix doivent rester
	// supportés.
	if rng.Intn(9) == 0 {
		return ""
	}

	return targets[rng.Intn(len(targets))]
}

// assertNoLeak vérifie qu'aucune vue ne trahit un rôle.
//
// C'est l'invariant de sécurité du moteur : seul un loup connaît les loups, et
// les rôles de tous n'apparaissent qu'une fois la partie finie.
func assertNoLeak(t *testing.T, g *Game, now time.Time) {
	t.Helper()

	for _, viewer := range g.players {
		view := g.Snapshot(viewer.id, now)
		isWolf := viewer.role == LoupGarou

		if view.YourRole != viewer.role {
			t.Fatalf("vue de %s : rôle %q, attendu %q", viewer.id, view.YourRole, viewer.role)
		}

		// L'invitation à agir n'est faite qu'à l'acteur de la veillée en cours.
		want := ActionKind("")
		if view.Phase == PhaseNight && viewer.alive {
			switch {
			case g.currentStep() == stepSeer && viewer.role == Voyante:
				want = ActionSee
			case g.currentStep() == stepWolves && viewer.role == LoupGarou:
				want = ActionDevour
			case g.currentStep() == stepWitch && viewer.role == Sorciere:
				want = ActionPotions
			}
		}
		if view.YourAction != want {
			t.Fatalf("vue de %s : invitation %q, attendue %q (veillée %s)",
				viewer.id, view.YourAction, want, g.currentStep())
		}

		// Chaque carnet privé reste chez son porteur, et seulement de son vivant.
		if view.YourVision != nil && (!viewer.alive || viewer.role != Voyante) {
			t.Fatalf("vue de %s : le carnet de la voyante fuit (%+v)", viewer.id, view.YourVision)
		}
		if (view.YourPotions != nil || view.WolfTarget != "") && (!viewer.alive || viewer.role != Sorciere) {
			t.Fatalf("vue de %s : les potions de la sorcière fuient (%+v %q)", viewer.id, view.YourPotions, view.WolfTarget)
		}
		if viewer.alive && viewer.role == Voyante && g.phase != PhaseOver {
			if len(view.YourVision) != len(g.vision[viewer.id]) {
				t.Fatalf("vue de %s : carnet incomplet (%+v)", viewer.id, view.YourVision)
			}
		}
		if view.WolfTarget != "" && g.currentStep() != stepWitch {
			t.Fatalf("vue de %s : la proie des loups sort hors de la veillée de la sorcière (%q)", viewer.id, view.WolfTarget)
		}

		// Les morts de la nuit ne sont annoncées qu'à l'aube.
		if view.Phase != PhaseDawn && view.Phase != PhaseOver && len(view.Deaths) > 0 {
			t.Fatalf("vue de %s : des morts annoncées en %s (%+v)", viewer.id, view.Phase, view.Deaths)
		}

		if !isWolf && (view.Wolves != nil || view.WolfVotes != nil) {
			t.Fatalf("vue de %s : l'antre des loups fuit (%v %v)", viewer.id, view.Wolves, view.WolfVotes)
		}
		// Le second tour de la nuit est une affaire de loups : le village ne
		// doit pas savoir que les loups se sont partagés, ni sur qui ils
		// hésitent.
		if view.Phase == PhaseNight && !isWolf {
			if view.Round != 1 || view.Tie || len(view.Candidates) != 0 || len(view.Voted) != 0 {
				t.Fatalf("vue de %s : le second tour de la nuit fuit (tour %d, égalité %v, candidats %v, votants %v)",
					viewer.id, view.Round, view.Tie, view.Candidates, view.Voted)
			}
		}
		// Le scrutin du jour est public, celui de la nuit ne sort jamais : même
		// un loup ne doit pas voir un bulletin nocturne.
		if view.Phase == PhaseNight && view.Ballot != nil {
			t.Fatalf("vue de %s : un scrutin apparaît la nuit (%v)", viewer.id, view.Ballot)
		}
		if g.Over() {
			continue
		}
		if view.Roles != nil {
			t.Fatalf("vue de %s : les rôles sont révélés en pleine partie", viewer.id)
		}
		if view.Winner != "" {
			t.Fatalf("vue de %s : un vainqueur en pleine partie (%q)", viewer.id, view.Winner)
		}
	}

	// Un spectateur n'obtient jamais rien de privé.
	spectator := g.Snapshot("spectateur", now)
	if spectator.YourRole != "" || spectator.Wolves != nil || spectator.Roles != nil || spectator.WolfVotes != nil {
		t.Fatal("la vue du spectateur n'est pas publique")
	}
	if spectator.YourAction != "" || spectator.YourVision != nil || spectator.YourPotions != nil || spectator.WolfTarget != "" {
		t.Fatal("la vue du spectateur contient un carnet de rôle")
	}
}
