package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/config"
	"github.com/abdou/loupgarou/internal/protocol"
)

// L'identité que le faux fournisseur annonce, et le nom qu'elle propose.
const (
	fakeIdentity = `{"sub":"42","email":"jean.dupont@example.com","given_name":"Jean","name":"Jean Dupont"}`
	fakeGoogleID = "42"
	// Les deux identités ci-dessous répondent à la fois au format Google et au
	// format Discord : de quoi jouer le même joueur d'un fournisseur à l'autre,
	// avec une adresse garantie… ou pas.
	fakeVerifiedIdentity = `{"sub":"42","id":"7","email":"jean.dupont@example.com",` +
		`"email_verified":true,"verified":true,"given_name":"Jean","name":"Jean Dupont",` +
		`"global_name":"Nelly","username":"nelly"}`
	fakeUnverifiedIdentity = `{"sub":"42","id":"7","email":"jean.dupont@example.com",` +
		`"email_verified":false,"verified":false,"given_name":"Jean","name":"Jean Dupont",` +
		`"global_name":"Nelly","username":"nelly"}`
)

// noRedirect laisse voir les redirections telles quelles : une connexion
// externe se juge à l'adresse qu'elle donne, pas à sa destination.
var noRedirect = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// redirectTransport adapte une fonction en http.RoundTripper.
type redirectTransport func(*http.Request) (*http.Response, error)

// RoundTrip implémente http.RoundTripper.
func (f redirectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

// fakeProvider répond à la place du fournisseur : échange du code, puis
// identité. Aucun test ne sort de la machine.
func fakeProvider(identity string) *http.Client {
	return &http.Client{Transport: redirectTransport(func(request *http.Request) (*http.Response, error) {
		body := identity
		if request.Method == http.MethodPost {
			body = `{"access_token":"jeton-acces","token_type":"Bearer"}`
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
}

// newOAuthServer démarre un serveur dont Google est configuré et branché sur un
// faux fournisseur. Il renvoie aussi le dossier des comptes, pour lire ce qui a
// été écrit.
func newOAuthServer(t *testing.T, identity string) (*Server, *httptest.Server, string) {
	t.Helper()

	data := t.TempDir()

	api := NewRouter(config.Config{
		Port:               "0",
		WebDist:            t.TempDir(),
		PublicURL:          "https://loup.example.com",
		AccountsFile:       filepath.Join(data, "accounts.json"),
		SessionsFile:       filepath.Join(data, "sessions.json"),
		SessionTTL:         time.Hour,
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
	})
	api.accounts.SetIterations(testIterations)
	api.oauth.SetHTTPClient(fakeProvider(identity))

	srv := httptest.NewServer(api)

	t.Cleanup(func() {
		srv.Close()
		api.Close()
	})

	return api, srv, data
}

// newProviderServer démarre un serveur dont Google *et* Discord sont configurés,
// tous deux branchés sur le même faux fournisseur : de quoi jouer le même
// joueur d'un fournisseur à l'autre.
func newProviderServer(t *testing.T, identity string) (*Server, *httptest.Server) {
	t.Helper()

	data := t.TempDir()

	api := NewRouter(config.Config{
		Port:                "0",
		WebDist:             t.TempDir(),
		PublicURL:           "https://loup.example.com",
		AccountsFile:        filepath.Join(data, "accounts.json"),
		SessionsFile:        filepath.Join(data, "sessions.json"),
		SessionTTL:          time.Hour,
		GoogleClientID:      "client-id",
		GoogleClientSecret:  "client-secret",
		DiscordClientID:     "discord-id",
		DiscordClientSecret: "discord-secret",
	})
	api.accounts.SetIterations(testIterations)
	api.oauth.SetHTTPClient(fakeProvider(identity))

	srv := httptest.NewServer(api)

	t.Cleanup(func() {
		srv.Close()
		api.Close()
	})

	return api, srv
}

// externalResponse est le résultat brut d'un appel qui redirige.
type externalResponse struct {
	status   int
	location string
	body     []byte
	// attempt est le cookie de tentative externe remis par le serveur.
	attempt string
	// forgetAttempt dit si le serveur demande de l'oublier.
	forgetAttempt bool
	// cookie est la session de compte, s'il y en a une.
	cookie string
}

// doExternal appelle une route de connexion externe sans suivre la redirection.
func doExternal(t *testing.T, method, url, body, attempt string) externalResponse {
	t.Helper()

	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}

	request, err := http.NewRequest(method, url, payload)
	if err != nil {
		t.Fatalf("requête impossible : %v", err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if attempt != "" {
		request.Header.Set("Cookie", oauthCookieName+"="+attempt)
	}

	response, err := noRedirect.Do(request)
	if err != nil {
		t.Fatalf("appel impossible : %v", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}

	out := externalResponse{
		status:   response.StatusCode,
		location: response.Header.Get("Location"),
		body:     raw,
	}

	for _, cookie := range response.Cookies() {
		switch cookie.Name {
		case oauthCookieName:
			if cookie.MaxAge < 0 || cookie.Value == "" {
				out.forgetAttempt = true
				continue
			}

			out.attempt = cookie.Value
		case sessionCookieName:
			if cookie.MaxAge >= 0 && cookie.Value != "" {
				out.cookie = cookie.Name + "=" + cookie.Value
			}
		}
	}

	return out
}

// errorCode extrait le code d'erreur applicatif d'une réponse qui ne redirige
// pas.
func (r externalResponse) errorCode(t *testing.T) protocol.ErrorCode {
	t.Helper()

	return apiResponse{body: r.body}.errorCode(t)
}

// startProvider lance une connexion et rend la tentative ainsi que le `state`
// donné au fournisseur.
func startProvider(t *testing.T, baseURL, provider string) (string, string) {
	t.Helper()

	response := doExternal(t, http.MethodGet, baseURL+"/api/auth/"+provider+"/start", "", "")

	if response.status != http.StatusFound {
		t.Fatalf("départ : statut = %d, corps = %s", response.status, response.body)
	}
	if response.attempt == "" {
		t.Fatal("le départ ne retient pas la tentative")
	}

	state := stateOf(t, response.location)
	if state == "" {
		t.Fatalf("aucun `state` dans l'adresse du fournisseur : %q", response.location)
	}

	return response.attempt, state
}

// stateOf extrait le `state` de l'adresse du fournisseur.
func stateOf(t *testing.T, target string) string {
	t.Helper()

	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("adresse illisible : %v", err)
	}

	return parsed.Query().Get("state")
}

// callbackProvider joue le retour du fournisseur.
func callbackProvider(t *testing.T, baseURL, attempt, state, code string) externalResponse {
	t.Helper()

	return callbackFor(t, baseURL, "google", attempt, state, code)
}

// callbackFor joue le retour d'un fournisseur nommé.
func callbackFor(t *testing.T, baseURL, provider, attempt, state, code string) externalResponse {
	t.Helper()

	return doExternal(t, http.MethodGet,
		baseURL+"/api/auth/"+provider+"/callback?code="+code+"&state="+state, "", attempt)
}

// linkProvider démarre le rattachement d'un fournisseur au nom d'une session, et
// rend la tentative ainsi que le `state` donné au fournisseur.
func linkProvider(t *testing.T, baseURL, provider, session string) (string, string) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/auth/"+provider+"/link", nil)
	if err != nil {
		t.Fatalf("requête impossible : %v", err)
	}
	request.Header.Set("Cookie", session)

	response, err := noRedirect.Do(request)
	if err != nil {
		t.Fatalf("appel impossible : %v", err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}

	if response.StatusCode != http.StatusFound {
		t.Fatalf("liaison : statut = %d, corps = %s", response.StatusCode, raw)
	}

	attempt := ""
	for _, cookie := range response.Cookies() {
		if cookie.Name == oauthCookieName && cookie.MaxAge >= 0 && cookie.Value != "" {
			attempt = cookie.Value
		}
	}
	if attempt == "" {
		t.Fatal("la liaison ne retient pas la tentative")
	}

	state := stateOf(t, response.Header.Get("Location"))
	if state == "" {
		t.Fatalf("aucun `state` dans l'adresse du fournisseur : %q", response.Header.Get("Location"))
	}

	return attempt, state
}

// completeProvider choisit le pseudo d'une connexion externe.
func completeProvider(t *testing.T, baseURL, attempt, username string) apiResponse {
	t.Helper()

	body, err := json.Marshal(completeRequest{Username: username})
	if err != nil {
		t.Fatalf("encodage impossible : %v", err)
	}

	return doJSONAs(t, http.MethodPost, baseURL+"/api/auth/complete", string(body),
		oauthCookieName+"="+attempt)
}

func TestLesFournisseursSontAnnonces(t *testing.T) {
	// Sans identifiants, rien n'est proposé.
	plain := newTestServer(t, t.TempDir())

	none := doJSON(t, http.MethodGet, plain.URL+"/api/auth/me", "").account(t)
	if none.Providers.Google || none.Providers.Discord {
		t.Errorf("fournisseurs = %+v", none.Providers)
	}

	// Avec les identifiants de Google, le bouton existe — et Discord, non.
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	account := doJSON(t, http.MethodGet, srv.URL+"/api/auth/me", "").account(t)
	if !account.Providers.Google {
		t.Error("Google n'est pas proposé alors qu'il est configuré")
	}
	if account.Providers.Discord {
		t.Error("Discord est proposé sans identifiants")
	}

	// La création d'un compte local annonce la même chose.
	signupResponse := doJSON(t, http.MethodPost, srv.URL+"/api/auth/signup", sessionBody(t, "Alice", testPassword))
	if !signupResponse.account(t).Providers.Google {
		t.Error("la réponse d'inscription ne dit pas quels fournisseurs existent")
	}
}

func TestLesBoutonsPeuventEtreForcesEnDeveloppement(t *testing.T) {
	// SHOW_OAUTH_BUTTONS montre les deux boutons sans identifiants : de quoi
	// voir et valider l'interface. Les routes, elles, restent muettes.
	data := t.TempDir()

	api := NewRouter(config.Config{
		Port:             "0",
		WebDist:          t.TempDir(),
		AccountsFile:     filepath.Join(data, "accounts.json"),
		SessionsFile:     filepath.Join(data, "sessions.json"),
		SessionTTL:       time.Hour,
		ShowOAuthButtons: true,
	})
	api.accounts.SetIterations(testIterations)

	srv := httptest.NewServer(api)

	t.Cleanup(func() {
		srv.Close()
		api.Close()
	})

	account := doJSON(t, http.MethodGet, srv.URL+"/api/auth/me", "").account(t)
	if !account.Providers.Google || !account.Providers.Discord {
		t.Errorf("fournisseurs = %+v, attendu les deux", account.Providers)
	}

	// Même affiché, un fournisseur sans identifiants ne mène nulle part.
	start := doExternal(t, http.MethodGet, srv.URL+"/api/auth/discord/start", "", "")
	if start.status != http.StatusNotImplemented {
		t.Errorf("statut = %d, attendu %d", start.status, http.StatusNotImplemented)
	}
}

func TestLeRetourRevientSurLeFront(t *testing.T) {
	// En développement, l'interface vit sur un autre port que l'API : le retour
	// doit y ramener le navigateur plutôt qu'à la racine du serveur Go.
	data := t.TempDir()

	api := NewRouter(config.Config{
		Port:               "0",
		WebDist:            t.TempDir(),
		PublicURL:          "http://localhost:8080",
		FrontURL:           "http://localhost:5173",
		AccountsFile:       filepath.Join(data, "accounts.json"),
		SessionsFile:       filepath.Join(data, "sessions.json"),
		SessionTTL:         time.Hour,
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
	})
	api.accounts.SetIterations(testIterations)
	api.oauth.SetHTTPClient(fakeProvider(fakeIdentity))

	srv := httptest.NewServer(api)

	t.Cleanup(func() {
		srv.Close()
		api.Close()
	})

	// Étapes réussies : identité nouvelle (pseudo à choisir) et compte connu.
	attempt, state := startProvider(t, srv.URL, "google")
	callback := callbackProvider(t, srv.URL, attempt, state, "code")

	if callback.status != http.StatusFound || callback.location != "http://localhost:5173/" {
		t.Fatalf("retour : statut = %d, destination = %q", callback.status, callback.location)
	}

	// Échec aussi : l'interface lit la raison par GET /api/auth/me, sur son
	// propre port.
	failedAttempt, _ := startProvider(t, srv.URL, "google")
	failed := callbackProvider(t, srv.URL, failedAttempt, "state-etranger", "code")

	if failed.location != "http://localhost:5173/" {
		t.Errorf("échec : destination = %q", failed.location)
	}
}

func TestLeDepartEnvoieChezLeFournisseur(t *testing.T) {
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	response := doExternal(t, http.MethodGet, srv.URL+"/api/auth/google/start", "", "")

	if response.status != http.StatusFound {
		t.Fatalf("statut = %d, corps = %s", response.status, response.body)
	}
	if !strings.HasPrefix(response.location, "https://accounts.google.com/o/oauth2/v2/auth?") {
		t.Errorf("destination = %q", response.location)
	}
	if response.attempt == "" {
		t.Error("aucune tentative retenue")
	}

	// Un fournisseur sans identifiants ne mène nulle part.
	unknown := doExternal(t, http.MethodGet, srv.URL+"/api/auth/discord/start", "", "")
	if unknown.status != http.StatusNotImplemented {
		t.Errorf("statut = %d, attendu %d", unknown.status, http.StatusNotImplemented)
	}
	if code := unknown.errorCode(t); code != protocol.CodeNotImplemented {
		t.Errorf("code = %q", code)
	}
}

func TestLeRetourDonneUnCompte(t *testing.T) {
	_, srv, data := newOAuthServer(t, fakeIdentity)

	attempt, state := startProvider(t, srv.URL, "google")

	// Le retour ne parle pas JSON : il ramène le navigateur à l'écran de
	// connexion, qui demandera le pseudo.
	callback := callbackProvider(t, srv.URL, attempt, state, "code-du-fournisseur")
	if callback.status != http.StatusFound || callback.location != "/" {
		t.Fatalf("retour : statut = %d, destination = %q, corps = %s",
			callback.status, callback.location, callback.body)
	}
	if callback.cookie != "" {
		t.Error("le compte est ouvert avant que le pseudo soit choisi")
	}

	waiting := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		oauthCookieName+"="+attempt).account(t)
	if waiting.Authenticated {
		t.Error("personne ne devrait être connecté")
	}
	if waiting.Pending == nil || waiting.Pending.Provider != "google" || waiting.Pending.Suggested != "Jean" {
		t.Fatalf("connexion en attente = %+v", waiting.Pending)
	}

	// Le pseudo peut être remplacé : celui du fournisseur n'est qu'une
	// proposition.
	completed := completeProvider(t, srv.URL, attempt, "Jeannot")
	if completed.status != http.StatusCreated {
		t.Fatalf("choix du pseudo : statut = %d, corps = %s", completed.status, completed.body)
	}
	if completed.cookie == "" {
		t.Fatal("aucune session ouverte")
	}
	if account := completed.account(t); !account.Authenticated || account.Username != "Jeannot" {
		t.Errorf("identité = %+v", account)
	}

	identified := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", completed.cookie).account(t)
	if !identified.Authenticated || identified.Username != "Jeannot" {
		t.Errorf("après le choix du pseudo : %+v", identified)
	}

	// La connexion est terminée : la tentative ne sert plus à rien, ni dans le
	// navigateur, ni sur le serveur.
	after := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", oauthCookieName+"="+attempt).account(t)
	if after.Pending != nil {
		t.Errorf("la tentative survit au choix du pseudo : %+v", after.Pending)
	}

	// Le compte garde son rattachement, et aucun mot de passe n'est écrit.
	raw, err := os.ReadFile(filepath.Join(data, "accounts.json"))
	if err != nil {
		t.Fatalf("comptes illisibles : %v", err)
	}

	var content struct {
		Accounts []struct {
			Username     string `json:"username"`
			Provider     string `json:"provider"`
			ProviderID   string `json:"providerId"`
			Email        string `json:"email"`
			PasswordHash string `json:"passwordHash"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatalf("comptes illisibles : %v", err)
	}

	if len(content.Accounts) != 1 {
		t.Fatalf("comptes = %+v", content.Accounts)
	}

	saved := content.Accounts[0]
	if saved.Provider != "google" || saved.ProviderID != fakeGoogleID {
		t.Errorf("rattachement = %q/%q", saved.Provider, saved.ProviderID)
	}
	if saved.PasswordHash != "" {
		t.Error("un compte externe a un mot de passe")
	}
	if saved.Email != "jean.dupont@example.com" {
		t.Errorf("adresse = %q", saved.Email)
	}
}

func TestLePseudoChoisiDoitEtreLibre(t *testing.T) {
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	// Un compte local occupe déjà le pseudo.
	signup(t, srv.URL, "Alice")

	attempt, state := startProvider(t, srv.URL, "google")
	if callback := callbackProvider(t, srv.URL, attempt, state, "code"); callback.status != http.StatusFound {
		t.Fatalf("retour : statut = %d, corps = %s", callback.status, callback.body)
	}

	taken := completeProvider(t, srv.URL, attempt, "alice")
	if taken.status != http.StatusConflict {
		t.Fatalf("statut = %d, corps = %s", taken.status, taken.body)
	}
	if code := taken.errorCode(t); code != protocol.CodeUsernameTaken {
		t.Errorf("code = %q", code)
	}
	if taken.cookie != "" {
		t.Error("aucune session ne doit s'ouvrir sur un refus")
	}

	// Le refus ne ferme pas la tentative : le joueur essaie un autre pseudo
	// sans repasser par le fournisseur.
	waiting := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		oauthCookieName+"="+attempt).account(t)
	if waiting.Pending == nil {
		t.Fatalf("la tentative a été perdue : %+v", waiting)
	}

	second := completeProvider(t, srv.URL, attempt, "Bob")
	if second.status != http.StatusCreated {
		t.Fatalf("second essai : statut = %d, corps = %s", second.status, second.body)
	}
	if account := second.account(t); account.Username != "Bob" {
		t.Errorf("pseudo = %q", account.Username)
	}
}

func TestLaReconnexionRouvreLeCompte(t *testing.T) {
	_, srv, data := newOAuthServer(t, fakeIdentity)

	attempt, state := startProvider(t, srv.URL, "google")
	callbackProvider(t, srv.URL, attempt, state, "code")
	first := completeProvider(t, srv.URL, attempt, "Jean")
	if first.status != http.StatusCreated {
		t.Fatalf("création : statut = %d, corps = %s", first.status, first.body)
	}

	// Deuxième passage : le rattachement suffit, aucun pseudo n'est demandé.
	attempt, state = startProvider(t, srv.URL, "google")

	second := callbackProvider(t, srv.URL, attempt, state, "code")
	if second.status != http.StatusFound || second.cookie == "" {
		t.Fatalf("reconnexion : statut = %d, corps = %s", second.status, second.body)
	}

	account := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", second.cookie).account(t)
	if !account.Authenticated || account.Username != "Jean" {
		t.Errorf("identité = %+v", account)
	}
	if account.Pending != nil {
		t.Errorf("un pseudo est redemandé : %+v", account.Pending)
	}

	// Un seul compte : la reconnexion n'en ouvre pas un second.
	raw, err := os.ReadFile(filepath.Join(data, "accounts.json"))
	if err != nil {
		t.Fatalf("comptes illisibles : %v", err)
	}
	if count := strings.Count(string(raw), "\"username\""); count != 1 {
		t.Errorf("comptes écrits = %d", count)
	}
}

func TestLEchecDEConnexionEstAnnonce(t *testing.T) {
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	attempt, _ := startProvider(t, srv.URL, "google")

	// Un retour dont le `state` n'est pas le nôtre : la tentative n'est pas
	// poursuivie, mais l'échec doit s'expliquer.
	failed := callbackProvider(t, srv.URL, attempt, "state-etranger", "code")
	if failed.status != http.StatusFound || failed.location != "/" {
		t.Fatalf("échec : statut = %d, destination = %q", failed.status, failed.location)
	}
	if failed.attempt == "" {
		t.Fatal("l'échec n'est retenu nulle part")
	}

	announced := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		oauthCookieName+"="+failed.attempt).account(t)
	if announced.Error == nil {
		t.Fatalf("aucun échec annoncé : %+v", announced)
	}
	if announced.Error.Code != protocol.CodeOAuthState {
		t.Errorf("code = %q, attendu %q", announced.Error.Code, protocol.CodeOAuthState)
	}
	if announced.Error.Message == "" {
		t.Error("l'échec n'a pas d'explication pour le joueur")
	}
	if announced.Pending != nil {
		t.Errorf("une connexion en attente traîne : %+v", announced.Pending)
	}

	// L'annonce est unique : au rechargement suivant, l'écran est muet.
	again := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		oauthCookieName+"="+failed.attempt).account(t)
	if again.Error != nil {
		t.Errorf("échec annoncé deux fois : %+v", again.Error)
	}
}

func TestLeRefusDUFournisseurEstAnnonce(t *testing.T) {
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	attempt, _ := startProvider(t, srv.URL, "google")

	// Le joueur a refusé l'accès : c'est un cas normal, annoncé comme tel.
	refused := doExternal(t, http.MethodGet,
		srv.URL+"/api/auth/google/callback?error=access_denied", "", attempt)
	if refused.status != http.StatusFound {
		t.Fatalf("statut = %d", refused.status)
	}

	announced := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		oauthCookieName+"="+refused.attempt).account(t)
	if announced.Error == nil || announced.Error.Code != protocol.CodeOAuthFailed {
		t.Fatalf("échec annoncé = %+v", announced.Error)
	}
}

func TestLePseudoNeSeChoisitPasSansConnexion(t *testing.T) {
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	// Sans tentative : la route ne peut rien créer.
	withoutAttempt := completeProvider(t, srv.URL, "", "Jean")
	if withoutAttempt.status != http.StatusBadRequest {
		t.Fatalf("statut = %d, corps = %s", withoutAttempt.status, withoutAttempt.body)
	}
	if code := withoutAttempt.errorCode(t); code != protocol.CodeOAuthState {
		t.Errorf("code = %q", code)
	}

	// Avec une tentative qui n'a jamais vu de fournisseur : non plus.
	attempt, _ := startProvider(t, srv.URL, "google")

	early := completeProvider(t, srv.URL, attempt, "Jean")
	if early.status != http.StatusBadRequest {
		t.Fatalf("statut = %d, corps = %s", early.status, early.body)
	}
	if code := early.errorCode(t); code != protocol.CodeOAuthState {
		t.Errorf("code = %q", code)
	}

	// Un pseudo invalide est refusé sans rien perdre.
	invalidAttempt, invalidState := startProvider(t, srv.URL, "google")
	callbackProvider(t, srv.URL, invalidAttempt, invalidState, "code")

	invalid := completeProvider(t, srv.URL, invalidAttempt, "A")
	if invalid.status != http.StatusBadRequest {
		t.Fatalf("statut = %d, corps = %s", invalid.status, invalid.body)
	}
	if code := invalid.errorCode(t); code != protocol.CodeInvalidPseudo {
		t.Errorf("code = %q", code)
	}
}

func TestLaLiaisonExigeUneSession(t *testing.T) {
	_, srv, _ := newOAuthServer(t, fakeIdentity)

	// Sans session, il n'y a rien à rattacher : le navigateur revient au guichet,
	// sans tentative.
	lonely := doExternal(t, http.MethodGet, srv.URL+"/api/auth/google/link", "", "")
	if lonely.status != http.StatusFound || lonely.location != "/" {
		t.Fatalf("liaison sans session : statut = %d, destination = %q", lonely.status, lonely.location)
	}
	if lonely.attempt != "" {
		t.Error("une tentative a été ouverte sans compte à rattacher")
	}
}

func TestLiaisonExpliciteNeCreePasDeCompte(t *testing.T) {
	api, srv, _ := newOAuthServer(t, fakeIdentity)

	// Un compte local, ouvert normalement.
	session := signup(t, srv.URL, "Alice")
	if api.accounts.Count() != 1 {
		t.Fatalf("comptes = %d, attendu 1", api.accounts.Count())
	}

	// Connecté, Alice associe Google.
	attempt, state := linkProvider(t, srv.URL, "google", session)

	callback := callbackProvider(t, srv.URL, attempt, state, "code")
	if callback.status != http.StatusFound || callback.location != "/" {
		t.Fatalf("retour : statut = %d, destination = %q, corps = %s",
			callback.status, callback.location, callback.body)
	}

	// Aucun compte de plus : l'identité a rejoint celui d'Alice.
	if api.accounts.Count() != 1 {
		t.Fatalf("comptes = %d, attendu 1", api.accounts.Count())
	}

	// Et l'interface sait quelles portes sont déjà ouvertes.
	me := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", session).account(t)
	if !me.Authenticated || me.Username != "Alice" {
		t.Fatalf("identité = %+v", me)
	}
	if !slices.Contains(me.Linked, "google") {
		t.Errorf("linked = %v, attendu google", me.Linked)
	}
	if slices.Contains(me.Linked, "discord") {
		t.Errorf("linked = %v, Discord ne devrait pas y être", me.Linked)
	}
}

func TestLiaisonExpliciteRefuseeSurUnAutreCompte(t *testing.T) {
	api, srv, _ := newOAuthServer(t, fakeIdentity)

	alice := signup(t, srv.URL, "Alice")
	bob := signup(t, srv.URL, "Bob")

	// Alice rattache Google.
	attempt, state := linkProvider(t, srv.URL, "google", alice)
	callbackProvider(t, srv.URL, attempt, state, "code")

	// Bob tente la même identité : elle est déjà prise, on ne la lui donne pas.
	attempt, state = linkProvider(t, srv.URL, "google", bob)

	failed := callbackProvider(t, srv.URL, attempt, state, "code")
	if failed.status != http.StatusFound {
		t.Fatalf("statut = %d, corps = %s", failed.status, failed.body)
	}

	// L'échec est annoncé à Bob, une fois, sur son propre écran de connexion.
	announced := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		bob+"; "+oauthCookieName+"="+failed.attempt).account(t)
	if announced.Error == nil || announced.Error.Code != protocol.CodeOAuthLinkedElsewhere {
		t.Fatalf("échec annoncé = %+v", announced.Error)
	}
	if announced.Username != "Bob" {
		t.Errorf("pseudo = %q", announced.Username)
	}
	if slices.Contains(announced.Linked, "google") {
		t.Errorf("linked = %v, Google ne doit pas être rattaché à Bob", announced.Linked)
	}

	// Personne n'a bougé : toujours deux comptes, Google reste chez Alice.
	if api.accounts.Count() != 2 {
		t.Errorf("comptes = %d, attendu 2", api.accounts.Count())
	}

	aliceMe := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", alice).account(t)
	if !slices.Contains(aliceMe.Linked, "google") {
		t.Errorf("linked d'Alice = %v, Google devrait y être", aliceMe.Linked)
	}
}

func TestAutoLiaisonSurEmailVerifie(t *testing.T) {
	api, srv := newProviderServer(t, fakeVerifiedIdentity)

	// Premier passage par Google : l'identité est nouvelle, un pseudo est choisi.
	attempt, state := startProvider(t, srv.URL, "google")
	callbackProvider(t, srv.URL, attempt, state, "code")

	created := completeProvider(t, srv.URL, attempt, "Jean")
	if created.status != http.StatusCreated {
		t.Fatalf("création : statut = %d, corps = %s", created.status, created.body)
	}
	if api.accounts.Count() != 1 {
		t.Fatalf("comptes = %d, attendu 1", api.accounts.Count())
	}

	// Second passage, par Discord, avec le même e-mail vérifié : le joueur garde
	// son compte au lieu d'en ouvrir un second.
	attempt, state = startProvider(t, srv.URL, "discord")

	second := callbackFor(t, srv.URL, "discord", attempt, state, "code")
	if second.status != http.StatusFound || second.cookie == "" {
		t.Fatalf("auto-liaison : statut = %d, corps = %s", second.status, second.body)
	}
	if api.accounts.Count() != 1 {
		t.Fatalf("un second compte a été créé : %d", api.accounts.Count())
	}

	me := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "", second.cookie).account(t)
	if !me.Authenticated || me.Username != "Jean" {
		t.Fatalf("identité = %+v", me)
	}
	if !slices.Contains(me.Linked, "google") || !slices.Contains(me.Linked, "discord") {
		t.Errorf("linked = %v, attendu google et discord", me.Linked)
	}
}

func TestPasDAutoLiaisonSansEmailVerifie(t *testing.T) {
	api, srv := newProviderServer(t, fakeUnverifiedIdentity)

	// Un premier compte, par Google.
	attempt, state := startProvider(t, srv.URL, "google")
	callbackProvider(t, srv.URL, attempt, state, "code")
	if created := completeProvider(t, srv.URL, attempt, "Jean"); created.status != http.StatusCreated {
		t.Fatalf("création : statut = %d, corps = %s", created.status, created.body)
	}
	if api.accounts.Count() != 1 {
		t.Fatalf("comptes = %d, attendu 1", api.accounts.Count())
	}

	// Discord annonce le même e-mail, mais non vérifié : il ne prouve rien, donc
	// aucune session ne s'ouvre et aucun compte n'est rattaché ; le joueur devra
	// choisir un pseudo.
	attempt, state = startProvider(t, srv.URL, "discord")

	second := callbackFor(t, srv.URL, "discord", attempt, state, "code")
	if second.cookie != "" {
		t.Error("une session s'est ouverte sur un e-mail non vérifié")
	}
	if api.accounts.Count() != 1 {
		t.Fatalf("comptes = %d, attendu 1", api.accounts.Count())
	}

	waiting := doJSONAs(t, http.MethodGet, srv.URL+"/api/auth/me", "",
		oauthCookieName+"="+attempt).account(t)
	if waiting.Authenticated {
		t.Error("personne ne devrait être connecté")
	}
	if waiting.Pending == nil || waiting.Pending.Provider != "discord" {
		t.Fatalf("connexion en attente = %+v", waiting.Pending)
	}
}
