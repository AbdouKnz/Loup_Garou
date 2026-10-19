package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/config"
	"github.com/abdou/loupgarou/internal/protocol"
)

// sessionBody encode un corps d'authentification.
func sessionBody(t *testing.T, username, password string) string {
	t.Helper()

	body, err := json.Marshal(authRequest{Username: username, Password: password})
	if err != nil {
		t.Fatalf("encodage impossible : %v", err)
	}

	return string(body)
}

// account extrait la réponse d'identité.
func (r apiResponse) account(t *testing.T) accountResponse {
	t.Helper()

	var out accountResponse
	if err := json.Unmarshal(r.body, &out); err != nil {
		t.Fatalf("identité illisible (%s) : %v", r.body, err)
	}

	return out
}

func TestCreerUnCompteOuvreUneSession(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	response := doJSON(t, http.MethodPost, srv.URL+"/api/auth/signup", sessionBody(t, "  Alice  ", "motdepasse"))
	if response.status != http.StatusCreated {
		t.Fatalf("statut = %d, corps = %s", response.status, response.body)
	}

	account := response.account(t)
	if !account.Authenticated || account.Username != "Alice" {
		t.Errorf("identité = %+v", account)
	}
	if response.cookie == "" {
		t.Fatal("aucun cookie de session")
	}

	// Sans le cookie, personne ; avec, on sait qui parle.
	anonymous := doJSON(t, http.MethodGet, srv.URL+"/api/auth/me", "").account(t)
	if anonymous.Authenticated || anonymous.Username != "" {
		t.Errorf("identité sans cookie = %+v", anonymous)
	}

	identified := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", response.cookie).account(t)
	if !identified.Authenticated || identified.Username != "Alice" {
		t.Errorf("identité avec cookie = %+v", identified)
	}
}

func TestCreerUnCompteRefuse(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	if response := doJSON(t, http.MethodPost, srv.URL+"/api/auth/signup", sessionBody(t, "Alice", "motdepasse")); response.status != http.StatusCreated {
		t.Fatalf("création : statut = %d, corps = %s", response.status, response.body)
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   protocol.ErrorCode
	}{
		{
			name: "pseudo trop court", body: `{"username":"A","password":"motdepasse"}`,
			wantStatus: http.StatusBadRequest, wantCode: protocol.CodeInvalidPseudo,
		},
		{
			name: "pseudo absent", body: `{"password":"motdepasse"}`,
			wantStatus: http.StatusBadRequest, wantCode: protocol.CodeInvalidPseudo,
		},
		{
			name: "mot de passe trop court", body: `{"username":"Bob","password":"court"}`,
			wantStatus: http.StatusBadRequest, wantCode: protocol.CodeWeakPassword,
		},
		{
			name: "champ inconnu", body: `{"username":"Bob","password":"motdepasse","triche":true}`,
			wantStatus: http.StatusBadRequest, wantCode: protocol.CodeInvalidMessage,
		},
		{
			name: "corps illisible", body: `{`,
			wantStatus: http.StatusBadRequest, wantCode: protocol.CodeInvalidMessage,
		},
		{
			name: "pseudo déjà pris", body: `{"username":"alice","password":"motdepasse"}`,
			wantStatus: http.StatusConflict, wantCode: protocol.CodeUsernameTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := doJSON(t, http.MethodPost, srv.URL+"/api/auth/signup", tt.body)

			if response.status != tt.wantStatus {
				t.Errorf("statut = %d, attendu %d (corps %s)", response.status, tt.wantStatus, response.body)
			}
			if code := response.errorCode(t); code != tt.wantCode {
				t.Errorf("code = %q, attendu %q", code, tt.wantCode)
			}
		})
	}
}

func TestSeConnecterPuisSeDeconnecter(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	signup(t, srv.URL, "Alice")

	cookie := loginCookie(t, srv.URL, "alice")
	if cookie == "" {
		t.Fatal("aucun cookie de session")
	}

	response := doJSONAs(t, http.MethodPost, srv.URL+"/api/auth/logout", "", cookie)
	if response.status != http.StatusNoContent {
		t.Fatalf("déconnexion : statut = %d, corps = %s", response.status, response.body)
	}

	// Le jeton ne vaut plus rien, et le navigateur reçoit de quoi l'oublier.
	if identified := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", cookie).account(t); identified.Authenticated {
		t.Errorf("après déconnexion : %+v", identified)
	}
	if !response.forget {
		t.Errorf("le cookie devrait être effacé, réponse %+v", response)
	}
}

func TestSeConnecterRefuse(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	signup(t, srv.URL, "Alice")

	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "mot de passe faux", username: "Alice", password: "motdepasse2"},
		{name: "pseudo inconnu", username: "Bob", password: "motdepasse"},
		{name: "pseudo vide", username: "", password: "motdepasse"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := doJSON(t, http.MethodPost, srv.URL+"/api/auth/login", sessionBody(t, tt.username, tt.password))

			if response.status != http.StatusUnauthorized {
				t.Errorf("statut = %d, attendu %d", response.status, http.StatusUnauthorized)
			}
			if code := response.errorCode(t); code != protocol.CodeBadCredentials {
				t.Errorf("code = %q, attendu %q", code, protocol.CodeBadCredentials)
			}
			if response.cookie != "" {
				t.Errorf("cookie inattendu : %q", response.cookie)
			}
		})
	}
}

// TestLesTentativesSontLimitees vérifie qu'une adresse ne peut pas essayer des
// mots de passe en série.
func TestLesTentativesSontLimitees(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	signup(t, srv.URL, "Alice")
	body := sessionBody(t, "Alice", "motdepasse-faux")

	refused := false
	for i := 0; i < attemptsPerMinute+2; i++ {
		response := doJSON(t, http.MethodPost, srv.URL+"/api/auth/login", body)
		if response.status == http.StatusTooManyRequests {
			refused = true
			if code := response.errorCode(t); code != protocol.CodeRateLimited {
				t.Errorf("code = %q, attendu %q", code, protocol.CodeRateLimited)
			}

			break
		}
	}

	if !refused {
		t.Fatalf("après %d essais, la limitation n'est pas intervenue", attemptsPerMinute+2)
	}
}

// TestLesComptesEtSessionsSurviventAuRedemarrage monte un serveur sur des
// fichiers, l'arrête, puis rouvre : le compte et sa session doivent être
// toujours là.
func TestLesComptesEtSessionsSurviventAuRedemarrage(t *testing.T) {
	data := t.TempDir()

	start := func() (*Server, *httptest.Server) {
		api := NewRouter(config.Config{
			Port:         "0",
			WebDist:      t.TempDir(),
			AccountsFile: filepath.Join(data, "accounts.json"),
			SessionsFile: filepath.Join(data, "sessions.json"),
			SessionTTL:   time.Hour,
		})
		api.accounts.SetIterations(testIterations)

		return api, httptest.NewServer(api)
	}

	api, srv := start()

	cookie := signup(t, srv.URL, "Alice")

	srv.Close()
	api.Close()

	// Le deuxième serveur ne connaît que ses fichiers : c'est tout l'intérêt.
	second, secondSrv := start()
	defer second.Close()

	defer secondSrv.Close()

	response := doJSONAs(t, http.MethodGet, secondSrv.URL+"/api/auth/me", "", cookie)
	if account := response.account(t); !account.Authenticated || account.Username != "Alice" {
		t.Errorf("après redémarrage : %+v", account)
	}

	// Le mot de passe fonctionne toujours, et la partie se crée de même.
	loginCookie(t, secondSrv.URL, "Alice")
	createRoomAs(t, secondSrv.URL, cookie)
}

// TestLesSecretsNeSontPasEcritsEnClair vérifie l'état du dossier de données :
// le mot de passe est haché, le jeton de session absent.
func TestLesSecretsNeSontPasEcritsEnClair(t *testing.T) {
	data := t.TempDir()

	api := NewRouter(config.Config{
		Port:         "0",
		WebDist:      t.TempDir(),
		AccountsFile: filepath.Join(data, "accounts.json"),
		SessionsFile: filepath.Join(data, "sessions.json"),
		SessionTTL:   time.Hour,
	})
	api.accounts.SetIterations(testIterations)

	srv := httptest.NewServer(api)
	defer srv.Close()

	defer api.Close()

	cookie := signup(t, srv.URL, "Alice")
	token := strings.TrimPrefix(cookie, sessionCookieName+"=")

	accountsRaw, err := os.ReadFile(filepath.Join(data, "accounts.json"))
	if err != nil {
		t.Fatalf("comptes illisibles : %v", err)
	}
	if strings.Contains(string(accountsRaw), testPassword) {
		t.Error("le mot de passe apparaît en clair dans le fichier des comptes")
	}

	sessionsRaw, err := os.ReadFile(filepath.Join(data, "sessions.json"))
	if err != nil {
		t.Fatalf("sessions illisibles : %v", err)
	}
	if strings.Contains(string(sessionsRaw), token) {
		t.Error("le jeton apparaît en clair dans le fichier des sessions")
	}
}
