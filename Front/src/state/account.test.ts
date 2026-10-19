import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '../api/client';
import type { AccountApi } from '../api/transport';
import type { Account } from '../api/types';
import { createAccountStore } from './account';

/** Fausse API de compte : le test décide de ce que le serveur répond. */
function fakeApi(overrides: Partial<AccountApi> = {}): AccountApi {
  return {
    me: () => Promise.resolve({ authenticated: false }),
    signup: () => Promise.resolve({ authenticated: true, username: 'Alice' }),
    login: () => Promise.resolve({ authenticated: true, username: 'Alice' }),
    complete: () => Promise.resolve({ authenticated: true, username: 'Alice' }),
    logout: () => Promise.resolve(),
    ...overrides,
  };
}

/** Les fournisseurs d'un serveur donné. */
const GOOGLE_ONLY = { google: true, discord: false };

describe('createAccountStore', () => {
  it('charge l’identité au démarrage', async () => {
    const api = fakeApi({
      me: () => Promise.resolve({ authenticated: true, username: 'Alice' } as Account),
    });
    const store = createAccountStore(api);

    await store.getState().load();

    expect(store.getState().ready).toBe(true);
    expect(store.getState().me?.username).toBe('Alice');
    expect(store.getState().error).toBeNull();
  });

  it('reste sur l’accueil quand personne n’est connecté', async () => {
    const store = createAccountStore(fakeApi());

    await store.getState().load();

    expect(store.getState().ready).toBe(true);
    expect(store.getState().me).toBeNull();
  });

  it('affiche la raison quand le serveur ne répond pas', async () => {
    const api = fakeApi({ me: () => Promise.reject(new Error('Failed to fetch')) });
    const store = createAccountStore(api);

    await store.getState().load();

    // L'écran de connexion prend la main, avec l'explication : rester bloqué
    // sur « Connexion… » serait pire.
    expect(store.getState().ready).toBe(true);
    expect(store.getState().me).toBeNull();
    expect(store.getState().error).toBe('Failed to fetch');
  });

  it('ouvre la session après une inscription', async () => {
    const signup = vi.fn(() => Promise.resolve({ authenticated: true, username: 'Alice' }));
    const store = createAccountStore(fakeApi({ signup }));

    await store.getState().signup('  Alice  ', 'motdepasse');

    expect(signup).toHaveBeenCalledWith('Alice', 'motdepasse');
    expect(store.getState().me?.username).toBe('Alice');
    expect(store.getState().busy).toBe(false);
  });

  it('rapporte le refus d’une inscription', async () => {
    const api = fakeApi({
      signup: () => Promise.reject(new ApiError('username_taken', 'Ce pseudo est déjà pris.')),
    });
    const store = createAccountStore(api);

    await store.getState().signup('Alice', 'motdepasse');

    expect(store.getState().me).toBeNull();
    expect(store.getState().error).toBe('Ce pseudo est déjà pris.');
    expect(store.getState().busy).toBe(false);
  });

  it('rapporte le refus d’une connexion', async () => {
    const api = fakeApi({
      login: () =>
        Promise.reject(new ApiError('bad_credentials', 'Pseudo ou mot de passe incorrect.')),
    });
    const store = createAccountStore(api);

    await store.getState().login('Alice', 'faux');

    expect(store.getState().me).toBeNull();
    expect(store.getState().error).toBe('Pseudo ou mot de passe incorrect.');
  });

  it('oublie l’identité après une déconnexion', async () => {
    const api = fakeApi({
      me: () => Promise.resolve({ authenticated: true, username: 'Alice' }),
    });
    const store = createAccountStore(api);

    await store.getState().load();
    await store.getState().logout();

    expect(store.getState().me).toBeNull();
    expect(store.getState().busy).toBe(false);
  });

  it('revient à l’écran de connexion même si le serveur refuse', async () => {
    const api = fakeApi({
      me: () => Promise.resolve({ authenticated: true, username: 'Alice' }),
      logout: () => Promise.reject(new Error('Failed to fetch')),
    });
    const store = createAccountStore(api);

    await store.getState().load();
    await store.getState().logout();

    expect(store.getState().me).toBeNull();
    expect(store.getState().error).toBe('Failed to fetch');
  });

  it('efface l’erreur affichée', async () => {
    const api = fakeApi({
      login: () =>
        Promise.reject(new ApiError('bad_credentials', 'Pseudo ou mot de passe incorrect.')),
    });
    const store = createAccountStore(api);

    await store.getState().login('Alice', 'faux');
    store.getState().dismissError();

    expect(store.getState().error).toBeNull();
  });

  it('retient les fournisseurs configurés sur le serveur', async () => {
    const api = fakeApi({
      me: () => Promise.resolve({ authenticated: false, providers: GOOGLE_ONLY }),
    });
    const store = createAccountStore(api);

    await store.getState().load();

    expect(store.getState().providers).toEqual(GOOGLE_ONLY);
  });

  it('retient les fournisseurs déjà rattachés au compte', async () => {
    const api = fakeApi({
      me: () =>
        Promise.resolve({ authenticated: true, username: 'Alice', linked: ['google'] } as Account),
    });
    const store = createAccountStore(api);

    await store.getState().load();

    expect(store.getState().me?.linked).toEqual(['google']);
  });

  it('retient le pseudo à choisir après une connexion externe', async () => {
    const api = fakeApi({
      me: () =>
        Promise.resolve({
          authenticated: false,
          providers: GOOGLE_ONLY,
          pending: { provider: 'google', suggested: 'Jean' },
        }),
    });
    const store = createAccountStore(api);

    await store.getState().load();

    expect(store.getState().me).toBeNull();
    expect(store.getState().pending).toEqual({ provider: 'google', suggested: 'Jean' });
  });

  it('annonce l’échec d’une connexion externe', async () => {
    const api = fakeApi({
      me: () =>
        Promise.resolve({
          authenticated: false,
          error: { code: 'oauth_state', message: 'Cette connexion a expiré. Recommencez.' },
        }),
    });
    const store = createAccountStore(api);

    await store.getState().load();

    expect(store.getState().error).toBe('Cette connexion a expiré. Recommencez.');
  });

  it('ouvre la session quand le pseudo est choisi', async () => {
    const complete = vi.fn(() => Promise.resolve({ authenticated: true, username: 'Jean' }));
    const store = createAccountStore(fakeApi({ complete }));

    await store.getState().complete('  Jean  ');

    expect(complete).toHaveBeenCalledWith('Jean');
    expect(store.getState().me?.username).toBe('Jean');
    expect(store.getState().pending).toBeNull();
  });

  it('rapporte le refus d’un pseudo déjà pris', async () => {
    const api = fakeApi({
      complete: () =>
        Promise.reject(
          new ApiError('username_taken', 'Ce pseudo est déjà pris. Choisissez-en un autre.'),
        ),
    });
    const store = createAccountStore(api);

    await store.getState().complete('Alice');

    expect(store.getState().me).toBeNull();
    expect(store.getState().error).toBe('Ce pseudo est déjà pris. Choisissez-en un autre.');
    expect(store.getState().busy).toBe(false);
  });
});
