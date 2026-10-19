import Avatar from './Avatar';
import { ROLE_LABELS } from './labels';
import type { GameEvent, PlayerView, Role, Winner } from '../../api/types';

type Props = {
  winner?: Winner;
  roles?: Record<string, Role>;
  players: PlayerView[];
  /** Faits de jeu accumulés, pour reconstituer le fil de la partie. */
  history: GameEvent[];
  isHost: boolean;
  online: boolean;
  onReplay: () => void;
};

/** Scoreboard est l'écran de fin : vainqueur, rôles révélés et fil de la partie. */
export default function Scoreboard({
  winner,
  roles,
  players,
  history,
  isHost,
  online,
  onReplay,
}: Props) {
  // Le bilan se limite aux morts : c'est ce que le narrateur a raconté.
  const recap = history.filter((event) => event.kind === 'victime' || event.kind === 'elimine');

  return (
    <section className="panel endgame">
      <h2>{winner === 'village' ? '🏆 Le village l’emporte !' : '🐺 Les loups ont gagné'}</h2>

      <p className="hint">
        {winner === 'village'
          ? 'Tous les loups ont été démasqués.'
          : 'Les loups sont désormais aussi nombreux que les villageois.'}
      </p>

      <ul className="roles">
        {players.map((player, index) => (
          <li key={player.id} className={roles?.[player.id] === 'loup-garou' ? 'wolf' : ''}>
            <span className="player-name">
              <Avatar index={index} dead={player.dead} />
              {player.pseudo}
              {player.dead && <span className="badge gone">mort</span>}
            </span>

            <span className="badge">
              {roles?.[player.id] ? ROLE_LABELS[roles[player.id]] : '—'}
            </span>
          </li>
        ))}
      </ul>

      {recap.length > 0 && (
        <>
          <h3 className="recap-title">Le fil de la partie</h3>
          <ol className="recap">
            {recap.map((event, index) => (
              <li key={`${event.kind}-${event.day}-${index}`}>
                <span className="recap-tag">
                  {event.phase === 'nuit' ? `Nuit ${event.day}` : `Jour ${event.day}`}
                </span>
                <span>{event.text}</span>
              </li>
            ))}
          </ol>
        </>
      )}

      {isHost ? (
        <div className="actions">
          <button type="button" className="btn primary" disabled={!online} onClick={onReplay}>
            Rejouer
          </button>
        </div>
      ) : (
        <p className="hint">L’hôte peut relancer une partie.</p>
      )}
    </section>
  );
}
