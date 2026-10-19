// Package ws expose la passerelle WebSocket du jeu.
//
// Elle ne s'occupe que du transport : montée en WebSocket, lecture et écriture
// des trames, maintien de connexion. Toute la logique appartient à la salle
// correspondante (paquet room), qui ne connaît ni HTTP ni WebSocket.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"github.com/abdou/loupgarou/internal/protocol"
	"github.com/abdou/loupgarou/internal/room"
)

// authTimeout est le délai laissé au client pour s'authentifier après la
// montée en WebSocket : au-delà, la connexion est fermée.
const authTimeout = 10 * time.Second

// Server est la passerelle WebSocket, branchée sur le registre des salles.
type Server struct {
	rooms        *room.Registry
	extraOrigins []string
}

// NewServer crée la passerelle.
//
// extraOrigins liste les origines supplémentaires acceptées en plus de celle de
// la requête : en développement, l'interface est servie par Vite sur un autre
// port, et l'origine diffère donc de celle du serveur Go.
func NewServer(rooms *room.Registry, extraOrigins []string) *Server {
	return &Server{rooms: rooms, extraOrigins: extraOrigins}
}

// Handler sert la passerelle d'une salle : GET /api/rooms/{code}/ws
//
// Le jeton de session n'est jamais transmis dans l'URL : la première trame
// reçue doit être « auth », faute de quoi la connexion est fermée.
func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
	code, err := protocol.NormalizeRoomCode(chi.URLParam(r, "code"))
	if err != nil {
		writeHandshakeError(w, err)
		return
	}

	salle, ok := s.rooms.Get(code)
	if !ok {
		writeHandshakeError(w, protocol.NewError(protocol.CodeRoomNotFound,
			"Cette partie n'existe pas ou a été fermée."))
		return
	}

	socket, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.originPatterns(r),
	})
	if err != nil {
		log.Printf("montée en WebSocket impossible : %v", err)
		return
	}
	socket.SetReadLimit(protocol.MaxFrameSize)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client := newConn(socket, salle, r.RemoteAddr)

	playerID, err := s.authenticate(ctx, client)
	if err != nil {
		writeSocketError(ctx, socket, err)
		_ = socket.Close(websocket.StatusPolicyViolation, "authentification refusée")
		return
	}
	client.player = playerID

	go client.writePump(ctx)
	client.readPump(ctx)

	// Le joueur garde sa place : il devient simplement « hors ligne ».
	client.close(websocket.StatusNormalClosure, "fin de session")
	salle.Disconnect(client)
}

// authenticate attend la trame « auth » et vérifie le jeton de session.
func (s *Server) authenticate(ctx context.Context, client *conn) (room.PlayerID, error) {
	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	var env protocol.Envelope
	if err := wsjson.Read(authCtx, client.socket, &env); err != nil {
		return "", protocol.NewError(protocol.CodeNotAuthenticated,
			"Authentification requise avant toute autre trame.")
	}

	if env.Type != protocol.TypeAuth {
		return "", protocol.NewError(protocol.CodeNotAuthenticated,
			"Première trame attendue : « auth ».")
	}

	var payload protocol.AuthPayload
	if err := env.DecodeData(&payload); err != nil {
		return "", err
	}

	playerID, err := client.room.Authenticate(payload.Token, client)
	if err != nil {
		if errors.Is(err, room.ErrClosed) {
			return "", protocol.NewError(protocol.CodeRoomNotFound, "Cette partie a été fermée.")
		}
		return "", err
	}

	return playerID, nil
}

// originPatterns autorise l'origine de la requête elle-même, plus celles
// configurées (par exemple localhost:5173 pendant le développement).
func (s *Server) originPatterns(r *http.Request) []string {
	patterns := make([]string, 0, len(s.extraOrigins)+1)
	patterns = append(patterns, r.Host)

	return append(patterns, s.extraOrigins...)
}

// writeHandshakeError refuse la poignée de main en JSON, comme le reste de l'API.
func writeHandshakeError(w http.ResponseWriter, err error) {
	body, marshalErr := json.Marshal(protocol.ErrorPayload{
		Code:    protocol.CodeOf(err),
		Message: protocol.MessageOf(err),
	})
	if marshalErr != nil {
		http.Error(w, "erreur interne", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(protocol.StatusFor(err))
	_, _ = w.Write(body)
}

// writeSocketError transmet une erreur au client juste avant la fermeture.
func writeSocketError(ctx context.Context, socket *websocket.Conn, err error) {
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	_ = wsjson.Write(writeCtx, socket, protocol.ErrorFor(err))
}
