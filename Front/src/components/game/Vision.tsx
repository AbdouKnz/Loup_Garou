import { ROLE_LABELS } from './labels';
import type { VisionView } from '../../api/types';

type Props = {
  vision?: VisionView[];
  nameOf: (id: string) => string;
};

/**
 * Vision est le carnet de la voyante : ce qu'elle a observé, nuit après nuit.
 *
 * Il n'apparaît que pour elle : le serveur ne le renseigne que dans sa vue.
 */
export default function Vision({ vision, nameOf }: Props) {
  const seen = vision ?? [];

  return (
    <section className="panel vision">
      <h2>
        🔮 Votre carnet <span className="count">(nuit par nuit)</span>
      </h2>

      {seen.length === 0 ? (
        <p className="hint">Vous n’avez encore observé personne.</p>
      ) : (
        <ul className="vision-list">
          {seen.map((entry) => (
            <li key={`${entry.day}-${entry.target}`}>
              <span className="vision-day">Nuit {entry.day}</span>
              <span className="player-name">{nameOf(entry.target)}</span>
              <span className={entry.role === 'loup-garou' ? 'role-name wolf' : 'role-name'}>
                {ROLE_LABELS[entry.role]}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
