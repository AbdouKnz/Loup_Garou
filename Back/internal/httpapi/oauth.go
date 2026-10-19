package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/abdou/loupgarou/internal/accounts"
	"github.com/abdou/loupgarou/internal/oauth"
	"github.com/abdou/loupgarou/internal/protocol"
)

// oauthCookieName est le nom du cookie qui porte la connexion externe en cours.
//
// Ce cookie ne vaut que le temps d'une connexion et ne porte aucun secret : il
// dit seulement quelle tentative est la nôtre. Il est effacé dès que la session
// du compte est ouverte.
const oauthCookieName = "lg_oauth"

// providersResponse dit quels fournisseurs externes sont utilisables ici.
//
// Un fournisseur sans identifiants d'application n'est pas proposé : c'est le
// serveur qui sait ce qu'il a le droit de proposer, pas l'interface.
type providersResponse struct {
	Google  bool `json:"google"`
	Discord bool `json:"discord"`
}

// oauthPending décrit la connexion externe qui attend un pseudo.
type oauthPending struct {
	Provider string `json:"provider"`
	// Suggested est le pseudo proposé par le fournisseur, s'il y en a un
	// d'utilisable.
	Suggested string `json:"suggested,omitempty"`
}

// completeRequest est le pseudo choisi pour une connexion externe.
type completeRequest struct {
	Username string `json:"username"`
}

// handleProviderStart envoie le navigateur chez le fournisseur :
// GET /api/auth/{provider}/start
func (s *Server) handleProviderStart(w http.ResponseWriter, r *http.Request) {
	id, target, err := s.oauth.Begin(chi.URLParam(r, "provider"))
	if err != nil {
		writeAPIError(w, err)

		return
	}

	// C'est le navigateur qui retient la tentative : au retour, ce cookie dit
	// quel `state` et quel vérificateur PKCE attendent le fournisseur.
	s.writeOAuthCookie(w, id)
	http.Redirect(w, r, target, http.StatusFound)
}

// handleProviderLink rattache une identité à un compte déjà ouvert :
// GET /api/auth/{provider}/link
//
// C'est la liaison explicite : connecté, le joueur associe son compte Google ou
// Discord sans en ouvrir un second. La tentative retient le compte visé ; au
// retour, le callback la rattachera.
func (s *Server) handleProviderLink(w http.ResponseWriter, r *http.Request) {
	account, ok := s.currentAccount(r)
	if !ok {
		// Sans session, il n'y a rien à rattacher : on revient au guichet.
		s.redirectToFront(w, r)

		return
	}

	id, target, err := s.oauth.BeginLink(chi.URLParam(r, "provider"), account.ID)
	if err != nil {
		s.failOAuth(w, r, "", err)

		return
	}

	s.writeOAuthCookie(w, id)
	http.Redirect(w, r, target, http.StatusFound)
}

// handleProviderCallback reçoit le retour du fournisseur :
// GET /api/auth/{provider}/callback
//
// Cette route ne répond jamais en JSON : le navigateur y arrive par une
// redirection, il doit en repartir par une redirection. Les échecs sont donc mis
// de côté et annoncés par GET /api/auth/me, à l'écran de connexion — un échec
// silencieux laisserait le joueur devant un formulaire sans explication.
func (s *Server) handleProviderCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := oauthAttempt(r)

	if !s.oauth.Configured(chi.URLParam(r, "provider")) {
		s.failOAuth(w, r, id, protocol.NewError(protocol.CodeNotImplemented,
			"Cette connexion n'est pas disponible sur ce serveur."))

		return
	}

	// Le fournisseur peut refuser : le joueur a changé d'avis, ou l'accès a été
	// révoqué. C'est un cas normal, pas une panne.
	if query.Get("error") != "" {
		s.failOAuth(w, r, id, protocol.NewError(protocol.CodeOAuthFailed,
			"Connexion annulée chez le fournisseur."))

		return
	}

	purpose, accountID, _ := s.oauth.Intention(id)

	identity, err := s.oauth.Complete(r.Context(), oauth.Callback{
		ID:     id,
		State:  query.Get("state"),
		Code:   query.Get("code"),
		Issuer: query.Get("iss"),
	})
	if err != nil {
		s.failOAuth(w, r, id, err)

		return
	}

	provider := accounts.Provider(identity.Provider)

	// Liaison : l'identité rejoint le compte visé, sans rien créer.
	if purpose == oauth.PurposeLink {
		if _, err := s.accounts.Link(accountID, provider, identity.ProviderID, identity.Email); err != nil {
			s.failOAuth(w, r, id, err)

			return
		}

		s.oauth.Forget(id)
		s.clearOAuthCookie(w)
		s.redirectToFront(w, r)

		return
	}

	// Déjà connu : la session s'ouvre sans rien demander.
	if account, known := s.accounts.ByProvider(provider, identity.ProviderID); known {
		s.oauth.Forget(id)
		s.clearOAuthCookie(w)

		if s.openSession(w, account) {
			s.redirectToFront(w, r)
		}

		return
	}

	// Identité inconnue mais e-mail vérifié déjà rattaché à un compte : le
	// joueur garde son compte au lieu d'en ouvrir un second. On ne se fie qu'à
	// un e-mail vérifié, et on ne devine rien s'il n'y en a pas.
	if identity.EmailVerified {
		if account, found := s.accounts.ByEmail(identity.Email); found {
			if _, err := s.accounts.Link(account.ID, provider, identity.ProviderID, identity.Email); err == nil {
				s.oauth.Forget(id)
				s.clearOAuthCookie(w)

				if s.openSession(w, account) {
					s.redirectToFront(w, r)
				}

				return
			}
		}
	}

	// Sinon, l'identité attend un pseudo, que l'écran de connexion fera choisir.
	s.redirectToFront(w, r)
}

// handleComplete crée le compte d'une connexion externe, avec le pseudo choisi :
// POST /api/auth/complete
func (s *Server) handleComplete(w http.ResponseWriter, r *http.Request) {
	var req completeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAPIError(w, err)

		return
	}

	id := oauthAttempt(r)

	identity, ok := s.oauth.Pending(id)
	if !ok {
		writeAPIError(w, protocol.NewError(protocol.CodeOAuthState,
			"Cette connexion a expiré. Recommencez."))

		return
	}

	account, err := s.accounts.CreateExternal(accounts.Provider(identity.Provider),
		identity.ProviderID, req.Username, identity.Email)
	if err != nil {
		// Pseudo refusé : la tentative reste ouverte, le joueur peut en essayer
		// un autre sans repasser par le fournisseur.
		writeAPIError(w, err)

		return
	}

	s.oauth.Forget(id)
	s.clearOAuthCookie(w)
	s.startSession(w, http.StatusCreated, account)
}

// failOAuth range un échec le temps d'une visite et ramène le navigateur à
// l'écran de connexion.
func (s *Server) failOAuth(w http.ResponseWriter, r *http.Request, id string, err error) {
	if next := s.oauth.Fail(id, err); next != "" {
		s.writeOAuthCookie(w, next)
	} else {
		s.clearOAuthCookie(w)
	}

	s.redirectToFront(w, r)
}

// redirectToFront renvoie le navigateur à l'interface après une connexion
// externe.
//
// L'interface peut être servie par un autre serveur que celui-ci : en
// développement, Vite sert l'application sur son propre port pendant que Go ne
// fournit que l'API. `frontURL` dit alors où aller ; sans elle, l'interface est
// servie par ce serveur et la racine « / » suffit.
func (s *Server) redirectToFront(w http.ResponseWriter, r *http.Request) {
	target := "/"
	if s.frontURL != "" {
		target = s.frontURL + "/"
	}

	http.Redirect(w, r, target, http.StatusFound)
}

// providers dit quels fournisseurs externes sont utilisables sur ce serveur.
//
// En mode développement (SHOW_OAUTH_BUTTONS), les deux boutons sont annoncés
// même sans identifiants : l'interface peut être vue et validée, à défaut de
// mener quelque part.
func (s *Server) providers() providersResponse {
	if s.showOAuthButtons {
		return providersResponse{Google: true, Discord: true}
	}

	return providersResponse{
		Google:  s.oauth.Configured(oauth.NameGoogle),
		Discord: s.oauth.Configured(oauth.NameDiscord),
	}
}

// oauthAttempt retrouve la tentative de connexion externe du navigateur.
func oauthAttempt(r *http.Request) string {
	cookie, err := r.Cookie(oauthCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// writeOAuthCookie retient la tentative de connexion externe en cours.
func (s *Server) writeOAuthCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies,
	})
}

// clearOAuthCookie efface la tentative de connexion externe.
func (s *Server) clearOAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies,
	})
}
