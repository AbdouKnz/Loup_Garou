import { create } from 'zustand';

import { messageErreur } from '../api/client';
import { httpAccountApi } from '../api/transport';
import type { AccountApi } from '../api/transport';
import { NO_PROVIDERS } from '../api/types';
import type { Account, PendingOAuth, Providers } from '../api/types';

/**
 * État du compte connecté : « qui suis-je ? ».
 *
 * Il est tenu à part de l'état de la partie : le cookie de session autorise les
 * appels à l'API, le jeton de siège identifie une place dans une salle. On peut
 * très bien être connecté sans jouer, et jouer dans plusieurs onglets.
 *
 * Les fournisseurs externes et la connexion en attente viennent de `load` : ce
 * sont des propriétés du serveur et du navigateur, pas de la session.
 */
export type AccountStoreState = {
  /** Vrai dès que le serveur a dit qui était connecté. */
  ready: boolean;
  /** Identité du compte connecté, ou null si personne ne l'est. */
  me: Account | null;
  /** Fournisseurs externes proposés par le serveur. */
  providers: Providers;
  /** Connexion externe vérifiée qui attend un pseudo, s'il y en a une. */
  pending: PendingOAuth | null;
  /** Vrai pendant un appel réseau (inscription, connexion). */
  busy: boolean;
  /** Message d'erreur à afficher, s'il y en a un. */
  error: string | null;

  /** load demande au serveur qui est connecté, au démarrage de l'application. */
  load(): Promise<void>;
  /** signup crée un compte et ouvre sa session. */
  signup(username: string, password: string): Promise<void>;
  /** login ouvre la session d'un compte existant. */
  login(username: string, password: string): Promise<void>;
  /** complete nomme une connexion externe et ouvre sa session. */
  complete(username: string): Promise<void>;
  /** logout ferme la session de compte. */
  logout(): Promise<void>;
  dismissError(): void;
};

/** createAccountStore construit un store isolé : un par onglet, un par test. */
export function createAccountStore(api: AccountApi) {
  return create<AccountStoreState>((set) => {
    /** authenticate enchaîne un appel d'identité et range son résultat. */
    const authenticate = async (call: () => Promise<Account>) => {
      set({ busy: true, error: null });

      try {
        const me = await call();
        // Le pseudo est choisi : plus rien n'attend au guichet.
        set({ me: me.authenticated ? me : null, pending: null, ready: true });
      } catch (error) {
        set({ error: messageErreur(error) });
      } finally {
        set({ busy: false });
      }
    };

    return {
      ready: false,
      me: null,
      providers: NO_PROVIDERS,
      pending: null,
      busy: false,
      error: null,

      load: async () => {
        try {
          const me = await api.me();

          set({
            me: me.authenticated ? me : null,
            providers: me.providers ?? NO_PROVIDERS,
            pending: me.pending ?? null,
            ready: true,
            // Le serveur annonce une fois l'échec d'une connexion externe :
            // c'est ici qu'il arrive, sur l'écran de connexion.
            error: me.error?.message ?? null,
          });
        } catch (error) {
          // Serveur injoignable : on montre l'écran de connexion avec la
          // raison, plutôt que de rester sur « Connexion… » indéfiniment.
          set({ me: null, ready: true, error: messageErreur(error) });
        }
      },

      signup: (username, password) => authenticate(() => api.signup(username.trim(), password)),

      login: (username, password) => authenticate(() => api.login(username.trim(), password)),

      complete: (username) => authenticate(() => api.complete(username.trim())),

      logout: async () => {
        set({ busy: true, error: null });

        try {
          await api.logout();
        } catch (error) {
          set({ error: messageErreur(error) });
        } finally {
          // Que le serveur ait répondu ou non, cette session ne vaut plus rien
          // ici : on revient à l'écran de connexion.
          set({ me: null, pending: null, busy: false });
        }
      },

      dismissError: () => set({ error: null }),
    };
  });
}

/** Store de l'application : l'identité du compte connecté. */
export const useAccountStore = createAccountStore(httpAccountApi());
