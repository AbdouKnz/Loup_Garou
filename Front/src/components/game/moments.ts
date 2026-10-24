import type { Phase } from '../../api/types';

/** Le village vit au rythme de deux moments : le jour et la nuit. */
export type Moment = 'day' | 'night';

/**
 * isNightPhase dit si la phase appartient à la nuit du village.
 *
 * La distribution des rôles se déroule déjà dans la pénombre : la partie
 * commence quand la nuit tombe, pas après elle.
 */
export function isNightPhase(phase: Phase): boolean {
  return phase === 'roles' || phase === 'nuit';
}

/**
 * momentForPhase donne le moment affiché dans l'en-tête, ou null quand la
 * phase n'en porte pas — la salle d'attente et la distribution.
 */
export function momentForPhase(phase: Phase): Moment | null {
  if (phase === 'lobby' || phase === 'roles') {
    return null;
  }

  return isNightPhase(phase) ? 'night' : 'day';
}
