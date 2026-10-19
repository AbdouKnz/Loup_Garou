// Package oauth mène les connexions aux fournisseurs d'identité externes
// (Google, Discord).
//
// Un fournisseur n'est ici qu'une **description** : ses points d'entrée, sa
// portée, son émetteur et la façon de traduire sa réponse en identité. Le
// moteur, lui, n'est écrit qu'une fois : il fabrique l'adresse d'autorisation
// (`state` + PKCE S256), échange au retour le code contre un jeton, puis
// demande l'identité au fournisseur.
//
// Aucune signature n'est vérifiée localement : l'identité n'est pas lue dans un
// jeton, elle est demandée au fournisseur avec le jeton qu'il vient de
// délivrer. Ce qu'on vérifie, c'est que c'est bien lui qui répond : le `state`
// garantit que la tentative est la nôtre (RFC 6749), et l'émetteur annoncé —
// quand le fournisseur le donne (RFC 9207) — doit être celui attendu.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/abdou/loupgarou/internal/protocol"
)

// Noms des fournisseurs : ce sont eux qui apparaissent dans les URL
// (/api/auth/google/start) et dans le compte rattaché.
const (
	// NameGoogle est le nom du fournisseur Google.
	NameGoogle = "google"
	// NameDiscord est le nom du fournisseur Discord.
	NameDiscord = "discord"
)

// Intentions d'une tentative de connexion externe.
//
// Une tentative qui ouvre une session est un login ; celle qui rattache une
// identité à un compte déjà ouvert est un link.
const (
	// PurposeLogin ouvre (ou crée) la session d'un compte : c'est le cas
	// ordinaire du bouton « Continuer avec… ».
	PurposeLogin = "login"
	// PurposeLink rattache l'identité à un compte existant, sans en créer.
	PurposeLink = "link"
)

// Bornes de la mécanique de connexion.
const (
	// attemptTTL est le temps laissé à une connexion pour aboutir, du clic au
	// choix du pseudo. Au-delà, le joueur recommence : rien n'est perdu, sinon
	// une redirection.
	attemptTTL = 15 * time.Minute
	// secretLength est la taille des valeurs tirées au hasard (identifiant de
	// tentative, `state`, vérificateur PKCE), en octets.
	secretLength = 32
	// maxAttempts borne les connexions en cours : sans elle, une pluie
	// d'appels au départ d'une connexion ferait grossir la table sans fin.
	maxAttempts = 1000
	// requestTimeout borne l'attente d'une réponse du fournisseur.
	requestTimeout = 10 * time.Second
	// maxResponseLength borne ce qu'on accepte de lire d'une réponse.
	maxResponseLength = 1 << 20
)

// Échecs d'une connexion externe, déjà rédigés pour le joueur.
var (
	// errUnreachable : le fournisseur n'a pas répondu, ou a refusé l'échange.
	errUnreachable = protocol.NewError(protocol.CodeOAuthFailed,
		"La connexion avec le fournisseur a échoué. Réessayez.")
	// errUnreadable : la réponse du fournisseur n'a pas de sens.
	errUnreadable = protocol.NewError(protocol.CodeOAuthFailed,
		"Réponse inattendue du fournisseur.")
	// errExpired : la tentative n'existe plus, ou n'est pas la nôtre.
	errExpired = protocol.NewError(protocol.CodeOAuthState,
		"Cette connexion a expiré. Recommencez.")
)

// Identity est une identité vérifiée chez un fournisseur.
type Identity struct {
	// Provider est le fournisseur qui a vérifié l'identité.
	Provider string
	// ProviderID est l'identifiant stable chez ce fournisseur : c'est lui qui
	// rattache un compte à une personne, jamais l'adresse e-mail.
	ProviderID string
	// Email est l'adresse annoncée, vide si le fournisseur n'en donne pas.
	// C'est une information, pas une identité : on ne s'en sert pas pour se
	// connecter et on ne l'affiche pas.
	Email string
	// EmailVerified dit si le fournisseur garantit l'adresse. Seule une adresse
	// vérifiée sert à rattacher une identité à un compte existant : une adresse
	// non vérifiée ne prouve rien.
	EmailVerified bool
	// Suggested est le pseudo proposé par le fournisseur, vide s'il n'y en a
	// pas d'utilisable. Le joueur reste libre de le remplacer.
	Suggested string
}

// parseIdentity traduit la réponse d'identité d'un fournisseur.
type parseIdentity func(raw map[string]any) (Identity, error)

// Provider décrit un fournisseur d'identité.
type Provider struct {
	// Name est le nom court du fournisseur.
	Name string
	// ClientID et ClientSecret identifient l'application chez le fournisseur.
	// Sans eux, le fournisseur n'est pas proposé.
	ClientID     string
	ClientSecret string
	// AuthURL, TokenURL et UserInfoURL sont les points d'entrée du fournisseur.
	AuthURL     string
	TokenURL    string
	UserInfoURL string
	// Scope demande au fournisseur ce qu'on a le droit de lire.
	Scope string
	// AuthParams ajoute des paramètres à l'adresse d'autorisation.
	AuthParams url.Values
	// Issuers liste les émetteurs acceptés : ceux dont le fournisseur peut
	// annoncer le nom, au retour de la redirection ou dans sa réponse.
	Issuers []string
	// Identity traduit la réponse d'identité.
	Identity parseIdentity
}

// Ready dit si le fournisseur est utilisable : sans identifiants
// d'application, il n'y a rien à proposer au joueur.
func (p Provider) Ready() bool {
	return p.ClientID != "" && p.ClientSecret != "" && p.Identity != nil
}

// Google décrit le fournisseur Google.
//
// Les points d'entrée sont ceux que Google publie dans son document de
// découverte (https://accounts.google.com/.well-known/openid-configuration).
func Google(clientID, clientSecret string) Provider {
	return Provider{
		Name:         NameGoogle,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
		Scope:        "openid email profile",
		// Google demande quel compte utiliser : sans cela, il reprend
		// silencieusement la session déjà ouverte dans le navigateur.
		AuthParams: url.Values{"prompt": {"select_account"}},
		Issuers:    []string{"https://accounts.google.com", "accounts.google.com"},
		Identity:   googleIdentity,
	}
}

// googleIdentity lit la réponse d'identité de Google.
func googleIdentity(raw map[string]any) (Identity, error) {
	id, err := requireField(raw, "sub")
	if err != nil {
		return Identity{}, err
	}

	email := stringField(raw, "email")

	return Identity{
		ProviderID: id,
		Email:      email,
		// Google annonce `email_verified` dans son document d'identité.
		EmailVerified: boolField(raw, "email_verified"),
		// Google ne donne pas de pseudo : un prénom fait une meilleure
		// proposition qu'une adresse e-mail.
		Suggested: suggestPseudo(stringField(raw, "given_name"), stringField(raw, "name"), emailLocalPart(email)),
	}, nil
}

// Discord décrit le fournisseur Discord.
//
// Les points d'entrée sont ceux que Discord publie dans son document de
// découverte (https://discord.com/.well-known/openid-configuration). La portée
// `identify email` suffit, et `global_name` — le nom affiché — en fait partie :
// il est plus lisible que le pseudo historique.
func Discord(clientID, clientSecret string) Provider {
	return Provider{
		Name:         NameDiscord,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      "https://discord.com/api/oauth2/authorize",
		TokenURL:     "https://discord.com/api/oauth2/token",
		UserInfoURL:  "https://discord.com/api/users/@me",
		Scope:        "identify email",
		Issuers:      []string{"https://discord.com"},
		Identity:     discordIdentity,
	}
}

// discordIdentity lit la réponse d'identité de Discord.
func discordIdentity(raw map[string]any) (Identity, error) {
	id, err := requireField(raw, "id")
	if err != nil {
		return Identity{}, err
	}

	return Identity{
		ProviderID: id,
		Email:      stringField(raw, "email"),
		// Discord annonce `verified` sur l'utilisateur.
		EmailVerified: boolField(raw, "verified"),
		Suggested:     suggestPseudo(stringField(raw, "global_name"), stringField(raw, "username")),
	}, nil
}

// attempt est une connexion externe en cours.
type attempt struct {
	provider string
	state    string
	verifier string
	// purpose dit ce que la tentative cherche à faire (login ou link) ;
	// accountID est le compte visé, pour une liaison.
	purpose   string
	accountID string
	createdAt time.Time
	// identity est l'identité vérifiée qui attend le choix du pseudo.
	identity *Identity
	// failure est la raison du dernier échec, à annoncer une seule fois.
	failure *protocol.AppError
}

// Engine mène les connexions aux fournisseurs d'identité.
type Engine struct {
	mu        sync.Mutex
	providers map[string]Provider
	attempts  map[string]*attempt
	// publicURL est l'adresse publique du site : c'est elle qui compose
	// l'adresse de retour déclarée au fournisseur.
	publicURL string
	client    *http.Client
	now       func() time.Time
}

// New construit le moteur à partir des fournisseurs décrits.
func New(publicURL string, providers ...Provider) *Engine {
	engine := &Engine{
		providers: make(map[string]Provider, len(providers)),
		attempts:  make(map[string]*attempt),
		publicURL: strings.TrimRight(publicURL, "/"),
		client:    &http.Client{Timeout: requestTimeout},
		now:       time.Now,
	}

	for _, provider := range providers {
		engine.providers[provider.Name] = provider
	}

	return engine
}

// SetHTTPClient remplace le client HTTP du moteur.
//
// Les tests s'en servent pour répondre à la place du fournisseur, sans sortir
// de la machine.
func (e *Engine) SetHTTPClient(client *http.Client) {
	if client == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.client = client
}

// Configured dit si un fournisseur est utilisable : sans identifiants
// d'application, son bouton reste masqué et ses routes ne mènent nulle part.
func (e *Engine) Configured(name string) bool {
	provider, ok := e.providers[name]

	return ok && provider.Ready()
}

// Begin prépare une connexion et renvoie l'identifiant de la tentative ainsi
// que l'adresse où envoyer le navigateur.
//
// C'est cet identifiant que le cookie du navigateur transporte : au retour du
// fournisseur, il retrouve le `state` et le vérificateur PKCE, qui n'ont jamais
// quitté le serveur.
func (e *Engine) Begin(name string) (string, string, error) {
	return e.begin(name, PurposeLogin, "")
}

// BeginLink prépare le rattachement d'une identité à un compte existant : la
// tentative saura, au retour, qu'elle ne doit pas créer de compte.
func (e *Engine) BeginLink(name, accountID string) (string, string, error) {
	return e.begin(name, PurposeLink, accountID)
}

// begin prépare une tentative, quelle qu'en soit l'intention.
func (e *Engine) begin(name, purpose, accountID string) (string, string, error) {
	provider, ok := e.providers[name]
	if !ok || !provider.Ready() {
		return "", "", protocol.NewError(protocol.CodeNotImplemented,
			"Cette connexion n'est pas disponible sur ce serveur.")
	}

	id, err := newSecret()
	if err != nil {
		return "", "", err
	}

	state, err := newSecret()
	if err != nil {
		return "", "", err
	}

	verifier, err := newSecret()
	if err != nil {
		return "", "", err
	}

	now := e.now()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.purge(now)

	if len(e.attempts) >= maxAttempts {
		return "", "", protocol.NewError(protocol.CodeRateLimited,
			"Trop de connexions en cours. Réessayez dans un instant.")
	}

	e.attempts[id] = &attempt{
		provider:  name,
		state:     state,
		verifier:  verifier,
		purpose:   purpose,
		accountID: accountID,
		createdAt: now,
	}

	return id, e.authorizeURL(provider, state, verifier), nil
}

// Intention dit ce qu'une tentative cherche à faire : ouvrir une session
// (PurposeLogin) ou rattacher une identité (PurposeLink), et à quel compte.
func (e *Engine) Intention(id string) (purpose, accountID string, ok bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, found := e.attempts[id]
	if !found {
		return "", "", false
	}

	return item.purpose, item.accountID, true
}

// Callback rassemble ce que le fournisseur renvoie au retour de la redirection.
type Callback struct {
	// ID est la tentative en cours, telle que le cookie la porte.
	ID string
	// State est le paramètre de sécurité, remis tel quel par le fournisseur.
	State string
	// Code est le code à échanger contre un jeton.
	Code string
	// Issuer est l'émetteur annoncé (RFC 9207), vide si le fournisseur se tait.
	Issuer string
}

// Complete termine une connexion : elle vérifie l'état, échange le code contre
// un jeton, demande l'identité au fournisseur et la garde en attente du choix
// du pseudo.
func (e *Engine) Complete(ctx context.Context, cb Callback) (Identity, error) {
	e.mu.Lock()

	item, ok := e.attempts[cb.ID]

	var providerName, expectedState, verifier string

	var createdAt time.Time

	if ok {
		providerName, expectedState, verifier, createdAt = item.provider, item.state, item.verifier, item.createdAt
	}

	e.mu.Unlock()

	// Le `state` reçu doit être celui qu'on a envoyé, et le vérificateur doit
	// être encore là : sinon la tentative n'est pas la nôtre, ou elle a déjà
	// servi. Une tentative périmée ne vaut pas mieux qu'une tentative inconnue.
	if !ok || e.now().Sub(createdAt) >= attemptTTL ||
		expectedState == "" || verifier == "" || cb.State != expectedState {
		return Identity{}, errExpired
	}

	provider, ok := e.providers[providerName]
	if !ok {
		return Identity{}, errUnreachable
	}

	if err := checkIssuer(provider, cb.Issuer); err != nil {
		return Identity{}, err
	}

	raw, err := e.fetchIdentity(ctx, provider, cb.Code, verifier)
	if err != nil {
		return Identity{}, err
	}

	if err := checkIssuer(provider, stringField(raw, "iss")); err != nil {
		return Identity{}, err
	}

	identity, err := provider.Identity(raw)
	if err != nil {
		return Identity{}, err
	}

	identity.Provider = provider.Name

	// Le code est consommé : la tentative ne sert plus qu'à choisir un pseudo.
	e.mu.Lock()
	defer e.mu.Unlock()

	if current, ok := e.attempts[cb.ID]; ok {
		current.state = ""
		current.verifier = ""
		current.identity = &identity
	}

	return identity, nil
}

// Pending renvoie l'identité vérifiée qui attend un pseudo.
func (e *Engine) Pending(id string) (Identity, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.purge(e.now())

	item, ok := e.attempts[id]
	if !ok || item.identity == nil {
		return Identity{}, false
	}

	return *item.identity, true
}

// Forget oublie une tentative : le pseudo a été choisi, ou le joueur a
// recommencé.
func (e *Engine) Forget(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.attempts, id)
}

// TakeError retire, s'il y en a une, la raison du dernier échec.
//
// L'annonce est unique : à la visite suivante, le joueur lit pourquoi sa
// connexion a échoué, et l'écran redevient muet plutôt que de ressasser.
func (e *Engine) TakeError(id string) *protocol.AppError {
	if id == "" {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.purge(e.now())

	item, ok := e.attempts[id]
	if !ok || item.failure == nil {
		return nil
	}

	failure := item.failure
	delete(e.attempts, id)

	return failure
}

// Fail range l'échec d'une connexion et renvoie la tentative qui le porte :
// celle qui vient d'échouer, ou une nouvelle si elle a disparu — un échec doit
// pouvoir être annoncé même quand la tentative a expiré.
//
// Elle renvoie « » si le tirage au sort échoue : l'erreur est alors perdue, ce
// qui n'a aucune conséquence pour le joueur, qui recommence.
func (e *Engine) Fail(id string, err error) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.purge(e.now())

	if _, ok := e.attempts[id]; !ok {
		fresh, freshErr := newSecret()
		if freshErr != nil {
			return ""
		}

		id = fresh
		e.attempts[id] = &attempt{}
	}

	item := e.attempts[id]
	item.state = ""
	item.verifier = ""
	item.identity = nil
	item.failure = failureOf(err)
	item.createdAt = e.now()

	return id
}

// authorizeURL compose l'adresse d'autorisation du fournisseur.
//
// Le vérificateur n'y apparaît pas : seul son défi s'y trouve. C'est ce qui
// permet de ne pas confier de secret au navigateur.
func (e *Engine) authorizeURL(provider Provider, state, verifier string) string {
	params := url.Values{
		"client_id":             {provider.ClientID},
		"redirect_uri":          {e.redirectURI(provider.Name)},
		"response_type":         {"code"},
		"scope":                 {provider.Scope},
		"state":                 {state},
		"code_challenge":        {challengeOf(verifier)},
		"code_challenge_method": {"S256"},
	}

	for key, values := range provider.AuthParams {
		params[key] = values
	}

	return provider.AuthURL + "?" + params.Encode()
}

// redirectURI est l'adresse de retour déclarée au fournisseur.
func (e *Engine) redirectURI(name string) string {
	return e.publicURL + "/api/auth/" + name + "/callback"
}

// fetchIdentity échange le code contre un jeton, puis demande l'identité.
func (e *Engine) fetchIdentity(ctx context.Context, provider Provider, code, verifier string) (map[string]any, error) {
	token, err := e.exchangeCode(ctx, provider, code, verifier)
	if err != nil {
		return nil, err
	}

	return e.fetchUser(ctx, provider, token)
}

// exchangeCode échange le code contre un jeton d'accès.
//
// Le corps est toujours un formulaire : Discord refuse l'encodage JSON sur
// cette route, et Google accepte les deux.
func (e *Engine) exchangeCode(ctx context.Context, provider Provider, code, verifier string) (string, error) {
	form := url.Values{
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {e.redirectURI(provider.Name)},
		"code_verifier": {verifier},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	var payload struct {
		AccessToken string `json:"access_token"`
	}

	if err := e.call(request, &payload); err != nil {
		return "", err
	}

	if payload.AccessToken == "" {
		return "", errUnreadable
	}

	return payload.AccessToken, nil
}

// fetchUser demande l'identité au fournisseur, avec le jeton obtenu.
func (e *Engine) fetchUser(ctx context.Context, provider Provider, token string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")

	payload := make(map[string]any)
	if err := e.call(request, &payload); err != nil {
		return nil, err
	}

	if len(payload) == 0 {
		return nil, errUnreadable
	}

	return payload, nil
}

// call envoie une requête au fournisseur et décode sa réponse.
//
// Tout échec — réseau, statut, réponse illisible — devient la même erreur : le
// joueur n'a rien à faire d'un détail technique, et ce détail renseignerait un
// attaquant sur la configuration du serveur.
func (e *Engine) call(request *http.Request, out any) error {
	e.mu.Lock()
	client := e.client
	e.mu.Unlock()

	response, err := client.Do(request)
	if err != nil {
		return errUnreachable
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errUnreachable
	}

	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseLength)).Decode(out); err != nil {
		return errUnreadable
	}

	return nil
}

// purge retire les tentatives périmées. À appeler verrou pris.
func (e *Engine) purge(now time.Time) {
	for id, item := range e.attempts {
		if now.Sub(item.createdAt) >= attemptTTL {
			delete(e.attempts, id)
		}
	}
}

// checkIssuer refuse un émetteur qui n'est pas celui du fournisseur.
//
// Un fournisseur peut annoncer son nom deux fois : au retour de la redirection
// (RFC 9207) et dans sa réponse d'identité. Quand il le fait, la valeur doit
// être celle attendue ; quand il se tait, il n'y a rien à vérifier. C'est ce qui
// empêche un mélange de fournisseurs, où l'un répondrait à la place de l'autre.
func checkIssuer(provider Provider, issuer string) error {
	if issuer == "" || slices.Contains(provider.Issuers, issuer) {
		return nil
	}

	return errUnreadable
}

// failureOf garde le message destiné au joueur d'une erreur.
func failureOf(err error) *protocol.AppError {
	var appErr *protocol.AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return errUnreachable
}

// challengeOf calcule le défi PKCE d'un vérificateur (méthode S256).
func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// newSecret tire une valeur aléatoire : identifiant de tentative, `state` ou
// vérificateur PKCE.
func newSecret() (string, error) {
	buf := make([]byte, secretLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("secret de connexion : %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// stringField lit un champ texte d'une réponse JSON.
func stringField(raw map[string]any, field string) string {
	value, ok := raw[field].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(value)
}

// boolField lit un champ booléen d'une réponse JSON. Certains fournisseurs
// l'annoncent en chaîne (« true ») : les deux formes sont acceptées.
func boolField(raw map[string]any, field string) bool {
	switch value := raw[field].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

// requireField refuse une réponse sans le champ attendu.
func requireField(raw map[string]any, field string) (string, error) {
	value := stringField(raw, field)
	if value == "" {
		return "", errUnreadable
	}

	return value, nil
}

// suggestPseudo propose un pseudo acceptable parmi les noms annoncés par le
// fournisseur.
//
// Un nom trop long est raccourci plutôt que perdu ; si rien ne convient, c'est
// au joueur de choisir, et la case reste vide.
func suggestPseudo(candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		clean, err := protocol.NormalizePseudo(truncate(candidate, protocol.MaxPseudoLength))
		if err == nil {
			return clean
		}
	}

	return ""
}

// truncate coupe une chaîne à n caractères, sans couper un caractère en deux.
func truncate(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}

	return string([]rune(value)[:limit])
}

// emailLocalPart renvoie ce qui précède l'arobase d'une adresse.
func emailLocalPart(email string) string {
	local, _, found := strings.Cut(email, "@")
	if !found {
		return ""
	}

	return local
}
