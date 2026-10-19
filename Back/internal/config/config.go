// Package config lit la configuration du serveur depuis l'environnement.
package config

import (
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/room"
)

// Config regroupe les réglages du serveur.
type Config struct {
	// Port est le port d'écoute HTTP (variable PORT).
	Port string
	// WebDist est le dossier de l'interface web construite (variable WEB_DIST).
	// Utilisé en production ; en développement, c'est Vite qui sert l'interface.
	WebDist string
	// AllowedOrigins liste les origines supplémentaires autorisées à ouvrir un
	// WebSocket (variable ALLOWED_ORIGINS, séparées par des virgules). L'origine
	// de la requête elle-même est toujours acceptée : cette liste ne sert qu'au
	// développement, où l'interface est servie par Vite sur un autre port.
	AllowedOrigins []string
	// AccountsFile est le fichier des comptes (variable ACCOUNTS_FILE).
	AccountsFile string
	// SessionsFile est le fichier des sessions ouvertes (variable
	// SESSIONS_FILE).
	SessionsFile string
	// SessionTTL est la durée de validité d'une session, prolongée à chaque
	// visite (variable SESSION_TTL_DAYS, en jours).
	SessionTTL time.Duration
	// PublicURL est l'adresse publique du site (variable PUBLIC_URL). Elle
	// détermine le drapeau « Secure » du cookie de session et l'adresse de
	// retour déclarée aux fournisseurs de connexion externe.
	PublicURL string
	// FrontURL est l'adresse de l'interface, où le navigateur est renvoyé après
	// une connexion externe (variable FRONT_URL). Elle vaut PUBLIC_URL par
	// défaut. En développement, l'interface est servie par Vite sur un autre
	// port que le serveur : on y met alors l'adresse de Vite
	// (http://localhost:5173), pour que le retour ramène bien à l'interface.
	FrontURL string
	// GoogleClientID et GoogleClientSecret identifient l'application auprès de
	// Google (variables GOOGLE_CLIENT_ID et GOOGLE_CLIENT_SECRET). Sans ces
	// identifiants, la connexion Google n'est pas proposée.
	GoogleClientID     string
	GoogleClientSecret string
	// DiscordClientID et DiscordClientSecret identifient l'application auprès
	// de Discord (variables DISCORD_CLIENT_ID et DISCORD_CLIENT_SECRET).
	DiscordClientID     string
	DiscordClientSecret string
	// ShowOAuthButtons force l'affichage des boutons de connexion externe même
	// quand aucun fournisseur n'est configuré (variable SHOW_OAUTH_BUTTONS).
	// Réservé au développement : voir et valider l'interface sans déclarer
	// d'application. Cliquer mène alors à « non disponible ».
	ShowOAuthButtons bool
	// Game règle les durées d'une partie (variables GAME_*_SECONDS).
	Game game.Config
	// BotDelay est le temps de réflexion des bots avant chaque vague d'actions
	// (variable GAME_BOT_DELAY_SECONDS). Une valeur négative les fait jouer
	// aussitôt.
	BotDelay time.Duration
}

// Emplacements par défaut des données personnelles, relatifs au dossier
// d'exécution du serveur (donc Back/data en développement).
const (
	// DefaultAccountsFile est le fichier des comptes.
	DefaultAccountsFile = "data/accounts.json"
	// DefaultSessionsFile est le fichier des sessions.
	DefaultSessionsFile = "data/sessions.json"
	// DefaultSessionTTL est la durée de validité d'une session.
	DefaultSessionTTL = 7 * 24 * time.Hour
	// DefaultEnvFile est le fichier de variables locales lu au démarrage, relatif
	// au dossier d'exécution du serveur (donc Back/.env en développement). Il
	// n'est pas versionné : `.gitignore` le protège, et un fichier absent est
	// simplement ignoré.
	DefaultEnvFile = ".env"
)

// Load construit la configuration en appliquant les valeurs par défaut.
func Load() Config {
	cfg := Config{
		Port:           envOr("PORT", "8080"),
		WebDist:        envOr("WEB_DIST", "Front/dist"),
		AllowedOrigins: splitList(envOr("ALLOWED_ORIGINS", "localhost:*,127.0.0.1:*")),
		AccountsFile:   envOr("ACCOUNTS_FILE", DefaultAccountsFile),
		SessionsFile:   envOr("SESSIONS_FILE", DefaultSessionsFile),
		SessionTTL:     envDays("SESSION_TTL_DAYS", DefaultSessionTTL),
		Game:           game.DefaultConfig(),
		BotDelay:       room.DefaultOptions().BotDelay,

		// Les identifiants des fournisseurs externes sont lus tels quels : ils
		// sont absents tant que l'application n'est pas déclarée chez eux.
		GoogleClientID:      envOr("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  envOr("GOOGLE_CLIENT_SECRET", ""),
		DiscordClientID:     envOr("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: envOr("DISCORD_CLIENT_SECRET", ""),
		ShowOAuthButtons:    envBool("SHOW_OAUTH_BUTTONS", false),
	}

	cfg.PublicURL = strings.TrimRight(envOr("PUBLIC_URL", "http://localhost:"+cfg.Port), "/")
	cfg.FrontURL = strings.TrimRight(envOr("FRONT_URL", cfg.PublicURL), "/")

	cfg.Game.Roles = envDuration("GAME_ROLES_SECONDS", cfg.Game.Roles)
	cfg.Game.Night = envDuration("GAME_NIGHT_SECONDS", cfg.Game.Night)
	cfg.Game.NightStep = envDuration("GAME_NIGHT_STEP_SECONDS", cfg.Game.NightStep)
	cfg.Game.Reveal = envDuration("GAME_REVEAL_SECONDS", cfg.Game.Reveal)
	cfg.Game.Speak = envDuration("GAME_SPEAK_SECONDS", cfg.Game.Speak)
	cfg.Game.Vote = envDuration("GAME_VOTE_SECONDS", cfg.Game.Vote)
	cfg.Game.MinPlayers = envInt("GAME_MIN_PLAYERS", cfg.Game.MinPlayers)
	cfg.Game.MaxPlayers = envInt("GAME_MAX_PLAYERS", cfg.Game.MaxPlayers)
	cfg.Game.SpecialRoles = envRoles("GAME_ROLES", cfg.Game.SpecialRoles)
	cfg.BotDelay = envDelay("GAME_BOT_DELAY_SECONDS", cfg.BotDelay)

	return cfg
}

// LoadEnvFile charge un fichier `.env` dans l'environnement, avant que la
// configuration ne soit lue.
//
// Le fichier est **optionnel** : absent, il n'y a rien à faire et le serveur
// démarre comme avant. Une variable **déjà présente** dans l'environnement n'est
// jamais écrasée : la ligne de commande prime toujours sur le fichier, ce qui
// permet de surcharger un réglage sans éditer `.env`.
//
// Le format reste volontairement simple, une variable par ligne :
//
//	CLE=valeur
//	# commentaire
//	export CLE="valeur entre guillemets"
//
// Les lignes vides, les commentaires et les lignes sans « = » sont ignorés ; les
// espaces de bord et les guillemets simples ou doubles qui entourent la valeur
// sont retirés.
func LoadEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// Un fichier absent n'est pas une erreur : `.env` est un confort.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := parseEnvLine(line)
		if !ok || os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return nil
}

// parseEnvLine lit une ligne de fichier `.env` : la clé, la valeur et si la ligne
// en porte une.
func parseEnvLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	line = strings.TrimPrefix(line, "export ")

	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	return key, unquote(strings.TrimSpace(value)), true
}

// unquote retire une paire de guillemets qui entoure une valeur, pour permettre
// les valeurs à espaces sans échappement.
func unquote(value string) string {
	if len(value) < 2 {
		return value
	}

	first, last := value[0], value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1]
	}

	return value
}

// envRoles lit la liste des rôles distribués en plus des loups et des
// villageois : « voyante,sorciere » par défaut, « aucun » pour n'en distribuer
// aucun. Un nom inconnu est ignoré ; une valeur absente conserve le défaut.
func envRoles(key string, fallback []game.Role) []game.Role {
	raw := envOr(key, "")
	if raw == "" {
		return fallback
	}

	roles := make([]game.Role, 0, len(fallback))

	for _, name := range splitList(raw) {
		if isNoRole(name) {
			return []game.Role{}
		}
		if role, ok := specialRole(name); ok && !hasRole(roles, role) {
			roles = append(roles, role)
		}
	}

	return roles
}

// specialRole traduit un nom de rôle spécial venu de l'environnement.
func specialRole(name string) (game.Role, bool) {
	switch strings.ToLower(name) {
	case string(game.Voyante):
		return game.Voyante, true
	case string(game.Sorciere):
		return game.Sorciere, true
	default:
		return "", false
	}
}

// isNoRole reconnaît la valeur qui demande une table sans rôle spécial.
func isNoRole(name string) bool {
	switch strings.ToLower(name) {
	case "aucun", "aucune", "vide", "none", "off":
		return true
	default:
		return false
	}
}

// hasRole indique si roles contient déjà role.
func hasRole(roles []game.Role, role game.Role) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}

	return false
}

// envDelay lit un délai en secondes ; « 0 » ou une valeur négative désactive
// l'attente, une valeur illisible conserve le défaut.
func envDelay(key string, fallback time.Duration) time.Duration {
	raw := envOr(key, "")
	if raw == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}

// envDuration lit une durée exprimée en secondes entières.
func envDuration(key string, fallback time.Duration) time.Duration {
	raw := envOr(key, "")
	if raw == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}

// envDays lit une durée exprimée en jours entiers.
func envDays(key string, fallback time.Duration) time.Duration {
	raw := envOr(key, "")
	if raw == "" {
		return fallback
	}

	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return fallback
	}

	return time.Duration(days) * 24 * time.Hour
}

// envInt lit un entier positif.
func envInt(key string, fallback int) int {
	raw := envOr(key, "")
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

// splitList découpe une liste séparée par des virgules, en ignorant le vide.
func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

// envOr retourne la variable d'environnement key après suppression des espaces
// de bord, ou fallback si elle est absente ou vide.
func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

// envBool lit un interrupteur : « 1 », « true », « oui », « yes » ou « on »
// l'allument, toute autre valeur l'éteignent. Une variable absente ou vide
// conserve le défaut.
func envBool(key string, fallback bool) bool {
	raw := strings.ToLower(envOr(key, ""))
	if raw == "" {
		return fallback
	}

	switch raw {
	case "1", "true", "oui", "yes", "on":
		return true
	default:
		return false
	}
}
