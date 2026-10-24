import { isNightPhase } from './moments';
import type { Phase } from '../../api/types';

/**
 * VillageBackdrop pose le village derrière l'écran de jeu.
 *
 * Les deux images restent montées et se croisent en fondu : le passage du jour
 * à la nuit n'a pas de trou, et les deux fichiers sont déjà en mémoire quand la
 * phase bascule. Le fond est purement décoratif — il ne porte aucune information
 * que la phase et la table ne disent déjà — et reste hors de portée des clics.
 */
export default function VillageBackdrop({ phase }: { phase: Phase }) {
  const night = isNightPhase(phase);

  return (
    <div
      className="village-backdrop"
      data-moment={night ? 'night' : 'day'}
      data-phase={phase}
      aria-hidden="true"
    >
      <img
        className="village-layer day"
        src="/Jour.webp"
        alt=""
        decoding="async"
        draggable={false}
      />
      <img
        className="village-layer night"
        src="/Nuit.webp"
        alt=""
        decoding="async"
        draggable={false}
      />
      <span className="village-scrim" />
    </div>
  );
}
