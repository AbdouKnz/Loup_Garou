package room

import (
	"context"
	"crypto/rand"
	"sync"

	"github.com/abdou/loupgarou/internal/protocol"
)

// Registry tient la liste des salles actives.
//
// Le registre ne connaît des salles que leur code et leur acteur : toute la
// logique de jeu reste enfermée dans la salle.
type Registry struct {
	mu     sync.RWMutex
	rooms  map[string]*Room
	opts   Options
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRegistry crée un registre et le contexte de vie de ses salles.
//
// La méthode Close arrête toutes les salles : c'est ce qui permet aux tests de
// ne laisser aucune goroutine derrière eux.
func NewRegistry(opts Options) *Registry {
	ctx, cancel := context.WithCancel(context.Background())

	return &Registry{
		rooms:  make(map[string]*Room),
		opts:   opts.withDefaults(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Create ouvre une salle et y inscrit son hôte.
//
// Elle renvoie le code de la partie, l'identifiant de l'hôte et son jeton de
// session (le seul moment où le jeton est transmis).
func (g *Registry) Create(account, pseudo string) (string, PlayerID, string, error) {
	clean, err := protocol.NormalizePseudo(pseudo)
	if err != nil {
		return "", "", "", err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	code, err := g.freeCode()
	if err != nil {
		return "", "", "", err
	}

	salle, hostID, token := New(code, account, clean, g.opts, g.forget)
	g.rooms[code] = salle

	go salle.Run(g.ctx)

	return code, hostID, token, nil
}

// Join inscrit un joueur dans une salle existante, au nom de son compte.
func (g *Registry) Join(code, account, pseudo string) (PlayerID, string, error) {
	clean, err := protocol.NormalizePseudo(pseudo)
	if err != nil {
		return "", "", err
	}

	salle, ok := g.Get(code)
	if !ok {
		return "", "", protocol.NewError(protocol.CodeRoomNotFound,
			"Cette partie n'existe pas ou a été fermée.")
	}

	return salle.Join(account, clean)
}

// Get retrouve une salle par son code.
func (g *Registry) Get(rawCode string) (*Room, bool) {
	code, err := protocol.NormalizeRoomCode(rawCode)
	if err != nil {
		return nil, false
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	salle, ok := g.rooms[code]

	return salle, ok
}

// Len renvoie le nombre de salles actives.
func (g *Registry) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.rooms)
}

// Close arrête toutes les salles : plus aucune commande n'est acceptée ensuite.
func (g *Registry) Close() {
	g.cancel()

	g.mu.Lock()
	defer g.mu.Unlock()

	for _, salle := range g.rooms {
		salle.Close()
	}
	g.rooms = make(map[string]*Room)
}

// forget retire une salle de la liste : appelée par la salle elle-même quand
// elle décide de se fermer (personne ne l'a rejointe, plus personne dedans…).
func (g *Registry) forget(code string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.rooms, code)
}

// freeCode tire un code de partie encore libre. À appeler verrou pris.
func (g *Registry) freeCode() (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		code, err := randomCode()
		if err != nil {
			return "", err
		}

		if _, taken := g.rooms[code]; !taken {
			return code, nil
		}
	}

	return "", protocol.NewError(protocol.CodeInternal, "Impossible de générer un code de partie.")
}

// randomCode tire un code de partie lisible, sans caractères ambigus.
//
// Le modulo introduit un biais négligeable (256 n'est pas un multiple de 30)
// qui n'a aucune importance pour un code de partie à usage humain.
func randomCode() (string, error) {
	alphabet := protocol.RoomAlphabet()

	buf := make([]byte, protocol.RoomCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}

	return string(buf), nil
}
