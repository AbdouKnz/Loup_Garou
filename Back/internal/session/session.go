// Package session gère les sessions de compte : des jetons opaques remis aux
// navigateurs, dont seule l'empreinte est conservée.
//
// Un jeton vaut une connexion : il est tiré au hasard, ne transite qu'une fois
// par le cookie, et n'est jamais écrit tel quel. Le fichier des sessions ne
// contient que des empreintes : le voler ne permet pas de se faire passer pour
// quelqu'un.
//
// Les sessions survivent au redémarrage du serveur : c'est le sens de cette
// persistance. Les parties, elles, restent en mémoire.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/abdou/loupgarou/internal/filestore"
)

// tokenLength est la taille d'un jeton de session, en octets.
const tokenLength = 32

// saveInterval est l'écart minimal entre deux écritures automatiques.
//
// Chaque visite prolonge la session : écrire le fichier à chaque requête
// n'apporterait rien et coûterait cher. La prolongation est donc regroupée, et
// l'arrêt du serveur écrit ce qui reste.
const saveInterval = time.Minute

// record est une session, telle qu'elle est écrite sur le disque.
type record struct {
	TokenHash string    `json:"tokenHash"`
	AccountID string    `json:"accountId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	SeenAt    time.Time `json:"seenAt"`
}

// file est la forme du fichier sur le disque.
type file struct {
	Sessions []record `json:"sessions"`
}

// Store tient les sessions ouvertes et les écrit sur le disque.
type Store struct {
	mu   sync.Mutex
	path string
	ttl  time.Duration
	now  func() time.Time
	// interval est le délai minimal entre deux écritures automatiques ; les
	// tests le baissent.
	interval time.Duration
	records  map[string]record // par empreinte de jeton
	dirty    bool
	savedAt  time.Time
}

// NewStore ouvre le fichier des sessions et charge celles qui y sont déjà.
//
// L'expiration est glissante : une session vit ttl après sa dernière visite.
func NewStore(path string, ttl time.Duration) (*Store, error) {
	store := &Store{
		path:     path,
		ttl:      ttl,
		now:      time.Now,
		interval: saveInterval,
		records:  make(map[string]record),
	}

	var content file
	loaded, err := filestore.Load(path, &content)
	if err != nil {
		return nil, err
	}

	now := store.now()

	for _, item := range content.Sessions {
		if !now.Before(item.ExpiresAt) {
			continue
		}

		store.records[item.TokenHash] = item
	}

	// Les sessions périmées ne survivent pas au démarrage.
	if loaded && len(store.records) != len(content.Sessions) {
		if err := store.save(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// Count renvoie le nombre de sessions ouvertes.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.records)
}

// Open ouvre une session pour un compte et renvoie son jeton.
//
// C'est le seul moment où le jeton existe en clair : il n'est ensuite retrouvé
// qu'en recalculant son empreinte.
func (s *Store) Open(accountID string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}

	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.purge(now)

	hash := hashToken(token)
	s.records[hash] = record{
		TokenHash: hash,
		AccountID: accountID,
		CreatedAt: now.UTC(),
		ExpiresAt: now.Add(s.ttl).UTC(),
		SeenAt:    now.UTC(),
	}
	s.dirty = true

	if err := s.save(); err != nil {
		return "", err
	}

	return token, nil
}

// Lookup retrouve le compte d'une session et prolonge sa validité.
func (s *Store) Lookup(token string) (string, bool) {
	if token == "" {
		return "", false
	}

	hash := hashToken(token)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.records[hash]
	if !ok {
		return "", false
	}

	if !now.Before(item.ExpiresAt) {
		delete(s.records, hash)
		s.dirty = true
		s.saveIfDue(now)

		return "", false
	}

	item.SeenAt = now.UTC()
	item.ExpiresAt = now.Add(s.ttl).UTC()
	s.records[hash] = item
	s.dirty = true
	s.saveIfDue(now)

	return item.AccountID, true
}

// Close ferme une session : le jeton ne vaut plus rien.
func (s *Store) Close(token string) {
	if token == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash := hashToken(token)
	if _, ok := s.records[hash]; !ok {
		return
	}

	delete(s.records, hash)
	s.dirty = true

	if err := s.save(); err != nil {
		log.Printf("sessions non écrites : %v", err)
	}
}

// Flush écrit les sessions si quelque chose a changé.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return nil
	}

	return s.save()
}

// saveIfDue écrit le fichier, sans plus d'une fois par intervalle. À appeler
// verrou pris.
func (s *Store) saveIfDue(now time.Time) {
	if !s.dirty || now.Sub(s.savedAt) < s.interval {
		return
	}

	if err := s.save(); err != nil {
		// Une prolongation non écrite ne ferme aucune session : elle sera
		// simplement oubliée si le serveur s'arrête brutalement. On le dit
		// sans interrompre la requête en cours.
		log.Printf("sessions non écrites : %v", err)
	}
}

// purge retire les sessions périmées. À appeler verrou pris.
func (s *Store) purge(now time.Time) {
	for hash, item := range s.records {
		if !now.Before(item.ExpiresAt) {
			delete(s.records, hash)
			s.dirty = true
		}
	}
}

// save écrit toutes les sessions. À appeler verrou pris.
func (s *Store) save() error {
	content := file{Sessions: make([]record, 0, len(s.records))}
	for _, item := range s.records {
		content.Sessions = append(content.Sessions, item)
	}

	sort.Slice(content.Sessions, func(i, j int) bool {
		if !content.Sessions[i].CreatedAt.Equal(content.Sessions[j].CreatedAt) {
			return content.Sessions[i].CreatedAt.Before(content.Sessions[j].CreatedAt)
		}

		return content.Sessions[i].TokenHash < content.Sessions[j].TokenHash
	})

	if err := filestore.Save(s.path, content); err != nil {
		return err
	}

	s.dirty = false
	s.savedAt = s.now()

	return nil
}

// newToken tire un jeton de session.
func newToken() (string, error) {
	buf := make([]byte, tokenLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("jeton de session : %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashToken calcule l'empreinte d'un jeton : c'est elle qui est conservée.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
