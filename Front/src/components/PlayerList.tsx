import type { PlayerView } from '../api/types';

type Props = {
  players: PlayerView[];
  /** Identifiant du joueur devant son propre écran. */
  you: string;
  /** Retire un bot (hôte seulement) ; absent, aucun bouton n'est affiché. */
  onRemoveBot?: (id: string) => void;
};

/** PlayerList affiche les joueurs de la partie et leur état. */
export default function PlayerList({ players, you, onRemoveBot }: Props) {
  return (
    <ul className="players">
      {players.map((player) => (
        <li key={player.id} className={player.online ? 'player' : 'player away'}>
          <span className="player-name">
            {player.pseudo}
            {player.id === you && <span className="tag-you">vous</span>}
          </span>

          <span className="player-badges">
            {player.host && <span className="badge host">hôte</span>}
            {player.bot && <span className="badge bot">bot</span>}
            {player.ready && <span className="badge ready">prêt</span>}
            {!player.online && <span className="badge away">hors ligne</span>}

            {player.bot && onRemoveBot && (
              <button type="button" className="link" onClick={() => onRemoveBot(player.id)}>
                retirer
              </button>
            )}
          </span>
        </li>
      ))}
    </ul>
  );
}
