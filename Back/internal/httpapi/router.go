// Package httpapi expose l'API JSON du jeu et sert l'interface web.
package httpapi

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/abdou/loupgarou/internal/accounts"
	"github.com/abdou/loupgarou/internal/config"
	"github.com/abdou/loupgarou/internal/oauth"
	"github.com/abdou/loupgarou/internal/room"
	"github.com/abdou/loupgarou/internal/session"
	"github.com/abdou/loupgarou/internal/ws"
)

// Server rassemble le routeur HTTP et les ressources partagées du serveur
// (les comptes, les sessions, le registre des salles et la passerelle
// WebSocket).
type Server struct {
	rooms   *room.Registry
	gateway *ws.Server
	handler http.Handler

	// accounts et sessions portent l'identité des joueurs : elle ne dépend pas
	// des parties, qui restent en mémoire.
	accounts *accounts.Store
	sessions *session.Store
	// oauth mène les connexions aux fournisseurs d'identité externes. Un
	// fournisseur sans identifiants reste hors service : bouton masqué et route
	// muette.
	oauth *oauth.Engine
	// frontURL est l'adresse de l'interface, où le navigateur est renvoyé après
	// une connexion externe. Vide, le retour se fait à la racine (« / »), servie
	// par ce même serveur.
	frontURL string
	// showOAuthButtons force l'affichage des boutons de connexion externe même
	// sans identifiants (mode développement) : les routes, elles, répondent
	// toujours « non disponible ».
	showOAuthButtons bool
	// sessionTTL est la durée d'une session, prolongée à chaque visite.
	sessionTTL time.Duration
	// secureCookies demande au navigateur de ne renvoyer le cookie de session
	// qu'en HTTPS.
	secureCookies bool
	// attempts borne les tentatives de connexion par adresse.
	attempts *attempts
}

// NewRouter construit le serveur HTTP complet.
//
//	/healthz            sonde de santé pour l'orchestrateur (Docker, K8s…)
//	/api/auth/*         création de compte, connexion, déconnexion, identité
//	/api/auth/{provider}/start, /callback   connexion Google ou Discord
//	/api/auth/{provider}/link               rattacher Google ou Discord à son compte
//	/api/auth/complete  choix du pseudo après une connexion externe
//	/api/rooms          création d'une partie (compte requis)
//	/api/rooms/{code}   résumé d'une partie (écran « rejoindre »)
//	/api/rooms/{code}/join   entrée dans une partie (compte requis)
//	/api/rooms/{code}/ws     passerelle WebSocket (temps réel)
//	/*                  interface web construite, avec repli SPA
func NewRouter(cfg config.Config) *Server {
	opts := room.DefaultOptions()
	opts.Game = cfg.Game
	opts.BotDelay = cfg.BotDelay

	rooms := room.NewRegistry(opts)

	// Comptes et sessions : un fichier illisible est une erreur franche. Mieux
	// vaut refuser de démarrer que de laisser jouer sans pouvoir se
	// reconnecter.
	accountsPath := cfg.AccountsFile
	if accountsPath == "" {
		accountsPath = config.DefaultAccountsFile
	}

	sessionsPath := cfg.SessionsFile
	if sessionsPath == "" {
		sessionsPath = config.DefaultSessionsFile
	}

	ttl := cfg.SessionTTL
	if ttl <= 0 {
		ttl = config.DefaultSessionTTL
	}

	accountStore, err := accounts.NewStore(accountsPath)
	if err != nil {
		log.Fatalf("comptes illisibles : %v", err)
	}

	sessionStore, err := session.NewStore(sessionsPath, ttl)
	if err != nil {
		log.Fatalf("sessions illisibles : %v", err)
	}

	// Les connexions externes se déclarent au fournisseur par l'adresse
	// publique du site : sans elle, l'adresse de retour ne serait pas la bonne.
	publicURL := strings.TrimRight(cfg.PublicURL, "/")
	if publicURL == "" {
		publicURL = "http://localhost:" + cfg.Port
	}

	server := &Server{
		rooms:         rooms,
		gateway:       ws.NewServer(rooms, cfg.AllowedOrigins),
		accounts:      accountStore,
		sessions:      sessionStore,
		sessionTTL:    ttl,
		secureCookies: strings.HasPrefix(publicURL, "https://"),
		attempts:      newAttempts(attemptsPerMinute, time.Minute),
		oauth: oauth.New(publicURL,
			oauth.Google(cfg.GoogleClientID, cfg.GoogleClientSecret),
			oauth.Discord(cfg.DiscordClientID, cfg.DiscordClientSecret),
		),
		showOAuthButtons: cfg.ShowOAuthButtons,
		frontURL:         strings.TrimRight(cfg.FrontURL, "/"),
	}
	server.handler = server.routes(cfg)

	return server
}

// routes assemble les routes.
func (s *Server) routes(cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(accessLog)

	r.Get("/healthz", handleHealth)
	r.Get("/api/health", handleHealth)

	r.Post("/api/auth/signup", s.handleSignup)
	r.Post("/api/auth/login", s.handleLogin)
	r.Post("/api/auth/logout", s.handleLogout)
	r.Get("/api/auth/me", s.handleMe)

	// Connexions externes : le départ et le retour sont des redirections, le
	// fournisseur ne parlant jamais JSON. Le pseudo se choisit ensuite, par un
	// appel normal.
	r.Get("/api/auth/{provider}/start", s.handleProviderStart)
	r.Get("/api/auth/{provider}/link", s.handleProviderLink)
	r.Get("/api/auth/{provider}/callback", s.handleProviderCallback)
	r.Post("/api/auth/complete", s.handleComplete)

	// Le résumé d'une partie et sa passerelle WebSocket restent publics : le
	// premier sert à valider un code avant d'entrer, la seconde s'authentifie
	// par le jeton de siège.
	r.Get("/api/rooms/{code}", s.handleRoomInfo)
	r.Get("/api/rooms/{code}/ws", s.handleRoomSocket)

	// Tout le reste demande un compte : le pseudo vient de l'identité, jamais
	// d'un corps de requête.
	r.Group(func(r chi.Router) {
		r.Use(s.requireAccount)

		r.Post("/api/rooms", s.handleCreateRoom)
		r.Post("/api/rooms/{code}/join", s.handleJoinRoom)
	})

	r.Handle("/*", newSPAHandler(cfg.WebDist))

	return r
}

// handleRoomSocket délègue à la passerelle WebSocket.
func (s *Server) handleRoomSocket(w http.ResponseWriter, r *http.Request) {
	s.gateway.Handler(w, r)
}

// ServeHTTP fait de Server un http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// Close arrête les salles en cours et écrit les sessions restantes.
//
// Seules les sessions sont durables : les parties disparaissent avec le
// serveur, les identités non.
func (s *Server) Close() {
	s.rooms.Close()

	if err := s.sessions.Flush(); err != nil {
		log.Printf("sessions non écrites : %v", err)
	}
}

// handleHealth répond avec l'état de vie du serveur.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "loupgarou",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}
