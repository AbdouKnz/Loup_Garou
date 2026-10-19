// Package accounts gère les comptes des joueurs.
//
// Un compte naît de deux façons : créé ici, avec un pseudo et un mot de passe,
// ou rattaché à un fournisseur externe (Google, Discord) dont l'identité a été
// vérifiée — c'est l'objet des champs Provider, ProviderID et Email. Dans les
// deux cas le joueur choisit son pseudo : c'est le nom que la table affiche.
//
// Les comptes vivent dans un fichier JSON : le serveur garde les parties en
// mémoire, mais une identité doit survivre au redémarrage.
package accounts

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/abdou/loupgarou/internal/filestore"
	"github.com/abdou/loupgarou/internal/protocol"
)

// Provider dit d'où vient une identité.
type Provider string

// ProviderLocal est le compte créé sur le site, avec un mot de passe.
const ProviderLocal Provider = "local"

// ProviderGoogle et ProviderDiscord sont les identités vérifiées par un
// fournisseur externe : ces comptes n'ont pas de mot de passe.
const (
	// ProviderGoogle est une identité vérifiée par Google.
	ProviderGoogle Provider = "google"
	// ProviderDiscord est une identité vérifiée par Discord.
	ProviderDiscord Provider = "discord"
)

// Coût du dérivatage du mot de passe et taille de la clé produite.
const (
	// DefaultIterations est le nombre d'itérations PBKDF2-SHA256. C'est le
	// rempart contre le cassage hors ligne : il coûte une fraction de seconde
	// au serveur, et des années à qui voudrait essayer des mots de passe.
	DefaultIterations = 600_000
	// MinIterations est le plancher accepté par SetIterations : en dessous, le
	// hachage ne protégerait plus rien.
	MinIterations = 1_000
	// keyLength est la taille de la clé dérivée, en octets.
	keyLength = 32
	// saltLength est la taille du sel tiré pour chaque compte.
	saltLength = 16
)

// Bornes d'un mot de passe, en caractères.
const (
	// MinPasswordLength est la longueur minimale acceptée.
	MinPasswordLength = 8
	// MaxPasswordLength borne ce qu'un joueur peut envoyer : au-delà, c'est
	// une erreur de saisie, pas un mot de passe.
	MaxPasswordLength = 200
)

// Account est l'identité d'un joueur, telle qu'elle est écrite sur le disque.
type Account struct {
	ID string `json:"id"`
	// Username est le pseudo affiché, tel que le joueur l'a saisi.
	Username string `json:"username"`
	// Key est le pseudo replié : c'est la clé d'unicité, insensible à la casse
	// et aux espaces superflus.
	Key string `json:"key"`
	// Provider dit qui a créé l'identité ; ProviderID est l'identifiant stable
	// chez ce fournisseur, vide pour un compte local.
	Provider   Provider `json:"provider"`
	ProviderID string   `json:"providerId,omitempty"`
	// Email vient d'un fournisseur externe. C'est une information, jamais une
	// identité : on ne s'en sert pas pour se connecter et on ne l'affiche pas.
	Email string `json:"email,omitempty"`
	// Links sont les identités externes rattachées *en plus* de l'origine : un
	// même joueur garde un seul compte, qu'il passe par Google puis Discord.
	// L'origine (Provider/ProviderID) reste la première identité.
	Links []Link `json:"links,omitempty"`
	// PasswordHash, Salt et Iterations sont vides pour un compte externe.
	PasswordHash string    `json:"passwordHash,omitempty"`
	Salt         string    `json:"salt,omitempty"`
	Iterations   int       `json:"iterations,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Link est une identité externe rattachée à un compte.
type Link struct {
	Provider   Provider  `json:"provider"`
	ProviderID string    `json:"providerId"`
	Email      string    `json:"email,omitempty"`
	LinkedAt   time.Time `json:"linkedAt"`
}

// Identities énumère les façons de s'identifier d'un compte : son origine
// d'abord, puis les identités rattachées. C'est ce qui permet à un même joueur
// d'entrer par Google ou par Discord sans ouvrir un second compte.
func (a *Account) Identities() []Link {
	if a == nil {
		return nil
	}

	identities := make([]Link, 0, len(a.Links)+1)
	if a.ProviderID != "" {
		identities = append(identities, Link{
			Provider:   a.Provider,
			ProviderID: a.ProviderID,
			Email:      a.Email,
			LinkedAt:   a.CreatedAt,
		})
	}

	return append(identities, a.Links...)
}

// file est la forme du fichier sur le disque.
type file struct {
	Accounts []Account `json:"accounts"`
}

// Store tient les comptes en mémoire et les écrit sur le disque.
type Store struct {
	mu       sync.Mutex
	path     string
	now      func() time.Time
	accounts map[string]*Account // par identifiant
	byKey    map[string]*Account // par pseudo replié
	// byProvider retrouve un compte par son fournisseur d'identité externe :
	// c'est ce qui rend une connexion externe idempotente. Il indexe l'origine
	// et chaque identité rattachée (Links).
	byProvider map[string]*Account
	// byEmail retrouve un compte par l'un de ses e-mails vérifiés : c'est ce
	// qui rattache une nouvelle identité au compte existant plutôt que d'en
	// ouvrir un second pour le même joueur.
	byEmail map[string]*Account
	// iterations est le coût appliqué aux comptes créés ensuite. Les comptes
	// existants gardent le leur, écrit à leur création.
	iterations int
}

// NewStore ouvre le fichier des comptes et charge ceux qui y sont déjà.
//
// Un fichier absent est normal : c'est le premier démarrage.
func NewStore(path string) (*Store, error) {
	store := &Store{
		path:       path,
		now:        time.Now,
		accounts:   make(map[string]*Account),
		byKey:      make(map[string]*Account),
		byProvider: make(map[string]*Account),
		byEmail:    make(map[string]*Account),
		iterations: DefaultIterations,
	}

	var content file
	if _, err := filestore.Load(path, &content); err != nil {
		return nil, err
	}

	for i := range content.Accounts {
		account := content.Accounts[i]
		store.accounts[account.ID] = &account

		// Un fichier modifié à la main pourrait contenir deux fois la même clé :
		// le premier compte garde la sienne, les suivants sont ignorés.
		if _, taken := store.byKey[account.Key]; !taken {
			store.byKey[account.Key] = &account
		}

		// Origine et identités rattachées partagent les mêmes index : entrer
		// par l'une ou par l'autre mène au même compte.
		for _, link := range account.Identities() {
			if key := providerKey(link.Provider, link.ProviderID); link.ProviderID != "" {
				if _, taken := store.byProvider[key]; !taken {
					store.byProvider[key] = &account
				}
			}

			if key := emailKey(link.Email); key != "" {
				if _, taken := store.byEmail[key]; !taken {
					store.byEmail[key] = &account
				}
			}
		}
	}

	return store, nil
}

// SetIterations règle le coût du dérivatage des comptes créés ensuite.
//
// Les tests le baissent : ce qui compte chez eux, c'est le chemin, pas la
// résistance à la force brute.
func (s *Store) SetIterations(n int) {
	if n < MinIterations {
		n = MinIterations
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.iterations = n
}

// Count renvoie le nombre de comptes connus.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.accounts)
}

// Create ouvre un compte local : pseudo et mot de passe.
func (s *Store) Create(username, password string) (*Account, error) {
	clean, err := protocol.NormalizePseudo(username)
	if err != nil {
		return nil, err
	}
	if err := checkPassword(password); err != nil {
		return nil, err
	}

	key := foldKey(clean)

	s.mu.Lock()
	_, taken := s.byKey[key]
	iterations := s.iterations
	s.mu.Unlock()

	// Un pseudo déjà pris se refuse avant le dérivatage : le refus est immédiat
	// et ne coûte pas une seconde de calcul au serveur.
	if taken {
		return nil, errUsernameTaken()
	}

	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("sel aléatoire : %w", err)
	}

	derived, err := derive(password, salt, iterations)
	if err != nil {
		return nil, err
	}

	return s.add(&Account{
		Username:     clean,
		Key:          key,
		Provider:     ProviderLocal,
		PasswordHash: base64.RawStdEncoding.EncodeToString(derived),
		Salt:         base64.RawStdEncoding.EncodeToString(salt),
		Iterations:   iterations,
	})
}

// CreateExternal rattache un compte à un fournisseur d'identité externe.
//
// Aucun mot de passe n'est demandé : le fournisseur a déjà vérifié l'identité, et
// le compte ne pourra se rouvrir que par lui. Le pseudo, lui, reste choisi par le
// joueur : c'est le nom que la table affiche.
func (s *Store) CreateExternal(provider Provider, providerID, username, email string) (*Account, error) {
	if provider == ProviderLocal || providerID == "" {
		return nil, errors.New("compte externe sans fournisseur")
	}

	clean, err := protocol.NormalizePseudo(username)
	if err != nil {
		return nil, err
	}

	return s.add(&Account{
		Username:   clean,
		Key:        foldKey(clean),
		Provider:   provider,
		ProviderID: providerID,
		Email:      email,
	})
}

// add range un nouveau compte, après avoir vérifié que le pseudo est libre.
//
// L'écriture n'est pas facultative : un compte absent du fichier disparaîtrait
// au redémarrage, et le joueur se retrouverait devant un pseudo pris — le sien,
// sans pouvoir le reprendre.
func (s *Store) add(account *Account) (*Account, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}

	account.ID = id
	account.CreatedAt = s.now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, taken := s.byKey[account.Key]; taken {
		return nil, errUsernameTaken()
	}

	s.accounts[account.ID] = account
	s.byKey[account.Key] = account

	providerIndex := ""
	if account.ProviderID != "" {
		providerIndex = providerKey(account.Provider, account.ProviderID)
		s.byProvider[providerIndex] = account
	}

	// L'e-mail est indexé s'il est libre : deux comptes peuvent le partager, et
	// le premier reste la référence.
	emailIndex := ""
	if key := emailKey(account.Email); key != "" {
		if _, taken := s.byEmail[key]; !taken {
			s.byEmail[key] = account
			emailIndex = key
		}
	}

	if err := s.save(); err != nil {
		// Sans écriture, le compte disparaîtrait au redémarrage : on préfère
		// annuler la création que de laisser une identité en trompe-l'œil.
		delete(s.accounts, account.ID)
		delete(s.byKey, account.Key)
		if providerIndex != "" {
			delete(s.byProvider, providerIndex)
		}
		if emailIndex != "" {
			delete(s.byEmail, emailIndex)
		}

		return nil, err
	}

	return account, nil
}

// Verify retrouve un compte local à partir de son pseudo et de son mot de passe.
//
// Un pseudo inconnu et un mot de passe faux donnent la même réponse : on
// n'apprend pas au visiteur quels comptes existent.
func (s *Store) Verify(username, password string) (*Account, error) {
	key := foldKey(username)

	s.mu.Lock()
	account, ok := s.byKey[key]
	s.mu.Unlock()

	if !ok || !verifyPassword(account, password) {
		return nil, protocol.NewError(protocol.CodeBadCredentials,
			"Pseudo ou mot de passe incorrect.")
	}

	return account, nil
}

// ByID retrouve un compte par son identifiant.
func (s *Store) ByID(id string) (*Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accounts[id]

	return account, ok
}

// ByUsername retrouve un compte par son pseudo, sans tenir compte de la casse.
func (s *Store) ByUsername(username string) (*Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.byKey[foldKey(username)]

	return account, ok
}

// ByProvider retrouve le compte rattaché à un fournisseur d'identité externe.
//
// C'est ce qui rend une connexion externe idempotente : la deuxième fois, on
// retrouve le compte au lieu d'en ouvrir un second.
func (s *Store) ByProvider(provider Provider, providerID string) (*Account, bool) {
	if providerID == "" {
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.byProvider[providerKey(provider, providerID)]

	return account, ok
}

// ByEmail retrouve un compte par l'un de ses e-mails vérifiés.
//
// C'est ce qui permet de rattacher une identité inconnue à un compte déjà
// existant — le même joueur qui passe par Google puis par Discord garde un seul
// compte — au lieu d'en créer un second.
func (s *Store) ByEmail(email string) (*Account, bool) {
	key := emailKey(email)
	if key == "" {
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.byEmail[key]

	return account, ok
}

// Link rattache une identité externe à un compte existant.
//
// L'opération est idempotente : rattacher une identité déjà portée par ce compte
// ne fait rien. Elle échoue en revanche si l'identité appartient à un autre
// compte, ou si le compte porte déjà une *autre* identité du même fournisseur —
// un compte n'a qu'une identité par fournisseur.
func (s *Store) Link(accountID string, provider Provider, providerID, email string) (*Account, error) {
	if provider == ProviderLocal || providerID == "" {
		return nil, errors.New("liaison sans fournisseur")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return nil, errors.New("compte inconnu")
	}

	key := providerKey(provider, providerID)

	// Déjà rattachée : à ce compte, il n'y a rien à faire ; à un autre, c'est un
	// refus — on ne déplace pas une identité dans le dos de son porteur.
	if owner, taken := s.byProvider[key]; taken {
		if owner.ID == account.ID {
			return account, nil
		}

		return nil, errLinkedElsewhere()
	}

	if _, found := identityOf(account, provider); found {
		return nil, errLinkedElsewhere()
	}

	link := Link{Provider: provider, ProviderID: providerID, Email: email, LinkedAt: s.now().UTC()}

	previous := account.Links
	account.Links = append(append([]Link{}, previous...), link)

	s.byProvider[key] = account

	// L'e-mail n'est indexé que s'il est libre : deux comptes peuvent le
	// partager, et le premier reste la référence.
	indexedEmail := ""
	if key := emailKey(email); key != "" {
		if _, taken := s.byEmail[key]; !taken {
			s.byEmail[key] = account
			indexedEmail = key
		}
	}

	if err := s.save(); err != nil {
		// Sans écriture, la liaison disparaîtrait au redémarrage : on préfère
		// annuler plutôt que de laisser croire à un rattachement durable.
		account.Links = previous
		delete(s.byProvider, key)
		if indexedEmail != "" {
			delete(s.byEmail, indexedEmail)
		}

		return nil, err
	}

	return account, nil
}

// identityOf dit si un compte porte déjà une identité de ce fournisseur.
func identityOf(account *Account, provider Provider) (Link, bool) {
	for _, link := range account.Identities() {
		if link.Provider == provider {
			return link, true
		}
	}

	return Link{}, false
}

// errLinkedElsewhere construit le refus d'une identité déjà rattachée.
func errLinkedElsewhere() error {
	return protocol.NewError(protocol.CodeOAuthLinkedElsewhere,
		"Ce compte externe est déjà associé à un autre compte.")
}

// emailKey replie un e-mail : c'est la clé de recherche par adresse.
func emailKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// save écrit tous les comptes. À appeler verrou pris.
func (s *Store) save() error {
	content := file{Accounts: make([]Account, 0, len(s.accounts))}
	for _, account := range s.accounts {
		content.Accounts = append(content.Accounts, *account)
	}

	// Ordre stable : le fichier reste lisible, et deux écritures du même état
	// donnent le même fichier.
	sort.Slice(content.Accounts, func(i, j int) bool {
		left, right := content.Accounts[i], content.Accounts[j]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}

		return left.ID < right.ID
	})

	return filestore.Save(s.path, content)
}

// verifyPassword compare le mot de passe au haché du compte, en temps constant.
func verifyPassword(account *Account, password string) bool {
	if account.PasswordHash == "" || account.Salt == "" || account.Iterations <= 0 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(account.Salt)
	if err != nil {
		return false
	}

	want, err := base64.RawStdEncoding.DecodeString(account.PasswordHash)
	if err != nil {
		return false
	}

	got, err := derive(password, salt, account.Iterations)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(got, want) == 1
}

// derive calcule la clé du mot de passe.
func derive(password string, salt []byte, iterations int) ([]byte, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
	if err != nil {
		return nil, fmt.Errorf("dérivatage du mot de passe : %w", err)
	}

	return key, nil
}

// checkPassword refuse un mot de passe hors bornes.
func checkPassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength || length > MaxPasswordLength {
		return protocol.NewError(protocol.CodeWeakPassword,
			"Le mot de passe doit contenir entre %d et %d caractères.",
			MinPasswordLength, MaxPasswordLength)
	}

	return nil
}

// errUsernameTaken construit le refus d'un pseudo déjà pris.
func errUsernameTaken() error {
	return protocol.NewError(protocol.CodeUsernameTaken,
		"Ce pseudo est déjà pris. Choisissez-en un autre.")
}

// providerKey est la clé d'index d'un compte externe : un même identifiant chez
// deux fournisseurs différents désigne deux personnes différentes.
func providerKey(provider Provider, providerID string) string {
	return string(provider) + ":" + providerID
}

// foldKey replie un pseudo : c'est la clé d'unicité des comptes.
func foldKey(username string) string {
	return strings.ToLower(strings.Join(strings.Fields(username), " "))
}

// newID tire un identifiant de compte.
func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("identifiant de compte : %w", err)
	}

	return hex.EncodeToString(buf), nil
}
