// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import NotesPanel from './NotesPanel';
import type { PlayerView } from '../../api/types';

/**
 * Le carnet est un aide-mémoire privé : l'écran montre les marques reçues et
 * demande au parent de passer à la suivante. Il ne décide de rien lui-même.
 */

const players: PlayerView[] = [
  { id: 'p1', pseudo: 'Alice', ready: true, online: true, host: true },
  { id: 'p2', pseudo: 'Bob', ready: true, online: true, host: false },
  { id: 'p3', pseudo: 'Chloé', ready: true, online: true, host: false, dead: true },
];

describe('NotesPanel', () => {
  afterEach(cleanup);

  it('affiche les marques et signale les joueurs morts', () => {
    render(
      <NotesPanel
        players={players}
        you="p1"
        notes={{ p2: 'suspect', p3: 'innocent' }}
        onCycle={() => {}}
      />,
    );

    expect(screen.getByText(/Mon carnet/)).toBeTruthy();
    expect(screen.getByRole('button', { name: /Bob : 🔴 Suspect/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /Chloé : 🟢 Innocent/ })).toBeTruthy();
    expect(screen.getByText('mort')).toBeTruthy();
    expect(screen.getByText('vous')).toBeTruthy();
  });

  it('propose la marque suivante au clic, sans la décider seule', () => {
    const onCycle = vi.fn();

    render(<NotesPanel players={players} you="p1" notes={{}} onCycle={onCycle} />);

    const bob = screen.getByRole('button', { name: /Bob : — Aucune/ });
    fireEvent.click(bob);

    expect(onCycle).toHaveBeenCalledWith('p2');
    expect(onCycle).toHaveBeenCalledTimes(1);
  });
});
