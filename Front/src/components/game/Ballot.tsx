import type { CSSProperties } from 'react';

import Avatar from './Avatar';
import type { PlayerView } from '../../api/types';

type Props = {
  /** Scrutin public : votant → cible (vide pour une abstention). */
  ballot: Record<string, string>;
  players: PlayerView[];
  you: string;
  /** clos : le vote est terminé (on affiche le scrutin pendant le verdict). */
  closed?: boolean;
};

/**
 * Ballot affiche le scrutin du village, en direct pendant le vote puis figé le
 * temps du verdict.
 *
 * Le scrutin du jour est public : on montre qui a voté pour qui, et le total
 * par cible. La nuit, en revanche, rien de tel n'est diffusé : ce panneau ne
 * peut donc jamais trahir les loups.
 */
export default function Ballot({ ballot, players, you, closed }: Props) {
  const indexOf = new Map(players.map((player, index) => [player.id, index]));
  const nameOf = (id: string) => players.find((player) => player.id === id)?.pseudo ?? '?';

  const counts = new Map<string, string[]>();
  const abstentions: string[] = [];
  for (const [voter, target] of Object.entries(ballot)) {
    if (target === '') {
      abstentions.push(voter);
      continue;
    }

    counts.set(target, [...(counts.get(target) ?? []), voter]);
  }

  const rows = [...counts.entries()].sort((a, b) => b[1].length - a[1].length);
  const best = rows[0]?.[1].length ?? 1;
  const total = Object.keys(ballot).length;

  return (
    <section className="panel ballot-panel">
      <h2>
        Le scrutin <span className="count">{closed ? '(clos)' : '(en direct)'}</span>
      </h2>

      {rows.length === 0 ? (
        <p className="hint">Aucune voix exprimée pour l’instant.</p>
      ) : (
        <ul className="ballot">
          {rows.map(([target, voters]) => (
            <li key={target} className={voters.length === best ? 'ballot-row lead' : 'ballot-row'}>
              <div className="ballot-head">
                <span className="ballot-name">
                  <Avatar index={indexOf.get(target) ?? 0} />
                  {nameOf(target)}
                </span>
                <span className="ballot-count">
                  {voters.length} voix
                  {voters.includes(you) && <em> · vous</em>}
                </span>
              </div>

              <div
                className="ballot-bar"
                style={
                  { '--share': `${Math.round((voters.length / best) * 100)}%` } as CSSProperties
                }
              />

              <p className="ballot-voters">{voters.map((voter) => nameOf(voter)).join(', ')}</p>
            </li>
          ))}
        </ul>
      )}

      {abstentions.length > 0 && (
        <p className="ballot-blank">
          Abstentions : {abstentions.map((voter) => nameOf(voter)).join(', ')}
        </p>
      )}

      <p className="hint">
        {total} bulletin{total > 1 ? 's' : ''} déposé{total > 1 ? 's' : ''}.
      </p>
    </section>
  );
}
