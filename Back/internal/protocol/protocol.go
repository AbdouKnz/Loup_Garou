// Package protocol décrit le fil JSON échangé entre le client et le serveur.
//
// Le serveur est la seule autorité du jeu : le client n'envoie que des
// intentions, et le serveur renvoie à chaque joueur une vue personnalisée de
// l'état (chaque joueur ne reçoit que ce qu'il a le droit de savoir).
//
// Toutes les trames partagent la même enveloppe :
//
//	{ "type": "chat.send", "data": { "text": "coucou" } }
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MessageType identifie une trame du protocole.
type MessageType string

// Trames envoyées par le client au serveur.
const (
	TypeAuth          MessageType = "auth"
	TypeChatSend      MessageType = "chat.send"
	TypePlayerReady   MessageType = "player.ready"
	TypeGameStart     MessageType = "game.start"
	TypeGameAgain     MessageType = "game.again"
	TypeGameVote      MessageType = "game.vote"
	TypeGameNightVote MessageType = "game.night.vote"
	// TypeGameNightAction est l'action d'un rôle pendant sa veillée : la voyante
	// observe un joueur, la sorcière se sert de ses potions.
	TypeGameNightAction MessageType = "game.night.action"
	// TypeGameSpeakDone clôt la prise de parole de son auteur : le débat passe
	// au vivant suivant sans attendre l'échéance.
	TypeGameSpeakDone MessageType = "game.speak.done"
	TypeBotAdd        MessageType = "bot.add"
	TypeBotRemove     MessageType = "bot.remove"
	TypePing          MessageType = "ping"
)

// Trames envoyées par le serveur au client.
const (
	TypeAuthOK      MessageType = "auth.ok"
	TypeRoomState   MessageType = "room.state"
	TypeChatMessage MessageType = "chat.message"
	TypeGameEvent   MessageType = "game.event"
	TypePong        MessageType = "pong"
	TypeError       MessageType = "error"
)

// Phase décrit l'étape d'une partie : la salle d'attente, puis le cycle
// nuit/ jour du moteur de jeu, et enfin l'écran de fin.
//
// Les valeurs sont celles du moteur (package game) : c'est ici qu'elles
// deviennent un contrat public, et c'est à ces chaînes que l'interface se fie.
type Phase string

// Étapes d'une partie.
const (
	// PhaseLobby est la salle d'attente, avant le lancement de la partie.
	PhaseLobby Phase = "lobby"
	// PhaseRoles est la distribution des rôles : la partie est en pause, le
	// temps que chacun découvre le sien.
	PhaseRoles Phase = "roles"
	// PhaseNight : le village dort, les loups choisissent leur victime.
	PhaseNight Phase = "nuit"
	// PhaseDawn : le village découvre la victime de la nuit.
	PhaseDawn Phase = "aube"
	// PhaseDebate : le village discute, à la recherche d'un loup.
	PhaseDebate Phase = "debat"
	// PhaseVote : chacun désigne un suspect.
	PhaseVote Phase = "vote"
	// PhaseVerdict : le village découvre qui il vient d'éliminer.
	PhaseVerdict Phase = "verdict"
	// PhaseOver : la partie est finie, tous les rôles sont révélés.
	PhaseOver Phase = "fin"
)

// Roles d'un joueur, tels qu'ils circulent sur le fil.
const (
	RoleVillageois = "villageois"
	RoleLoupGarou  = "loup-garou"
	// RoleVoyante observe un joueur chaque nuit et découvre son rôle.
	RoleVoyante = "voyante"
	// RoleSorciere dispose d'une potion de vie et d'une potion de mort.
	RoleSorciere = "sorciere"
)

// Actions d'une veillée, telles qu'elles circulent sur le fil.
//
// Le serveur invite le joueur avec « devorer », « voir » ou « potions » ; le
// joueur répond « voir », « soigner », « empoisonner » ou « passer ».
const (
	ActionDevour  = "devorer"
	ActionSee     = "voir"
	ActionPotions = "potions"
	ActionHeal    = "soigner"
	ActionPoison  = "empoisonner"
	ActionPass    = "passer"
)

// Causes de mort, telles qu'elles circulent sur le fil.
const (
	// CauseWolves : la proie des loups.
	CauseWolves = "loups"
	// CausePoison : la potion de mort de la sorcière.
	CausePoison = "poison"
)

// Limites du protocole, partagées par le serveur et l'interface.
const (
	// MinPlayers est le nombre minimal de joueurs pour lancer une partie.
	MinPlayers = 6
	// MaxPlayers est le nombre maximal de joueurs par partie.
	MaxPlayers = 12
	// MinPseudoLength et MaxPseudoLength bornent un pseudo, en runes.
	MinPseudoLength = 2
	MaxPseudoLength = 20
	// MaxChatLength borne un message de chat, en runes.
	MaxChatLength = 500
	// ChatBurst et ChatWindow limitent le débit de chat d'un joueur.
	ChatBurst  = 5
	ChatWindow = 5 * time.Second
	// MaxMessages est le nombre de messages conservés en mémoire par salle.
	MaxMessages = 100
	// MessagesInState est le nombre de messages inclus dans un instantané.
	MessagesInState = 50
	// RoomCodeLength est le nombre de caractères d'un code de partie.
	RoomCodeLength = 6
	// MaxFrameSize borne la taille d'une trame WebSocket acceptée (anti-DoS).
	MaxFrameSize = 8 << 10
)

// Envelope est la trame générique échangée sur le WebSocket.
type Envelope struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// DecodeData décode la charge utile de l'enveloppe dans dst.
func (e Envelope) DecodeData(dst any) error {
	if len(e.Data) == 0 {
		return NewError(CodeInvalidMessage, "Charge utile manquante pour la trame « %s ».", e.Type)
	}
	if err := json.Unmarshal(e.Data, dst); err != nil {
		return NewError(CodeInvalidMessage, "Charge utile illisible pour la trame « %s ».", e.Type)
	}
	return nil
}

// Encode construit une trame prête à être envoyée.
func Encode(t MessageType, payload any) (Envelope, error) {
	env := Envelope{Type: t}
	if payload == nil {
		return env, nil
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("encodage de la trame %s : %w", t, err)
	}
	env.Data = raw

	return env, nil
}

// MustEncode est Encode pour les charges utiles dont l'encodage ne peut pas
// échouer (toutes les trames du serveur sont des structures simples).
func MustEncode(t MessageType, payload any) Envelope {
	env, err := Encode(t, payload)
	if err != nil {
		panic(err)
	}
	return env
}

// IsClientType indique si t est une trame qu'un client a le droit d'envoyer.
func IsClientType(t MessageType) bool {
	switch t {
	case TypeAuth, TypeChatSend, TypePlayerReady, TypeGameStart, TypeGameAgain,
		TypeGameVote, TypeGameNightVote, TypeGameNightAction, TypeGameSpeakDone,
		TypeBotAdd, TypeBotRemove, TypePing:
		return true
	default:
		return false
	}
}

// AuthPayload ouvre une session : le joueur présente le jeton obtenu en
// créant ou en rejoignant la partie.
type AuthPayload struct {
	Token string `json:"token"`
}

// AuthOKPayload confirme l'authentification et rappelle l'identité du joueur.
type AuthOKPayload struct {
	PlayerID string `json:"playerId"`
}

// ChatSendPayload est un message de chat envoyé par un joueur.
type ChatSendPayload struct {
	Text string `json:"text"`
}

// PlayerReadyPayload indique qu'un joueur est prêt (ou ne l'est plus).
type PlayerReadyPayload struct {
	Ready bool `json:"ready"`
}

// VotePayload désigne une cible : la victime de la nuit pour « game.night.vote »,
// le suspect du village pour « game.vote ».
//
// Un champ Target vide est un vote blanc, c'est-à-dire une abstention.
type VotePayload struct {
	Target string `json:"target"`
}

// NightActionPayload est l'action d'un rôle pendant sa veillée.
//
// Action dit ce que le joueur fait : « voir » (voyante), « soigner »,
// « empoisonner » ou « passer » (sorcière). Target est la cible choisie,
// ignorée par « passer ».
type NightActionPayload struct {
	Action string `json:"action"`
	Target string `json:"target"`
}

// BotAddPayload demande l'ajout de joueurs automatiques en salle d'attente.
//
// Count est le nombre de bots souhaités ; la salle le plafonne aux places
// restantes. Réservé à l'hôte.
type BotAddPayload struct {
	Count int `json:"count"`
}

// BotRemovePayload désigne un bot à retirer.
//
// Un Target vide signifie « tous les bots » : c'est le bouton « Retirer tous les
// bots » de l'interface. Réservé à l'hôte.
type BotRemovePayload struct {
	Target string `json:"target"`
}

// GameEventPayload raconte un fait de jeu au client.
//
// Le texte est la phrase du narrateur — le système, jamais un joueur — et il
// est également publié dans le fil de discussion comme message système. Cette
// trame sert à l'interface pour réagir à l'instant (bandeau, couleur).
type GameEventPayload struct {
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	Phase Phase  `json:"phase"`
	Day   int    `json:"day"`
	Round int    `json:"round"`
	// Target est le joueur concerné, le cas échéant.
	Target string `json:"target,omitempty"`
	// Cause dit de quoi une victime est morte (« loups », « poison »).
	Cause string `json:"cause,omitempty"`
	// Role n'est renseigné que lorsque le rôle est public (élimination du jour,
	// fin de partie).
	Role string `json:"role,omitempty"`
	// Winners n'est renseigné qu'à la fin : « village » ou « loups ».
	Winners string `json:"winners,omitempty"`
}

// ErrorPayload transmet une erreur applicative au client.
type ErrorPayload struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// PlayerView est la vue d'un joueur dans une salle.
//
// En salle d'attente, elle ne contient que l'identité de la table. Pendant une
// partie, Dead et Voted résument ce que tout le monde peut voir : qui est mort,
// et qui a déjà voté.
type PlayerView struct {
	ID     string `json:"id"`
	Pseudo string `json:"pseudo"`
	Ready  bool   `json:"ready"`
	Online bool   `json:"online"`
	Host   bool   `json:"host"`
	// Dead indique un joueur éliminé (ou dévoré) : il ne joue plus.
	Dead bool `json:"dead,omitempty"`
	// Voted indique que ce joueur a déjà voté au tour courant.
	Voted bool `json:"voted,omitempty"`
	// Bot signale un joueur automatique ajouté par l'hôte.
	Bot bool `json:"bot,omitempty"`
}

// GameView est la partie telle qu'un joueur donné a le droit de la voir.
//
// Les champs privés (YourRole, Wolves, WolfVotes, Roles) sont remplis par le
// moteur selon le destinataire : le serveur ne calcule jamais cette vue
// lui-même, il recopie celle du moteur.
type GameView struct {
	Phase       Phase     `json:"phase"`
	Day         int       `json:"day"`
	Round       int       `json:"round"`
	Deadline    time.Time `json:"deadline,omitempty"`
	SecondsLeft int       `json:"secondsLeft"`
	// Winner est vide tant que la partie dure.
	Winner string `json:"winner,omitempty"`
	// YourRole est le rôle du destinataire de la vue.
	YourRole string `json:"yourRole,omitempty"`
	// YourVote est la cible déjà désignée par le destinataire, au tour courant.
	YourVote string `json:"yourVote,omitempty"`

	// Alive et Dead listent les joueurs, dans l'ordre de la table.
	Alive []string `json:"alive"`
	Dead  []string `json:"dead"`
	// Voted liste les joueurs ayant voté au tour courant. La nuit, seuls les
	// loups y figurent — et seuls les loups reçoivent la liste.
	Voted []string `json:"voted"`
	// Ballot est le scrutin du village, public : votant → cible (vide pour une
	// abstention). Renseigné pendant le vote du jour et le verdict qui suit ;
	// la nuit, il est absent.
	Ballot map[string]string `json:"ballot,omitempty"`

	// Wolves et WolfVotes sont réservés aux loups.
	Wolves    []string          `json:"wolves,omitempty"`
	WolfVotes map[string]string `json:"wolfVotes,omitempty"`

	// YourAction est l'action que la veillée en cours attend du destinataire :
	// « devorer », « voir » ou « potions ». Absent s'il n'a rien à faire.
	YourAction string `json:"yourAction,omitempty"`
	// YourVision est le carnet de la voyante, réservé à elle.
	YourVision []VisionView `json:"yourVision,omitempty"`
	// YourPotions dit ce qu'il reste à la sorcière, réservé à elle.
	YourPotions *PotionsView `json:"yourPotions,omitempty"`
	// WolfTarget est la proie que les loups viennent de désigner, révélée à la
	// sorcière pendant sa veillée.
	WolfTarget string `json:"wolfTarget,omitempty"`

	// Deaths sont les morts de la nuit (rôle non révélé), Eliminated l'éliminé
	// du jour (rôle révélé).
	Deaths         []DeathView `json:"deaths,omitempty"`
	Eliminated     string      `json:"eliminated,omitempty"`
	EliminatedRole string      `json:"eliminatedRole,omitempty"`

	// Tie et Candidates décrivent un second tour en cours.
	Tie        bool     `json:"tie,omitempty"`
	Candidates []string `json:"candidates,omitempty"`

	// Speaker, Speakers et Spoken décrivent le tour de parole du débat : qui
	// parle à cet instant, l'ordre des vivants, et ceux qui ont déjà parlé.
	// C'est une information publique, sans rapport avec les rôles.
	Speaker  string   `json:"speaker,omitempty"`
	Speakers []string `json:"speakers,omitempty"`
	Spoken   []string `json:"spoken,omitempty"`

	// Roles n'est rempli qu'une fois la partie finie : tous les rôles sont
	// alors publics.
	Roles map[string]string `json:"roles,omitempty"`
}

// IsOver indique si la vue décrit une partie terminée.
func (g *GameView) IsOver() bool { return g != nil && g.Phase == PhaseOver }

// VisionView est une observation de la voyante : ce qu'elle a vu, et quand.
type VisionView struct {
	Day    int    `json:"day"`
	Target string `json:"target"`
	Role   string `json:"role"`
}

// PotionsView dit ce qu'il reste à la sorcière : chaque potion ne sert qu'une
// fois.
type PotionsView struct {
	Heal   bool `json:"heal"`
	Poison bool `json:"poison"`
}

// DeathView est un mort de la nuit et sa cause, jamais son rôle.
type DeathView struct {
	Target string `json:"target"`
	Cause  string `json:"cause"`
}

// ChatMessage est une ligne du fil de discussion d'une salle.
type ChatMessage struct {
	ID       string    `json:"id"`
	PlayerID string    `json:"playerId,omitempty"`
	Pseudo   string    `json:"pseudo"`
	Text     string    `json:"text"`
	At       time.Time `json:"at"`
	// System signale les nouvelles « automatiques » (arrivées, départs).
	System bool `json:"system,omitempty"`
}

// RoomState est l'instantané complet d'une salle, tel qu'un joueur donné le voit.
type RoomState struct {
	Code       string        `json:"code"`
	Phase      Phase         `json:"phase"`
	HostID     string        `json:"hostId"`
	You        string        `json:"you"`
	MaxPlayers int           `json:"maxPlayers"`
	CanStart   bool          `json:"canStart"`
	Players    []PlayerView  `json:"players"`
	Messages   []ChatMessage `json:"messages"`
	// Composition est la répartition des rôles de la table, telle qu'elle sera
	// distribuée : annoncée à tout le monde, elle ne dépend que du nombre de
	// joueurs.
	Composition []string `json:"composition"`
	// Game n'est présent que pendant une partie (fin comprise).
	Game *GameView `json:"game,omitempty"`
}

// IsPlaying indique si la phase décrit une partie en cours ou terminée, par
// opposition à la salle d'attente. C'est la seule phase où l'on peut entrer.
func (p Phase) IsPlaying() bool { return p != PhaseLobby }

// RoomSummary décrit une salle sans révéler son état interne : c'est ce que
// voit un joueur avant d'entrer (écran « rejoindre une partie »).
type RoomSummary struct {
	Code       string `json:"code"`
	Phase      Phase  `json:"phase"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"maxPlayers"`
}

// ErrorCode identifie une erreur applicative, côté serveur comme côté client.
type ErrorCode string

// Codes d'erreur du protocole.
const (
	CodeInvalidMessage   ErrorCode = "invalid_message"
	CodeInvalidPseudo    ErrorCode = "invalid_pseudo"
	CodeInvalidCode      ErrorCode = "invalid_room_code"
	CodeRoomNotFound     ErrorCode = "room_not_found"
	CodeRoomFull         ErrorCode = "room_full"
	CodeNotAuthenticated ErrorCode = "not_authenticated"
	CodeUnauthorized     ErrorCode = "unauthorized"
	CodeRateLimited      ErrorCode = "rate_limited"
	CodeNotImplemented   ErrorCode = "not_implemented"
	CodeInternal         ErrorCode = "internal"

	// Codes propres aux comptes.
	CodeUsernameTaken  ErrorCode = "username_taken"
	CodeBadCredentials ErrorCode = "bad_credentials"
	CodeWeakPassword   ErrorCode = "weak_password"

	// Codes propres aux connexions externes (Google, Discord).
	//
	// OAuthState dit que la tentative n'est plus là, ou n'est pas la nôtre ;
	// OAuthFailed, que le fournisseur a répondu de travers ou pas du tout ;
	// OAuthLinkedElsewhere, que l'identité est déjà rattachée à un autre compte.
	CodeOAuthState           ErrorCode = "oauth_state"
	CodeOAuthFailed          ErrorCode = "oauth_failed"
	CodeOAuthLinkedElsewhere ErrorCode = "oauth_linked_elsewhere"

	// Codes propres au jeu.
	//
	// NotEnoughPlayers et GameInProgress sont des refus de table ; les autres
	// sont des refus d'action, adressés au seul joueur concerné.
	CodeNotEnoughPlayers ErrorCode = "not_enough_players"
	CodeGameInProgress   ErrorCode = "game_in_progress"
	CodeGameOver         ErrorCode = "game_over"
	CodeWrongPhase       ErrorCode = "wrong_phase"
	CodeNotWolf          ErrorCode = "not_a_wolf"
	CodeWrongAction      ErrorCode = "wrong_action"
	CodePlayerDead       ErrorCode = "player_dead"
	CodeBadTarget        ErrorCode = "bad_target"
)

// AppError est une erreur applicative porteuse d'un code transmissible au client.
type AppError struct {
	Code    ErrorCode
	Message string
}

// Error implémente error.
func (e *AppError) Error() string {
	return string(e.Code) + " : " + e.Message
}

// NewError construit une erreur applicative dont le message est destiné au joueur.
func NewError(code ErrorCode, format string, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CodeOf extrait le code d'une erreur, CodeInternal si elle n'est pas applicative.
func CodeOf(err error) ErrorCode {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}

// MessageOf extrait le message d'une erreur applicative ; les erreurs techniques
// sont masquées pour ne rien divulguer de l'état du serveur.
func MessageOf(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "Erreur interne du serveur."
}

// ErrorFor construit la trame d'erreur correspondant à err.
func ErrorFor(err error) Envelope {
	return MustEncode(TypeError, ErrorPayload{
		Code:    CodeOf(err),
		Message: MessageOf(err),
	})
}

// StatusFor associe un code d'erreur applicatif au statut HTTP correspondant
// (réponses REST et échecs de poignée de main WebSocket).
func StatusFor(err error) int {
	switch CodeOf(err) {
	case CodeInvalidMessage, CodeInvalidPseudo, CodeInvalidCode, CodeBadTarget, CodeWeakPassword,
		CodeOAuthState, CodeOAuthFailed:
		return http.StatusBadRequest
	case CodeNotAuthenticated, CodeBadCredentials:
		return http.StatusUnauthorized
	case CodeUnauthorized, CodeNotWolf, CodeWrongAction, CodePlayerDead, CodeWrongPhase:
		return http.StatusForbidden
	case CodeRoomNotFound:
		return http.StatusNotFound
	case CodeRoomFull, CodeNotEnoughPlayers, CodeGameInProgress, CodeGameOver, CodeUsernameTaken,
		CodeOAuthLinkedElsewhere:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeNotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

// roomAlphabet écarte les caractères ambigus (I, L, O, 0, 1) : un code de
// partie dicté de vive voix reste sans erreur.
const roomAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// RoomAlphabet renvoie l'alphabet utilisé pour composer les codes de partie.
func RoomAlphabet() string { return roomAlphabet }

// NormalizeRoomCode met le code en majuscules et vérifie sa forme.
func NormalizeRoomCode(raw string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if utf8.RuneCountInString(code) != RoomCodeLength {
		return "", NewError(CodeInvalidCode, "Le code de partie doit contenir %d caractères.", RoomCodeLength)
	}
	for _, r := range code {
		if !strings.ContainsRune(roomAlphabet, r) {
			return "", NewError(CodeInvalidCode, "Ce code de partie est invalide.")
		}
	}
	return code, nil
}

// NormalizePseudo nettoie un pseudo et vérifie sa longueur.
func NormalizePseudo(raw string) (string, error) {
	pseudo := strings.Join(strings.Fields(raw), " ")

	length := utf8.RuneCountInString(pseudo)
	if length < MinPseudoLength || length > MaxPseudoLength {
		return "", NewError(CodeInvalidPseudo,
			"Le pseudo doit contenir entre %d et %d caractères.", MinPseudoLength, MaxPseudoLength)
	}

	for _, r := range pseudo {
		if unicode.IsControl(r) {
			return "", NewError(CodeInvalidPseudo, "Le pseudo contient des caractères interdits.")
		}
	}

	return pseudo, nil
}

// NormalizeChat nettoie un message de chat et vérifie sa longueur.
func NormalizeChat(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", NewError(CodeInvalidMessage, "Le message est vide.")
	}
	if utf8.RuneCountInString(text) > MaxChatLength {
		return "", NewError(CodeInvalidMessage, "Le message dépasse %d caractères.", MaxChatLength)
	}
	return text, nil
}
