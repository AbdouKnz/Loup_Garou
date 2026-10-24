// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';

import Lobby from './Lobby';
import { useAccountStore } from '../state/account';
import { useRoomStore } from '../state/room';
import { MAX_PLAYERS } from '../api/types';
import type { RoomState } from '../api/types';

const accountActions = useAccountStore.getState();
const roomActions = useRoomStore.getState();

const room: RoomState = {
  code: 'ABC234',
  phase: 'lobby',
  hostId: 'p1',
  you: 'p1',
  maxPlayers: MAX_PLAYERS,
  canStart: false,
  players: [
    { id: 'p1', pseudo: 'Alice', ready: false, online: true, host: true },
    { id: 'p2', pseudo: 'Bob', ready: false, online: true, host: false },
  ],
  messages: [],
  composition: ['villageois', 'villageois', 'villageois', 'loup-garou', 'loup-garou', 'voyante'],
};

beforeEach(() => {
  useAccountStore.setState({
    me: { authenticated: true, username: 'Alice', linked: [] },
    providers: { google: false, discord: false },
    ready: true,
    pending: null,
    error: null,
    busy: false,
  });

  useRoomStore.setState({ connection: 'online', error: null });
});

afterEach(() => {
  cleanup();
  useAccountStore.setState({
    me: accountActions.me,
    providers: accountActions.providers,
    ready: accountActions.ready,
    pending: accountActions.pending,
    error: accountActions.error,
    busy: accountActions.busy,
  });
  useRoomStore.setState({
    connection: roomActions.connection,
    error: roomActions.error,
    leave: roomActions.leave,
  });
});

describe('Lobby', () => {
  it('garde l’en-tête et les amis, et place Quitter dans la carte', () => {
    const leave = vi.fn();
    useRoomStore.setState({ leave });

    render(<Lobby room={room} />);

    expect(document.querySelector('.lobby-topbar')).toBeTruthy();
    expect(screen.getByRole('complementary', { name: 'Amis' })).toBeTruthy();

    const card = screen.getByRole('region', { name: 'Salle d’attente' });
    expect(within(card).getByText('ABC234')).toBeTruthy();

    fireEvent.click(within(card).getByRole('button', { name: 'Quitter' }));
    expect(leave).toHaveBeenCalledTimes(1);

    expect(within(card).queryByRole('button', { name: 'Règles du jeu' })).toBeNull();
    expect(within(card).queryByRole('button', { name: 'Copier le lien' })).toBeNull();
    expect(within(card).queryByRole('button', { name: 'Se déconnecter' })).toBeNull();
    expect(document.querySelector('.lobby-tools')).toBeNull();
    expect(document.querySelector('.pill')).toBeNull();
  });
});
