import { describe, expect, it } from 'vitest';

import { dawnNarration, toSpeech } from './narration';
import type { GameEvent } from '../../api/types';

const nameOf = (id: string) => ({ p1: 'Alice', p2: 'Bob' })[id] ?? '?';

/** event assemble un fait du narrateur, comme le serveur l'enverrait. */
function event(overrides: Partial<GameEvent>): GameEvent {
  return { kind: 'phase', text: '', phase: 'nuit', day: 1, round: 1, ...overrides };
}

describe('toSpeech', () => {
  it('retire les emoji et normalise les espaces', () => {
    expect(toSpeech('🐺  Bob a été dévoré.  🌅')).toBe('Bob a été dévoré.');
  });
});

describe('dawnNarration', () => {
  it('reprend le récit de la nuit et l’ouverture de l’aube', () => {
    const history = [
      event({
        kind: 'victime',
        text: '🐺 Bob a été dévoré pendant la nuit.',
        target: 'p2',
        cause: 'loups',
      }),
      event({ kind: 'phase', text: '🌅 Le jour se lève sur le village.', phase: 'aube' }),
    ];

    const narration = dawnNarration(history, 1, [{ target: 'p2', cause: 'loups' }], nameOf);

    expect(narration.title).toBe('Nuit 1');
    expect(narration.lines).toEqual([
      '🌅 Le jour se lève sur le village.',
      '🐺 Bob a été dévoré pendant la nuit.',
    ]);
    expect(narration.spoken).toBe(
      'Le jour se lève sur le village. Bob a été dévoré pendant la nuit.',
    );
  });

  it('raconte une nuit sauvée sans nommer la proie', () => {
    const history = [
      event({ kind: 'sauve', text: '🌿 Les loups ont frappé, mais la victime a été sauvée.' }),
      event({ kind: 'phase', text: '🌅 Le jour se lève sur le village.', phase: 'aube' }),
    ];

    const narration = dawnNarration(history, 1, [], nameOf);

    expect(narration.lines).toContain('🌿 Les loups ont frappé, mais la victime a été sauvée.');
  });

  it('ne garde que la nuit du jour courant', () => {
    const history = [
      event({ kind: 'victime', text: 'Ancienne nuit', day: 1 }),
      event({ kind: 'phase', text: '🌅 Le jour se lève.', phase: 'aube', day: 2 }),
    ];

    const narration = dawnNarration(history, 2, [], nameOf);

    expect(narration.lines).toEqual(['🌅 Le jour se lève.']);
  });

  it('retombe sur les morts de la vue si le fil manque', () => {
    const narration = dawnNarration([], 1, [{ target: 'p2', cause: 'poison' }], nameOf);

    expect(narration.lines).toContain('Bob est mort dans d’étranges circonstances.');
  });
});
