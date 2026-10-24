// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';

import VillageBackdrop from './VillageBackdrop';
import { isNightPhase } from './moments';
import type { Phase } from '../../api/types';

/**
 * Le fond ne raconte rien : il suit la phase, et rien d'autre. Ces tests
 * vérifient la seule chose qui compte — la bonne image au bon moment — et que
 * les deux couches restent montées pour que le fondu soit complet.
 */

describe('isNightPhase', () => {
  it('plonge la distribution et la veillée dans la nuit', () => {
    expect(isNightPhase('roles')).toBe(true);
    expect(isNightPhase('nuit')).toBe(true);
  });

  it('rend le jour à toutes les autres phases', () => {
    const day: Phase[] = ['lobby', 'aube', 'debat', 'vote', 'verdict', 'fin'];

    for (const phase of day) {
      expect(isNightPhase(phase)).toBe(false);
    }
  });
});

describe('VillageBackdrop', () => {
  afterEach(cleanup);

  it('garde les deux images montées, prêtes pour le fondu', () => {
    const { container } = render(<VillageBackdrop phase="nuit" />);

    const day = container.querySelector<HTMLImageElement>('.village-layer.day');
    const night = container.querySelector<HTMLImageElement>('.village-layer.night');

    expect(day?.getAttribute('src')).toBe('/Jour.webp');
    expect(night?.getAttribute('src')).toBe('/Nuit.webp');
    expect(day?.getAttribute('alt')).toBe('');
    expect(night?.getAttribute('alt')).toBe('');
  });

  it('choisit la nuit pour la distribution et la veillée', () => {
    const { container, rerender } = render(<VillageBackdrop phase="roles" />);

    expect(container.querySelector('.village-backdrop')?.getAttribute('data-moment')).toBe('night');

    rerender(<VillageBackdrop phase="nuit" />);
    expect(container.querySelector('.village-backdrop')?.getAttribute('data-moment')).toBe('night');
  });

  it('choisit le jour pour l’aube, le débat, le vote et le verdict', () => {
    const { container, rerender } = render(<VillageBackdrop phase="aube" />);

    for (const phase of ['aube', 'debat', 'vote', 'verdict', 'fin'] as Phase[]) {
      rerender(<VillageBackdrop phase={phase} />);
      expect(container.querySelector('.village-backdrop')?.getAttribute('data-moment')).toBe('day');
    }
  });

  it('reste invisible pour les lecteurs d’écran', () => {
    const { container } = render(<VillageBackdrop phase="debat" />);

    expect(container.querySelector('.village-backdrop')?.getAttribute('aria-hidden')).toBe('true');
  });
});
