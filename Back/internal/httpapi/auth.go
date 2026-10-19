package httpapi

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/abdou/loupgarou/internal/accounts"
	"github.com/abdou/loupgarou/internal/protocol"
)

// sessionCookieName est le nom du cookie qui porte la session de compte.
//
// Ce cookie autorise les appels à l'API ; le jeton de siège, lui, est remis à
// la création ou à l'entrée dans une partie et sert à ouvrir le WebSocket.
const sessionCookieName = "lg_session"

// attemptsPerMinute borne les tentatives de connexion d'une même adresse.
const attemptsPerMinute = 10

// authRequest est le corps de la création de compte et de la connexion.
type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// accountResponse décrit la session de compte pour l'interface.
//
// Elle porte aussi l'état du serveur en matière de connexion : quels
// fournisseurs externes sont utilisables, s'il reste un pseudo à choisir, et
// sinon pourquoi la dernière tentative a échoué.
type accountResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
	// Providers dit quels fournisseurs externes sont proposés.
	Providers providersResponse `json:"providers"`
	// Linked liste les fournisseurs déjà rattachés à ce compte : l'interface
	// propose d'associer les autres, jamais de rattacher deux fois.
	Linked []string `json:"linked"`
	// Pending est la connexion externe vérifiée qui attend un pseudo.
	Pending *oauthPending `json:"pending,omitempty"`
	// Error annonce, une seule fois, l'échec de la dernière connexion externe.
	Error *protocol.ErrorPayload `json:"error,omitempty"`
}

// accountKey est la clé de contexte qui porte le compte authentifié.
type accountKey struct{}

// withAccount attache le compte authentifié à une requête.
func withAccount(ctx context.Context, account *accounts.Account) context.Context {
	return context.WithValue(ctx, accountKey{}, account)
}

// accountFrom retrouve le compte authentifié d'une requête.
func accountFrom(ctx context.Context) (*accounts.Account, bool) {
	account, ok := ctx.Value(accountKey{}).(*accounts.Account)

	return account, ok
}

// handleSignup crée un compte local : POST /api/auth/signup
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAPIError(w, err)
		return
	}

	account, err := s.accounts.Create(req.Username, req.Password)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	s.startSession(w, http.StatusCreated, account)
}

// handleLogin ouvre la session d'un compte existant : POST /api/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAPIError(w, err)
		return
	}

	// Le dérivatage du mot de passe est lent par construction : c'est un
	// rempart contre les essais en série, qu'on borne aussi par adresse.
	if !s.attempts.allow(clientIP(r), time.Now()) {
		writeAPIError(w, protocol.NewError(protocol.CodeRateLimited,
			"Trop de tentatives de connexion. Patientez une minute."))
		return
	}

	account, err := s.accounts.Verify(req.Username, req.Password)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	s.startSession(w, http.StatusOK, account)
}

// handleLogout ferme la session de compte : POST /api/auth/logout
//
// L'appel est idempotent : fermer une session déjà fermée n'est pas une erreur.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.Close(cookie.Value)
	}

	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleMe dit qui est connecté : GET /api/auth/me
//
// L'interface s'en sert au chargement : sans compte, elle propose de se
// connecter plutôt que de laisser jouer. C'est aussi là qu'elle apprend quelles
// connexions externes sont possibles, qu'un pseudo reste à choisir, ou pourquoi
// la dernière tentative a échoué.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	account, ok := s.currentAccount(r)
	if !ok {
		s.clearSessionCookie(w)
		writeJSON(w, http.StatusOK, s.anonymousResponse(w, r))

		return
	}

	// La session glisse : chaque visite la prolonge, côté serveur comme dans le
	// navigateur. Sans cela, un joueur assidu se retrouverait déconnecté au
	// bout de la durée de vie initiale.
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.writeSessionCookie(w, cookie.Value)
	}

	response := s.identityResponse(account)

	// Un échec de liaison attend peut-être derrière le cookie de tentative : un
	// joueur déjà connecté doit l'apprendre lui aussi, une seule fois.
	if failure := s.oauth.TakeError(oauthAttempt(r)); failure != nil {
		response.Error = &protocol.ErrorPayload{Code: failure.Code, Message: failure.Message}
		s.clearOAuthCookie(w)
	}

	writeJSON(w, http.StatusOK, response)
}

// anonymousResponse décrit le visiteur sans compte : il peut y avoir une
// connexion externe en cours derrière le cookie de tentative.
func (s *Server) anonymousResponse(w http.ResponseWriter, r *http.Request) accountResponse {
	response := accountResponse{Providers: s.providers(), Linked: []string{}}
	id := oauthAttempt(r)

	if id == "" {
		return response
	}

	// Un pseudo à choisir passe avant un échec : l'identité est vérifiée, il ne
	// reste qu'à la nommer.
	if pending, ok := s.oauth.Pending(id); ok {
		response.Pending = &oauthPending{Provider: pending.Provider, Suggested: pending.Suggested}

		return response
	}

	if failure := s.oauth.TakeError(id); failure != nil {
		response.Error = &protocol.ErrorPayload{Code: failure.Code, Message: failure.Message}
		s.clearOAuthCookie(w)
	}

	return response
}

// requireAccount ferme l'accès à tout ce qui demande une identité.
//
// Les routes des salles s'appuient dessus : le pseudo ne vient plus du corps de
// la requête, il vient du compte.
func (s *Server) requireAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, ok := s.currentAccount(r)
		if !ok {
			writeAPIError(w, protocol.NewError(protocol.CodeNotAuthenticated,
				"Créez un compte ou connectez-vous pour jouer."))

			return
		}

		next.ServeHTTP(w, r.WithContext(withAccount(r.Context(), account)))
	})
}

// currentAccount retrouve le compte derrière le cookie de session.
func (s *Server) currentAccount(r *http.Request) (*accounts.Account, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, false
	}

	accountID, ok := s.sessions.Lookup(cookie.Value)
	if !ok {
		return nil, false
	}

	return s.accounts.ByID(accountID)
}

// startSession ouvre une session et remet le cookie au navigateur.
func (s *Server) startSession(w http.ResponseWriter, status int, account *accounts.Account) {
	if !s.openSession(w, account) {
		return
	}

	writeJSON(w, status, s.identityResponse(account))
}

// openSession ouvre une session de compte et pose son cookie.
//
// Elle sert aux réponses JSON comme aux redirections de connexion externe, où
// le navigateur attend une adresse et non un corps.
func (s *Server) openSession(w http.ResponseWriter, account *accounts.Account) bool {
	token, err := s.sessions.Open(account.ID)
	if err != nil {
		writeAPIError(w, err)

		return false
	}

	s.writeSessionCookie(w, token)

	return true
}

// identityResponse décrit un compte connecté.
func (s *Server) identityResponse(account *accounts.Account) accountResponse {
	return accountResponse{
		Authenticated: true,
		Username:      account.Username,
		Providers:     s.providers(),
		Linked:        linkedProviders(account),
	}
}

// linkedProviders nomme les fournisseurs déjà rattachés au compte.
func linkedProviders(account *accounts.Account) []string {
	identities := account.Identities()

	linked := make([]string, 0, len(identities))
	for _, identity := range identities {
		linked = append(linked, string(identity.Provider))
	}

	return linked
}

// writeSessionCookie remet le cookie de session, avec son échéance.
func (s *Server) writeSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		Expires:  time.Now().Add(s.sessionTTL),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies,
	})
}

// clearSessionCookie efface le cookie de session.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies,
	})
}

// attempts compte les tentatives récentes, par adresse.
type attempts struct {
	mu     sync.Mutex
	window time.Duration
	limit  int
	seen   map[string][]time.Time
}

// newAttempts prépare un compteur de tentatives.
func newAttempts(limit int, window time.Duration) *attempts {
	return &attempts{window: window, limit: limit, seen: make(map[string][]time.Time)}
}

// allow dit si l'adresse peut tenter sa chance maintenant, et enregistre la
// tentative.
func (a *attempts) allow(key string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.forget(now)

	kept := make([]time.Time, 0, len(a.seen[key]))
	for _, at := range a.seen[key] {
		if now.Sub(at) < a.window {
			kept = append(kept, at)
		}
	}

	if len(kept) >= a.limit {
		a.seen[key] = kept

		return false
	}

	a.seen[key] = append(kept, now)

	return true
}

// forget élague les tentatives périmées : la table reste petite, même après des
// mois d'activité. À appeler verrou pris.
func (a *attempts) forget(now time.Time) {
	if len(a.seen) < 256 {
		return
	}

	for key, times := range a.seen {
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= a.window {
			delete(a.seen, key)
		}
	}
}

// clientIP rend l'adresse du client, sans son port.
//
// Le middleware RealIP a déjà remplacé l'adresse de la connexion par celle
// annoncée par un éventuel proxy.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
