import type { Providers } from '../../api/types';
import { providerLabel, providerLinkURL } from '../../api/types';
import Avatar from '../game/Avatar';
import { WolfIcon } from '../WolfIcon';
import { ChevronIcon, LogoutIcon, UsersIcon } from './icons';

type Props = {
  username: string;
  providers: Providers;
  linked: string[];
  onlineFriends: number;
  friendsOpen: boolean;
  /** Absent en salle d'attente : la marque reste alors un simple repère. */
  onLogo?: () => void;
  onOpenFriends: () => void;
  onLogout: () => void;
};

/** avatarIndex donne une pastille stable au compte, sans dépendre du serveur. */
function avatarIndex(username: string): number {
  let hash = 0;

  for (const character of username) {
    hash = (hash * 31 + character.codePointAt(0)!) % 12;
  }

  return hash;
}

/**
 * TopBar rassemble la marque, l'accès aux amis sur mobile et le compte.
 *
 * Les liens de rattachement restent des navigations vers le fournisseur : le
 * retour ramène dans l'application avec la session courante.
 */
export default function TopBar({
  username,
  providers,
  linked,
  onlineFriends,
  friendsOpen,
  onLogo,
  onOpenFriends,
  onLogout,
}: Props) {
  const hasProviders = providers.google || providers.discord;

  const identity = (
    <>
      <Avatar index={avatarIndex(username)} />
      <span className="lobby-user-identity">
        <span className="lobby-user-name">{username}</span>
        <span className="lobby-user-status">
          <span className="lobby-online-dot" aria-hidden="true" />
          En ligne
        </span>
      </span>
    </>
  );

  const brand = (
    <>
      <span className="lobby-logo-mark" aria-hidden="true">
        <WolfIcon />
      </span>
      <span className="lobby-wordmark">LOUP GAROU</span>
    </>
  );

  return (
    <header className="lobby-topbar">
      {onLogo ? (
        <button
          type="button"
          className="lobby-logo"
          onClick={onLogo}
          aria-label="Loup-Garou — accueil"
        >
          {brand}
        </button>
      ) : (
        <div className="lobby-logo lobby-logo-static">{brand}</div>
      )}

      <div className="lobby-topbar-right">
        <button
          type="button"
          className="lobby-friends-trigger"
          aria-expanded={friendsOpen}
          aria-controls={friendsOpen ? 'lobby-friends-drawer' : undefined}
          onClick={onOpenFriends}
        >
          <UsersIcon />
          Amis
          <span className="lobby-friends-count">{onlineFriends}</span>
        </button>

        <div className="lobby-user-zone">
          {hasProviders ? (
            <details className="lobby-account">
              <summary className="lobby-user">
                <span className="visually-hidden">Compte de </span>
                {identity}
                <ChevronIcon className="lobby-account-chevron" />
              </summary>

              <div className="lobby-account-panel">
                <p className="lobby-account-title">Comptes associés</p>
                {providers.google && (
                  <LinkRow provider="google" linked={linked.includes('google')} />
                )}
                {providers.discord && (
                  <LinkRow provider="discord" linked={linked.includes('discord')} />
                )}
              </div>
            </details>
          ) : (
            <div className="lobby-user">{identity}</div>
          )}

          <button
            type="button"
            className="lobby-icon-button"
            onClick={onLogout}
            aria-label="Se déconnecter"
          >
            <LogoutIcon />
          </button>
        </div>
      </div>
    </header>
  );
}

/**
 * LinkRow propose d'associer un fournisseur au compte, ou rappelle qu'il l'est
 * déjà. Associer un fournisseur déjà rattaché n'est jamais proposé.
 */
function LinkRow({ provider, linked }: { provider: string; linked: boolean }) {
  const label = providerLabel(provider);

  return (
    <p className="lobby-account-link">
      <span>{label}</span>
      {linked ? (
        <span className="lobby-account-linked">associé ✓</span>
      ) : (
        <a className="lobby-account-associer" href={providerLinkURL(provider)}>
          Associer {label}
        </a>
      )}
    </p>
  );
}
