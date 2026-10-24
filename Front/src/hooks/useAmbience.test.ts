import { describe, expect, it } from 'vitest';

import { moodForPhase } from './useAmbience';
import type { Phase } from '../api/types';

/**
 * Chaque phase a sa couleur sonore. Le mode reste une ambiance : il ne dit rien
 * de plus que la phase déjà affichée à l'écran.
 */

describe('moodForPhase', () => {
  it('choisit le mystère, la nuit, le jour ou la tension', () => {
    const expected: Array<[Phase, string]> = [
      ['roles', 'mystery'],
      ['nuit', 'night'],
      ['aube', 'day'],
      ['debat', 'day'],
      ['vote', 'tension'],
      ['verdict', 'tension'],
      ['fin', 'day'],
      ['lobby', 'day'],
    ];

    for (const [phase, mood] of expected) {
      expect(moodForPhase(phase)).toBe(mood);
    }
  });
});
