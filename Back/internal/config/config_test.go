package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/game"
)

func TestRolesParDefaut(t *testing.T) {
	t.Setenv("GAME_ROLES", "")

	cfg := Load()

	voulu := []game.Role{game.Voyante, game.Sorciere}
	if !sameRoles(cfg.Game.SpecialRoles, voulu) {
		t.Fatalf("rôles par défaut = %v, voulu %v", cfg.Game.SpecialRoles, voulu)
	}
}

func TestRolesAucun(t *testing.T) {
	t.Setenv("GAME_ROLES", "aucun")

	cfg := Load()

	// Une liste vide n'est pas nil : elle demande explicitement une table sans
	// rôle spécial, là où nil réclame le défaut.
	if cfg.Game.SpecialRoles == nil {
		t.Fatal("GAME_ROLES=aucun doit renvoyer une liste vide, pas nil")
	}
	if len(cfg.Game.SpecialRoles) != 0 {
		t.Fatalf("rôles = %v, voulu aucun", cfg.Game.SpecialRoles)
	}
}

func TestRolesLusEtFiltres(t *testing.T) {
	// Les espaces, la casse, un doublon et un nom inconnu sont tolérés.
	t.Setenv("GAME_ROLES", " Sorciere , inconnu, voyante, vOyAnTe ")

	cfg := Load()

	voulu := []game.Role{game.Sorciere, game.Voyante}
	if !sameRoles(cfg.Game.SpecialRoles, voulu) {
		t.Fatalf("rôles = %v, voulu %v", cfg.Game.SpecialRoles, voulu)
	}
}

func TestRolesVidesSansVoyante(t *testing.T) {
	// Une table qui ne demande que la sorcière n'aura pas de voyante : la
	// composition suit la demande, elle n'ajoute rien.
	t.Setenv("GAME_ROLES", "sorciere")

	cfg := Load()

	voulu := []game.Role{
		game.LoupGarou, game.Sorciere,
		game.Villageois, game.Villageois,
		game.Villageois, game.Villageois,
	}
	if got := cfg.Game.CompositionFor(6); !sameRoles(got, voulu) {
		t.Fatalf("CompositionFor(6) = %v, voulu %v", got, voulu)
	}
}

func TestDureeDeLaDistribution(t *testing.T) {
	t.Setenv("GAME_ROLES_SECONDS", "7")

	cfg := Load()

	if cfg.Game.Roles != 7*time.Second {
		t.Fatalf("Roles = %v, voulu 7s", cfg.Game.Roles)
	}
}

func TestDureeDeLaDistributionIllisible(t *testing.T) {
	// Une distribution sans échéance bloquerait la partie : une valeur nulle ou
	// illisible conserve le défaut.
	t.Setenv("GAME_ROLES_SECONDS", "0")

	cfg := Load()

	if cfg.Game.Roles != game.DefaultConfig().Roles {
		t.Fatalf("Roles = %v, voulu %v", cfg.Game.Roles, game.DefaultConfig().Roles)
	}
}

func TestDureeDesVeillees(t *testing.T) {
	t.Setenv("GAME_NIGHT_STEP_SECONDS", "7")

	cfg := Load()

	if cfg.Game.NightStep != 7*time.Second {
		t.Fatalf("NightStep = %v, voulu 7s", cfg.Game.NightStep)
	}
}

func TestDureeDesVeilleesIllisible(t *testing.T) {
	// Une valeur illisible ou nulle conserve le défaut : une veillée sans
	// échéance bloquerait la partie.
	t.Setenv("GAME_NIGHT_STEP_SECONDS", "0")

	cfg := Load()

	if cfg.Game.NightStep != game.DefaultConfig().NightStep {
		t.Fatalf("NightStep = %v, voulu %v", cfg.Game.NightStep, game.DefaultConfig().NightStep)
	}
}

func TestComptesEtSessionsParDefaut(t *testing.T) {
	t.Setenv("ACCOUNTS_FILE", "")
	t.Setenv("SESSIONS_FILE", "")
	t.Setenv("SESSION_TTL_DAYS", "")

	cfg := Load()

	if cfg.AccountsFile != DefaultAccountsFile {
		t.Errorf("AccountsFile = %q, voulu %q", cfg.AccountsFile, DefaultAccountsFile)
	}
	if cfg.SessionsFile != DefaultSessionsFile {
		t.Errorf("SessionsFile = %q, voulu %q", cfg.SessionsFile, DefaultSessionsFile)
	}
	if cfg.SessionTTL != DefaultSessionTTL {
		t.Errorf("SessionTTL = %v, voulu %v", cfg.SessionTTL, DefaultSessionTTL)
	}
}

func TestComptesEtSessionsRegles(t *testing.T) {
	t.Setenv("ACCOUNTS_FILE", "autre/comptes.json")
	t.Setenv("SESSIONS_FILE", "autre/sessions.json")
	t.Setenv("SESSION_TTL_DAYS", "30")

	cfg := Load()

	if cfg.AccountsFile != "autre/comptes.json" {
		t.Errorf("AccountsFile = %q", cfg.AccountsFile)
	}
	if cfg.SessionsFile != "autre/sessions.json" {
		t.Errorf("SessionsFile = %q", cfg.SessionsFile)
	}
	if cfg.SessionTTL != 30*24*time.Hour {
		t.Errorf("SessionTTL = %v, voulu 30 jours", cfg.SessionTTL)
	}
}

func TestDureeDeSessionIllisible(t *testing.T) {
	// Une durée nulle fermerait toutes les sessions aussitôt : on garde le
	// défaut.
	for _, raw := range []string{"0", "-3", "bientôt"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("SESSION_TTL_DAYS", raw)

			if ttl := Load().SessionTTL; ttl != DefaultSessionTTL {
				t.Errorf("SessionTTL = %v, voulu %v", ttl, DefaultSessionTTL)
			}
		})
	}
}

func TestPublicURLParDefaut(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	t.Setenv("PORT", "9000")

	if url := Load().PublicURL; url != "http://localhost:9000" {
		t.Errorf("PublicURL = %q, voulu %q", url, "http://localhost:9000")
	}
}

func TestPublicURLSansBarreFinale(t *testing.T) {
	// La valeur sert à composer des adresses : une barre finale donnerait des
	// doubles barres.
	t.Setenv("PUBLIC_URL", "https://loup.example.com/")

	if url := Load().PublicURL; url != "https://loup.example.com" {
		t.Errorf("PublicURL = %q", url)
	}
}

func TestFournisseursSansIdentifiants(t *testing.T) {
	// Rien n'est déclaré par défaut : aucun fournisseur externe n'est proposé
	// tant que l'application n'est pas enregistrée chez lui.
	for _, key := range []string{
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"DISCORD_CLIENT_ID", "DISCORD_CLIENT_SECRET",
	} {
		t.Setenv(key, "")
	}

	cfg := Load()

	if cfg.GoogleClientID != "" || cfg.GoogleClientSecret != "" {
		t.Errorf("Google = %q/%q", cfg.GoogleClientID, cfg.GoogleClientSecret)
	}
	if cfg.DiscordClientID != "" || cfg.DiscordClientSecret != "" {
		t.Errorf("Discord = %q/%q", cfg.DiscordClientID, cfg.DiscordClientSecret)
	}
}

func TestFournisseursAvecIdentifiants(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", " 123-abc.apps.googleusercontent.com ")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret-google")
	t.Setenv("DISCORD_CLIENT_ID", "1234567890")
	t.Setenv("DISCORD_CLIENT_SECRET", "secret-discord")

	cfg := Load()

	if cfg.GoogleClientID != "123-abc.apps.googleusercontent.com" {
		t.Errorf("GoogleClientID = %q", cfg.GoogleClientID)
	}
	if cfg.GoogleClientSecret != "secret-google" {
		t.Errorf("GoogleClientSecret = %q", cfg.GoogleClientSecret)
	}
	if cfg.DiscordClientID != "1234567890" {
		t.Errorf("DiscordClientID = %q", cfg.DiscordClientID)
	}
	if cfg.DiscordClientSecret != "secret-discord" {
		t.Errorf("DiscordClientSecret = %q", cfg.DiscordClientSecret)
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	content := strings.Join([]string{
		"# un commentaire",
		"",
		"GOOGLE_CLIENT_ID=123-abc.apps.googleusercontent.com",
		"  DISCORD_CLIENT_SECRET = \"secret avec espaces\"  ",
		"export SHOW_OAUTH_BUTTONS=1",
		"ligne sans signe egal",
		"CLESANSVALEUR=",
		"=sanscle",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	keys := []string{"GOOGLE_CLIENT_ID", "DISCORD_CLIENT_SECRET", "SHOW_OAUTH_BUTTONS", "CLESANSVALEUR"}
	for _, key := range keys {
		t.Setenv(key, "")
	}

	if err := LoadEnvFile(path); err != nil {
		t.Fatalf("chargement impossible : %v", err)
	}

	if got := os.Getenv("GOOGLE_CLIENT_ID"); got != "123-abc.apps.googleusercontent.com" {
		t.Errorf("GOOGLE_CLIENT_ID = %q", got)
	}
	// Les espaces de bord et les guillemets sont retirés.
	if got := os.Getenv("DISCORD_CLIENT_SECRET"); got != "secret avec espaces" {
		t.Errorf("DISCORD_CLIENT_SECRET = %q", got)
	}
	// Le préfixe `export` est toléré.
	if got := os.Getenv("SHOW_OAUTH_BUTTONS"); got != "1" {
		t.Errorf("SHOW_OAUTH_BUTTONS = %q", got)
	}

	// Les valeurs lues alimentent bien la configuration.
	if cfg := Load(); cfg.GoogleClientID != "123-abc.apps.googleusercontent.com" || !cfg.ShowOAuthButtons {
		t.Errorf("configuration = %+v", cfg)
	}
}

func TestLoadEnvFileNeRemplacePasLEnvironnement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := os.WriteFile(path, []byte("PORT=9999\n"), 0o600); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	// Déjà réglé dans l'environnement : le fichier ne doit pas l'écraser.
	t.Setenv("PORT", "1234")

	if err := LoadEnvFile(path); err != nil {
		t.Fatalf("chargement impossible : %v", err)
	}

	if got := os.Getenv("PORT"); got != "1234" {
		t.Errorf("PORT = %q, la ligne de commande doit primer", got)
	}
}

func TestLoadEnvFileAbsent(t *testing.T) {
	// `.env` est un confort : son absence ne doit pas empêcher le démarrage.
	if err := LoadEnvFile(filepath.Join(t.TempDir(), "absent.env")); err != nil {
		t.Errorf("un fichier absent ne doit pas échouer : %v", err)
	}
}

func TestFrontURLSuitPublicURL(t *testing.T) {
	// Sans réglage explicite, l'interface est servie par le même serveur : le
	// retour se fait donc là où l'API est publique.
	t.Setenv("FRONT_URL", "")
	t.Setenv("PUBLIC_URL", "https://loup.example.com/")

	if got := Load().FrontURL; got != "https://loup.example.com" {
		t.Errorf("FrontURL = %q, voulu PUBLIC_URL", got)
	}
}

func TestFrontURLSurcharge(t *testing.T) {
	// En développement, l'interface vit sur le port de Vite : le retour doit y
	// mener, et PUBLIC_URL n'est pas touchée (elle reste l'adresse de l'API).
	t.Setenv("PUBLIC_URL", "http://localhost:8080")
	t.Setenv("FRONT_URL", "http://localhost:5173/")

	cfg := Load()

	if cfg.FrontURL != "http://localhost:5173" {
		t.Errorf("FrontURL = %q", cfg.FrontURL)
	}
	if cfg.PublicURL != "http://localhost:8080" {
		t.Errorf("PublicURL = %q, elle ne doit pas changer", cfg.PublicURL)
	}
}

func TestBoutonsExternesForces(t *testing.T) {
	// Le mode développement est éteint par défaut : un interrupteur absent ne
	// montre rien de plus que les fournisseurs configurés.
	t.Setenv("SHOW_OAUTH_BUTTONS", "")

	if cfg := Load(); cfg.ShowOAuthButtons {
		t.Error("SHOW_OAUTH_BUTTONS vide doit valoir false")
	}

	// Les écritures courantes d'un interrupteur l'allument.
	for _, value := range []string{"1", "true", "OUI", "on"} {
		t.Setenv("SHOW_OAUTH_BUTTONS", value)

		if cfg := Load(); !cfg.ShowOAuthButtons {
			t.Errorf("SHOW_OAUTH_BUTTONS=%q doit valoir true", value)
		}
	}

	// Toute autre valeur l'éteint plutôt que de l'allumer par erreur.
	t.Setenv("SHOW_OAUTH_BUTTONS", "peut-etre")

	if cfg := Load(); cfg.ShowOAuthButtons {
		t.Error("une valeur inconnue doit valoir false")
	}
}

// sameRoles compare deux listes de rôles dans l'ordre.
func sameRoles(got, want []game.Role) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
