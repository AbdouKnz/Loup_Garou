package game

import "time"

// Config règle une partie. Les durées sont surchargées par l'environnement
// (voir internal/config) et raccourcies dans les tests.
type Config struct {
	// MinPlayers et MaxPlayers bornent la table.
	MinPlayers int
	MaxPlayers int
	// Roles est le temps laissé à la table pour découvrir son rôle : la partie
	// commence par cette pause, et la première nuit ne s'écoule qu'ensuite.
	Roles time.Duration
	// Night est le temps laissé aux loups pour choisir leur victime.
	Night time.Duration
	// NightStep est le temps laissé aux rôles qui se lèvent seuls (voyante,
	// sorcière) : une décision suffit, leur veillée est plus courte.
	NightStep time.Duration
	// Reveal est le temps d'affichage des révélations (aube, verdict).
	Reveal time.Duration
	// Speak est le temps de parole laissé à chaque joueur pendant le débat :
	// le jour, les vivants parlent l'un après l'autre, et chacun a ce temps.
	Speak time.Duration
	// Vote est le temps du vote du village.
	Vote time.Duration
	// SpecialRoles liste les rôles de village distribués en plus des loups et
	// des villageois, dans l'ordre où ils se réveillent. Nil demande la
	// composition par défaut ; une liste vide n'en distribue aucun.
	SpecialRoles []Role
}

// DefaultConfig renvoie les réglages de production.
func DefaultConfig() Config {
	return Config{
		MinPlayers: 6,
		MaxPlayers: 12,
		Roles:      3 * time.Second,
		Night:      60 * time.Second,
		NightStep:  25 * time.Second,
		Reveal:     8 * time.Second,
		Speak:      30 * time.Second,
		Vote:       60 * time.Second,
		// Le socle : la voyante éclaire le village, la sorcière peut sauver une
		// proie ou en achever une autre.
		SpecialRoles: []Role{Voyante, Sorciere},
	}
}

// withDefaults complète les champs laissés à zéro : un test ne précise que ce
// qui l'intéresse.
func (c Config) WithDefaults() Config {
	def := DefaultConfig()

	if c.MinPlayers > 0 {
		def.MinPlayers = c.MinPlayers
	}
	if c.MaxPlayers > 0 {
		def.MaxPlayers = c.MaxPlayers
	}
	if c.Roles > 0 {
		def.Roles = c.Roles
	}
	if c.Night > 0 {
		def.Night = c.Night
	}
	if c.NightStep > 0 {
		def.NightStep = c.NightStep
	}
	if c.Reveal > 0 {
		def.Reveal = c.Reveal
	}
	if c.Speak > 0 {
		def.Speak = c.Speak
	}
	if c.Vote > 0 {
		def.Vote = c.Vote
	}
	// Nil garde la composition par défaut ; une liste vide la vide vraiment.
	// La copie préserve cette distinction, y compris si la configuration est
	// complétée deux fois : sans elle, « aucun rôle spécial » redeviendrait le
	// défaut.
	if c.SpecialRoles != nil {
		def.SpecialRoles = make([]Role, len(c.SpecialRoles))
		copy(def.SpecialRoles, c.SpecialRoles)
	}

	return def
}

// CompositionFor renvoie la répartition des rôles d'une table de n joueurs.
//
// Les loups d'abord, puis les rôles de village activés, le reste en villageois.
// Elle ne dépend que de la taille de la table, jamais du tirage : c'est ce qui
// permet de l'annoncer à la salle d'attente avant de distribuer.
func (c Config) CompositionFor(n int) []Role {
	roles := make([]Role, 0, n)

	for i := 0; i < c.WolvesFor(n); i++ {
		roles = append(roles, LoupGarou)
	}

	// On garde toujours au moins un villageois : sans lui, la table n'aurait
	// plus de camp majoritaire à défendre.
	for _, role := range c.SpecialRoles {
		if len(roles) >= n-1 {
			break
		}
		if role == LoupGarou || role == Villageois || containsRole(roles, role) {
			continue
		}
		roles = append(roles, role)
	}

	for len(roles) < n {
		roles = append(roles, Villageois)
	}

	return roles
}

// containsRole indique si liste contient déjà role.
func containsRole(list []Role, role Role) bool {
	for _, item := range list {
		if item == role {
			return true
		}
	}

	return false
}

// WolvesFor renvoie le nombre de loups pour une table de n joueurs.
//
// La répartition suit la taille : un loup jusqu'à 6 joueurs, deux jusqu'à 9,
// trois au-delà. Avec la règle de victoire (les loups gagnent à égalité
// numérique), c'est l'équilibre retenu pour le MVP.
func (c Config) WolvesFor(n int) int {
	switch {
	case n >= 10:
		return 3
	case n >= 7:
		return 2
	case n >= 6:
		return 1
	default:
		return 0
	}
}

// Duration renvoie la durée d'une phase.
func (c Config) Duration(phase Phase) time.Duration {
	switch phase {
	case PhaseRoles:
		return c.Roles
	case PhaseNight:
		return c.Night
	case PhaseDawn, PhaseVerdict:
		return c.Reveal
	case PhaseDebate:
		// La phase dure le temps d'une prise de parole : l'échéance est
		// repoussée à chaque joueur, et le débat se referme sur le vote.
		return c.Speak
	case PhaseVote:
		return c.Vote
	default:
		return 0
	}
}

// NightStepDuration est le temps d'une veillée de rôle solitaire.
func (c Config) NightStepDuration() time.Duration {
	if c.NightStep > 0 {
		return c.NightStep
	}

	return c.Night
}
