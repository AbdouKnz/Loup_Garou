package game

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"
)

// base est l'instant de départ des tests : fixe, donc reproductible.
var base = time.Date(2026, time.January, 2, 20, 0, 0, 0, time.UTC)

// clock est l'horloge des tests : le moteur ne lit jamais l'heure lui-même, on
// la lui présente.
type clock struct{ now time.Time }

func newClock() *clock { return &clock{now: base} }

func (c *clock) advance(d time.Duration) { c.now = c.now.Add(d) }

// testConfig raccourcit les phases : ce sont les règles qu'on teste, pas la
// patience. Il ne distribue aucun rôle spécial : les tests du village contre les
// loups restent lisibles.
func testConfig() Config {
	return Config{
		MinPlayers:   6,
		MaxPlayers:   12,
		Night:        10 * time.Second,
		NightStep:    5 * time.Second,
		Reveal:       time.Second,
		Speak:        30 * time.Second,
		Vote:         20 * time.Second,
		SpecialRoles: []Role{},
	}
}

// rolesConfig est la partie telle qu'elle se joue par défaut : la voyante et la
// sorcière se lèvent en plus des loups.
func rolesConfig() Config {
	cfg := testConfig()
	cfg.SpecialRoles = []Role{Voyante, Sorciere}

	return cfg
}

// table fabrique n joueurs numérotés.
func table(n int) []Player {
	out := make([]Player, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, Player{ID: pID(i), Name: pName(i)})
	}

	return out
}

func pID(i int) string { return fmt.Sprintf("p%d", i) }

func pName(i int) string { return fmt.Sprintf("Joueur%d", i) }

// deadOf renvoie les identifiants des morts.
func deadOf(g *Game) []string {
	out := []string{}
	for _, p := range g.players {
		if !p.alive {
			out = append(out, p.id)
		}
	}

	return out
}

// newGame lance une partie de n joueurs sur une horloge de test.
func newGame(t *testing.T, n int) (*Game, *clock) {
	t.Helper()

	return newGameWith(t, n, testConfig())
}

// newGameWith lance une partie avec la configuration donnée, distribution des
// rôles traversée : la partie commence sur la première nuit.
func newGameWith(t *testing.T, n int, cfg Config) (*Game, *clock) {
	t.Helper()

	c := newClock()
	g, _, err := New(table(n), cfg, rand.New(rand.NewSource(int64(n))), c.now)
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	crossDistribution(t, g, c)

	return g, c
}

// crossDistribution traverse la distribution des rôles en poussant l'horloge à
// son échéance : la partie est ensuite dans sa première nuit.
func crossDistribution(t *testing.T, g *Game, c *clock) {
	t.Helper()

	if g.Phase() != PhaseRoles {
		t.Fatalf("la partie ne s'ouvre pas sur la distribution : %s", g.Phase())
	}

	c.advance(g.cfg.Duration(PhaseRoles))

	if _, err := g.Apply(Command{Kind: CommandAdvance}, c.now); err != nil {
		t.Fatalf("avance pendant la distribution : %v", err)
	}
}

// roleOf renvoie le joueur qui porte ce rôle, vide s'il n'y en a pas.
func roleOf(g *Game, role Role) string {
	for _, p := range g.players {
		if p.role == role {
			return p.id
		}
	}

	return ""
}

// step clôt la phase en cours en poussant l'horloge à l'échéance.
func step(t *testing.T, g *Game, c *clock) []Event {
	t.Helper()

	if g.Over() {
		t.Fatalf("la partie est déjà finie")
	}

	c.advance(g.cfg.Duration(g.Phase()))

	events, err := g.Apply(Command{Kind: CommandAdvance}, c.now)
	if err != nil {
		t.Fatalf("Apply(advance) en %s : %v", g.Phase(), err)
	}

	return events
}

// goTo fait avancer la partie jusqu'à la phase voulue.
func goTo(t *testing.T, g *Game, c *clock, want Phase) {
	t.Helper()

	for i := 0; i < 40; i++ {
		if g.Phase() == want {
			return
		}
		if g.Over() {
			t.Fatalf("la partie s'est terminée avant d'atteindre %s", want)
		}
		step(t, g, c)
	}

	t.Fatalf("impossible d'atteindre la phase %s (bloqué en %s)", want, g.Phase())
}

// wolvesOf renvoie les identifiants des loups.
func wolvesOf(g *Game) []string {
	out := []string{}
	for _, p := range g.players {
		if p.role == LoupGarou {
			out = append(out, p.id)
		}
	}

	return out
}

// villagersOf renvoie les identifiants des villageois.
func villagersOf(g *Game) []string {
	out := []string{}
	for _, p := range g.players {
		if p.role == Villageois {
			out = append(out, p.id)
		}
	}

	return out
}

func mustRefuse(t *testing.T, err error, want Reason, what string) {
	t.Helper()

	if err == nil {
		t.Fatalf("%s : le refus attendu n'est pas venu", what)
	}
	if got := ReasonOf(err); got != want {
		t.Fatalf("%s : motif %q, attendu %q (%v)", what, got, want, err)
	}
}

func TestNewRefuseUneTableHorsBornes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	_, _, err := New(table(5), testConfig(), rng, base)
	mustRefuse(t, err, ReasonTooFewPlayers, "cinq joueurs")

	_, _, err = New(table(13), testConfig(), rng, base)
	mustRefuse(t, err, ReasonTooManyPlayers, "treize joueurs")
}

func TestNewRefuseDesIdentifiantsEnDouble(t *testing.T) {
	players := table(6)
	players[1].ID = players[0].ID

	_, _, err := New(players, testConfig(), rand.New(rand.NewSource(1)), base)
	mustRefuse(t, err, ReasonBadConfig, "identifiants en double")
}

func TestRepartitionDesLoupsSelonLaTaille(t *testing.T) {
	cases := map[int]int{6: 1, 7: 2, 8: 2, 9: 2, 10: 3, 11: 3, 12: 3}

	for n, want := range cases {
		g, _ := newGame(t, n)
		if got := len(wolvesOf(g)); got != want {
			t.Errorf("%d joueurs : %d loups, attendu %d", n, got, want)
		}
		if len(villagersOf(g)) <= want {
			t.Errorf("%d joueurs : les loups sont déjà majoritaires au départ", n)
		}
	}
}

func TestWithDefaultsConserveUneTableSansRoleSpecial(t *testing.T) {
	// Nil demande la composition par défaut ; une liste vide n'en distribue
	// aucun — et la redemander ne doit pas la réveiller.
	cfg := testConfig().WithDefaults().WithDefaults()

	if cfg.SpecialRoles == nil {
		t.Fatal("une table sans rôle spécial est redevenue la table par défaut")
	}
	if len(cfg.SpecialRoles) != 0 {
		t.Fatalf("rôles spéciaux = %v, attendu aucun", cfg.SpecialRoles)
	}
	if got := cfg.CompositionFor(6); len(got) != 6 || containsRole(got, Voyante) {
		t.Fatalf("composition = %v, attendu six rôles sans voyante", got)
	}
}

func TestLaPartieOuvreSurLaDistributionDesRoles(t *testing.T) {
	c := newClock()
	cfg := testConfig().WithDefaults()

	g, events, err := New(table(6), cfg, rand.New(rand.NewSource(6)), c.now)
	if err != nil {
		t.Fatalf("New : %v", err)
	}

	if g.Phase() != PhaseRoles {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseRoles)
	}
	if g.Day() != 1 {
		t.Fatalf("jour %d, attendu 1", g.Day())
	}
	if want := c.now.Add(cfg.Roles); !g.Deadline().Equal(want) {
		t.Fatalf("échéance %s, attendu %s", g.Deadline(), want)
	}
	if len(events) != 1 || strings.TrimSpace(events[0].Text) == "" {
		t.Fatalf("événements d'ouverture = %+v", events)
	}

	// La distribution n'appelle ni vote ni action : il n'y a qu'à lire.
	if pending := g.Pending(); len(pending) != 0 {
		t.Fatalf("la distribution n'attend personne, or Pending = %v", pending)
	}
	for _, p := range g.players {
		vue := g.Snapshot(p.id, c.now)
		if vue.YourRole != p.role {
			t.Fatalf("vue de %s : rôle %q, attendu %q", p.id, vue.YourRole, p.role)
		}
		if vue.YourAction != "" {
			t.Fatalf("vue de %s : action %q pendant la distribution", p.id, vue.YourAction)
		}
	}

	// Ni vote, ni avance prématurée : la table a le temps de lire.
	_, err = g.Apply(Command{Kind: CommandWolfVote, Actor: wolvesOf(g)[0], Target: villagersOf(g)[0]}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "vote pendant la distribution")

	_, err = g.Apply(Command{Kind: CommandAdvance}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "avance avant l'échéance de la distribution")

	if g.Phase() != PhaseRoles {
		t.Fatalf("la distribution s'est refermée trop tôt : %s", g.Phase())
	}

	c.advance(cfg.Roles)
	if _, err := g.Apply(Command{Kind: CommandAdvance}, c.now); err != nil {
		t.Fatalf("avance en %s : %v", g.Phase(), err)
	}

	if g.Phase() != PhaseNight {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseNight)
	}
	// La nuit a son échéance pleine : la distribution ne lui a rien pris.
	if want := c.now.Add(cfg.Night); !g.Deadline().Equal(want) {
		t.Fatalf("échéance de la nuit %s, attendu %s", g.Deadline(), want)
	}
}

func TestLaPartieOuvreSurLaPremiereNuit(t *testing.T) {
	g, c := newGame(t, 6)

	if g.Phase() != PhaseNight {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseNight)
	}
	if g.Day() != 1 {
		t.Fatalf("jour %d, attendu 1", g.Day())
	}
	if want := c.now.Add(testConfig().Night); !g.Deadline().Equal(want) {
		t.Fatalf("échéance %s, attendu %s", g.Deadline(), want)
	}
	if g.Over() {
		t.Fatal("la partie est finie avant d'avoir commencé")
	}
}

func TestOnNAdvancePasAvantLEcheance(t *testing.T) {
	g, c := newGame(t, 6)

	_, err := g.Apply(Command{Kind: CommandAdvance}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "avance immédiate")

	if g.Phase() != PhaseNight {
		t.Fatalf("la phase a changé malgré le refus : %s", g.Phase())
	}
}

func TestLesVillageoisNeDevoientPasLaNuit(t *testing.T) {
	g, _ := newGame(t, 6)
	villager := villagersOf(g)[0]

	_, err := g.Apply(Command{Kind: CommandWolfVote, Actor: villager, Target: villagersOf(g)[1]}, base)
	mustRefuse(t, err, ReasonNotWolf, "vote de nuit d'un villageois")

	if len(g.votes) != 0 {
		t.Fatalf("un vote a été enregistré malgré le refus : %v", g.votes)
	}
}

func TestOnNeVotePasHorsPhase(t *testing.T) {
	g, _ := newGame(t, 6)

	_, err := g.Apply(Command{Kind: CommandVillageVote, Actor: villagersOf(g)[0], Target: villagersOf(g)[1]}, base)
	mustRefuse(t, err, ReasonWrongPhase, "vote du village pendant la nuit")
}

func TestUnInconnuNePeutPasVoter(t *testing.T) {
	g, _ := newGame(t, 6)

	_, err := g.Apply(Command{Kind: CommandWolfVote, Actor: "fantome", Target: villagersOf(g)[0]}, base)
	mustRefuse(t, err, ReasonUnknownPlayer, "vote d'un inconnu")
}

func TestUneCibleInconnueEstRefusee(t *testing.T) {
	g, _ := newGame(t, 6)
	wolf := wolvesOf(g)[0]

	_, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: "fantome"}, base)
	mustRefuse(t, err, ReasonBadTarget, "cible inconnue")

	_, err = g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: wolf}, base)
	if err != nil {
		t.Fatalf("un loup devrait pouvoir se désigner lui-même : %v", err)
	}
}

func TestLaNuitDeVoreUneVictime(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	victim := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victim}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}

	events := step(t, g, c)

	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}
	if len(deadOf(g)) != 1 || deadOf(g)[0] != victim {
		t.Fatalf("morts %v, attendu [%s]", deadOf(g), victim)
	}

	var announced bool
	for _, event := range events {
		if event.Kind == EventVictim {
			announced = true
			if event.Target != victim {
				t.Errorf("victime annoncée %q, attendu %q", event.Target, victim)
			}
			if strings.Contains(strings.ToLower(event.Text), "loups ont gagné") {
				t.Errorf("le texte annonce la fin de la partie : %q", event.Text)
			}
		}
	}
	if !announced {
		t.Fatalf("aucun événement de victime : %+v", events)
	}
}

func TestLaVictimeDeLaNuitNeRevelePasSonRole(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	victim := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victim}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}
	events := step(t, g, c)

	for _, event := range events {
		if event.Kind == EventVictim {
			if event.Role != "" {
				t.Errorf("la victime part avec son rôle : %q", event.Role)
			}
			if strings.Contains(strings.ToLower(event.Text), "c'était") {
				t.Errorf("le texte révèle le rôle : %q", event.Text)
			}
		}
	}

	// Et la vue publique ne dit rien de plus.
	view := g.Snapshot(victim, c.now)
	if view.Roles != nil {
		t.Error("les rôles sont révélés avant la fin de la partie")
	}
	if view.EliminatedRole != "" {
		t.Error("un rôle d'éliminé traîne dans la vue")
	}
	if view.YourRole != Villageois {
		t.Errorf("la victime ne voit plus son rôle : %q", view.YourRole)
	}
}

func TestLesMortsNeVotentPlus(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	victim := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victim}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}
	step(t, g, c)
	goTo(t, g, c, PhaseVote)

	_, err := g.Apply(Command{Kind: CommandVillageVote, Actor: victim, Target: wolf}, c.now)
	mustRefuse(t, err, ReasonDead, "vote d'un mort")
}

func TestLeVillageElimineEtReveleLeRole(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	target := villagersOf(g)[0]

	goTo(t, g, c, PhaseVote)

	// Le village entier se range derrière une seule voix : la majorité suffit.
	for _, voter := range g.alive() {
		if voter.id == target {
			continue
		}
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter.id, Target: target}, c.now); err != nil {
			t.Fatalf("vote de %s : %v", voter.id, err)
		}
	}

	events := step(t, g, c)

	if g.Phase() != PhaseVerdict {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseVerdict)
	}

	var announced bool
	for _, event := range events {
		if event.Kind == EventEliminated {
			announced = true
			if event.Target != target {
				t.Errorf("éliminé %q, attendu %q", event.Target, target)
			}
			if event.Role != Villageois {
				t.Errorf("rôle annoncé %q, attendu %q", event.Role, Villageois)
			}
			if !strings.Contains(event.Text, string(Villageois)) {
				t.Errorf("le texte ne dit pas le rôle : %q", event.Text)
			}
		}
	}
	if !announced {
		t.Fatalf("aucun événement d'élimination : %+v", events)
	}

	view := g.Snapshot(wolf, c.now)
	if view.Eliminated != target || view.EliminatedRole != Villageois {
		t.Errorf("verdict dans la vue : %q %q", view.Eliminated, view.EliminatedRole)
	}
}

func TestUnTourSansAucuneVoixNFaitRien(t *testing.T) {
	g, c := newGame(t, 6)

	goTo(t, g, c, PhaseVote)
	events := step(t, g, c)

	for _, event := range events {
		if event.Kind == EventEliminated {
			t.Fatalf("le village a éliminé quelqu'un sans qu'aucune voix ne soit exprimée")
		}
	}

	if len(deadOf(g)) != 0 {
		t.Fatalf("des morts sans vote : %v", deadOf(g))
	}
	if g.Phase() != PhaseVerdict {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseVerdict)
	}
}

// TestLaNuitSeDerouleParVeillees decrit la charpente de la nuit : elle n'est pas
// un bloc, mais une suite de veillées, une par rôle qui se réveille. Chacun son
// tour, et personne d'autre.
func TestLaNuitSeDerouleParVeillees(t *testing.T) {
	g, c := newGameWith(t, 6, rolesConfig())

	if g.Phase() != PhaseNight {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseNight)
	}
	if want := []stepKind{stepSeer, stepWolves, stepWitch}; !slices.Equal(g.steps, want) {
		t.Fatalf("veillées %v, attendu %v", g.steps, want)
	}

	seer, witch := roleOf(g, Voyante), roleOf(g, Sorciere)
	wolf := wolvesOf(g)[0]

	// La voyante se lève la première, et elle seule est attendue : la nuit est
	// une suite de tête-à-tête, jamais une table ronde.
	if got := g.Pending(); len(got) != 1 || got[0] != seer {
		t.Fatalf("attente %v, attendu la voyante %s", got, seer)
	}
	if view := g.Snapshot(seer, c.now); view.YourAction != ActionSee {
		t.Errorf("la voyante ne sait pas qu'on l'attend : %q", view.YourAction)
	}
	if view := g.Snapshot(wolf, c.now); view.YourAction != "" {
		t.Errorf("un loup est attendu pendant la veillée de la voyante : %q", view.YourAction)
	}

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: seer, Action: ActionSee, Target: wolf}, c.now); err != nil {
		t.Fatalf("observation de la voyante : %v", err)
	}
	step(t, g, c)

	// Puis les loups.
	if g.currentStep() != stepWolves {
		t.Fatalf("veillée %s, attendu %s", g.currentStep(), stepWolves)
	}
	if view := g.Snapshot(wolf, c.now); view.YourAction != ActionDevour {
		t.Errorf("le loup ne sait pas qu'on l'attend : %q", view.YourAction)
	}
	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: seer}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}
	step(t, g, c)

	// Enfin la sorcière, qui connaît la proie à juger.
	if g.currentStep() != stepWitch {
		t.Fatalf("veillée %s, attendu %s", g.currentStep(), stepWitch)
	}
	view := g.Snapshot(witch, c.now)
	if view.YourAction != ActionPotions {
		t.Fatalf("la sorcière ne sait pas qu'on l'attend : %q", view.YourAction)
	}
	if view.WolfTarget != seer {
		t.Fatalf("proie annoncée %q, attendue %q", view.WolfTarget, seer)
	}
	if view.YourPotions == nil || !view.YourPotions.Heal || !view.YourPotions.Poison {
		t.Fatalf("potions de la sorcière : %+v", view.YourPotions)
	}

	// Elle renonce : la nuit se referme, la proie y reste.
	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: witch, Action: ActionPass}, c.now); err != nil {
		t.Fatalf("passage de la sorcière : %v", err)
	}
	step(t, g, c)

	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}
	if dead := deadOf(g); len(dead) != 1 || dead[0] != seer {
		t.Fatalf("morts %v, attendu [%s]", dead, seer)
	}
}

// TestLaNuitNEveilleQueLesRolesVivants : une veillée n'a lieu que s'il reste
// quelqu'un pour s'y lever, et de quoi y décider.
func TestLaNuitNEveilleQueLesRolesVivants(t *testing.T) {
	g, _ := newGameWith(t, 6, rolesConfig())

	if got := g.nightPlan(); !slices.Equal(got, []stepKind{stepSeer, stepWolves, stepWitch}) {
		t.Fatalf("veillées %v, attendu les trois rôles", got)
	}

	// Une voyante morte ne se lève plus.
	g.kill(roleOf(g, Voyante))
	if got := g.nightPlan(); !slices.Equal(got, []stepKind{stepWolves, stepWitch}) {
		t.Errorf("voyante morte : veillées %v", got)
	}

	// Une sorcière sans potion non plus : sa veillée n'aurait rien à décider.
	g.healUsed, g.poisonUsed = true, true
	if got := g.nightPlan(); !slices.Equal(got, []stepKind{stepWolves}) {
		t.Errorf("sorcière sans potion : veillées %v", got)
	}
}

// TestLaVoyanteDecouvreUnRoleEtLeGardePourElle : elle observe un joueur, elle
// en apprend le rôle, et ce carnet ne sort jamais de sa vue.
func TestLaVoyanteDecouvreUnRoleEtLeGardePourElle(t *testing.T) {
	g, c := newGameWith(t, 6, rolesConfig())
	seer := roleOf(g, Voyante)
	wolf := wolvesOf(g)[0]
	other := villagersOf(g)[0]

	// Un villageois ne peut pas se faire passer pour elle.
	_, err := g.Apply(Command{Kind: CommandNightAction, Actor: other, Action: ActionSee, Target: wolf}, c.now)
	mustRefuse(t, err, ReasonNotSeer, "observation d'un villageois")

	// Elle ne s'observe pas elle-même : ce serait gâcher sa nuit.
	_, err = g.Apply(Command{Kind: CommandNightAction, Actor: seer, Action: ActionSee, Target: seer}, c.now)
	mustRefuse(t, err, ReasonBadTarget, "la voyante s'observe elle-même")

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: seer, Action: ActionSee, Target: wolf}, c.now); err != nil {
		t.Fatalf("observation de la voyante : %v", err)
	}

	if len(g.Pending()) != 0 {
		t.Errorf("la voyante est encore attendue : %v", g.Pending())
	}

	// Son carnet : ce qu'elle a vu, et quand.
	view := g.Snapshot(seer, c.now)
	want := Observation{Day: 1, Target: wolf, Role: LoupGarou}
	if len(view.YourVision) != 1 || view.YourVision[0] != want {
		t.Fatalf("carnet de la voyante %+v, attendu [%+v]", view.YourVision, want)
	}

	// Et personne d'autre n'y a accès.
	for _, p := range g.players {
		if p.id == seer {
			continue
		}
		if other := g.Snapshot(p.id, c.now); len(other.YourVision) != 0 {
			t.Errorf("%s lit le carnet de la voyante : %+v", p.id, other.YourVision)
		}
	}
}

// TestLaSorciereSauveLaProieDesLoups : la potion de vie annule la morsure, et
// l'aube le raconte sans nommer la proie sauvée.
func TestLaSorciereSauveLaProieDesLoups(t *testing.T) {
	g, c := newGameWith(t, 6, rolesConfig())
	seer, witch := roleOf(g, Voyante), roleOf(g, Sorciere)
	wolf := wolvesOf(g)[0]
	prey := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: seer, Action: ActionSee, Target: prey}, c.now); err != nil {
		t.Fatalf("observation de la voyante : %v", err)
	}
	step(t, g, c)

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: prey}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}
	step(t, g, c)

	if g.currentStep() != stepWitch {
		t.Fatalf("veillée %s, attendu %s", g.currentStep(), stepWitch)
	}

	// La sorcière voit la proie ; les loups ne voient pas ce qu'elle sait.
	if view := g.Snapshot(witch, c.now); view.WolfTarget != prey {
		t.Fatalf("la sorcière ne voit pas la proie : %q", view.WolfTarget)
	}
	if view := g.Snapshot(wolf, c.now); view.WolfTarget != "" {
		t.Errorf("un loup voit la proie révélée à la sorcière : %q", view.WolfTarget)
	}

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: witch, Action: ActionHeal}, c.now); err != nil {
		t.Fatalf("potion de vie : %v", err)
	}

	events := step(t, g, c)

	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}
	if dead := deadOf(g); len(dead) != 0 {
		t.Fatalf("la proie sauvée est morte : %v", dead)
	}
	if deaths := g.Snapshot(witch, c.now).Deaths; len(deaths) != 0 {
		t.Errorf("des morts annoncées : %v", deaths)
	}

	saved := false
	for _, event := range events {
		switch event.Kind {
		case EventSaved:
			saved = true
			if !strings.Contains(event.Text, "sauvée") {
				t.Errorf("le texte ne dit pas la sauvegarde : %q", event.Text)
			}
		case EventVictim:
			t.Errorf("victime annoncée alors que la proie a été sauvée : %+v", event)
		}
	}
	if !saved {
		t.Fatalf("aucune annonce de sauvegarde : %+v", events)
	}
}

// TestLaSorciereEmpoisonneUnJoueur : la potion de mort s'ajoute à la morsure ;
// chaque mort est annoncée avec sa cause, jamais avec son rôle.
func TestLaSorciereEmpoisonneUnJoueur(t *testing.T) {
	g, c := newGameWith(t, 6, rolesConfig())
	seer, witch := roleOf(g, Voyante), roleOf(g, Sorciere)
	wolf := wolvesOf(g)[0]
	prey, poisoned := villagersOf(g)[0], villagersOf(g)[1]

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: seer, Action: ActionSee, Target: poisoned}, c.now); err != nil {
		t.Fatalf("observation de la voyante : %v", err)
	}
	step(t, g, c)

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: prey}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}
	step(t, g, c)

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: witch, Action: ActionPoison, Target: poisoned}, c.now); err != nil {
		t.Fatalf("potion de mort : %v", err)
	}

	events := step(t, g, c)

	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}

	want := []Death{{Target: prey, Cause: DeathWolves}, {Target: poisoned, Cause: DeathPoison}}
	if !slices.Equal(g.deaths, want) {
		t.Fatalf("morts %+v, attendu %+v", g.deaths, want)
	}

	dead := deadOf(g)
	slices.Sort(dead)
	expected := []string{prey, poisoned}
	slices.Sort(expected)
	if !slices.Equal(dead, expected) {
		t.Fatalf("morts %v, attendu %v", dead, expected)
	}

	// Chaque mort a sa phrase, et aucune ne révèle un rôle.
	texts := map[DeathCause]string{}
	for _, event := range events {
		if event.Kind != EventVictim {
			continue
		}
		if event.Role != "" {
			t.Errorf("la victime part avec son rôle : %q", event.Role)
		}
		texts[event.Cause] = event.Text
	}
	if len(texts) != 2 || texts[DeathWolves] == "" || texts[DeathPoison] == "" {
		t.Fatalf("morts annoncées sans leur cause : %+v", texts)
	}
	if texts[DeathWolves] == texts[DeathPoison] {
		t.Errorf("les deux morts portent la même phrase : %q", texts[DeathWolves])
	}
}

// TestLaSorciereNeSeSertQUnePotionParNuit : sa veillée ne vaut qu'une décision.
// Celle qu'elle n'a pas prise lui reste pour une autre nuit.
func TestLaSorciereNeSeSertQUnePotionParNuit(t *testing.T) {
	g, c := newGameWith(t, 6, rolesConfig())
	seer, witch := roleOf(g, Voyante), roleOf(g, Sorciere)
	wolf := wolvesOf(g)[0]
	prey := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: seer, Action: ActionSee, Target: prey}, c.now); err != nil {
		t.Fatalf("observation de la voyante : %v", err)
	}
	step(t, g, c)

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: prey}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}
	step(t, g, c)

	if _, err := g.Apply(Command{Kind: CommandNightAction, Actor: witch, Action: ActionHeal}, c.now); err != nil {
		t.Fatalf("potion de vie : %v", err)
	}

	// Elle a parlé : la potion de mort devra attendre une autre nuit.
	_, err := g.Apply(Command{Kind: CommandNightAction, Actor: witch, Action: ActionPoison, Target: prey}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "seconde potion dans la même nuit")

	if !g.healUsed || g.poisonUsed {
		t.Fatalf("potions consommées : soin %v, poison %v", g.healUsed, g.poisonUsed)
	}
	if potions := g.Snapshot(witch, c.now).YourPotions; potions == nil || potions.Heal || !potions.Poison {
		t.Fatalf("potions restantes : %+v", potions)
	}

	// Et sans potion de vie, la potion de mort ne ressuscite personne.
	_, err = g.Apply(Command{Kind: CommandNightAction, Actor: witch, Action: ActionHeal}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "seconde potion de vie")
}

// TestLaCompositionAnnonceeEstCelleDistribuee : la table sait ce qui l'attend,
// et reçoit exactement cela. Seules les places sont tirées au sort.
func TestLaCompositionAnnonceeEstCelleDistribuee(t *testing.T) {
	cfg := DefaultConfig()

	for _, n := range []int{6, 7, 9, 10, 12} {
		want := cfg.CompositionFor(n)
		if len(want) != n {
			t.Fatalf("%d joueurs : composition %v", n, want)
		}

		g, _, err := New(table(n), cfg, rand.New(rand.NewSource(int64(n))), base)
		if err != nil {
			t.Fatalf("%d joueurs : New : %v", n, err)
		}

		got := make([]Role, 0, n)
		for _, p := range g.players {
			got = append(got, p.role)
		}
		slices.Sort(got)
		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Errorf("%d joueurs : rôles %v, attendu %v", n, got, want)
		}
		if wolves := len(wolvesOf(g)); wolves != cfg.WolvesFor(n) {
			t.Errorf("%d joueurs : %d loups, attendu %d", n, wolves, cfg.WolvesFor(n))
		}
	}
}

// TestLaCompositionGardeToujoursUnVillageois : une table réduite ne peut pas
// caser tous les rôles ; le village garde alors sa place, sans quoi il n'y
// aurait plus de camp majoritaire à défendre.
func TestLaCompositionGardeToujoursUnVillageois(t *testing.T) {
	cfg := DefaultConfig()
	for n := 1; n <= 3; n++ {
		roles := cfg.CompositionFor(n)
		if len(roles) != n {
			t.Fatalf("%d joueurs : composition %v", n, roles)
		}
		if !containsRole(roles, Villageois) {
			t.Errorf("%d joueurs : plus un seul villageois dans %v", n, roles)
		}
	}
}

// TestUnVoteDeLoupsNeTuePasEncore : la proie désignée par les loups n'est
// retirée qu'à la fin de la nuit, une fois toutes les veillées passées. C'est
// ce délai qui permettra aux veillées suivantes d'intervenir.
func TestUnVoteDeLoupsNeTuePasEncore(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	victim := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victim}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}

	// Le loup est seul : sa voix suffit. La proie est désignée, pas encore morte.
	if len(g.Pending()) != 0 {
		t.Fatalf("les loups n'ont pas fini de voter : %v", g.Pending())
	}
	if got := g.votes[wolf]; got != victim {
		t.Fatalf("voix retenue %q, attendue %q", got, victim)
	}
	if len(deadOf(g)) != 0 {
		t.Fatalf("un mort avant la fin de la nuit : %v", deadOf(g))
	}

	events := step(t, g, c)

	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}
	if dead := deadOf(g); len(dead) != 1 || dead[0] != victim {
		t.Fatalf("morts %v, attendu [%s]", dead, victim)
	}

	var announced bool
	for _, event := range events {
		if event.Kind == EventVictim && event.Target == victim {
			announced = true
		}
	}
	if !announced {
		t.Fatalf("la victime n'est pas annoncée : %+v", events)
	}
}

func TestEgaliteLaNuitOuvreUnSecondTour(t *testing.T) {
	g, c := newGame(t, 7)
	wolves := wolvesOf(g)
	if len(wolves) != 2 {
		t.Fatalf("%d loups, attendu 2", len(wolves))
	}
	victims := villagersOf(g)[:2]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[0], Target: victims[0]}, c.now); err != nil {
		t.Fatalf("vote du premier loup : %v", err)
	}
	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[1], Target: victims[1]}, c.now); err != nil {
		t.Fatalf("vote du second loup : %v", err)
	}

	events := step(t, g, c)

	if g.Phase() != PhaseNight {
		t.Fatalf("phase %s, attendu %s (second tour)", g.Phase(), PhaseNight)
	}
	if g.round != 2 {
		t.Fatalf("tour %d, attendu 2", g.round)
	}
	if len(g.candidates) != 2 {
		t.Fatalf("candidats %v, attendu les deux victimes", g.candidates)
	}
	if w := c.now.Add(testConfig().Night); !g.Deadline().Equal(w) {
		t.Errorf("échéance %s, attendu %s : le second tour est chronométré lui aussi", g.Deadline(), w)
	}

	var tied bool
	for _, event := range events {
		if event.Kind == EventTie {
			tied = true
		}
		if event.Kind == EventVictim {
			t.Fatal("une victime a été désignée alors que les loups sont à égalité")
		}
	}
	if !tied {
		t.Fatalf("aucun événement d'égalité : %+v", events)
	}

	// Le second tour de la nuit ne regarde que les loups : le village ne doit
	// pas savoir que les loups se sont partagés, ni sur qui.
	villageView := g.Snapshot(victims[0], c.now)
	if villageView.Round != 1 || villageView.Tie || len(villageView.Candidates) != 0 || len(villageView.Voted) != 0 {
		t.Errorf("la vue du village trahit le second tour de la nuit : tour %d, égalité %v, candidats %v, votants %v",
			villageView.Round, villageView.Tie, villageView.Candidates, villageView.Voted)
	}

	wolfView := g.Snapshot(wolves[0], c.now)
	if wolfView.Round != 2 || !wolfView.Tie || len(wolfView.Candidates) != 2 {
		t.Errorf("la vue du loup ne montre pas le second tour : tour %d, égalité %v, candidats %v",
			wolfView.Round, wolfView.Tie, wolfView.Candidates)
	}

	// Le second tour est limité aux joueurs à égalité.
	outsider := ""
	for _, id := range villagersOf(g) {
		if id != victims[0] && id != victims[1] {
			outsider = id
			break
		}
	}
	_, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[0], Target: outsider}, c.now)
	mustRefuse(t, err, ReasonBadTarget, "voix hors ballotage")

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[0], Target: victims[0]}, c.now); err != nil {
		t.Fatalf("second tour : %v", err)
	}
	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[1], Target: victims[0]}, c.now); err != nil {
		t.Fatalf("second tour : %v", err)
	}

	step(t, g, c)

	if len(deadOf(g)) != 1 || deadOf(g)[0] != victims[0] {
		t.Fatalf("morts %v, attendu [%s]", deadOf(g), victims[0])
	}
	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}
}

func TestEgaliteLaNuitPersistanteNEuePersonne(t *testing.T) {
	g, c := newGame(t, 7)
	wolves := wolvesOf(g)
	victims := villagersOf(g)[:2]

	for i, wolf := range wolves {
		if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victims[i]}, c.now); err != nil {
			t.Fatalf("premier tour : %v", err)
		}
	}
	step(t, g, c)

	for i, wolf := range wolves {
		if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victims[i]}, c.now); err != nil {
			t.Fatalf("second tour : %v", err)
		}
	}

	events := step(t, g, c)

	if len(deadOf(g)) != 0 {
		t.Fatalf("des morts malgré l'égalité persistante : %v", deadOf(g))
	}
	if g.Phase() != PhaseDawn {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseDawn)
	}

	var spared bool
	for _, event := range events {
		if event.Kind == EventNoVictim {
			spared = true
		}
	}
	if !spared {
		t.Fatalf("aucun événement d'épargne : %+v", events)
	}
}

func TestEgaliteLeJourOuvreUnSecondTour(t *testing.T) {
	g, c := newGame(t, 6)
	alive := g.alive()
	first, second := alive[0].id, alive[1].id

	goTo(t, g, c, PhaseVote)

	// Trois voix contre trois : personne ne se dégage.
	for i, target := range []string{first, first, first, second, second, second} {
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: alive[i].id, Target: target}, c.now); err != nil {
			t.Fatalf("vote de %s : %v", alive[i].id, err)
		}
	}

	step(t, g, c)

	if g.Phase() != PhaseVote || g.round != 2 {
		t.Fatalf("phase %s tour %d, attendu %s tour 2", g.Phase(), g.round, PhaseVote)
	}
	if len(g.candidates) != 2 {
		t.Fatalf("candidats %v, attendu deux joueurs", g.candidates)
	}

	view := g.Snapshot(first, c.now)
	if !view.Tie || len(view.Candidates) != 2 {
		t.Errorf("la vue ne signale pas le second tour : égalité=%v candidats=%v", view.Tie, view.Candidates)
	}
}

func TestEgaliteLeJourPersistanteNEliminePersonne(t *testing.T) {
	g, c := newGame(t, 6)
	alive := g.alive()
	first, second := alive[0].id, alive[1].id

	goTo(t, g, c, PhaseVote)

	for i, target := range []string{first, first, first, second, second, second} {
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: alive[i].id, Target: target}, c.now); err != nil {
			t.Fatalf("premier tour : %v", err)
		}
	}
	step(t, g, c)

	for i, target := range []string{first, first, first, second, second, second} {
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: alive[i].id, Target: target}, c.now); err != nil {
			t.Fatalf("second tour : %v", err)
		}
	}

	events := step(t, g, c)

	if len(deadOf(g)) != 0 {
		t.Fatalf("élimination malgré l'égalité persistante : %v", deadOf(g))
	}
	if g.Phase() != PhaseVerdict {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseVerdict)
	}

	var spared bool
	for _, event := range events {
		if event.Kind == EventNoElimination {
			spared = true
		}
	}
	if !spared {
		t.Fatalf("aucun événement d'épargne : %+v", events)
	}
}

func TestLeVillageGagneQuandLeDernierLoupMeurt(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]

	goTo(t, g, c, PhaseVote)

	for _, voter := range g.alive() {
		if voter.id == wolf {
			continue
		}
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter.id, Target: wolf}, c.now); err != nil {
			t.Fatalf("vote de %s : %v", voter.id, err)
		}
	}

	events := step(t, g, c)

	if !g.Over() || g.Winner() != WinnerVillage {
		t.Fatalf("fin : %v, vainqueur %q", g.Over(), g.Winner())
	}
	if g.Phase() != PhaseOver {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseOver)
	}
	if !g.Deadline().IsZero() {
		t.Errorf("l'échéance survit à la fin : %s", g.Deadline())
	}

	var over bool
	for _, event := range events {
		if event.Kind == EventGameOver {
			over = true
			if event.Winners != WinnerVillage {
				t.Errorf("vainqueur annoncé %q", event.Winners)
			}
		}
	}
	if !over {
		t.Fatalf("aucun événement de fin : %+v", events)
	}

	// Tous les rôles deviennent publics.
	view := g.Snapshot(wolf, c.now)
	if len(view.Roles) != 6 {
		t.Fatalf("rôles révélés à la fin : %v", view.Roles)
	}
	for _, p := range g.players {
		if view.Roles[p.id] != p.role {
			t.Errorf("rôle de %s : %q, attendu %q", p.id, view.Roles[p.id], p.role)
		}
	}
	if view.Winner != WinnerVillage {
		t.Errorf("vainqueur dans la vue : %q", view.Winner)
	}
}

func TestLesLoupsGagnentAParitieNumerique(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	villagers := villagersOf(g)

	// Chaque nuit le loup dévore, chaque jour le village se trompe : le camp
	// des loups rattrape inexorablement les villageois.
	for i := 0; i < 3; i++ {
		if g.Over() {
			break
		}
		goTo(t, g, c, PhaseNight)
		target := ""
		for _, id := range villagers {
			if g.index[id].alive && id != wolf {
				target = id
				break
			}
		}
		if target == "" {
			t.Fatalf("plus aucun villageois à dévorer au jour %d", g.Day())
		}
		if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: target}, c.now); err != nil {
			t.Fatalf("nuit %d : %v", g.Day(), err)
		}
		step(t, g, c)

		if g.Over() {
			break
		}

		goTo(t, g, c, PhaseVote)
		prey := ""
		for _, p := range g.alive() {
			if p.id != wolf {
				prey = p.id
				break
			}
		}
		if prey == "" {
			t.Fatalf("plus aucun villageois à condamner au jour %d", g.Day())
		}
		for _, voter := range g.alive() {
			if voter.id == prey {
				continue
			}
			if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter.id, Target: prey}, c.now); err != nil {
				t.Fatalf("vote du jour %d : %v", g.Day(), err)
			}
		}
		step(t, g, c)
	}

	if !g.Over() || g.Winner() != WinnerWolves {
		t.Fatalf("fin : %v, vainqueur %q (morts %v)", g.Over(), g.Winner(), deadOf(g))
	}
}

func TestApresLaFinPlusRienNePasse(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]

	goTo(t, g, c, PhaseVote)
	for _, voter := range g.alive() {
		if voter.id == wolf {
			continue
		}
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter.id, Target: wolf}, c.now); err != nil {
			t.Fatalf("vote de %s : %v", voter.id, err)
		}
	}
	step(t, g, c)

	_, err := g.Apply(Command{Kind: CommandVillageVote, Actor: villagersOf(g)[0], Target: wolf}, c.now)
	mustRefuse(t, err, ReasonGameOver, "vote après la fin")

	c.advance(time.Hour)
	_, err = g.Apply(Command{Kind: CommandAdvance}, c.now)
	mustRefuse(t, err, ReasonGameOver, "avance après la fin")
}

func TestSnapshotReserveLesLoupsAuxLoups(t *testing.T) {
	g, c := newGame(t, 7)
	wolf := wolvesOf(g)[0]
	villager := villagersOf(g)[0]

	// Un loup voit ses semblables et leurs choix.
	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: villager}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}

	wolfView := g.Snapshot(wolf, c.now)
	if len(wolfView.Wolves) != 2 {
		t.Errorf("un loup devrait voir les deux loups : %v", wolfView.Wolves)
	}
	if len(wolfView.WolfVotes) != 1 || wolfView.WolfVotes[wolf] != villager {
		t.Errorf("un loup devrait voir les voix de la nuit : %v", wolfView.WolfVotes)
	}
	if len(wolfView.Voted) != 1 {
		t.Errorf("la nuit, seuls les votants doivent apparaître : %v", wolfView.Voted)
	}

	villagerView := g.Snapshot(villager, c.now)
	if villagerView.Wolves != nil || villagerView.WolfVotes != nil {
		t.Errorf("un villageois voit l'antre des loups : %v %v", villagerView.Wolves, villagerView.WolfVotes)
	}
	if len(villagerView.Voted) != 0 {
		t.Errorf("un villageois voit les votes de la nuit : %v", villagerView.Voted)
	}
	if villagerView.YourRole != Villageois {
		t.Errorf("un villageois devrait connaître son rôle : %q", villagerView.YourRole)
	}

	// Le vote du village, lui, est public.
	goTo(t, g, c, PhaseVote)
	other := villagersOf(g)[1]
	if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: wolf, Target: other}, c.now); err != nil {
		t.Fatalf("vote du village : %v", err)
	}
	public := g.Snapshot(other, c.now)
	if len(public.Voted) != 1 || public.Voted[0] != wolf {
		t.Errorf("le vote du jour devrait être public : %v", public.Voted)
	}
	if public.YourVote != "" {
		t.Errorf("ce joueur n'a pas voté : %q", public.YourVote)
	}
}

func TestLeScrutinDuJourEstPublic(t *testing.T) {
	g, c := newGame(t, 7)
	wolf := wolvesOf(g)[0]
	villagers := villagersOf(g)

	goTo(t, g, c, PhaseVote)

	// Avant le premier bulletin, le scrutin est vide.
	if ballot := g.Snapshot(wolf, c.now).Ballot; len(ballot) != 0 {
		t.Fatalf("scrutin d'ouverture = %v, attendu vide", ballot)
	}

	// Un villageois vote : le loup lui-même voit son bulletin.
	cible := villagers[0]
	if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: villagers[1], Target: cible}, c.now); err != nil {
		t.Fatalf("vote : %v", err)
	}
	if got := g.Snapshot(wolf, c.now).Ballot[villagers[1]]; got != cible {
		t.Errorf("bulletin de %s = %q, attendu %q", villagers[1], got, cible)
	}

	// Une abstention est publique elle aussi, mais sans cible.
	if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: villagers[2], Target: ""}, c.now); err != nil {
		t.Fatalf("abstention : %v", err)
	}
	ballot := g.Snapshot(villagers[3], c.now).Ballot
	if got, voted := ballot[villagers[2]]; !voted || got != "" {
		t.Errorf("l'abstention de %s devrait apparaître sans cible : %v", villagers[2], ballot)
	}
}

func TestLeSecondTourRemetLeScrutinAZero(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	villagers := villagersOf(g)

	goTo(t, g, c, PhaseVote)

	cibleA, cibleB := villagers[0], villagers[1]
	vote := func(actor, target string) {
		t.Helper()
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: actor, Target: target}, c.now); err != nil {
			t.Fatalf("vote de %s : %v", actor, err)
		}
	}

	// Deux contre deux : égalité, un second tour s'ouvre.
	vote(villagers[2], cibleA)
	vote(villagers[3], cibleA)
	vote(villagers[4], cibleB)
	vote(wolf, cibleB)

	step(t, g, c)

	view := g.Snapshot(villagers[2], c.now)
	if view.Phase != PhaseVote || !view.Tie || view.Round != 2 {
		t.Fatalf("second tour attendu : phase %q, tour %d, égalité %v", view.Phase, view.Round, view.Tie)
	}
	if len(view.Ballot) != 0 {
		t.Errorf("le scrutin du second tour devrait repartir de zéro : %v", view.Ballot)
	}

	// Seuls les joueurs à égalité restent votables.
	_, err := g.Apply(Command{Kind: CommandVillageVote, Actor: villagers[2], Target: villagers[4]}, c.now)
	mustRefuse(t, err, ReasonBadTarget, "vote hors ballotage")

	vote(villagers[2], cibleB)
	if got := g.Snapshot(wolf, c.now).Ballot[villagers[2]]; got != cibleB {
		t.Errorf("bulletin du second tour = %q, attendu %q", got, cibleB)
	}
}

func TestLeVerdictGardeLeScrutin(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	villagers := villagersOf(g)
	cible := villagers[0]

	goTo(t, g, c, PhaseVote)

	// Tout le monde (le loup compris) vote contre la même cible : elle est
	// éliminée, mais le village n'est pas encore sauvé.
	votants := append([]string{wolf}, villagers[1:]...)
	for _, voter := range votants {
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter, Target: cible}, c.now); err != nil {
			t.Fatalf("vote de %s : %v", voter, err)
		}
	}

	step(t, g, c)

	view := g.Snapshot(wolf, c.now)
	if view.Phase != PhaseVerdict {
		t.Fatalf("phase = %q, attendu verdict", view.Phase)
	}
	if got := view.Ballot[villagers[1]]; got != cible {
		t.Errorf("le scrutin devrait rester affiché au verdict : %v", view.Ballot)
	}
}

func TestSnapshotDUneSpectatriceRestePublique(t *testing.T) {
	g, c := newGame(t, 6)

	view := g.Snapshot("inconnue", c.now)

	if view.YourRole != "" {
		t.Errorf("un spectateur reçoit un rôle : %q", view.YourRole)
	}
	if view.Wolves != nil || view.Roles != nil {
		t.Error("un spectateur reçoit des secrets")
	}
	if len(view.Alive) != 6 {
		t.Errorf("vivants %v, attendu six", view.Alive)
	}
	if len(view.Dead) != 0 {
		t.Errorf("morts %v, attendu aucun", view.Dead)
	}
}

func TestLeCompteAReboursNeDevientPasNegatif(t *testing.T) {
	g, c := newGame(t, 6)

	c.advance(time.Hour)

	if got := g.Snapshot(wolvesOf(g)[0], c.now).SecondsLeft; got != 0 {
		t.Errorf("secondes restantes %d, attendu 0", got)
	}
}

func TestPendingListeCeuxQuiDoiventParler(t *testing.T) {
	g, c := newGame(t, 7)
	wolves := wolvesOf(g)
	victims := villagersOf(g)[:2]

	// La nuit, seuls les loups sont attendus.
	if len(g.Pending()) != 2 {
		t.Fatalf("au début de la nuit, Pending = %v", g.Pending())
	}

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[0], Target: victims[0]}, c.now); err != nil {
		t.Fatalf("vote du premier loup : %v", err)
	}
	if pending := g.Pending(); len(pending) != 1 || pending[0] != wolves[1] {
		t.Fatalf("après un vote, Pending = %v, attendu [%s]", pending, wolves[1])
	}

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolves[1], Target: victims[0]}, c.now); err != nil {
		t.Fatalf("vote du second loup : %v", err)
	}
	if len(g.Pending()) != 0 {
		t.Fatalf("tout le monde a voté, Pending = %v", g.Pending())
	}

	// À l'aube, la phase n'attend personne.
	step(t, g, c)
	if g.Pending() != nil {
		t.Errorf("l'aube n'attend aucun vote : %v", g.Pending())
	}

	// Le vote du village, lui, attend tous les vivants.
	goTo(t, g, c, PhaseVote)
	if len(g.Pending()) != len(g.alive()) {
		t.Errorf("Pending = %v, attendu les %d vivants", g.Pending(), len(g.alive()))
	}

	// CommandClose clôt la phase sans attendre l'échéance.
	for _, voter := range g.alive() {
		if _, err := g.Apply(Command{Kind: CommandVillageVote, Actor: voter.id, Target: ""}, c.now); err != nil {
			t.Fatalf("vote blanc de %s : %v", voter.id, err)
		}
	}

	before := g.Phase()
	if before != PhaseVote {
		t.Fatalf("phase %s avant la clôture", before)
	}

	events, err := g.Apply(Command{Kind: CommandClose}, c.now)
	if err != nil {
		t.Fatalf("clôture immédiate refusée : %v", err)
	}
	if g.Phase() != PhaseVerdict {
		t.Fatalf("phase %s, attendu %s", g.Phase(), PhaseVerdict)
	}

	var spared bool
	for _, event := range events {
		if event.Kind == EventNoElimination {
			spared = true
		}
	}
	if !spared {
		t.Fatalf("aucun événement d'épargne : %+v", events)
	}
}

func TestChaqueEvenementPorteUnePhrase(t *testing.T) {
	g, c := newGame(t, 6)
	wolf := wolvesOf(g)[0]
	victim := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victim}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}

	events := step(t, g, c)
	goTo(t, g, c, PhaseVote)

	for _, event := range events {
		if strings.TrimSpace(event.Text) == "" {
			t.Errorf("événement sans texte : %+v", event)
		}
	}
}

// upToDebate amène la partie au débat en laissant la nuit faire sa victime.
func upToDebate(t *testing.T, g *Game, c *clock) {
	t.Helper()

	wolf := wolvesOf(g)[0]
	victim := villagersOf(g)[0]

	if _, err := g.Apply(Command{Kind: CommandWolfVote, Actor: wolf, Target: victim}, c.now); err != nil {
		t.Fatalf("vote du loup : %v", err)
	}

	goTo(t, g, c, PhaseDebate)
}

func TestLeDebatDonneLaParoleAuxVivantsDansLOrdre(t *testing.T) {
	g, c := newGame(t, 7)
	upToDebate(t, g, c)

	alive := make([]string, 0, len(g.alive()))
	for _, p := range g.alive() {
		alive = append(alive, p.id)
	}

	view := g.Snapshot(alive[0], c.now)
	if !slices.Equal(view.Speakers, alive) {
		t.Fatalf("ordre de parole %v, attendu %v", view.Speakers, alive)
	}
	if view.Speaker != alive[0] {
		t.Errorf("orateur %q, attendu %q", view.Speaker, alive[0])
	}
	if len(view.Spoken) != 0 {
		t.Errorf("personne n'a encore parlé : %v", view.Spoken)
	}
	for _, dead := range deadOf(g) {
		if slices.Contains(view.Speakers, dead) {
			t.Errorf("le mort %q figure dans l'ordre de parole", dead)
		}
	}

	// Le débat n'attend que l'orateur : la salle ne peut pas le refermer tant
	// qu'il n'a pas parlé, même si les bots sont passés.
	if pending := g.Pending(); len(pending) != 1 || pending[0] != alive[0] {
		t.Errorf("Pending = %v, attendu [%s]", pending, alive[0])
	}
	if got := g.CurrentSpeaker(); got != alive[0] {
		t.Errorf("CurrentSpeaker = %q, attendu %q", got, alive[0])
	}
}

func TestChaquePriseDeParoleAvanceLeDebat(t *testing.T) {
	g, c := newGame(t, 6)
	upToDebate(t, g, c)

	speakers := g.Snapshot(g.alive()[0].id, c.now).Speakers

	for i, speaker := range speakers {
		if got := g.CurrentSpeaker(); got != speaker {
			t.Fatalf("orateur %d = %q, attendu %q", i, got, speaker)
		}

		before := g.Snapshot(speaker, c.now).Deadline

		// L'orateur rend la parole de lui-même : le tour suivant s'ouvre sans
		// attendre l'échéance.
		c.advance(10 * time.Second)

		events, err := g.Apply(Command{Kind: CommandSpeak, Actor: speaker}, c.now)
		if err != nil {
			t.Fatalf("prise de parole de %s : %v", speaker, err)
		}

		if i == len(speakers)-1 {
			if g.Phase() != PhaseVote {
				t.Fatalf("après le dernier orateur, phase = %s, attendu %s", g.Phase(), PhaseVote)
			}
			continue
		}

		if len(events) != 1 || events[0].Kind != EventSpeak || events[0].Target != speakers[i+1] {
			t.Errorf("prise de parole %d : événements %+v, attendu l'annonce de %q", i, events, speakers[i+1])
		}

		// Chaque prise de parole a son propre temps : l'échéance est repoussée.
		if after := g.Snapshot(speaker, c.now).Deadline; !after.After(before) {
			t.Errorf("le tour suivant n'a pas d'échéance neuve : %v puis %v", before, after)
		}

		if view := g.Snapshot(speaker, c.now); !slices.Contains(view.Spoken, speaker) {
			t.Errorf("%s a parlé mais ne figure pas parmi les orateurs passés : %v", speaker, view.Spoken)
		}
	}

	if g.CurrentSpeaker() != "" {
		t.Errorf("hors débat, CurrentSpeaker = %q", g.CurrentSpeaker())
	}
}

func TestOnNInterromptPasLaParoleDAutrui(t *testing.T) {
	g, c := newGame(t, 6)

	goTo(t, g, c, PhaseDebate)

	other := ""
	for _, p := range g.alive() {
		if p.id != g.CurrentSpeaker() {
			other = p.id
			break
		}
	}

	_, err := g.Apply(Command{Kind: CommandSpeak, Actor: other}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "parole volée")

	// L'orateur, lui, garde la parole après la tentative.
	if got := g.CurrentSpeaker(); got == other || got == "" {
		t.Errorf("l'orateur a changé après un refus : %q", got)
	}
}

func TestOnNePrendPasLaParoleHorsDebat(t *testing.T) {
	g, c := newGame(t, 6)

	_, err := g.Apply(Command{Kind: CommandSpeak, Actor: wolvesOf(g)[0]}, c.now)
	mustRefuse(t, err, ReasonWrongPhase, "parole pendant la nuit")
}
