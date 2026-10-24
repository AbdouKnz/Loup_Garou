// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import OpenTablesList from './OpenTablesList';
import type { OpenTable } from './types';

const TABLES: OpenTable[] = [
  {
    id: 't1',
    code: 'BRLUNE',
    name: 'La clairière',
    host: 'Camille',
    players: 4,
    maxPlayers: 12,
    status: 'waiting',
  },
  {
    id: 't2',
    code: 'NUIT89',
    name: 'Les loups veillent',
    host: 'Nassim',
    players: 9,
    maxPlayers: 12,
    status: 'in_progress',
  },
];

afterEach(cleanup);

describe('OpenTablesList', () => {
  it('liste les tables et leur état', () => {
    render(<OpenTablesList tables={TABLES} busy={false} onJoin={() => {}} />);

    expect(screen.getByText('La clairière')).toBeTruthy();
    expect(screen.getByText('En attente')).toBeTruthy();
    expect(screen.getByText('En cours')).toBeTruthy();
    expect(screen.getByText(/4\/12 joueurs/)).toBeTruthy();
  });

  it('transmet la table choisie', () => {
    const onJoin = vi.fn();
    render(<OpenTablesList tables={TABLES} busy={false} onJoin={onJoin} />);

    fireEvent.click(screen.getByRole('button', { name: 'Rejoindre Les loups veillent' }));

    expect(onJoin).toHaveBeenCalledWith(TABLES[1]);
  });

  it('affiche un état vide', () => {
    render(<OpenTablesList tables={[]} busy={false} onJoin={() => {}} />);

    expect(screen.getByText('Aucune table ouverte. Créez la première.')).toBeTruthy();
  });
});
