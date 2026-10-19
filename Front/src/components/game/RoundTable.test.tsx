// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';

import RoundTable from './RoundTable';
import type { PlayerView } from '../../api/types';

/**
 * Ces tests vérifient la table ronde : où sont assis les joueurs, ce que chaque
 * siège raconte, et ce que la table propose. Elle ne déduit jamais rien du
 * serveur : un siège ne s'éclaire que si on le lui dit.
 */

const roster: PlayerView[] = [
  { id: 'p1', pseudo: 'Alice', ready: true, online: true, host: true },
  { id: 'p2', pseudo: 'Bob', ready: true, online: true, host: false },
  { id: 'p3', pseudo: 'Chloé', ready: true, online: true, host: false, dead: true },
  { id: 'p4', pseudo: 'David', ready: true, online: false, host: false },
];

const center = { emoji: '🌙', title: 'Nuit 1', note: '3 vivants' };

describe('RoundTable', () => {
  afterEach(cleanup);

  it('assied chaque joueur à sa place', () => {
    render(<RoundTable players={roster} you="p1" center={center} />);

    const seats = screen.getAllByRole('listitem');
    expect(seats).toHaveLength(4);

    // Chaque siège a sa place sur l'ellipse, et deux sièges ne se superposent pas.
    const places = seats.map((seat) => seat.getAttribute('style'));
    expect(new Set(places).size).toBe(4);
    for (const place of places) {
      expect(place).toMatch(/--seat-x/);
      expect(place).toMatch(/--seat-y/);
    }

    expect(screen.getByText('Nuit 1')).toBeTruthy();
    expect(screen.getByText('3 vivants')).toBeTruthy();
  });

  it('marque les morts et les joueurs hors ligne', () => {
    render(<RoundTable players={roster} you="p1" center={center} />);

    const mort = screen.getByText('Chloé').closest('li');
    expect(mort?.className).toContain('gone');
    expect(within(mort!).getByText('mort')).toBeTruthy();

    expect(screen.getByText('David').closest('li')?.className).toContain('away');
  });

  it('ne fait un bouton que des cibles autorisées', () => {
    const onChoose = vi.fn();

    render(
      <RoundTable
        players={roster}
        you="p1"
        center={center}
        action={{
          label: 'Voter',
          can: (player) => !player.dead && player.id !== 'p2',
          onChoose,
        }}
      />,
    );

    const boutons = screen.getAllByRole('button', { name: /^Voter/ });
    expect(boutons.map((bouton) => bouton.getAttribute('aria-label'))).toEqual([
      'Voter Alice',
      'Voter David',
    ]);

    fireEvent.click(screen.getByRole('button', { name: 'Voter David' }));
    expect(onChoose).toHaveBeenCalledWith('p4');

    // Sans invitation du serveur, la table ne propose rien.
    expect(screen.getByText('Chloé').closest('li')?.querySelector('button')).toBeNull();
  });

  it('pose les jetons du scrutin sur les sièges visés', () => {
    render(<RoundTable players={roster} you="p1" center={center} votes={{ p2: 3 }} />);

    expect(screen.getByText('×3')).toBeTruthy();
    expect(screen.queryByText('×0')).toBeNull();
  });

  it('n’éclaire que les sièges annoncés par le serveur', () => {
    render(<RoundTable players={roster} you="p1" center={center} awake={['p2']} marked="p3" />);

    expect(screen.getByText('Bob').closest('li')?.className).toContain('awake');
    expect(screen.getByText('Chloé').closest('li')?.className).toContain('marked');
    expect(screen.getByText('Alice').closest('li')?.className).not.toContain('awake');
  });

  it('met en vedette le joueur qui a la parole', () => {
    render(<RoundTable players={roster} you="p1" center={center} speaking="p2" />);

    const orateur = screen.getByText('Bob').closest('li');
    expect(orateur?.className).toContain('speaking');
    expect(within(orateur!).getByText('🎤')).toBeTruthy();

    // Sans tour de parole annoncé, aucun siège ne prend la vedette.
    expect(screen.getByText('Alice').closest('li')?.className).not.toContain('speaking');
  });
});
