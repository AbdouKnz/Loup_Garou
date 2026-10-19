package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abdou/loupgarou/internal/protocol"
)

// Les réponses des fournisseurs, telles que les points d'entrée les rendent.
const (
	fakeToken = `{"access_token":"jeton-acces","token_type":"Bearer","expires_in":3600}`
	fakeUser  = `{"sub":"42","id":"42","email":"jean.dupont@example.com","given_name":"Jean","username":"jean.dupont"}`
)

// transportFunc adapte une fonction en http.RoundTripper.
type transportFunc func(*http.Request) (*http.Response, error)

// RoundTrip implémente http.RoundTripper.
func (f transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

// fakeProvider joue le rôle du fournisseur : il retient ce qu'on lui envoie et
// rend ce que le test a prévu, sans jamais sortir de la machine.
type fakeProvider struct {
	mu     sync.Mutex
	token  string
	user   string
	status int
	form   url.Values
	seen   []string
}

// newFakeProvider prépare un fournisseur qui accepte tout.
func newFakeProvider() *fakeProvider {
	return &fakeProvider{token: fakeToken, user: fakeUser, status: http.StatusOK}
}

// client branche le moteur sur ce fournisseur.
func (f *fakeProvider) client() *http.Client {
	return &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		body := f.user

		f.mu.Lock()
		f.seen = append(f.seen, request.Method+" "+request.URL.Path)
		if request.Method == http.MethodPost {
			raw, err := io.ReadAll(request.Body)
			if err != nil {
				f.mu.Unlock()

				return nil, err
			}

			f.form, _ = url.ParseQuery(string(raw))
			body = f.token
		}
		status := f.status
		f.mu.Unlock()

		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
}

// calls dit combien d'appels le fournisseur a reçus.
func (f *fakeProvider) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.seen)
}

// sent retrouve un champ du formulaire envoyé à l'échange de jeton.
func (f *fakeProvider) sent(t *testing.T, field string) string {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.form == nil {
		t.Fatal("aucun échange de jeton n'a été envoyé")

		return ""
	}

	return f.form.Get(field)
}

// testEngine prépare un moteur dont Google est branché sur un faux fournisseur.
func testEngine(t *testing.T) (*Engine, *fakeProvider) {
	t.Helper()

	fake := newFakeProvider()
	engine := New("https://loup.example.com", Google("client-id", "client-secret"))
	engine.SetHTTPClient(fake.client())

	return engine, fake
}

// begin lance une connexion et renvoie la tentative, son `state` et l'adresse
// donnée au navigateur (dont le `state` et le défi PKCE se lisent).
func begin(t *testing.T, engine *Engine, provider string) (string, string, url.Values) {
	t.Helper()

	id, target, err := engine.Begin(provider)
	if err != nil {
		t.Fatalf("départ impossible : %v", err)
	}

	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatalf("adresse illisible : %v", err)
	}

	params := parsed.Query()

	return id, params.Get("state"), params
}

func TestLeDepartEnvoieChezLeFournisseur(t *testing.T) {
	engine, _ := testEngine(t)

	_, state, params := begin(t, engine, NameGoogle)

	if params.Get("client_id") != "client-id" {
		t.Errorf("client_id = %q", params.Get("client_id"))
	}
	if params.Get("response_type") != "code" {
		t.Errorf("response_type = %q", params.Get("response_type"))
	}
	if params.Get("scope") != "openid email profile" {
		t.Errorf("scope = %q", params.Get("scope"))
	}
	if params.Get("prompt") != "select_account" {
		t.Errorf("prompt = %q", params.Get("prompt"))
	}
	if want := "https://loup.example.com/api/auth/google/callback"; params.Get("redirect_uri") != want {
		t.Errorf("redirect_uri = %q, attendu %q", params.Get("redirect_uri"), want)
	}
	if params.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q", params.Get("code_challenge_method"))
	}
	if params.Get("code_challenge") == "" {
		t.Error("aucun défi PKCE")
	}
	if len(state) < 20 {
		t.Errorf("state trop court : %q", state)
	}

	// Le vérificateur ne part jamais chez le navigateur : l'adresse ne porte que
	// son défi, et le secret reste au serveur.
	for _, value := range params {
		if value[0] == params.Get("code_challenge") {
			continue
		}
		if strings.Contains(value[0], "verifier") {
			t.Errorf("l'adresse parle de vérificateur : %v", value)
		}
	}
}

func TestSansIdentifiantsRienNEstPropose(t *testing.T) {
	engine := New("http://localhost:8080", Google("", ""), Discord("", ""))

	if engine.Configured(NameGoogle) {
		t.Error("Google est proposé sans identifiants")
	}
	if engine.Configured(NameDiscord) {
		t.Error("Discord est proposé sans identifiants")
	}

	for _, name := range []string{NameGoogle, NameDiscord, "facebook"} {
		_, _, err := engine.Begin(name)
		if code := protocol.CodeOf(err); code != protocol.CodeNotImplemented {
			t.Errorf("%s : code = %q, attendu %q", name, code, protocol.CodeNotImplemented)
		}
	}
}

func TestLeRetourExigeLeBonEtat(t *testing.T) {
	engine, fake := testEngine(t)
	ctx := context.Background()

	id, state, _ := begin(t, engine, NameGoogle)

	tests := []struct {
		name string
		call Callback
	}{
		{name: "state étranger", call: Callback{ID: id, State: "celui-d-un-autre", Code: "code"}},
		{name: "state absent", call: Callback{ID: id, Code: "code"}},
		{name: "tentative inconnue", call: Callback{ID: "inconnue", State: state, Code: "code"}},
		{name: "aucune tentative", call: Callback{State: state, Code: "code"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.Complete(ctx, tt.call)
			if code := protocol.CodeOf(err); code != protocol.CodeOAuthState {
				t.Errorf("code = %q, attendu %q", code, protocol.CodeOAuthState)
			}

			// Une tentative qui n'est pas la nôtre ne mérite pas d'appel au
			// fournisseur : c'est le sens du `state`.
			if fake.calls() != 0 {
				t.Errorf("le fournisseur a été appelé %d fois", fake.calls())
			}
		})
	}

	// La tentative est intacte : un retour légitime passe encore.
	if _, err := engine.Complete(ctx, Callback{ID: id, State: state, Code: "code"}); err != nil {
		t.Fatalf("retour légitime refusé : %v", err)
	}
}

func TestLeRetourDonneLIdentite(t *testing.T) {
	engine, fake := testEngine(t)
	ctx := context.Background()

	id, state, params := begin(t, engine, NameGoogle)

	identity, err := engine.Complete(ctx, Callback{ID: id, State: state, Code: "code-du-fournisseur"})
	if err != nil {
		t.Fatalf("retour refusé : %v", err)
	}

	if identity.Provider != NameGoogle {
		t.Errorf("fournisseur = %q", identity.Provider)
	}
	if identity.ProviderID != "42" {
		t.Errorf("identifiant = %q, attendu 42", identity.ProviderID)
	}
	if identity.Email != "jean.dupont@example.com" {
		t.Errorf("adresse = %q", identity.Email)
	}
	if identity.Suggested != "Jean" {
		t.Errorf("pseudo proposé = %q, attendu Jean", identity.Suggested)
	}

	// L'échange est un formulaire, et le vérificateur envoyé est bien celui dont
	// le défi a été confié au navigateur.
	if got := fake.sent(t, "code_verifier"); challengeOf(got) != params.Get("code_challenge") {
		t.Errorf("le vérificateur ne correspond pas au défi : %q", got)
	}
	if fake.sent(t, "grant_type") != "authorization_code" {
		t.Errorf("grant_type = %q", fake.sent(t, "grant_type"))
	}
	if fake.sent(t, "redirect_uri") != "https://loup.example.com/api/auth/google/callback" {
		t.Errorf("redirect_uri = %q", fake.sent(t, "redirect_uri"))
	}

	// L'identité attend le pseudo, et le code ne sert plus deux fois.
	if pending, ok := engine.Pending(id); !ok || pending.ProviderID != "42" {
		t.Errorf("identité en attente = %+v, %v", pending, ok)
	}
	if _, err := engine.Complete(ctx, Callback{ID: id, State: state, Code: "code"}); protocol.CodeOf(err) != protocol.CodeOAuthState {
		t.Errorf("rejeu accepté : %v", err)
	}
}

func TestLEmetteurEstVerifie(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		user    string
		issuer  string
		wantErr bool
	}{
		{name: "Discord annonce son nom", user: `{"id":"7","username":"nelly"}`, issuer: "https://discord.com"},
		{
			name: "l'identité annonce le même nom",
			user: `{"id":"7","username":"nelly","iss":"https://discord.com"}`,
		},
		{
			name:    "un autre émetteur au retour",
			user:    `{"id":"7","username":"nelly"}`,
			issuer:  "https://accounts.google.com",
			wantErr: true,
		},
		{
			name:    "un autre émetteur dans la réponse",
			user:    `{"id":"7","username":"nelly","iss":"https://exemple.test"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeProvider()
			fake.user = tt.user

			engine := New("https://loup.example.com", Discord("client-id", "client-secret"))
			engine.SetHTTPClient(fake.client())

			id, state, _ := begin(t, engine, NameDiscord)

			_, err := engine.Complete(ctx, Callback{
				ID:     id,
				State:  state,
				Code:   "code",
				Issuer: tt.issuer,
			})

			if tt.wantErr {
				if code := protocol.CodeOf(err); code != protocol.CodeOAuthFailed {
					t.Fatalf("code = %q, attendu %q", code, protocol.CodeOAuthFailed)
				}

				return
			}

			if err != nil {
				t.Fatalf("retour refusé : %v", err)
			}
		})
	}
}

func TestLeFournisseurEnPanne(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		user string
		body string
	}{
		{name: "réponse sans jeton", user: `{}`, body: "{}"},
		{name: "identité sans identifiant", user: `{"email":"x@example.com"}`},
		{name: "identité illisible", user: "pas du json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, fake := testEngine(t)
			if tt.user != "" {
				fake.user = tt.user
			}
			if tt.body != "" {
				fake.token = tt.body
			}

			id, state, _ := begin(t, engine, NameGoogle)

			_, err := engine.Complete(ctx, Callback{ID: id, State: state, Code: "code"})
			if code := protocol.CodeOf(err); code != protocol.CodeOAuthFailed {
				t.Errorf("code = %q, attendu %q", code, protocol.CodeOAuthFailed)
			}
		})
	}
}

func TestUneTentativePerimee(t *testing.T) {
	engine, _ := testEngine(t)

	base := time.Now()
	engine.now = func() time.Time { return base }

	id, state, _ := begin(t, engine, NameGoogle)

	// Le joueur a laissé l'onglet ouvert et revient plus tard.
	engine.now = func() time.Time { return base.Add(attemptTTL + time.Second) }

	_, err := engine.Complete(context.Background(), Callback{ID: id, State: state, Code: "code"})
	if code := protocol.CodeOf(err); code != protocol.CodeOAuthState {
		t.Errorf("code = %q, attendu %q", code, protocol.CodeOAuthState)
	}
}

func TestLesTentativesSontBornees(t *testing.T) {
	engine, _ := testEngine(t)

	for i := 0; i < maxAttempts; i++ {
		if _, _, err := engine.Begin(NameGoogle); err != nil {
			t.Fatalf("départ %d refusé : %v", i, err)
		}
	}

	_, _, err := engine.Begin(NameGoogle)
	if code := protocol.CodeOf(err); code != protocol.CodeRateLimited {
		t.Errorf("code = %q, attendu %q", code, protocol.CodeRateLimited)
	}
}

func TestLEchecEstAnnonceUneFois(t *testing.T) {
	engine, _ := testEngine(t)

	id, _, _ := begin(t, engine, NameGoogle)

	// Un échec se range dans la tentative en cours…
	failure := protocol.NewError(protocol.CodeOAuthFailed, "Connexion annulée.")
	if next := engine.Fail(id, failure); next != id {
		t.Errorf("tentative d'échec = %q, attendu %q", next, id)
	}

	if got := engine.TakeError(id); got == nil || got.Message != "Connexion annulée." {
		t.Errorf("échec annoncé = %+v", got)
	}

	// …et ne s'annonce qu'une fois.
	if got := engine.TakeError(id); got != nil {
		t.Errorf("échec annoncé deux fois : %+v", got)
	}

	// Une tentative inconnue en reçoit une : un échec doit pouvoir s'annoncer
	// même quand la tentative a expiré.
	orphan := engine.Fail("disparue", failure)
	if orphan == "" {
		t.Fatal("aucune tentative pour porter l'échec")
	}
	if got := engine.TakeError(orphan); got == nil || got.Message != "Connexion annulée." {
		t.Errorf("échec annoncé = %+v", got)
	}
}

func TestUneErreurTechniqueDevientUnMessagePourLeJoueur(t *testing.T) {
	engine, _ := testEngine(t)

	id, _, _ := begin(t, engine, NameGoogle)

	// Une panne réseau n'a pas de message : le joueur en reçoit un quand même.
	if next := engine.Fail(id, io.ErrUnexpectedEOF); next == "" {
		t.Fatal("aucune tentative pour porter l'échec")
	}

	if got := engine.TakeError(id); got == nil || got.Code != protocol.CodeOAuthFailed {
		t.Errorf("échec = %+v", got)
	}
}

func TestLePseudoPropose(t *testing.T) {
	long := strings.Repeat("a", protocol.MaxPseudoLength+5)

	tests := []struct {
		name  string
		parse parseIdentity
		raw   string
		want  string
	}{
		{
			name:  "Google donne le prénom",
			parse: googleIdentity,
			raw:   `{"sub":"1","given_name":"Jean","name":"Jean Dupont","email":"j@example.com"}`,
			want:  "Jean",
		},
		{
			name:  "Google à défaut le nom",
			parse: googleIdentity,
			raw:   `{"sub":"1","name":"Jean Dupont","email":"j@example.com"}`,
			want:  "Jean Dupont",
		},
		{
			name:  "Google à défaut l'adresse",
			parse: googleIdentity,
			raw:   `{"sub":"1","email":"jean.dupont@example.com"}`,
			want:  "jean.dupont",
		},
		{
			name:  "Google sans rien d'utilisable",
			parse: googleIdentity,
			raw:   `{"sub":"1","email":"a@example.com"}`,
			want:  "",
		},
		{
			name:  "un nom trop long est raccourci",
			parse: googleIdentity,
			raw:   `{"sub":"1","given_name":"` + long + `"}`,
			want:  long[:protocol.MaxPseudoLength],
		},
		{
			name:  "Discord donne le nom affiché",
			parse: discordIdentity,
			raw:   `{"id":"7","global_name":"Nelly","username":"nelly"}`,
			want:  "Nelly",
		},
		{
			name:  "Discord à défaut le pseudo",
			parse: discordIdentity,
			raw:   `{"id":"7","username":"nelly"}`,
			want:  "nelly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]any{}
			if err := json.Unmarshal([]byte(tt.raw), &raw); err != nil {
				t.Fatalf("réponse illisible : %v", err)
			}

			identity, err := tt.parse(raw)
			if err != nil {
				t.Fatalf("identité refusée : %v", err)
			}

			if identity.Suggested != tt.want {
				t.Errorf("pseudo proposé = %q, attendu %q", identity.Suggested, tt.want)
			}
		})
	}
}

func TestLIdentiteExigeSonIdentifiant(t *testing.T) {
	tests := []struct {
		name  string
		parse parseIdentity
		raw   string
	}{
		{name: "Google", parse: googleIdentity, raw: `{"email":"x@example.com"}`},
		{name: "Discord", parse: discordIdentity, raw: `{"username":"nelly"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]any{}
			if err := json.Unmarshal([]byte(tt.raw), &raw); err != nil {
				t.Fatalf("réponse illisible : %v", err)
			}

			if _, err := tt.parse(raw); protocol.CodeOf(err) != protocol.CodeOAuthFailed {
				t.Errorf("erreur = %v", err)
			}
		})
	}
}

func TestLEmailVerifie(t *testing.T) {
	tests := []struct {
		name  string
		parse parseIdentity
		raw   string
		want  bool
	}{
		{
			name: "Google vérifié", parse: googleIdentity,
			raw: `{"sub":"1","email":"j@example.com","email_verified":true}`, want: true,
		},
		{
			name: "Google non vérifié", parse: googleIdentity,
			raw: `{"sub":"1","email":"j@example.com","email_verified":false}`, want: false,
		},
		{
			name: "Google muet", parse: googleIdentity,
			raw: `{"sub":"1","email":"j@example.com"}`, want: false,
		},
		{
			name: "Google vérifié en chaîne", parse: googleIdentity,
			raw: `{"sub":"1","email":"j@example.com","email_verified":"true"}`, want: true,
		},
		{
			name: "Discord vérifié", parse: discordIdentity,
			raw: `{"id":"7","username":"nelly","verified":true}`, want: true,
		},
		{
			name: "Discord non vérifié", parse: discordIdentity,
			raw: `{"id":"7","username":"nelly","verified":false}`, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]any{}
			if err := json.Unmarshal([]byte(tt.raw), &raw); err != nil {
				t.Fatalf("réponse illisible : %v", err)
			}

			identity, err := tt.parse(raw)
			if err != nil {
				t.Fatalf("identité refusée : %v", err)
			}
			if identity.EmailVerified != tt.want {
				t.Errorf("EmailVerified = %v, attendu %v", identity.EmailVerified, tt.want)
			}
		})
	}
}

func TestLIntentionDuneTentative(t *testing.T) {
	engine, _ := testEngine(t)

	// Une connexion ordinaire ouvre une session.
	login, _, err := engine.Begin(NameGoogle)
	if err != nil {
		t.Fatalf("départ impossible : %v", err)
	}
	if purpose, accountID, ok := engine.Intention(login); !ok || purpose != PurposeLogin || accountID != "" {
		t.Errorf("intention = %q/%q, %v, attendu login", purpose, accountID, ok)
	}

	// Une liaison retient le compte visé.
	link, _, err := engine.BeginLink(NameGoogle, "compte-1")
	if err != nil {
		t.Fatalf("liaison impossible : %v", err)
	}
	if purpose, accountID, ok := engine.Intention(link); !ok || purpose != PurposeLink || accountID != "compte-1" {
		t.Errorf("intention = %q/%q, %v, attendu link sur compte-1", purpose, accountID, ok)
	}

	// Une tentative inconnue n'a pas d'intention.
	if _, _, ok := engine.Intention("inconnue"); ok {
		t.Error("une tentative inconnue ne devrait pas avoir d'intention")
	}
}
