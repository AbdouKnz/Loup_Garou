import { describe, expect, it } from 'vitest';

import { createSoundEngine, cueForEvent, cueForPhase } from './sounds';
import type { GameEvent } from '../api/types';

/** fait assemble un fait de jeu, comme le narrateur l'enverrait. */
function fait(kind: GameEvent['kind']): GameEvent {
  return { kind, text: '', phase: 'nuit', day: 1, round: 1 };
}

describe('cueForEvent', () => {
  it('donne un son aux faits qui comptent', () => {
    expect(cueForEvent(fait('victime'))).toBe('victim');
    expect(cueForEvent(fait('sauve'))).toBe('saved');
    expect(cueForEvent(fait('elimine'))).toBe('eliminated');
    expect(cueForEvent(fait('egalite'))).toBe('tie');
    expect(cueForEvent(fait('fin'))).toBe('gameOver');
  });

  it('reste muet sur les nouvelles sans relief', () => {
    // Le rappel d'une phase est déjà sonorisé par le changement de phase.
    expect(cueForEvent(fait('phase'))).toBeNull();
    expect(cueForEvent(fait('aucune-victime'))).toBeNull();
    expect(cueForEvent(fait('aucune-elimination'))).toBeNull();
  });
});

describe('cueForPhase', () => {
  it('reste muet au premier instantané, même en pleine nuit', () => {
    expect(cueForPhase('nuit', null)).toBeNull();
  });

  it('reste muet quand la phase ne change pas', () => {
    expect(cueForPhase('nuit', 'nuit')).toBeNull();
  });

  it('annonce chaque étape du cycle', () => {
    expect(cueForPhase('roles', 'lobby')).toBe('roles');
    expect(cueForPhase('nuit', 'roles')).toBe('night');
    expect(cueForPhase('aube', 'nuit')).toBe('dawn');
    expect(cueForPhase('debat', 'aube')).toBe('debate');
    expect(cueForPhase('vote', 'debat')).toBe('vote');
    expect(cueForPhase('verdict', 'vote')).toBe('verdict');
  });

  it('laisse la fin au fait « fin », pas à la phase', () => {
    expect(cueForPhase('fin', 'verdict')).toBeNull();
    expect(cueForPhase('lobby', 'fin')).toBeNull();
  });
});

describe('createSoundEngine', () => {
  it('reste silencieux sans API audio, sans jamais échouer', () => {
    // L'environnement de test n'expose pas AudioContext : le moteur ne doit ni
    // lever ni bloquer le jeu.
    const engine = createSoundEngine();

    expect(() => {
      engine.unlock();
      engine.setMuted(false);
      engine.play('night');
      engine.setMuted(true);
      engine.play('gameOver');
    }).not.toThrow();
  });
});
