package room

import (
	"time"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/protocol"
)

// stateFor construit la trame « room.state » telle que le joueur playerID la voit.
//
// C'est le seul point où l'état d'une salle devient public. La partie, en
// particulier, n'y entre que par la vue calculée par le moteur pour ce joueur
// précis : aucun autre chemin ne diffuse l'état, et rien d'autre ne sort d'ici.
func (r *Room) stateFor(playerID PlayerID, now time.Time) protocol.Envelope {
	return protocol.MustEncode(protocol.TypeRoomState, r.snapshotFor(playerID, now))
}

// snapshotFor assemble la vue d'un joueur sur la salle.
func (r *Room) snapshotFor(playerID PlayerID, now time.Time) protocol.RoomState {
	var party *protocol.GameView
	if r.game != nil {
		view := r.game.Snapshot(string(playerID), now)
		party = publicView(view)
	}

	dead, voted := marks(party)

	// La composition ne dépend que de la taille de la table : la salle peut
	// l'annoncer avant même de distribuer les rôles.
	composition := make([]string, 0, len(r.order))
	for _, role := range r.opts.Game.CompositionFor(len(r.order)) {
		composition = append(composition, string(role))
	}

	players := make([]protocol.PlayerView, 0, len(r.order))

	for _, id := range r.order {
		player := r.players[id]
		_, online := r.clients[id]

		players = append(players, protocol.PlayerView{
			ID:     string(player.ID),
			Pseudo: player.Pseudo,
			Ready:  player.Ready,
			Online: player.IsBot || online,
			Host:   player.IsHost,
			Dead:   dead[string(id)],
			Voted:  voted[string(id)],
			Bot:    player.IsBot,
		})
	}

	return protocol.RoomState{
		Code:        r.code,
		Phase:       r.phase(),
		HostID:      string(r.hostID),
		You:         string(playerID),
		MaxPlayers:  r.opts.MaxPlayers,
		CanStart:    r.canStart(playerID),
		Players:     players,
		Messages:    r.messagesView(),
		Composition: composition,
		Game:        party,
	}
}

// canStart dit si ce joueur peut lancer la partie : il faut être l'hôte, avoir
// assez de monde, et qu'aucune partie ne soit déjà en cours.
func (r *Room) canStart(playerID PlayerID) bool {
	return r.game == nil && playerID == r.hostID && len(r.order) >= r.opts.Game.MinPlayers
}

// publicView traduit la vue du moteur en vue du fil. La traduction est
// volontairement littérale : le moteur décide, la salle recopie.
func publicView(view game.View) *protocol.GameView {
	out := &protocol.GameView{
		Phase:          protocol.Phase(view.Phase),
		Day:            view.Day,
		Round:          view.Round,
		Deadline:       view.Deadline,
		SecondsLeft:    view.SecondsLeft,
		Winner:         string(view.Winner),
		YourRole:       string(view.YourRole),
		YourVote:       view.YourVote,
		YourAction:     string(view.YourAction),
		WolfTarget:     view.WolfTarget,
		Alive:          view.Alive,
		Dead:           view.Dead,
		Voted:          view.Voted,
		Wolves:         view.Wolves,
		Eliminated:     view.Eliminated,
		EliminatedRole: string(view.EliminatedRole),
		Tie:            view.Tie,
		Candidates:     view.Candidates,
		Speaker:        view.Speaker,
		Speakers:       view.Speakers,
		Spoken:         view.Spoken,
	}

	if len(view.YourVision) > 0 {
		out.YourVision = make([]protocol.VisionView, 0, len(view.YourVision))
		for _, observation := range view.YourVision {
			out.YourVision = append(out.YourVision, protocol.VisionView{
				Day:    observation.Day,
				Target: observation.Target,
				Role:   string(observation.Role),
			})
		}
	}

	if view.YourPotions != nil {
		out.YourPotions = &protocol.PotionsView{
			Heal:   view.YourPotions.Heal,
			Poison: view.YourPotions.Poison,
		}
	}

	if len(view.Deaths) > 0 {
		out.Deaths = make([]protocol.DeathView, 0, len(view.Deaths))
		for _, death := range view.Deaths {
			out.Deaths = append(out.Deaths, protocol.DeathView{
				Target: death.Target,
				Cause:  string(death.Cause),
			})
		}
	}

	if len(view.WolfVotes) > 0 {
		out.WolfVotes = make(map[string]string, len(view.WolfVotes))
		for wolf, target := range view.WolfVotes {
			out.WolfVotes[wolf] = target
		}
	}

	// Le scrutin du jour est public : on le recopie tel quel, hors de la carte
	// interne du moteur.
	if len(view.Ballot) > 0 {
		out.Ballot = make(map[string]string, len(view.Ballot))
		for voter, target := range view.Ballot {
			out.Ballot[voter] = target
		}
	}

	if len(view.Roles) > 0 {
		out.Roles = make(map[string]string, len(view.Roles))
		for id, role := range view.Roles {
			out.Roles[id] = string(role)
		}
	}

	return out
}

// marks indexe les joueurs morts et les votants du tour, pour les recopier dans
// chaque vue de joueur.
func marks(view *protocol.GameView) (dead, voted map[string]bool) {
	dead = make(map[string]bool, 0)
	voted = make(map[string]bool, 0)

	if view == nil {
		return dead, voted
	}

	for _, id := range view.Dead {
		dead[id] = true
	}
	for _, id := range view.Voted {
		voted[id] = true
	}

	return dead, voted
}

// messagesView renvoie les derniers messages, du plus ancien au plus récent.
func (r *Room) messagesView() []protocol.ChatMessage {
	if len(r.messages) <= protocol.MessagesInState {
		out := make([]protocol.ChatMessage, len(r.messages))
		copy(out, r.messages)
		return out
	}

	start := len(r.messages) - protocol.MessagesInState
	out := make([]protocol.ChatMessage, protocol.MessagesInState)
	copy(out, r.messages[start:])

	return out
}
