package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/abdou/loupgarou/internal/protocol"
)

// maxBodySize borne la taille d'un corps de requête JSON (anti-DoS).
const maxBodySize = 4 << 10

// sessionResponse est renvoyée à la création ou à l'entrée dans une partie.
//
// Le jeton est le secret de session : il n'est transmis qu'ici, une seule fois,
// et sert ensuite à ouvrir le WebSocket.
type sessionResponse struct {
	Code     string `json:"code"`
	PlayerID string `json:"playerId"`
	Token    string `json:"token"`
	Pseudo   string `json:"pseudo"`
}

// handleCreateRoom ouvre une partie au nom du compte connecté : POST /api/rooms
func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFrom(r.Context())
	if !ok {
		writeAPIError(w, protocol.NewError(protocol.CodeNotAuthenticated,
			"Créez un compte ou connectez-vous pour jouer."))

		return
	}

	code, playerID, token, err := s.rooms.Create(account.ID, account.Username)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, sessionResponse{
		Code:     code,
		PlayerID: string(playerID),
		Token:    token,
		Pseudo:   account.Username,
	})
}

// handleJoinRoom fait entrer un joueur : POST /api/rooms/{code}/join
//
// Un compte qui a déjà une place la retrouve : la même requête rejouée ne
// duplique personne.
func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFrom(r.Context())
	if !ok {
		writeAPIError(w, protocol.NewError(protocol.CodeNotAuthenticated,
			"Créez un compte ou connectez-vous pour jouer."))

		return
	}

	code, err := protocol.NormalizeRoomCode(chi.URLParam(r, "code"))
	if err != nil {
		writeAPIError(w, err)
		return
	}

	playerID, token, err := s.rooms.Join(code, account.ID, account.Username)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionResponse{
		Code:     code,
		PlayerID: string(playerID),
		Token:    token,
		Pseudo:   account.Username,
	})
}

// handleRoomInfo décrit une partie : GET /api/rooms/{code}
//
// Sert à valider un code avant d'entrer (écran « rejoindre ») : la route reste
// donc accessible sans compte.
func (s *Server) handleRoomInfo(w http.ResponseWriter, r *http.Request) {
	code, err := protocol.NormalizeRoomCode(chi.URLParam(r, "code"))
	if err != nil {
		writeAPIError(w, err)
		return
	}

	salle, ok := s.rooms.Get(code)
	if !ok {
		writeAPIError(w, protocol.NewError(protocol.CodeRoomNotFound,
			"Cette partie n'existe pas ou a été fermée."))
		return
	}

	summary, err := salle.Summary()
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// decodeJSON lit un corps JSON strict : champs inconnus refusés, taille bornée.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return protocol.NewError(protocol.CodeInvalidMessage, "Requête invalide.")
	}

	return nil
}

// writeAPIError traduit une erreur applicative en réponse JSON.
func writeAPIError(w http.ResponseWriter, err error) {
	status := protocol.StatusFor(err)
	if status == http.StatusInternalServerError {
		log.Printf("erreur interne : %v", err)
	}

	writeJSON(w, status, protocol.ErrorPayload{
		Code:    protocol.CodeOf(err),
		Message: protocol.MessageOf(err),
	})
}
