package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/accounts"
	"github.com/abdou/loupgarou/internal/config"
	"github.com/abdou/loupgarou/internal/game"
)

// testIterations est le coût du dérivatage des mots de passe dans les tests :
// ce qui compte ici, c'est le chemin, pas la résistance à la force brute.
const testIterations = accounts.MinIterations

// newTestServer démarre un serveur de test sur un dossier web factice.
func newTestServer(t *testing.T, webDist string) *httptest.Server {
	t.Helper()

	return newTestServerWithGame(t, webDist, game.Config{})
}

// newTestServerWithGame démarre un serveur dont les durées de partie sont
// réglées par le test (les phases par défaut sont trop longues pour un test).
//
// Comptes et sessions vivent dans un dossier temporaire : un test ne laisse
// rien derrière lui et n'hérite de rien.
func newTestServerWithGame(t *testing.T, webDist string, party game.Config) *httptest.Server {
	t.Helper()

	data := t.TempDir()

	api := NewRouter(config.Config{
		Port:         "0",
		WebDist:      webDist,
		Game:         party,
		AccountsFile: filepath.Join(data, "accounts.json"),
		SessionsFile: filepath.Join(data, "sessions.json"),
		SessionTTL:   time.Hour,
	})
	api.accounts.SetIterations(testIterations)

	srv := httptest.NewServer(api)

	t.Cleanup(func() {
		srv.Close()
		api.Close()
	})

	return srv
}

func TestHealthRepondOK(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	resp, err := http.Get(srv.URL + "/api/health") //nolint:noctx // test
	if err != nil {
		t.Fatalf("requête impossible : %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("statut = %d, attendu %d", resp.StatusCode, http.StatusOK)
	}

	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("réponse illisible : %v", err)
	}
	if payload["status"] != "ok" {
		t.Errorf("status = %q, attendu %q", payload["status"], "ok")
	}
	if payload["service"] != "loupgarou" {
		t.Errorf("service = %q, attendu %q", payload["service"], "loupgarou")
	}
}

func TestInterfaceNonConstruiteSignaleLErreur(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	resp, err := http.Get(srv.URL + "/nimporte-quoi") //nolint:noctx // test
	if err != nil {
		t.Fatalf("requête impossible : %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("statut = %d, attendu %d", resp.StatusCode, http.StatusNotFound)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "npm run build") {
		t.Errorf("message inattendu : %s", body)
	}
}

func TestInterfaceSertIndexEtRoutesReact(t *testing.T) {
	webDist := t.TempDir()
	index := filepath.Join(webDist, "index.html")

	if err := os.WriteFile(index, []byte("<!doctype html><title>Loup-Garou</title>"), 0o644); err != nil {
		t.Fatalf("impossible d'écrire index.html : %v", err)
	}

	srv := newTestServer(t, webDist)

	for _, route := range []string{"/", "/partie/abc123"} {
		resp, err := http.Get(srv.URL + route) //nolint:noctx // test
		if err != nil {
			t.Fatalf("requête %s impossible : %v", route, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s : statut = %d, attendu %d", route, resp.StatusCode, http.StatusOK)
		}
		if !strings.Contains(string(body), "Loup-Garou") {
			t.Errorf("%s : index.html non servi, corps = %q", route, body)
		}
	}
}

func TestMethodeNonAutoriseeRefusee(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	resp, err := http.Post(srv.URL+"/", "application/json", strings.NewReader("{}")) //nolint:noctx // test
	if err != nil {
		t.Fatalf("requête impossible : %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("statut = %d, attendu %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
