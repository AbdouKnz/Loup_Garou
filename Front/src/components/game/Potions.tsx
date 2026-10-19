import type { PotionsView } from '../../api/types';

type Props = {
  /** Potions restantes ; la veillée n'ouvre que s'il en reste au moins une. */
  potions?: PotionsView;
  /** Proie désignée par les loups, vide s'ils n'ont désigné personne. */
  wolfTarget?: string;
  /** Vrai quand la sorcière choisit sa victime sur la table. */
  picking: boolean;
  nameOf: (id: string) => string;
  onHeal: () => void;
  onPickPoison: () => void;
  onCancelPoison: () => void;
  onPass: () => void;
};

/**
 * Potions est la veillée de la sorcière : elle connaît la proie des loups, et
 * décide de l'une de ses deux potions — ou de rien.
 *
 * Chaque potion ne sert qu'une fois, et une seule par nuit : c'est le serveur
 * qui en décide, cette vue ne fait que proposer.
 */
export default function Potions({
  potions,
  wolfTarget,
  picking,
  nameOf,
  onHeal,
  onPickPoison,
  onCancelPoison,
  onPass,
}: Props) {
  const heal = potions?.heal === true;
  const poison = potions?.poison === true;

  return (
    <section className="panel potions">
      <h2>
        🧪 Votre veillée <span className="count">(une seule potion par nuit)</span>
      </h2>

      <p className="hint">
        {wolfTarget
          ? `Les loups ont désigné ${nameOf(wolfTarget)}.`
          : 'Les loups n’ont désigné personne cette nuit.'}
      </p>

      <ul className="potion-list">
        <li className={heal ? 'potion' : 'potion used'}>
          🌿 Potion de vie {heal ? '— disponible' : '— bue'}
        </li>
        <li className={poison ? 'potion' : 'potion used'}>
          ☠️ Potion de mort {poison ? '— disponible' : '— bue'}
        </li>
      </ul>

      {picking ? (
        <>
          <p className="hint" role="status">
            Désignez votre victime sur la table.
          </p>
          <div className="actions">
            <button type="button" className="btn" onClick={onCancelPoison}>
              Renoncer
            </button>
          </div>
        </>
      ) : (
        <div className="actions">
          {heal && wolfTarget && (
            <button type="button" className="btn primary" onClick={onHeal}>
              🌿 Soigner {nameOf(wolfTarget)}
            </button>
          )}

          {poison && (
            <button type="button" className="btn" onClick={onPickPoison}>
              ☠️ Empoisonner…
            </button>
          )}

          <button type="button" className="btn" onClick={onPass}>
            Ne rien faire
          </button>
        </div>
      )}
    </section>
  );
}
