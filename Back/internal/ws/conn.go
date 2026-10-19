package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/abdou/loupgarou/internal/protocol"
	"github.com/abdou/loupgarou/internal/room"
)

const (
	// writeTimeout borne l'écriture d'une trame vers le client.
	writeTimeout = 10 * time.Second
	// pingInterval est la fréquence des pings de maintien de connexion.
	pingInterval = 30 * time.Second
	// outboundBuffer est le nombre de trames en attente avant qu'un client soit
	// considéré comme trop lent (il est alors déconnecté).
	outboundBuffer = 32
)

// outbound est une trame destinée au joueur, éventuellement la dernière de sa
// session : la trame et la fermeture voyagent dans la même file, ce qui garantit
// l'ordre (le client apprend toujours pourquoi on le déconnecte).
type outbound struct {
	env   protocol.Envelope
	final bool
}

// conn est la connexion WebSocket d'un joueur.
//
// Une goroutine lit, une autre écrit : l'acteur de la salle ne fait que déposer
// des trames dans un tampon, il ne touche jamais directement au socket.
type conn struct {
	socket *websocket.Conn
	room   *room.Room
	remote string

	// player est renseigné après l'authentification ; il n'est relu que par la
	// goroutine de lecture.
	player room.PlayerID

	out  chan outbound
	quit chan struct{}
	once sync.Once
}

// newConn prépare la connexion d'un joueur, avant son authentification.
func newConn(socket *websocket.Conn, salle *room.Room, remote string) *conn {
	return &conn{
		socket: socket,
		room:   salle,
		remote: remote,
		out:    make(chan outbound, outboundBuffer),
		quit:   make(chan struct{}),
	}
}

// Send dépose une trame à destination du joueur (implémente room.Client).
//
// Cette méthode est appelée par l'acteur de la salle : elle ne doit jamais
// bloquer, sinon toute la partie se figerait derrière un client trop lent.
func (c *conn) Send(env protocol.Envelope) {
	c.enqueue(outbound{env: env})
}

// Kick dépose une dernière trame puis ferme la connexion (implémente room.Client).
func (c *conn) Kick(env protocol.Envelope) {
	c.enqueue(outbound{env: env, final: true})
}

// enqueue met une trame en file d'attente ; un client qui n'avale rien est
// déconnecté plutôt que de ralentir la partie.
func (c *conn) enqueue(message outbound) {
	select {
	case c.out <- message:
	case <-c.quit:
	default:
		log.Printf("client %s trop lent : fermeture de la connexion", c.remote)
		c.close(websocket.StatusPolicyViolation, "trop de trames en attente")
	}
}

// close ferme la connexion au plus une fois, depuis n'importe quelle goroutine.
func (c *conn) close(status websocket.StatusCode, reason string) {
	c.once.Do(func() {
		close(c.quit)
		_ = c.socket.Close(status, reason)
	})
}

// readPump lit les trames du client, les route vers la salle, et se termine
// quand la connexion se ferme.
func (c *conn) readPump(ctx context.Context) {
	defer c.close(websocket.StatusNormalClosure, "fin de session")

	for {
		var env protocol.Envelope
		if err := wsjson.Read(ctx, c.socket, &env); err != nil {
			return
		}

		if err := c.dispatch(env); err != nil {
			c.Send(protocol.ErrorFor(err))
		}
	}
}

// writePump envoie les trames en attente et maintient la connexion en vie.
func (c *conn) writePump(ctx context.Context) {
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			c.close(websocket.StatusGoingAway, "serveur arrêté")
			return

		case <-c.quit:
			return

		case message := <-c.out:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := wsjson.Write(writeCtx, c.socket, message.env)
			cancel()

			if err != nil {
				c.close(websocket.StatusInternalError, "écriture impossible")
				return
			}

			// Dernière trame écrite : la place a été reprise ailleurs.
			if message.final {
				c.close(websocket.StatusPolicyViolation, "place reprise")
				return
			}

		case <-ping.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.socket.Ping(pingCtx)
			cancel()

			if err != nil {
				c.close(websocket.StatusPolicyViolation, "client injoignable")
				return
			}
		}
	}
}

// dispatch applique une trame reçue du client.
func (c *conn) dispatch(env protocol.Envelope) error {
	if !protocol.IsClientType(env.Type) {
		return protocol.NewError(protocol.CodeInvalidMessage, "Trame inconnue : « %s ».", env.Type)
	}

	switch env.Type {
	case protocol.TypeAuth:
		return protocol.NewError(protocol.CodeInvalidMessage, "Cette session est déjà authentifiée.")

	case protocol.TypeChatSend:
		var payload protocol.ChatSendPayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.Chat(c.player, payload.Text)

	case protocol.TypePlayerReady:
		var payload protocol.PlayerReadyPayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.SetReady(c.player, payload.Ready)

	case protocol.TypeGameStart:
		return c.room.StartGame(c.player)

	case protocol.TypeGameAgain:
		return c.room.Again(c.player)

	case protocol.TypeGameVote:
		var payload protocol.VotePayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.Vote(c.player, payload.Target)

	case protocol.TypeGameNightVote:
		var payload protocol.VotePayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.NightVote(c.player, payload.Target)

	case protocol.TypeGameNightAction:
		var payload protocol.NightActionPayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.NightAction(c.player, payload.Action, payload.Target)

	case protocol.TypeGameSpeakDone:
		return c.room.SpeakDone(c.player)

	case protocol.TypeBotAdd:
		var payload protocol.BotAddPayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.AddBots(c.player, payload.Count)

	case protocol.TypeBotRemove:
		var payload protocol.BotRemovePayload
		if err := env.DecodeData(&payload); err != nil {
			return err
		}
		return c.room.RemoveBots(c.player, payload.Target)

	case protocol.TypePing:
		c.Send(protocol.MustEncode(protocol.TypePong, nil))
		return nil

	default:
		return protocol.NewError(protocol.CodeInvalidMessage, "Trame non prise en charge : « %s ».", env.Type)
	}
}
