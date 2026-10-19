package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/abdou/loupgarou/internal/protocol"
)

// testPassword est le mot de passe des comptes de test.
const testPassword = "motdepasse"

// apiResponse est le résultat brut d'un appel HTTP de test.
type apiResponse struct {
	status int
	body   []byte
	// cookie est la session de compte remise par le serveur, s'il y en a une.
	cookie string
	// forget dit si la réponse demande au navigateur d'oublier le cookie.
	forget bool
}

// doJSON envoie une requête JSON sans session de compte.
func doJSON(t *testing.T, method, url, body string) apiResponse {
	t.Helper()

	return doJSONAs(t, method, url, body, "")
}

// doJSONAs envoie une requête JSON au nom du compte identifié par cookie.
func doJSONAs(t *testing.T, method, url, body, cookie string) apiResponse {
	t.Helper()

	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		t.Fatalf("requête impossible : %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("appel impossible : %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("lecture de la réponse impossible : %v", err)
	}

	cookie, forget := sessionCookie(resp)

	return apiResponse{status: resp.StatusCode, body: raw, cookie: cookie, forget: forget}
}

// sessionCookie extrait le cookie de session d'une réponse, et dit si cette
// réponse demande au navigateur de l'oublier.
func sessionCookie(resp *http.Response) (string, bool) {
	for _, cookie := range resp.Cookies() {
		if cookie.Name != sessionCookieName {
			continue
		}
		if cookie.MaxAge < 0 || cookie.Value == "" {
			return "", true
		}

		return cookie.Name + "=" + cookie.Value, false
	}

	return "", false
}

// signup crée un compte et renvoie le cookie de sa session.
func signup(t *testing.T, baseURL, username string) string {
	t.Helper()

	body, err := json.Marshal(authRequest{Username: username, Password: testPassword})
	if err != nil {
		t.Fatalf("encodage impossible : %v", err)
	}

	response := doJSON(t, http.MethodPost, baseURL+"/api/auth/signup", string(body))
	if response.status != http.StatusCreated {
		t.Fatalf("création de compte : statut = %d, corps = %s", response.status, response.body)
	}
	if response.cookie == "" {
		t.Fatal("la création d'un compte devrait ouvrir une session")
	}

	return response.cookie
}

// loginCookie se connecte avec un compte existant et renvoie sa session.
func loginCookie(t *testing.T, baseURL, username string) string {
	t.Helper()

	body, err := json.Marshal(authRequest{Username: username, Password: testPassword})
	if err != nil {
		t.Fatalf("encodage impossible : %v", err)
	}

	response := doJSON(t, http.MethodPost, baseURL+"/api/auth/login", string(body))
	if response.status != http.StatusOK {
		t.Fatalf("connexion : statut = %d, corps = %s", response.status, response.body)
	}

	return response.cookie
}

// session extrait une session de la réponse.
func (r apiResponse) session(t *testing.T) sessionResponse {
	t.Helper()

	var out sessionResponse
	if err := json.Unmarshal(r.body, &out); err != nil {
		t.Fatalf("session illisible (%s) : %v", r.body, err)
	}

	return out
}

// errorCode extrait le code d'erreur applicatif de la réponse.
func (r apiResponse) errorCode(t *testing.T) protocol.ErrorCode {
	t.Helper()

	var payload protocol.ErrorPayload
	if err := json.Unmarshal(r.body, &payload); err != nil {
		t.Fatalf("erreur illisible (%s) : %v", r.body, err)
	}
	if payload.Message == "" {
		t.Error("le message d'erreur destiné au joueur est vide")
	}

	return payload.Code
}

// createRoom ouvre une partie au nom d'un nouveau compte.
func createRoom(t *testing.T, baseURL, pseudo string) sessionResponse {
	t.Helper()

	return createRoomAs(t, baseURL, signup(t, baseURL, pseudo))
}

// createRoomAs ouvre une partie au nom du compte identifié par cookie.
func createRoomAs(t *testing.T, baseURL, cookie string) sessionResponse {
	t.Helper()

	response := doJSONAs(t, http.MethodPost, baseURL+"/api/rooms", "", cookie)
	if response.status != http.StatusCreated {
		t.Fatalf("création : statut = %d, corps = %s", response.status, response.body)
	}

	return response.session(t)
}

// joinRoom fait entrer un nouveau compte dans une partie existante.
func joinRoom(t *testing.T, baseURL, code, pseudo string) sessionResponse {
	t.Helper()

	return joinRoomAs(t, baseURL, code, signup(t, baseURL, pseudo))
}

// joinRoomAs fait entrer le compte identifié par cookie dans une partie.
func joinRoomAs(t *testing.T, baseURL, code, cookie string) sessionResponse {
	t.Helper()

	response := doJSONAs(t, http.MethodPost, baseURL+"/api/rooms/"+code+"/join", "", cookie)
	if response.status != http.StatusOK {
		t.Fatalf("entrée : statut = %d, corps = %s", response.status, response.body)
	}

	return response.session(t)
}

func TestCreationDePartie(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	session := createRoom(t, srv.URL, "  Alice  ")

	if _, err := protocol.NormalizeRoomCode(session.Code); err != nil {
		t.Errorf("code renvoyé invalide (%q) : %v", session.Code, err)
	}
	if session.Pseudo != "Alice" {
		t.Errorf("pseudo = %q, attendu %q", session.Pseudo, "Alice")
	}
	if session.PlayerID == "" {
		t.Error("identifiant du joueur manquant")
	}
	if len(session.Token) < 20 {
		t.Errorf("jeton de session trop court : %q", session.Token)
	}

	// Le résumé d'une partie est consultable sans être connecté.
	response := doJSON(t, http.MethodGet, srv.URL+"/api/rooms/"+session.Code, "")
	if response.status != http.StatusOK {
		t.Fatalf("résumé : statut = %d, corps = %s", response.status, response.body)
	}

	var summary protocol.RoomSummary
	if err := json.Unmarshal(response.body, &summary); err != nil {
		t.Fatalf("résumé illisible : %v", err)
	}
	if summary.Code != session.Code || summary.Players != 1 {
		t.Errorf("résumé inattendu : %+v", summary)
	}
	if summary.MaxPlayers != protocol.MaxPlayers {
		t.Errorf("maxPlayers = %d, attendu %d", summary.MaxPlayers, protocol.MaxPlayers)
	}
	if summary.Phase != protocol.PhaseLobby {
		t.Errorf("phase = %q, attendu %q", summary.Phase, protocol.PhaseLobby)
	}
}

func TestLesSallesDemandentUnCompte(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	tests := []struct {
		name string
		path string
	}{
		{name: "création", path: "/api/rooms"},
		{name: "entrée", path: "/api/rooms/ABC234/join"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := doJSON(t, http.MethodPost, srv.URL+tt.path, "")

			if response.status != http.StatusUnauthorized {
				t.Errorf("statut = %d, attendu %d (corps %s)", response.status, http.StatusUnauthorized, response.body)
			}
			if code := response.errorCode(t); code != protocol.CodeNotAuthenticated {
				t.Errorf("code = %q, attendu %q", code, protocol.CodeNotAuthenticated)
			}
		})
	}

	// Un cookie inventé ne vaut pas mieux que pas de cookie.
	response := doJSONAs(t, http.MethodPost, srv.URL+"/api/rooms", "", sessionCookieName+"=jeton-inventé")
	if response.status != http.StatusUnauthorized {
		t.Errorf("cookie inconnu : statut = %d, attendu %d", response.status, http.StatusUnauthorized)
	}
}

func TestRejoindreUnePartie(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	host := createRoom(t, srv.URL, "Alice")

	bob := joinRoom(t, srv.URL, host.Code, "Bob")
	if bob.Code != host.Code {
		t.Errorf("code = %q, attendu %q", bob.Code, host.Code)
	}
	if bob.PlayerID == host.PlayerID || bob.Token == host.Token {
		t.Error("Bob doit avoir sa propre identité")
	}

	// Le code est insensible à la casse.
	lower := strings.ToLower(host.Code)
	chloe := joinRoom(t, srv.URL, lower, "Chloé")
	if chloe.Code != host.Code {
		t.Errorf("code = %q, attendu %q", chloe.Code, host.Code)
	}

	response := doJSON(t, http.MethodGet, srv.URL+"/api/rooms/"+host.Code, "")
	var summary protocol.RoomSummary
	if err := json.Unmarshal(response.body, &summary); err != nil {
		t.Fatalf("résumé illisible : %v", err)
	}
	if summary.Players != 3 {
		t.Errorf("joueurs = %d, attendu 3", summary.Players)
	}
}

func TestRejoindreRefuse(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	createRoom(t, srv.URL, "Alice")
	cookie := signup(t, srv.URL, "Bob")

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   protocol.ErrorCode
	}{
		{
			name: "partie inconnue", path: "/api/rooms/ZZZZZZ/join",
			wantStatus: http.StatusNotFound, wantCode: protocol.CodeRoomNotFound,
		},
		{
			name: "code mal formé", path: "/api/rooms/ABC/join",
			wantStatus: http.StatusBadRequest, wantCode: protocol.CodeInvalidCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := doJSONAs(t, http.MethodPost, srv.URL+tt.path, "", cookie)

			if response.status != tt.wantStatus {
				t.Errorf("statut = %d, attendu %d (corps %s)", response.status, tt.wantStatus, response.body)
			}
			if code := response.errorCode(t); code != tt.wantCode {
				t.Errorf("code = %q, attendu %q", code, tt.wantCode)
			}
		})
	}
}

func TestRejouerLEntreeRendLaMemePlace(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	host := createRoom(t, srv.URL, "Alice")
	cookie := signup(t, srv.URL, "Bob")

	first := joinRoomAs(t, srv.URL, host.Code, cookie)
	second := joinRoomAs(t, srv.URL, host.Code, cookie)

	if second.PlayerID != first.PlayerID || second.Token != first.Token {
		t.Errorf("seconde entrée %+v, attendue identique à %+v", second, first)
	}

	// La table compte deux joueurs : Bob n'a pas été annoncé deux fois.
	response := doJSON(t, http.MethodGet, srv.URL+"/api/rooms/"+host.Code, "")

	var summary protocol.RoomSummary
	if err := json.Unmarshal(response.body, &summary); err != nil {
		t.Fatalf("résumé illisible : %v", err)
	}
	if summary.Players != 2 {
		t.Errorf("joueurs = %d, attendu 2", summary.Players)
	}

	// L'hôte aussi retrouve sa place : sa session de siège ne se duplique pas.
	hostAgain := joinRoomAs(t, srv.URL, host.Code, loginCookie(t, srv.URL, "Alice"))
	if hostAgain.PlayerID != host.PlayerID {
		t.Errorf("l'hôte a changé de place : %q, attendu %q", hostAgain.PlayerID, host.PlayerID)
	}
}

func TestResumeDePartieInconnue(t *testing.T) {
	srv := newTestServer(t, t.TempDir())

	response := doJSON(t, http.MethodGet, srv.URL+"/api/rooms/ZZZZZZ", "")
	if response.status != http.StatusNotFound {
		t.Errorf("statut = %d, attendu %d", response.status, http.StatusNotFound)
	}
	if code := response.errorCode(t); code != protocol.CodeRoomNotFound {
		t.Errorf("code = %q, attendu %q", code, protocol.CodeRoomNotFound)
	}
}

func TestPartieCompleteRefusee(t *testing.T) {
	srv := newTestServer(t, t.TempDir())
	host := createRoom(t, srv.URL, "Joueur 1")

	// L'hôte occupe la première place : il en reste MaxPlayers-1.
	for i := 2; i <= protocol.MaxPlayers; i++ {
		joinRoom(t, srv.URL, host.Code, "Joueur "+strconv.Itoa(i))
	}

	cookie := signup(t, srv.URL, "Trop")
	response := doJSONAs(t, http.MethodPost, srv.URL+"/api/rooms/"+host.Code+"/join", "", cookie)
	if response.status != http.StatusConflict {
		t.Errorf("statut = %d, attendu %d (corps %s)", response.status, http.StatusConflict, response.body)
	}
	if code := response.errorCode(t); code != protocol.CodeRoomFull {
		t.Errorf("code = %q, attendu %q", code, protocol.CodeRoomFull)
	}
}
