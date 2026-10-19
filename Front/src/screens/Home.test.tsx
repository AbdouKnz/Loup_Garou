// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';

import Home from './Home';
import { useAccountStore } from '../state/account';
import { NO_PROVIDERS } from '../api/types';

/** Les actions réelles du store, rendues à la fin de chaque test. */
const actions = useAccountStore.getState();

/** Les deux fournisseurs, tels qu'un serveur bien configuré les annonce. */
const BOTH_PROVIDERS = { google: true, discord: true };

beforeEach(() => {
  useAccountStore.setState({
    me: { authenticated: true, username: 'Alice', linked: [] },
    providers: BOTH_PROVIDERS,
    ready: true,
    pending: null,
    error: null,
  });
});

afterEach(() => {
  cleanup();
  useAccountStore.setState({
    ...actions,
    me: null,
    providers: NO_PROVIDERS,
    ready: true,
    pending: null,
    error: null,
  });
});

describe('Home', () => {
  it('propose d’associer chaque fournisseur non rattaché', () => {
    render(<Home />);

    // Le rattachement est une navigation : ce sont des liens, pas des boutons.
    const google = screen.getByRole('link', { name: 'Associer Google' });
    expect(google.getAttribute('href')).toBe('/api/auth/google/link');

    const discord = screen.getByRole('link', { name: 'Associer Discord' });
    expect(discord.getAttribute('href')).toBe('/api/auth/discord/link');
  });

  it('rappelle les fournisseurs déjà rattachés et ne les propose plus', () => {
    useAccountStore.setState({
      me: { authenticated: true, username: 'Alice', linked: ['google'] },
    });

    render(<Home />);

    expect(screen.queryByRole('link', { name: 'Associer Google' })).toBeNull();
    expect(screen.getByText('associé ✓')).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Associer Discord' })).toBeTruthy();
  });

  it('n’affiche pas de section de rattachement sans fournisseur configuré', () => {
    useAccountStore.setState({ providers: NO_PROVIDERS });

    render(<Home />);

    expect(screen.queryByText('Comptes associés')).toBeNull();
  });
});
