import { useState } from 'react';
import type { FormEvent } from 'react';

import {
  ArrowRightIcon,
  DiscordMark,
  EyeIcon,
  EyeOffIcon,
  GoogleMark,
  LockIcon,
  UserIcon,
} from '../components/icons';
import { WolfIcon } from '../components/WolfIcon';
import {
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  PSEUDO_MAX_LENGTH,
  PSEUDO_MIN_LENGTH,
  providerLabel,
  providerStartURL,
} from '../api/types';
import type { Providers } from '../api/types';
import { useAccountStore } from '../state/account';

type Mode = 'creer' | 'connecter';

/**
 * Auth est la page d'atterrissage : la marque du loup en haut à gauche, la carte
 * de connexion au centre de l'écran.
 *
 * Un compte est obligatoire pour jouer : c'est lui qui porte le pseudo affiché
 * à table, et qui permet de retrouver sa place après un rafraîchissement.
 *
 * Trois chemins mènent au même compte, et dans les trois cas le pseudo est
 * choisi ici : une connexion externe vérifie l'identité, elle ne nomme
 * personne. C'est pourquoi la carte a deux visages — un formulaire complet, ou
 * la seule demande de pseudo quand le fournisseur a déjà répondu.
 */
export default function Auth() {
  const busy = useAccountStore((state) => state.busy);
  const error = useAccountStore((state) => state.error);
  const providers = useAccountStore((state) => state.providers);
  const pending = useAccountStore((state) => state.pending);
  const signup = useAccountStore((state) => state.signup);
  const login = useAccountStore((state) => state.login);
  const complete = useAccountStore((state) => state.complete);
  const dismissError = useAccountStore((state) => state.dismissError);

  const [mode, setMode] = useState<Mode>('creer');
  // Le pseudo proposé par le fournisseur pré-remplit la case : le joueur le
  // garde d'une touche, ou le remplace.
  const [name, setName] = useState(pending?.suggested ?? '');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  const ready = name.trim().length >= PSEUDO_MIN_LENGTH && password.length >= PASSWORD_MIN_LENGTH;
  const named = name.trim().length >= PSEUDO_MIN_LENGTH;
  const canSubmit = pending ? named : ready;

  /** choisir change d'onglet sans laisser traîner la saisie précédente. */
  const choisir = (next: Mode) => {
    setMode(next);
    setPassword('');
    dismissError();
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }

    if (pending) {
      void complete(name);
      return;
    }

    if (mode === 'creer') {
      void signup(name, password);
      return;
    }

    void login(name, password);
  };

  return (
    <div className="auth-shell">
      {/* Halos flous qui donnent sa profondeur au fond. */}
      <div className="auth-glow top" aria-hidden="true" />
      <div className="auth-glow bottom" aria-hidden="true" />
      <div className="auth-glow left" aria-hidden="true" />
      <div className="auth-glow right" aria-hidden="true" />

      <header className="auth-brand" data-testid="auth-brand">
        <WolfIcon className="wolf" />
        <span className="auth-brand-name">Loup-Garou</span>
      </header>

      <main className="auth-stage">
        {/* Carte de verre : les faisceaux font le tour de sa bordure. */}
        <section className="auth-card">
          <span className="auth-beam top" aria-hidden="true" />
          <span className="auth-beam right" aria-hidden="true" />
          <span className="auth-beam bottom" aria-hidden="true" />
          <span className="auth-beam left" aria-hidden="true" />

          <div className="auth-head">
            <span className="auth-logo" aria-hidden="true">
              <WolfIcon className="wolf" />
            </span>
            <h1 className="auth-title">{pending ? 'Choisissez votre pseudo' : 'Bon retour'}</h1>
            <p className="auth-caption">
              {pending ? (
                <>
                  Votre compte {providerLabel(pending.provider)} est vérifié. Choisissez le pseudo
                  qui s’affichera à table.
                </>
              ) : (
                'Un compte, c’est votre place : votre pseudo à table, vos parties qui vous attendent.'
              )}
            </p>
          </div>

          {error && (
            <p className="alert" role="alert">
              <span>{error}</span>
              <button type="button" className="link" onClick={dismissError}>
                fermer
              </button>
            </p>
          )}

          {!pending && (
            <div className="tabs">
              <button
                type="button"
                className={mode === 'creer' ? 'tab active' : 'tab'}
                aria-pressed={mode === 'creer'}
                onClick={() => choisir('creer')}
              >
                Créer un compte
              </button>
              <button
                type="button"
                className={mode === 'connecter' ? 'tab active' : 'tab'}
                aria-pressed={mode === 'connecter'}
                onClick={() => choisir('connecter')}
              >
                J’ai déjà un compte
              </button>
            </div>
          )}

          <form onSubmit={submit}>
            <div className="auth-field">
              <label className="visually-hidden" htmlFor="auth-pseudo">
                Votre pseudo
              </label>
              <UserIcon className="auth-icon" />
              <input
                id="auth-pseudo"
                type="text"
                value={name}
                maxLength={PSEUDO_MAX_LENGTH}
                placeholder="Votre pseudo"
                autoComplete="username"
                autoFocus
                onChange={(event) => setName(event.target.value)}
              />
            </div>

            {!pending && (
              <div className="auth-field">
                <label className="visually-hidden" htmlFor="auth-password">
                  Mot de passe
                </label>
                <LockIcon className="auth-icon" />
                <input
                  id="auth-password"
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  maxLength={PASSWORD_MAX_LENGTH}
                  placeholder="Mot de passe"
                  autoComplete={mode === 'creer' ? 'new-password' : 'current-password'}
                  onChange={(event) => setPassword(event.target.value)}
                />
                <button
                  type="button"
                  className="auth-eye"
                  aria-label={showPassword ? 'Masquer le mot de passe' : 'Afficher le mot de passe'}
                  onClick={() => setShowPassword((value) => !value)}
                >
                  {showPassword ? <EyeOffIcon /> : <EyeIcon />}
                </button>
              </div>
            )}

            <button type="submit" className="auth-submit" disabled={!canSubmit || busy}>
              {pending
                ? 'Choisir ce pseudo'
                : mode === 'creer'
                  ? 'Créer mon compte'
                  : 'Se connecter'}
              <ArrowRightIcon className="auth-submit-arrow" />
            </button>

            {pending ? (
              <p className="hint">
                {PSEUDO_MIN_LENGTH} à {PSEUDO_MAX_LENGTH} caractères.{' '}
                {providerLabel(pending.provider)} a seulement vérifié que c’est bien vous : ce
                pseudo reste le vôtre.
              </p>
            ) : (
              mode === 'creer' && (
                <p className="hint">
                  {PSEUDO_MIN_LENGTH} à {PSEUDO_MAX_LENGTH} caractères pour le pseudo, au moins{' '}
                  {PASSWORD_MIN_LENGTH} pour le mot de passe.
                </p>
              )
            )}
          </form>

          {!pending && <External providers={providers} />}

          {busy && <p className="hint">Un instant…</p>}
        </section>
      </main>
    </div>
  );
}

/**
 * External propose les fournisseurs configurés sur ce serveur.
 *
 * Ce sont des liens, pas des boutons : se connecter chez un fournisseur est une
 * navigation, dont le retour ramène à l'application.
 */
function External({ providers }: { providers: Providers }) {
  if (!providers.google && !providers.discord) {
    return null;
  }

  return (
    <div className="external">
      <p className="separator">
        <span>ou</span>
      </p>

      <div className="providers">
        {providers.google && (
          <a className="btn provider" href={providerStartURL('google')}>
            <GoogleMark className="provider-mark" />
            Continuer avec Google
          </a>
        )}
        {providers.discord && (
          <a className="btn provider" href={providerStartURL('discord')}>
            <DiscordMark className="provider-mark" />
            Continuer avec Discord
          </a>
        )}
      </div>
    </div>
  );
}
