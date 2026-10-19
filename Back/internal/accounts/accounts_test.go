package accounts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdou/loupgarou/internal/protocol"
)

// newStore ouvre un magasin dans un dossier temporaire, à coût réduit.
func newStore(t *testing.T) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "accounts.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}
	store.SetIterations(MinIterations)

	return store, path
}

func TestCreateEtVerify(t *testing.T) {
	store, _ := newStore(t)

	account, err := store.Create("  Alice  ", "motdepasse")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}
	if account.Username != "Alice" {
		t.Errorf("pseudo = %q, attendu %q", account.Username, "Alice")
	}
	if account.ID == "" || account.Salt == "" || account.PasswordHash == "" {
		t.Errorf("compte incomplet : %+v", account)
	}
	if account.Provider != ProviderLocal {
		t.Errorf("provider = %q, attendu %q", account.Provider, ProviderLocal)
	}
	// Le mot de passe n'est jamais écrit tel quel.
	if strings.Contains(account.PasswordHash, "motdepasse") {
		t.Error("le mot de passe apparaît dans le haché")
	}

	found, err := store.Verify("alice", "motdepasse")
	if err != nil {
		t.Fatalf("connexion refusée : %v", err)
	}
	if found.ID != account.ID {
		t.Errorf("compte retrouvé %q, attendu %q", found.ID, account.ID)
	}
}

func TestPseudoUniqueSansTenirCompteDeLaCasse(t *testing.T) {
	store, _ := newStore(t)

	if _, err := store.Create("Alice", "motdepasse"); err != nil {
		t.Fatalf("première création impossible : %v", err)
	}

	for _, pseudo := range []string{"Alice", "alice", "  ALICE  ", "aLiCe"} {
		_, err := store.Create(pseudo, "motdepasse")
		if protocol.CodeOf(err) != protocol.CodeUsernameTaken {
			t.Errorf("%q : code = %q, attendu %q", pseudo, protocol.CodeOf(err), protocol.CodeUsernameTaken)
		}
	}

	if store.Count() != 1 {
		t.Errorf("comptes = %d, attendu 1", store.Count())
	}
}

func TestVerifyRefuse(t *testing.T) {
	store, _ := newStore(t)

	if _, err := store.Create("Alice", "motdepasse"); err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "mot de passe faux", username: "Alice", password: "motdepass"},
		{name: "mot de passe vide", username: "Alice", password: ""},
		{name: "pseudo inconnu", username: "Bob", password: "motdepasse"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Verify(tt.username, tt.password)
			if protocol.CodeOf(err) != protocol.CodeBadCredentials {
				t.Errorf("code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeBadCredentials)
			}
		})
	}
}

func TestCreateRefuse(t *testing.T) {
	store, _ := newStore(t)

	tests := []struct {
		name     string
		username string
		password string
		wantCode protocol.ErrorCode
	}{
		{name: "pseudo trop court", username: "A", password: "motdepasse", wantCode: protocol.CodeInvalidPseudo},
		{name: "pseudo vide", username: "", password: "motdepasse", wantCode: protocol.CodeInvalidPseudo},
		{name: "pseudo trop long", username: strings.Repeat("a", 21), password: "motdepasse", wantCode: protocol.CodeInvalidPseudo},
		{name: "mot de passe trop court", username: "Alice", password: "court", wantCode: protocol.CodeWeakPassword},
		{name: "mot de passe vide", username: "Alice", password: "", wantCode: protocol.CodeWeakPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Create(tt.username, tt.password)
			if code := protocol.CodeOf(err); code != tt.wantCode {
				t.Errorf("code = %q, attendu %q", code, tt.wantCode)
			}
		})
	}

	if store.Count() != 0 {
		t.Errorf("comptes = %d, attendu 0", store.Count())
	}
}

func TestLesComptesSurviventAuRedemarrage(t *testing.T) {
	store, path := newStore(t)

	created, err := store.Create("Alice", "motdepasse")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	// Redémarrage simulé : un nouveau magasin sur le même fichier.
	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("réouverture impossible : %v", err)
	}

	if reopened.Count() != 1 {
		t.Fatalf("comptes = %d, attendu 1", reopened.Count())
	}

	found, err := reopened.Verify("Alice", "motdepasse")
	if err != nil {
		t.Fatalf("connexion refusée après redémarrage : %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("compte retrouvé %q, attendu %q", found.ID, created.ID)
	}

	same, ok := reopened.ByUsername("ALICE")
	if !ok || same.ID != created.ID {
		t.Errorf("ByUsername = %v, %v", same, ok)
	}

	byID, ok := reopened.ByID(created.ID)
	if !ok || byID.Username != "Alice" {
		t.Errorf("ByID = %v, %v", byID, ok)
	}
}

func TestLeFichierNeContientPasLeMotDePasse(t *testing.T) {
	store, path := newStore(t)

	if _, err := store.Create("Alice", "motdepasse"); err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}
	if strings.Contains(string(raw), "motdepasse") {
		t.Error("le mot de passe apparaît en clair dans le fichier")
	}
}

func TestVerifySurUnHachageIllisible(t *testing.T) {
	store, _ := newStore(t)

	account, err := store.Create("Alice", "motdepasse")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	account.PasswordHash = "pas du base64 !"
	if _, err := store.Verify("Alice", "motdepasse"); protocol.CodeOf(err) != protocol.CodeBadCredentials {
		t.Errorf("code = %q, attendu %q", protocol.CodeOf(err), protocol.CodeBadCredentials)
	}
}

func TestSetIterationsGardeUnPlancher(t *testing.T) {
	store, _ := newStore(t)

	store.SetIterations(1)

	account, err := store.Create("Alice", "motdepasse")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}
	if account.Iterations < MinIterations {
		t.Errorf("itérations = %d, attendu au moins %d", account.Iterations, MinIterations)
	}
}

func TestCreateExternal(t *testing.T) {
	store, path := newStore(t)

	account, err := store.CreateExternal(ProviderGoogle, "42", "  Jean  ", "jean@example.com")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}
	if account.Username != "Jean" {
		t.Errorf("pseudo = %q, attendu %q", account.Username, "Jean")
	}
	if account.Provider != ProviderGoogle || account.ProviderID != "42" {
		t.Errorf("rattachement = %q/%q", account.Provider, account.ProviderID)
	}
	// Aucun mot de passe : c'est le fournisseur qui a vérifié l'identité.
	if account.PasswordHash != "" || account.Salt != "" || account.Iterations != 0 {
		t.Errorf("compte externe avec mot de passe : %+v", account)
	}

	// L'adresse e-mail est une information : elle n'ouvre aucune session.
	if _, err := store.Verify("Jean", ""); protocol.CodeOf(err) != protocol.CodeBadCredentials {
		t.Errorf("connexion par e-mail : %v", err)
	}

	// Le rattachement permet de retrouver le compte, et il survit au
	// redémarrage : c'est lui qui rend une reconnexion idempotente.
	if found, ok := store.ByProvider(ProviderGoogle, "42"); !ok || found.ID != account.ID {
		t.Errorf("compte retrouvé = %+v, %v", found, ok)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("réouverture impossible : %v", err)
	}
	if _, ok := reopened.ByProvider(ProviderGoogle, "42"); !ok {
		t.Error("le rattachement ne survit pas au redémarrage")
	}
}

func TestCreateExternalRefuse(t *testing.T) {
	store, _ := newStore(t)

	if _, err := store.CreateExternal(ProviderGoogle, "42", "Alice", ""); err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	tests := []struct {
		name       string
		provider   Provider
		providerID string
		username   string
		wantCode   protocol.ErrorCode
	}{
		{
			name: "pseudo déjà pris", provider: ProviderDiscord, providerID: "7",
			username: "alice", wantCode: protocol.CodeUsernameTaken,
		},
		{
			name: "pseudo trop court", provider: ProviderDiscord, providerID: "7",
			username: "A", wantCode: protocol.CodeInvalidPseudo,
		},
		{
			name: "sans fournisseur", provider: ProviderLocal, providerID: "7",
			username: "Bob", wantCode: protocol.CodeInternal,
		},
		{
			name: "sans identifiant", provider: ProviderDiscord, providerID: "",
			username: "Bob", wantCode: protocol.CodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.CreateExternal(tt.provider, tt.providerID, tt.username, "")
			if code := protocol.CodeOf(err); code != tt.wantCode {
				t.Errorf("code = %q, attendu %q", code, tt.wantCode)
			}
		})
	}

	// Le refus ne laisse rien derrière lui.
	if _, ok := store.ByProvider(ProviderDiscord, "7"); ok {
		t.Error("un compte refusé a été rattaché")
	}
	if account, ok := store.ByUsername("Bob"); ok {
		t.Errorf("un compte refusé a été créé : %+v", account)
	}
}

// TestLesFournisseursNeSeMelangentPas : un même identifiant chez deux
// fournisseurs désigne deux personnes différentes.
func TestLesFournisseursNeSeMelangentPas(t *testing.T) {
	store, _ := newStore(t)

	google, err := store.CreateExternal(ProviderGoogle, "42", "Alice", "")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	discord, err := store.CreateExternal(ProviderDiscord, "42", "Bob", "")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	if found, _ := store.ByProvider(ProviderGoogle, "42"); found.ID != google.ID {
		t.Errorf("Google renvoie %q", found.Username)
	}
	if found, _ := store.ByProvider(ProviderDiscord, "42"); found.ID != discord.ID {
		t.Errorf("Discord renvoie %q", found.Username)
	}
	if _, ok := store.ByProvider(ProviderDiscord, "8"); ok {
		t.Error("un identifiant inconnu est retrouvé")
	}
}

func TestLinkRattacheUneIdentite(t *testing.T) {
	store, path := newStore(t)

	account, err := store.Create("Alice", "motdepasse")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	linked, err := store.Link(account.ID, ProviderGoogle, "42", "alice@example.com")
	if err != nil {
		t.Fatalf("liaison impossible : %v", err)
	}
	if linked.ID != account.ID {
		t.Fatalf("compte lié = %q, attendu %q", linked.ID, account.ID)
	}

	// L'identité devient une façon d'entrer, et l'e-mail une clé de recherche.
	if found, ok := store.ByProvider(ProviderGoogle, "42"); !ok || found.ID != account.ID {
		t.Errorf("ByProvider = %+v, %v", found, ok)
	}
	if found, ok := store.ByEmail("  ALICE@example.com  "); !ok || found.ID != account.ID {
		t.Errorf("ByEmail = %+v, %v", found, ok)
	}

	identities := account.Identities()
	if len(identities) != 1 || identities[0].Provider != ProviderGoogle || identities[0].ProviderID != "42" {
		t.Errorf("identités = %+v", identities)
	}

	// La liaison survit au redémarrage.
	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("réouverture impossible : %v", err)
	}
	if _, ok := reopened.ByProvider(ProviderGoogle, "42"); !ok {
		t.Error("la liaison ne survit pas au redémarrage")
	}
}

func TestLinkEstIdempotent(t *testing.T) {
	store, _ := newStore(t)

	account, err := store.CreateExternal(ProviderGoogle, "42", "Alice", "alice@example.com")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	// Rattacher l'identité d'origine est un sans-effet.
	if _, err := store.Link(account.ID, ProviderGoogle, "42", "alice@example.com"); err != nil {
		t.Fatalf("liaison impossible : %v", err)
	}

	// Et rattacher deux fois une même identité n'ajoute rien non plus.
	if _, err := store.Link(account.ID, ProviderDiscord, "7", "alice@example.com"); err != nil {
		t.Fatalf("liaison impossible : %v", err)
	}
	if _, err := store.Link(account.ID, ProviderDiscord, "7", ""); err != nil {
		t.Fatalf("liaison répétée impossible : %v", err)
	}

	identities := account.Identities()
	if len(identities) != 2 {
		t.Fatalf("identités = %+v, attendu origine + Discord", identities)
	}
}

func TestLinkRefuseUneIdentiteDAutrui(t *testing.T) {
	store, _ := newStore(t)

	alice, err := store.CreateExternal(ProviderGoogle, "42", "Alice", "")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	bob, err := store.CreateExternal(ProviderDiscord, "7", "Bob", "")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	// L'identité Google appartient déjà à Alice : elle ne rejoint pas Bob.
	_, err = store.Link(bob.ID, ProviderGoogle, "42", "")
	if code := protocol.CodeOf(err); code != protocol.CodeOAuthLinkedElsewhere {
		t.Fatalf("code = %q, attendu %q", code, protocol.CodeOAuthLinkedElsewhere)
	}

	if found, _ := store.ByProvider(ProviderGoogle, "42"); found.ID != alice.ID {
		t.Errorf("l'identité a été déplacée vers %q", found.Username)
	}
	if len(bob.Identities()) != 1 {
		t.Errorf("Bob a reçu une identité : %+v", bob.Identities())
	}
}

func TestLinkRefuseDeuxIdentitesDuMemeFournisseur(t *testing.T) {
	store, _ := newStore(t)

	account, err := store.CreateExternal(ProviderGoogle, "42", "Alice", "")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	// Un compte n'a qu'une identité par fournisseur : une seconde, chez Google,
	// désigne quelqu'un d'autre.
	_, err = store.Link(account.ID, ProviderGoogle, "99", "")
	if code := protocol.CodeOf(err); code != protocol.CodeOAuthLinkedElsewhere {
		t.Fatalf("code = %q, attendu %q", code, protocol.CodeOAuthLinkedElsewhere)
	}
}

func TestLinkCompteInconnu(t *testing.T) {
	store, _ := newStore(t)

	if _, err := store.Link("inconnu", ProviderGoogle, "42", ""); err == nil {
		t.Error("une liaison sur un compte inconnu devrait échouer")
	}
}

func TestByEmailNeConfondPasLesCasses(t *testing.T) {
	store, _ := newStore(t)

	account, err := store.CreateExternal(ProviderGoogle, "42", "Alice", "Alice@Example.com")
	if err != nil {
		t.Fatalf("création impossible : %v", err)
	}

	if found, ok := store.ByEmail("alice@example.com"); !ok || found.ID != account.ID {
		t.Errorf("ByEmail = %+v, %v", found, ok)
	}
	if _, ok := store.ByEmail(""); ok {
		t.Error("une adresse vide ne doit rien retrouver")
	}
	if _, ok := store.ByEmail("autre@example.com"); ok {
		t.Error("une adresse inconnue ne doit rien retrouver")
	}
}

// TestAncienFormatSeCharge : un fichier écrit avant l'apparition des liaisons
// (origine seule, sans `links`) reste lisible, et son e-mail est indexé.
func TestAncienFormatSeCharge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")

	content := `{"accounts":[{"id":"abc","username":"Jean","key":"jean",` +
		`"provider":"google","providerId":"42","email":"jean@example.com",` +
		`"createdAt":"2024-01-01T00:00:00Z"}]}`

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("ouverture impossible : %v", err)
	}

	account, ok := store.ByProvider(ProviderGoogle, "42")
	if !ok {
		t.Fatal("l'origine n'est pas indexée")
	}
	if found, ok := store.ByEmail("jean@example.com"); !ok || found.ID != "abc" {
		t.Errorf("ByEmail = %+v, %v", found, ok)
	}
	if identities := account.Identities(); len(identities) != 1 || identities[0].ProviderID != "42" {
		t.Errorf("identités = %+v", identities)
	}
}
