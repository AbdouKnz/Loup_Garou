import { useState } from 'react';
import type { FormEvent } from 'react';

import { ROOM_CODE_LENGTH, providerLabel, providerLinkURL } from '../api/types';
import { useAccountStore } from '../state/account';
import { useRoomStore } from '../state/room';

type Mode = 'creer' | 'rejoindre';

/** codeDepuisUrl lit /partie/CODE dans l'adresse : c'est le lien de partage. */
function codeDepuisUrl(): string {
  const match = /^\/partie\/([A-Za-z0-9]+)\/?$/.exec(window.location.pathname);

  return match ? match[1].toUpperCase() : '';
}

/** Home est l'écran d'accueil : créer une partie ou en rejoindre une. */
export default function Home() {
  const username = useAccountStore((state) => state.me?.username ?? '');
  const linked = useAccountStore((state) => state.me?.linked ?? []);
  const providers = useAccountStore((state) => state.providers);
  const logout = useAccountStore((state) => state.logout);
  const busy = useRoomStore((state) => state.busy);
  const error = useRoomStore((state) => state.error);
  const createRoom = useRoomStore((state) => state.createRoom);
  const joinRoom = useRoomStore((state) => state.joinRoom);
  const dismissError = useRoomStore((state) => state.dismissError);

  const [mode, setMode] = useState<Mode>(() => (codeDepuisUrl() ? 'rejoindre' : 'creer'));
  const [code, setCode] = useState(codeDepuisUrl);

  const codeOk = code.trim().length === ROOM_CODE_LENGTH;
  const ready = mode === 'creer' || codeOk;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!ready) {
      return;
    }

    if (mode === 'creer') {
      void createRoom();
      return;
    }

    void joinRoom(code);
  };

  return (
    <main className="card">
      <p className="eyebrow">Étape M6 — la table immersive</p>
      <h1>🐺 Loup-Garou</h1>

      <p className="account-bar">
        <span>
          Connecté en tant que <strong>{username}</strong>
        </span>
        <button type="button" className="link" onClick={() => void logout()}>
          Se déconnecter
        </button>
      </p>

      {(providers.google || providers.discord) && (
        <div className="account-links">
          <p className="account-links-title">Comptes associés</p>
          {providers.google && <LinkRow provider="google" linked={linked.includes('google')} />}
          {providers.discord && <LinkRow provider="discord" linked={linked.includes('discord')} />}
        </div>
      )}

      <p className="subtitle">
        Créez une partie, partagez le code à vos amis, puis jouez : nuit, débat, vote et victoire.
        Il faut au moins 6 joueurs.
      </p>

      {error && (
        <p className="alert" role="alert">
          <span>{error}</span>
          <button type="button" className="link" onClick={dismissError}>
            fermer
          </button>
        </p>
      )}

      <form className="panel" onSubmit={submit}>
        {mode === 'rejoindre' && (
          <label className="field">
            <span>Code de la partie</span>
            <input
              type="text"
              className="code-input"
              value={code}
              maxLength={ROOM_CODE_LENGTH}
              placeholder="ABC234"
              autoComplete="off"
              autoFocus
              onChange={(event) => setCode(event.target.value.toUpperCase())}
            />
          </label>
        )}

        <div className="actions">
          <button type="submit" className="btn primary" disabled={!ready || busy}>
            {mode === 'creer' ? 'Créer une partie' : 'Rejoindre la partie'}
          </button>

          <button
            type="button"
            className="btn"
            disabled={busy}
            onClick={() => setMode(mode === 'creer' ? 'rejoindre' : 'creer')}
          >
            {mode === 'creer' ? 'J’ai un code' : 'Créer plutôt'}
          </button>
        </div>

        {busy && <p className="hint">Un instant…</p>}
      </form>

      <ul className="roadmap">
        <li className="done">M0 — squelette Go + React, lint et tests</li>
        <li className="done">M1 — salles, lobby, WebSocket, chat</li>
        <li className="done">M2 — moteur de jeu : nuit, jour, vote, victoire</li>
        <li className="done">M3 — interface de jeu : ambiance, scrutin public, bilan</li>
        <li className="done">M4.1 — rôles spéciaux : voyante et sorcière</li>
        <li className="done">M4.2 — comptes : inscription et connexion</li>
        <li className="done">M4.3 — Google et Discord</li>
        <li className="done">M4.4 — un seul compte par joueur</li>
        <li className="done">M5.1 — sons synthétisés, sans fichier audio</li>
        <li className="done">
          M6 — table immersive : tours de parole, récit de l’aube, transitions
        </li>
      </ul>
    </main>
  );
}

/**
 * LinkRow propose d'associer un fournisseur au compte connecté, ou rappelle
 * qu'il l'est déjà.
 *
 * C'est un lien, pas un bouton : le rattachement passe par le fournisseur, dont
 * le retour ramène à l'application. Associer un fournisseur déjà rattaché n'est
 * jamais proposé.
 */
function LinkRow({ provider, linked }: { provider: string; linked: boolean }) {
  const label = providerLabel(provider);

  return (
    <p className="account-link">
      <span>{label}</span>
      {linked ? (
        <span className="account-linked">associé ✓</span>
      ) : (
        <a className="account-associer" href={providerLinkURL(provider)}>
          Associer {label}
        </a>
      )}
    </p>
  );
}
