package room

import (
	"fmt"
	mathrand "math/rand"
	"time"

	"github.com/abdou/loupgarou/internal/game"
	"github.com/abdou/loupgarou/internal/protocol"
)

// Les bots sont des joueurs automatiques ajoutés par l'hôte pour tester le
// gameplay sans réunir une table. Ils vivent dans la salle (aucune connexion
// réseau) et ne consultent le moteur que par la vue de leur propre joueur :
// ils ne peuvent donc pas tricher, exactement comme un client.

// botNames alimente les pseudos des bots, dans l'ordre d'arrivée. Les noms sont
// assez nombreux pour couvrir une table pleine (11 bots au plus).
var botNames = []string{
	"Anatole", "Berthe", "Casimir", "Denise", "Eustache", "Fanny",
	"Gaspard", "Hortense", "Isidore", "Joséphine", "Léon", "Marcelle",
}

// botChatter est la réserve de répliques de débat : le bot ne fait que bavarder,
// il ne dit jamais rien qu'un joueur ne pourrait pas lire.
var botChatter = []string{
	"Je n'ai rien vu de la nuit, moi.",
	"Il y a quelque chose de louche chez quelqu'un ici…",
	"Moi je dis qu'il faut se méfier des silencieux.",
	"Je voterai pour le premier qui se tait trop longtemps.",
	"Le village doit rester uni.",
	"Quelqu'un a une idée, ou on vote au hasard ?",
}

// botChatChance est la probabilité (en pour cent) qu'un bot prenne la parole
// pendant un débat : quelques voix suffisent à animer la discussion.
const botChatChance = 45

// runBots fait jouer chaque bot qui a quelque chose à faire, puis rediffuse la
// table. C'est le cœur d'une vague : tout se passe dans la goroutine de l'acteur.
func (r *Room) runBots(now time.Time) {
	for _, id := range append([]PlayerID(nil), r.order...) {
		player := r.players[id]
		if player == nil || !player.IsBot {
			continue
		}

		r.botAct(player, now)
	}

	// Un bot qui vient de voter peut avoir tout bouclé : la phase se clôt alors
	// sans faire patienter la table.
	r.closeIfEveryoneVoted()
	r.broadcastState()
}

// botAct fait agir un bot pour la phase en cours, au plus une fois par phase.
func (r *Room) botAct(player *Player, now time.Time) {
	if r.game == nil || r.game.Over() || r.game.Dead(string(player.ID)) {
		return
	}

	// La vue du bot est calculée pour lui seul : le rôle des autres reste secret,
	// même à ses yeux.
	view := r.game.Snapshot(string(player.ID), now)

	// Une clé par « phase-jour-tour-action » : un bot ne joue pas deux fois la
	// même veillée, mais il reprend la parole au débat suivant, au second tour,
	// et à chaque nouvelle veillée de la nuit.
	key := fmt.Sprintf("%s-%d-%d-%s", view.Phase, view.Day, view.Round, view.YourAction)
	if player.botAction == key {
		return
	}

	switch view.Phase {
	case game.PhaseNight:
		r.botNight(player, view, key, now)

	case game.PhaseVote:
		if r.botVote(player, game.CommandVillageVote, view, nil, now) {
			player.botAction = key
		}

	case game.PhaseDebate:
		// Le débat est un tour de parole : seul le bot qui a la parole parle,
		// et il la rend aussitôt — un bot ne fait pas patienter la table.
		if view.Speaker != string(player.ID) {
			return
		}
		player.botAction = key
		if r.rng.Intn(100) < botChatChance {
			r.botSay(player, botChatter[r.rng.Intn(len(botChatter))])
		}
		_ = r.play(game.Command{Kind: game.CommandSpeak, Actor: string(player.ID)}, now)
	}
}

// botNight fait jouer un bot pendant la veillée en cours : les loups désignent
// leur proie, la voyante observe un joueur, la sorcière décide de ses potions.
func (r *Room) botNight(player *Player, view game.View, key string, now time.Time) {
	switch view.YourAction {
	case game.ActionDevour:
		if r.botVote(player, game.CommandWolfVote, view, view.Wolves, now) {
			player.botAction = key
		}

	case game.ActionSee:
		target := botChoice(r.rng, view, string(player.ID), nil)
		if target != "" && r.botNightAction(player, game.ActionSee, target, now) {
			player.botAction = key
		}

	case game.ActionPotions:
		r.botPotions(player, view, key, now)
	}
}

// botPotions décide à la place de la sorcière.
//
// Elle sauve la proie des loups quand sa potion de vie lui reste — sauver un
// villageois est presque toujours un bon coup —, sinon elle empoisonne un
// joueur au hasard. Sans rien d'utile à faire, elle passe : sa veillée se
// referme aussitôt, la table n'attend pas.
func (r *Room) botPotions(player *Player, view game.View, key string, now time.Time) {
	potions := view.YourPotions

	switch {
	case potions != nil && potions.Heal && view.WolfTarget != "":
		if r.botNightAction(player, game.ActionHeal, view.WolfTarget, now) {
			player.botAction = key
		}

	case potions != nil && potions.Poison:
		if target := botChoice(r.rng, view, string(player.ID), nil); target != "" {
			if r.botNightAction(player, game.ActionPoison, target, now) {
				player.botAction = key
			}
			return
		}
		fallthrough

	default:
		if r.botNightAction(player, game.ActionPass, "", now) {
			player.botAction = key
		}
	}
}

// botNightAction soumet l'action d'un bot au moteur. Renvoie vrai si le moteur
// l'a acceptée.
func (r *Room) botNightAction(player *Player, action game.ActionKind, target string, now time.Time) bool {
	return r.play(game.Command{
		Kind:   game.CommandNightAction,
		Actor:  string(player.ID),
		Action: action,
		Target: target,
	}, now) == nil
}

// botVote désigne une cible et la soumet au moteur. forbidden écarte des joueurs
// (les loups, la nuit). Renvoie vrai si le moteur a accepté le vote.
func (r *Room) botVote(player *Player, kind game.CommandKind, view game.View, forbidden []string, now time.Time) bool {
	target := botChoice(r.rng, view, string(player.ID), forbidden)

	return r.play(game.Command{
		Kind:   kind,
		Actor:  string(player.ID),
		Target: target,
	}, now) == nil
}

// botChoice choisit une cible vivante au hasard, ou "" pour s'abstenir.
//
// self est le bot lui-même (il ne se vise pas) ; forbidden écarte des
// identifiants. Quand un second tour est en cours, seuls les joueurs en
// ballotage sont visables.
func botChoice(rng *mathrand.Rand, view game.View, self string, forbidden []string) string {
	excluded := make(map[string]bool, len(forbidden)+1)
	excluded[self] = true
	for _, id := range forbidden {
		excluded[id] = true
	}

	allowed := make([]string, 0, len(view.Alive))
	for _, id := range view.Alive {
		if excluded[id] {
			continue
		}
		if len(view.Candidates) > 0 && !listContains(view.Candidates, id) {
			continue
		}
		allowed = append(allowed, id)
	}

	if len(allowed) == 0 {
		return ""
	}

	return allowed[rng.Intn(len(allowed))]
}

// botSay publie une réplique de bot : même chemin que le chat, sans limite de
// débit (un bot n'abuse pas).
func (r *Room) botSay(player *Player, text string) {
	message := protocol.ChatMessage{
		ID:       newID("M"),
		PlayerID: string(player.ID),
		Pseudo:   player.Pseudo,
		Text:     text,
		At:       time.Now().UTC(),
	}

	r.appendMessage(message)
	r.broadcastChat(message)
}

// botsArrivedText annonce les bots qui viennent d'entrer.
func botsArrivedText(names []string) string {
	if len(names) == 1 {
		return names[0] + " (bot) a rejoint la partie."
	}

	return fmt.Sprintf("%d bots ont rejoint la partie.", len(names))
}

// botsLeftText annonce les bots qui viennent d'être retirés.
func botsLeftText(names []string) string {
	if len(names) == 1 {
		return names[0] + " (bot) a été retiré."
	}

	return fmt.Sprintf("%d bots ont été retirés.", len(names))
}

// listContains indique si liste contient value.
func listContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}

	return false
}
