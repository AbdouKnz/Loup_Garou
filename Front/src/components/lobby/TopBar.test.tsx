// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';

import TopBar from './TopBar';

const PROVIDERS = { google: true, discord: true };

afterEach(cleanup);

describe('TopBar', () => {
  it('ramène à l’accueil et ouvre les amis', () => {
    const onLogo = vi.fn();
    const onOpenFriends = vi.fn();
    render(
      <TopBar
        username="Alice"
        providers={PROVIDERS}
        linked={[]}
        onlineFriends={3}
        friendsOpen={false}
        onLogo={onLogo}
        onOpenFriends={onOpenFriends}
        onLogout={() => {}}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Loup-Garou — accueil' }));
    expect(onLogo).toHaveBeenCalledTimes(1);

    const friends = screen.getByRole('button', { name: /Amis/ });
    expect(friends.getAttribute('aria-expanded')).toBe('false');
    fireEvent.click(friends);
    expect(onOpenFriends).toHaveBeenCalledTimes(1);
  });

  it('garde les rattachements dans le menu de compte', () => {
    const { container } = render(
      <TopBar
        username="Alice"
        providers={PROVIDERS}
        linked={['google']}
        onlineFriends={3}
        friendsOpen={false}
        onLogo={() => {}}
        onOpenFriends={() => {}}
        onLogout={() => {}}
      />,
    );

    const panel = container.querySelector('.lobby-account-panel');
    expect(panel).not.toBeNull();
    expect(within(panel as HTMLElement).getByText('associé ✓')).toBeTruthy();
    expect(
      within(panel as HTMLElement).getByRole('link', { name: 'Associer Discord' }),
    ).toBeTruthy();
  });

  it('déconnecte le compte', () => {
    const onLogout = vi.fn();
    render(
      <TopBar
        username="Alice"
        providers={PROVIDERS}
        linked={[]}
        onlineFriends={3}
        friendsOpen={false}
        onLogo={() => {}}
        onOpenFriends={() => {}}
        onLogout={onLogout}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Se déconnecter' }));

    expect(onLogout).toHaveBeenCalledTimes(1);
  });

  it('ne crée pas de menu sans fournisseur configuré', () => {
    const { container } = render(
      <TopBar
        username="Alice"
        providers={{ google: false, discord: false }}
        linked={[]}
        onlineFriends={0}
        friendsOpen={false}
        onLogo={() => {}}
        onOpenFriends={() => {}}
        onLogout={() => {}}
      />,
    );

    expect(container.querySelector('.lobby-account')).toBeNull();
    expect(screen.queryByText('Comptes associés')).toBeNull();
  });
});
