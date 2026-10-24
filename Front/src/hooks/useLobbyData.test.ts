// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';

import { useLobbyData } from './useLobbyData';

afterEach(cleanup);

describe('useLobbyData', () => {
  it('sépare les amis en ligne et hors ligne', () => {
    const { result } = renderHook(() => useLobbyData());

    expect(result.current.friends).toHaveLength(6);
    expect(result.current.onlineFriends.map((friend) => friend.username)).toEqual([
      'Camille',
      'Nassim',
      'Lou',
    ]);
    expect(result.current.offlineFriends).toHaveLength(3);
  });

  it('recherche puis ajoute un joueur', () => {
    const { result } = renderHook(() => useLobbyData());

    act(() => result.current.setSearch('jade'));
    act(() => result.current.runSearch());

    expect(result.current.searched).toBe(true);
    expect(result.current.searchResults.map((friend) => friend.username)).toEqual(['Jade']);

    act(() => result.current.addFriend(result.current.searchResults[0]));

    expect(result.current.friends.some((friend) => friend.username === 'Jade')).toBe(true);
    expect(result.current.onlineFriends).toHaveLength(4);
    expect(result.current.searchResults).toHaveLength(0);
    expect(result.current.searched).toBe(false);
  });

  it('retire un ami et ses invitations', () => {
    const { result } = renderHook(() => useLobbyData());

    act(() => result.current.toggleInvite('f-ines'));
    expect(result.current.invited).toContain('f-ines');

    act(() => result.current.removeFriend('f-ines'));

    expect(result.current.friends.some((friend) => friend.id === 'f-ines')).toBe(false);
    expect(result.current.invited).not.toContain('f-ines');
  });

  it('borne les réglages et active les rôles', () => {
    const { result } = renderHook(() => useLobbyData());

    act(() => result.current.setPlayers(2));
    expect(result.current.settings.players).toBe(6);

    act(() => result.current.setPlayers(30));
    expect(result.current.settings.players).toBe(18);

    act(() => result.current.setNightTimer(5));
    expect(result.current.settings.nightTimer).toBe(30);

    act(() => result.current.setDebateTimer(999));
    expect(result.current.settings.debateTimer).toBe(180);

    act(() => result.current.toggleRole('chasseur'));
    expect(result.current.settings.roles.chasseur).toBe(true);
  });

  it('génère un code quand la table devient privée', () => {
    const { result } = renderHook(() => useLobbyData());

    act(() => result.current.setPrivate(true));

    expect(result.current.settings.privateTable).toBe(true);
    expect(result.current.settings.privateCode).toMatch(/^[A-Z2-9]{6}$/);

    act(() => result.current.setPrivate(false));

    expect(result.current.settings.privateTable).toBe(false);
    expect(result.current.settings.privateCode).toBeUndefined();

    act(() => result.current.setPrivate(true));
    expect(result.current.settings.privateCode).toMatch(/^[A-Z2-9]{6}$/);
  });
});
