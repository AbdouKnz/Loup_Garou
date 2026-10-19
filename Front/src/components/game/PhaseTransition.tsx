import { useEffect, useState } from 'react';

import { PHASES } from './labels';
import type { Phase } from '../../api/types';

/**
 * Durée du voile. Elle est aussi écrite dans la feuille de style : les deux
 * doivent rester d'accord, sinon le voile disparaît en plein mouvement — ou
 * reste posé sur l'écran.
 */
export const PHASE_TRANSITION_MS = 1600;

/** Le voile en cours : la bascule qui l'a déclenché, et de quelle couleur. */
type Passage = { id: string; phase: 'nuit' | 'aube' };

/**
 * PhaseTransition voile l'écran au moment où la nuit tombe ou où le jour se
 * lève.
 *
 * Le voile est **purement décoratif** : il ne bloque aucun clic, et il ne dit
 * rien que le titre de la phase et le récit ne disent déjà — il porte d'ailleurs
 * `aria-hidden`, pour ne pas doubler l'annonce. Il ne se déclenche que sur une
 * vraie bascule jour/nuit : passer d'une veillée à la suivante, au cœur de la
 * nuit, ne rejoue rien.
 */
export default function PhaseTransition({ phase, day }: { phase: Phase; day: number }) {
  // La clé ne bouge pas d'une veillée à l'autre : seule la phase compte, pas
  // l'échéance, qui change pourtant plusieurs fois par nuit.
  const key = `${phase}:${day}`;

  const [previous, setPrevious] = useState(key);
  const [passage, setPassage] = useState<Passage | null>(null);

  // Ajustement pendant le rendu : on retient la dernière bascule, sans effet
  // ni rendu en cascade. Le premier instantané ne déclenche rien — on arrive
  // sur une partie déjà en cours.
  if (previous !== key) {
    setPrevious(key);
    if (phase === 'nuit' || phase === 'aube') {
      setPassage({ id: key, phase });
    }
  }

  useEffect(() => {
    if (passage === null) {
      return;
    }

    const timer = window.setTimeout(() => setPassage(null), PHASE_TRANSITION_MS);

    return () => window.clearTimeout(timer);
  }, [passage]);

  if (passage === null) {
    return null;
  }

  const label = PHASES[passage.phase];

  return (
    <div
      key={passage.id}
      className={passage.phase === 'nuit' ? 'phase-transition night' : 'phase-transition dawn'}
      aria-hidden="true"
    >
      <span className="phase-transition-veil" />
      <p className="phase-transition-title">
        <span className="phase-transition-emoji">{label.emoji}</span>
        <span>{label.title}</span>
      </p>
    </div>
  );
}
