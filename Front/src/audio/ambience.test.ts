import { describe, expect, it } from 'vitest';

import { createAmbienceEngine } from './ambience';

/**
 * Sans API Web Audio — le cas des navigateurs anciens, et de Node — le moteur
 * reste silencieux. Aucune méthode ne doit lever : le jeu continue sans fond.
 */

describe('createAmbienceEngine', () => {
  it('reste silencieux et sans erreur quand Web Audio manque', () => {
    const engine = createAmbienceEngine();

    expect(() => {
      engine.unlock();
      engine.setVolume(0.5);
      engine.setMuted(true);
      engine.setMood('night');
      engine.start();
      engine.pause();
      engine.resume();
      engine.stop();
    }).not.toThrow();
  });
});
