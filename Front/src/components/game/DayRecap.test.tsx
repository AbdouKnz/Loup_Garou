// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';

import DayRecap from './DayRecap';
import type { GameEvent, GameView } from '../../api/types';

/**
 * Le rappel du jour ne s'appuie que sur des faits publics : les nouvelles de la
 * nuit déjà racontées et le scrutin en cours. Il ne devine aucun rôle.
 */

function game(overrides: Partial<GameView> = {}): GameView {
  return {
    phase: 'vote',
    day: 1,
    round: 1,
    secondsLeft: 30,
    alive: ['p1', 'p2', 'p3'],
    dead: ['p4'],
    ...overrides,
  };
}

const history: GameEvent[] = [
  {
    kind: 'victime',
    text: '🌙 Bob a été dévoré pendant la nuit.',
    phase: 'aube',
    day: 1,
    round: 1,
    target: 'p2',
  },
];

const names: Record<string, string> = { p1: 'Alice', p2: 'Bob', p3: 'Chloé' };

function nameOf(id: string): string {
  return names[id] ?? id;
}

describe('DayRecap', () => {
  afterEach(cleanup);

  it('rappelle les faits de la nuit et l’état du scrutin', () => {
    render(
      <DayRecap
        game={game({ ballot: { p1: 'p3', p2: 'p3' } })}
        history={history}
        nameOf={nameOf}
      />,
    );

    expect(screen.getByText(/Bob a été dévoré pendant la nuit/)).toBeTruthy();
    expect(screen.getByText(/3 vivants/)).toBeTruthy();
    expect(screen.getByText(/2 votants/)).toBeTruthy();
    expect(screen.getByText(/2 voix pour Chloé/)).toBeTruthy();
    expect(screen.queryByText('Loup-Garou')).toBeNull();
  });

  it('reste honnête quand le fil local est vide', () => {
    render(<DayRecap game={game({ ballot: {} })} history={[]} nameOf={nameOf} />);

    expect(screen.getByText(/Le fil de la nuit n’est plus affiché/)).toBeTruthy();
    expect(screen.getByText(/aucun vote pour le moment/)).toBeTruthy();
  });

  it('ne s’affiche pas en dehors du vote', () => {
    const { container } = render(
      <DayRecap game={game({ phase: 'debat' })} history={history} nameOf={nameOf} />,
    );

    expect(container.firstChild).toBeNull();
  });
});
