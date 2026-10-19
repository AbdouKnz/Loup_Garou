package game

import (
	"math"
	"time"
)

// View est la partie telle qu'un joueur a le droit de la voir.
//
// C'est le seul filtre : les rôles ne sortent d'ici que par YourRole, par la
// liste des loups (réservée aux loups) et par Roles, qui reste vide tant que la
// partie n'est pas terminée. Aucune structure du moteur n'est exposée.
type View struct {
	// Phase, Day et Deadline situent la partie dans le temps.
	Phase       Phase
	Day         int
	Deadline    time.Time
	SecondsLeft int
	// Round vaut 2 pendant un second tour — sauf la nuit, où le village voit
	// toujours le tour 1 : les hésitations des loups ne le regardent pas.
	Round int
	// Winner est vide tant que la partie dure.
	Winner Winner
	// YourRole est le rôle du joueur à qui la vue est destinée.
	YourRole Role
	// YourVote est la cible déjà choisie par ce joueur au tour courant.
	YourVote string

	// Alive et Dead listent les joueurs, dans l'ordre de la table.
	Alive []string
	Dead  []string
	// Voted liste les joueurs qui ont déjà voté au tour courant. La nuit, seuls
	// les loups y figurent — et seuls les loups reçoivent la liste.
	Voted []string
	// Ballot est le scrutin du village, public : il associe chaque votant à sa
	// cible (vide pour une abstention). Renseigné pendant le vote du jour et le
	// verdict qui suit, jamais la nuit.
	Ballot map[string]string

	// Wolves liste les loups et WolfVotes leurs choix : réservés aux loups.
	Wolves    []string
	WolfVotes map[string]string

	// YourAction est ce que la veillée en cours attend de ce joueur : vide, il
	// n'a rien à y faire. Réservé aux acteurs de la veillée.
	YourAction ActionKind
	// YourVision est le carnet de la voyante, nuit après nuit. Réservé à elle.
	YourVision []Observation
	// YourPotions dit ce qu'il lui reste. Réservé à la sorcière.
	YourPotions *Potions
	// WolfTarget est la proie que les loups viennent de désigner : la sorcière
	// doit la connaître pour décider si elle la sauve. Réservé à elle.
	WolfTarget string

	// Deaths sont les morts de la nuit, avec leur cause, annoncées à l'aube —
	// sans leur rôle. Eliminated est annoncé au verdict, avec le sien. Les deux
	// restent renseignés une fois la partie finie.
	Deaths         []Death
	Eliminated     string
	EliminatedRole Role

	// Tie et Candidates décrivent un second tour en cours : le vote du village
	// est public, celui de la nuit est réservé aux loups.
	Tie        bool
	Candidates []string

	// Speaker, Speakers et Spoken décrivent le tour de parole du débat : qui
	// parle à cet instant, l'ordre complet des vivants, et ceux qui ont déjà
	// parlé. C'est une information publique : savoir qui parle n'apprend aucun
	// rôle.
	Speaker  string
	Speakers []string
	Spoken   []string

	// Roles n'est rempli qu'à la fin : tous les rôles sont alors publics.
	Roles map[string]Role
}

// Snapshot construit la vue d'un joueur à un instant donné.
//
// Un spectateur inconnu (viewer absent de la table) obtient la vue publique :
// c'est ce que voit un mort, et c'est tout ce qu'un mort voit.
func (g *Game) Snapshot(viewer string, now time.Time) View {
	me, isPlayer := g.index[viewer]

	view := View{
		Phase:  g.phase,
		Day:    g.day,
		Round:  g.round,
		Winner: g.winner,
		Alive:  make([]string, 0, len(g.players)),
		Dead:   make([]string, 0, len(g.players)),
	}

	if !g.deadline.IsZero() {
		view.Deadline = g.deadline
		view.SecondsLeft = secondsLeft(g.deadline, now)
	}

	for _, p := range g.players {
		if p.alive {
			view.Alive = append(view.Alive, p.id)
		} else {
			view.Dead = append(view.Dead, p.id)
		}
	}

	if isPlayer {
		view.YourRole = me.role
		view.YourVote = g.votes[me.id]
	}

	// Le loup est le seul à tout savoir de la nuit : qui a voté, et sur qui.
	wolf := isPlayer && me.role == LoupGarou

	// Chaque rôle ne reçoit que son propre carnet : ce que la voyante a vu, ce
	// qu'il reste à la sorcière, et la proie qu'elle doit juger.
	if isPlayer && me.alive && g.phase != PhaseOver {
		switch me.role {
		case Voyante:
			view.YourVision = copyVision(g.vision[me.id])
		case Sorciere:
			view.YourPotions = &Potions{Heal: !g.healUsed, Poison: !g.poisonUsed}
		}
	}

	if g.phase == PhaseNight && isPlayer && me.alive {
		view.YourAction = g.yourAction(me)

		if g.currentStep() == stepWitch && me.role == Sorciere {
			view.WolfTarget = g.wolfTarget
		}
	}

	// Le vote du village est public ; celui de la nuit reste entre loups.
	switch g.phase {
	case PhaseVote, PhaseVerdict:
		view.Voted = g.votedIDs(g.alive())
	case PhaseNight:
		if wolf {
			view.Voted = g.votedIDs(g.wolves())
		} else {
			// Le village ne sait rien de la nuit : ni qu'un second tour a lieu,
			// ni les proies que les loups se disputent. Le tour est donc annoncé
			// comme le premier, et la partie reprend au tour 1 à l'aube.
			view.Round = 1
		}
	}

	// Le scrutin du jour est public : le village voit les bulletins tomber.
	view.Ballot = g.ballotView()

	// Le tour de parole du débat est public lui aussi : la table voit qui a la
	// parole, et qui l'a déjà eue. La mort fige l'ordre : les morts n'y figurent
	// pas, ils écoutent.
	if g.phase == PhaseDebate {
		view.Speaker = g.currentSpeaker()
		view.Speakers = append([]string(nil), g.speakers...)
		view.Spoken = append([]string(nil), g.speakers[:g.speakerIndex]...)
	}

	if wolf && g.phase != PhaseOver {
		view.Wolves = make([]string, 0, 3)
		for _, wolf := range g.wolves() {
			view.Wolves = append(view.Wolves, wolf.id)
		}
		view.WolfVotes = make(map[string]string, len(view.Wolves))
		for _, wolf := range g.wolves() {
			if target, voted := g.votes[wolf.id]; voted {
				view.WolfVotes[wolf.id] = target
			}
		}
	}

	// Un second tour vit le jour pour tout le monde, et la nuit pour les loups.
	if g.phase == PhaseVote || (g.phase == PhaseNight && wolf) {
		view.Tie = g.round > 1
		view.Candidates = append([]string(nil), g.candidates...)
	}

	// Les révélations de la phase qui vient de s'achever restent affichées.
	if g.phase == PhaseDawn || g.phase == PhaseOver {
		view.Deaths = copyDeaths(g.deaths)
	}
	if g.phase == PhaseVerdict || g.phase == PhaseOver {
		view.Eliminated = g.eliminated
		if p, ok := g.index[g.eliminated]; ok {
			view.EliminatedRole = p.role
		}
	}

	if g.Over() {
		view.Roles = make(map[string]Role, len(g.players))
		for _, p := range g.players {
			view.Roles[p.id] = p.role
		}
	}

	return view
}

// votedIDs renvoie les votants d'une population, dans l'ordre de la table.
func (g *Game) votedIDs(voters []*player) []string {
	out := make([]string, 0, len(voters))
	for _, voter := range voters {
		if _, voted := g.votes[voter.id]; voted {
			out = append(out, voter.id)
		}
	}

	return out
}

// ballotView renvoie le scrutin public à afficher selon la phase : le vote du
// village en cours, ou celui qui vient de s'achever, le temps du verdict.
//
// La nuit n'en laisse rien sortir : les loups votent en secret.
func (g *Game) ballotView() map[string]string {
	switch g.phase {
	case PhaseVote:
		return copyBallot(g.votes)
	case PhaseVerdict:
		return copyBallot(g.dayBallot)
	default:
		return nil
	}
}

// copyBallot recopie un scrutin, pour ne jamais laisser fuir la carte interne
// du moteur dans une vue.
func copyBallot(votes map[string]string) map[string]string {
	if len(votes) == 0 {
		return nil
	}

	out := make(map[string]string, len(votes))
	for voter, target := range votes {
		out[voter] = target
	}

	return out
}

// copyVision recopie le carnet de la voyante : le moteur ne laisse jamais
// fuir la tranche qu'il garde en mémoire.
func copyVision(vision []Observation) []Observation {
	if len(vision) == 0 {
		return nil
	}

	return append([]Observation(nil), vision...)
}

// copyDeaths recopie les morts de la nuit.
func copyDeaths(deaths []Death) []Death {
	if len(deaths) == 0 {
		return nil
	}

	return append([]Death(nil), deaths...)
}

// secondsLeft arrondit une échéance en secondes affichables, jamais négatives.
func secondsLeft(deadline, now time.Time) int {
	left := int(math.Ceil(deadline.Sub(now).Seconds()))
	if left < 0 {
		return 0
	}

	return left
}
