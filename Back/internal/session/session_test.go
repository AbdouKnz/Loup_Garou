package session

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testTTL = time.Hour

func newStore(t *testing.T) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sessions.json")

	store, err := NewStore(path, testTTL)
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	return store, path
}

func read(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}

	return raw
}

func TestOpenPuisLookup(t *testing.T) {
	store, _ := newStore(t)

	token, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}
	if len(token) < 20 {
		t.Fatalf("jeton trop court : %q", token)
	}

	accountID, ok := store.Lookup(token)
	if !ok || accountID != "compte-1" {
		t.Errorf("Lookup = %q, %v", accountID, ok)
	}

	if _, ok := store.Lookup("jeton-inventé"); ok {
		t.Error("un jeton inconnu ne doit pas être accepté")
	}
	if _, ok := store.Lookup(""); ok {
		t.Error("un jeton vide ne doit pas être accepté")
	}
}

func TestLeJetonNAPasEnClairSurLeDisque(t *testing.T) {
	store, path := newStore(t)

	token, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	raw := read(t, path)
	if bytes.Contains(raw, []byte(token)) {
		t.Error("le jeton apparaît en clair dans le fichier")
	}
	if !bytes.Contains(raw, []byte(hashToken(token))) {
		t.Error("l'empreinte du jeton devrait être conservée")
	}
}

func TestCloseFermeLaSession(t *testing.T) {
	store, path := newStore(t)

	token, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	store.Close(token)

	if _, ok := store.Lookup(token); ok {
		t.Error("une session fermée ne doit plus valoir un jeton")
	}

	// La fermeture est écrite tout de suite : le redémarrage ne la ressuscite
	// pas.
	reopened, err := NewStore(path, testTTL)
	if err != nil {
		t.Fatalf("réouverture impossible : %v", err)
	}
	if reopened.Count() != 0 {
		t.Errorf("sessions = %d, attendu 0", reopened.Count())
	}
}

func TestLaSessionExpire(t *testing.T) {
	store, _ := newStore(t)

	clock := time.Now()
	store.now = func() time.Time { return clock }

	token, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	// La visite prolonge la session : elle tient encore juste avant l'échéance.
	clock = clock.Add(testTTL - time.Minute)
	if _, ok := store.Lookup(token); !ok {
		t.Fatal("la session devrait encore être valide")
	}
	if _, ok := store.Lookup(token); !ok {
		t.Fatal("la visite aurait dû prolonger la session")
	}

	clock = clock.Add(testTTL + time.Minute)
	if _, ok := store.Lookup(token); ok {
		t.Error("une session périmée ne doit plus être acceptée")
	}
	if store.Count() != 0 {
		t.Errorf("sessions = %d, attendu 0", store.Count())
	}
}

func TestLesSessionsSurviventAuRedemarrage(t *testing.T) {
	store, path := newStore(t)

	token, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	reopened, err := NewStore(path, testTTL)
	if err != nil {
		t.Fatalf("réouverture impossible : %v", err)
	}

	accountID, ok := reopened.Lookup(token)
	if !ok || accountID != "compte-1" {
		t.Errorf("Lookup après redémarrage = %q, %v", accountID, ok)
	}
}

func TestLesSessionsPerimeesNeSurviventPasAuRedemarrage(t *testing.T) {
	store, path := newStore(t)

	// L'horloge du magasin est en retard : la session naît déjà périmée pour
	// l'horloge réelle, comme si le serveur avait dormi deux jours.
	store.now = func() time.Time { return time.Now().Add(-48 * time.Hour) }

	if _, err := store.Open("compte-1"); err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	reopened, err := NewStore(path, testTTL)
	if err != nil {
		t.Fatalf("réouverture impossible : %v", err)
	}
	if reopened.Count() != 0 {
		t.Errorf("sessions = %d, attendu 0", reopened.Count())
	}

	if raw := read(t, path); strings.Contains(string(raw), "compte-1") {
		t.Errorf("la session périmée est restée sur le disque : %s", raw)
	}
}

func TestLEcritureEstGroupee(t *testing.T) {
	store, path := newStore(t)

	token, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	before := read(t, path)
	store.interval = time.Hour

	// La visite prolonge la session en mémoire, sans réécrire le fichier.
	store.Lookup(token)

	if after := read(t, path); !bytes.Equal(before, after) {
		t.Error("le fichier a été réécrit à chaque visite")
	}

	// L'arrêt du serveur, lui, écrit ce qui reste.
	if err := store.Flush(); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}
	if after := read(t, path); bytes.Equal(before, after) {
		t.Error("Flush devrait écrire la prolongation")
	}
}

func TestPlusieursSessionsPourUnCompte(t *testing.T) {
	store, _ := newStore(t)

	first, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}
	second, err := store.Open("compte-1")
	if err != nil {
		t.Fatalf("seconde ouverture impossible : %v", err)
	}

	for _, token := range []string{first, second} {
		if accountID, ok := store.Lookup(token); !ok || accountID != "compte-1" {
			t.Errorf("Lookup(%q) = %q, %v", token, accountID, ok)
		}
	}

	store.Close(first)
	if _, ok := store.Lookup(second); !ok {
		t.Error("fermer une session ne doit pas fermer les autres")
	}
}
