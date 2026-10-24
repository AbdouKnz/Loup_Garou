import { useId, useRef, useState } from 'react';

import { useFocusTrap } from '../../hooks/useFocusTrap';
import type { LobbyData } from '../../hooks/useLobbyData';
import Avatar from '../game/Avatar';
import { LockIcon } from '../icons';
import TableSettingsCard from './TableSettingsCard';
import { CheckIcon, CloseIcon, TableIcon } from './icons';

type Props = {
  mode: 'create' | 'friends';
  data: LobbyData;
  busy: boolean;
  error?: string;
  onCreate: () => void;
  onClose: () => void;
  onDismissError: () => void;
  /** Pseudo du joueur : propose un nom de table sans l'imposer. */
  username?: string;
};

/** Les trois ambiances proposées. Elles restent locales : le serveur crée la table. */
const MODES: Array<{ key: string; label: string; hint: string }> = [
  { key: 'classique', label: 'Classique', hint: 'Le village au complet.' },
  { key: 'rapide', label: 'Rapide', hint: 'Une veillée courte.' },
  { key: 'privee', label: 'Privée', hint: 'Sur invitation seulement.' },
];

/**
 * CreateTableDialog accueille la création (nom, ambiance, réglages avancés) et
 * le mode « entre amis ».
 *
 * Le nom et l'ambiance ne partent pas au serveur : `POST /api/rooms` ne les
 * accepte pas encore. Ils habillent la préparation sans changer la partie.
 */
export default function CreateTableDialog({
  mode,
  data,
  busy,
  error,
  onCreate,
  onClose,
  onDismissError,
  username,
}: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useFocusTrap(ref, onClose);

  const [name, setName] = useState(username ? `La table de ${username}` : '');
  const [ambience, setAmbience] = useState(data.settings.privateTable ? 'privee' : 'classique');

  const title = mode === 'friends' ? 'Table entre amis' : 'Créer une table';

  const chooseMode = (key: string) => {
    setAmbience(key);
    data.setPrivate(key === 'privee');
  };

  return (
    <div
      className="lobby-modal"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <div
        className="lobby-modal-card lobby-create-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        ref={ref}
      >
        <div className="lobby-modal-head">
          <div>
            <p className="eyebrow">Avant la veillée</p>
            <h2 id={titleId}>{title}</h2>
          </div>
          <button type="button" className="lobby-icon-button" onClick={onClose} aria-label="Fermer">
            <CloseIcon />
          </button>
        </div>

        {mode === 'create' ? (
          <>
            <label className="lobby-field">
              <span className="setting-label">
                <TableIcon />
                Nom de la table
              </span>
              <input
                type="text"
                value={name}
                maxLength={40}
                placeholder="La clairière"
                autoComplete="off"
                onChange={(event) => setName(event.target.value)}
              />
            </label>

            <fieldset className="lobby-fieldset lobby-mode">
              <legend className="setting-label">Ambiance</legend>
              <div className="lobby-mode-grid">
                {MODES.map((item) => (
                  <button
                    key={item.key}
                    type="button"
                    className={
                      ambience === item.key ? 'lobby-mode-card is-active' : 'lobby-mode-card'
                    }
                    aria-pressed={ambience === item.key}
                    onClick={() => chooseMode(item.key)}
                  >
                    <span className="lobby-mode-label">
                      {ambience === item.key && <CheckIcon />}
                      {item.label}
                    </span>
                    <span className="lobby-mode-hint">{item.hint}</span>
                  </button>
                ))}
              </div>
            </fieldset>

            <div className="lobby-advanced">
              <TableSettingsCard
                settings={data.settings}
                busy={busy}
                onCreate={onCreate}
                onPlayersChange={data.setPlayers}
                onRoleToggle={data.toggleRole}
                onNightTimerChange={data.setNightTimer}
                onDebateTimerChange={data.setDebateTimer}
                onPrivateChange={data.setPrivate}
                showSubmit={false}
                collapsible
                defaultOpen={false}
              />
            </div>
          </>
        ) : (
          <div className="lobby-private-summary">
            <span className="lobby-private-icon" aria-hidden="true">
              <LockIcon />
            </span>
            <div>
              <p className="lobby-private-title">Table privée</p>
              <p className="lobby-private-text">
                Seuls les amis invités pourront rejoindre la partie.
              </p>
              {data.settings.privateCode && (
                <p className="lobby-private-code-inline">
                  Code : <code>{data.settings.privateCode}</code>
                </p>
              )}
            </div>
          </div>
        )}

        <section className="lobby-invite" aria-labelledby={`${titleId}-invite`}>
          <div className="lobby-invite-head">
            <h3 id={`${titleId}-invite`}>Inviter des amis</h3>
            <span className="lobby-count">{data.invited.length}</span>
          </div>

          {data.friends.length === 0 ? (
            <p className="friends-empty">Votre liste d’amis est vide.</p>
          ) : (
            <ul className="invite-list">
              {[...data.onlineFriends, ...data.offlineFriends].map((friend) => {
                const selected = data.invited.includes(friend.id);

                return (
                  <li key={friend.id}>
                    <button
                      type="button"
                      className={selected ? 'invite-row is-invited' : 'invite-row'}
                      aria-pressed={selected}
                      onClick={() => data.toggleInvite(friend.id)}
                    >
                      <Avatar index={friend.avatarIndex} />
                      <span className="invite-identity">
                        <span className="invite-name">
                          {friend.status === 'online' && (
                            <span className="live-dot" aria-hidden="true" />
                          )}
                          {friend.username}
                        </span>
                        <span className="invite-status">
                          {friend.status === 'online'
                            ? 'En ligne'
                            : (friend.lastSeen ?? 'Hors ligne')}
                        </span>
                      </span>
                      <span className="invite-action">{selected ? 'Invité ✓' : 'Inviter'}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        {error && (
          <p className="alert lobby-dialog-error" role="alert">
            <span>{error}</span>
            <button type="button" className="link" onClick={onDismissError}>
              fermer
            </button>
          </p>
        )}

        <div className="lobby-modal-actions">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>
            Annuler
          </button>
          <button type="button" className="btn primary" onClick={onCreate} disabled={busy}>
            {mode === 'friends'
              ? `Inviter${data.invited.length > 0 ? ` (${data.invited.length})` : ''} et créer`
              : 'Créer la table'}
          </button>
        </div>
        {busy && <p className="lobby-hint">Un instant…</p>}
      </div>
    </div>
  );
}
