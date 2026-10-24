import { ROOM_CODE_LENGTH } from '../../api/types';
import { CheckIcon, CodeIcon, TableIcon, UsersIcon } from './icons';
import type { LobbyAction } from './types';

type Props = {
  active: LobbyAction;
  busy: boolean;
  joinOpen: boolean;
  joinCode: string;
  joinNotice?: string;
  onSelect: (action: LobbyAction) => void;
  onCreate: () => void;
  onJoin: () => void;
  onFriends: () => void;
  onJoinCodeChange: (code: string) => void;
  onJoinSubmit: () => void;
};

/**
 * ActionCards propose les trois entrées de l'accueil : créer, rejoindre, inviter.
 *
 * La création et l'invitation ouvrent leur dialogue ; le code se saisit ici,
 * dans la carte, et n'est envoyé qu'une fois complet. La sélection ne déclenche
 * jamais une action à la place du joueur.
 */
export default function ActionCards({
  active,
  busy,
  joinOpen,
  joinCode,
  joinNotice,
  onSelect,
  onCreate,
  onJoin,
  onFriends,
  onJoinCodeChange,
  onJoinSubmit,
}: Props) {
  const codeOk = joinCode.trim().length === ROOM_CODE_LENGTH;

  return (
    <div className="lobby-actions">
      <div className="action-grid">
        <article
          className={
            active === 'create' ? 'action-card is-create is-active' : 'action-card is-create'
          }
        >
          <span className="action-index" aria-hidden="true">
            01
          </span>
          {active === 'create' && (
            <span className="action-check" aria-hidden="true">
              <CheckIcon />
            </span>
          )}
          <span className="action-icon" aria-hidden="true">
            <TableIcon />
          </span>
          <h2 className="action-title">Créer une table</h2>
          <p className="action-text">Choisissez vos rôles et lancez la veillée.</p>
          <button
            type="button"
            className="btn primary action-button"
            disabled={busy}
            onClick={() => {
              onSelect('create');
              onCreate();
            }}
          >
            Créer une table
          </button>
        </article>

        <article
          className={active === 'join' ? 'action-card is-join is-active' : 'action-card is-join'}
        >
          <span className="action-index" aria-hidden="true">
            02
          </span>
          {active === 'join' && (
            <span className="action-check" aria-hidden="true">
              <CheckIcon />
            </span>
          )}
          <span className="action-icon" aria-hidden="true">
            <CodeIcon />
          </span>
          <h2 className="action-title">Rejoindre</h2>
          <p className="action-text">Entrez le code partagé par un ami.</p>
          <button
            type="button"
            className="btn action-button action-button-quiet"
            aria-expanded={joinOpen}
            aria-controls={joinOpen ? 'lobby-join-form' : undefined}
            disabled={busy}
            onClick={() => {
              onSelect('join');
              onJoin();
            }}
          >
            J’ai un code
          </button>
        </article>

        <article
          className={active === 'friends' ? 'action-card is-invite is-active' : 'action-card is-invite'}
        >
          <span className="action-index" aria-hidden="true">
            03
          </span>
          {active === 'friends' && (
            <span className="action-check" aria-hidden="true">
              <CheckIcon />
            </span>
          )}
          <span className="action-icon" aria-hidden="true">
            <UsersIcon />
          </span>
          <h2 className="action-title">Entre amis</h2>
          <p className="action-text">Invitez votre groupe dans une table privée.</p>
          <button
            type="button"
            className="btn action-button action-button-invite"
            disabled={busy}
            onClick={() => {
              onSelect('friends');
              onFriends();
            }}
          >
            Inviter des amis
          </button>
        </article>
      </div>

      {joinOpen && (
        <form
          className="join-form"
          id="lobby-join-form"
          onSubmit={(event) => {
            event.preventDefault();
            onJoinSubmit();
          }}
        >
          <label className="join-field">
            <span className="setting-label">Code de la table</span>
            <input
              type="text"
              className="code-input"
              value={joinCode}
              maxLength={ROOM_CODE_LENGTH}
              placeholder="ABC234"
              autoComplete="off"
              autoFocus
              aria-describedby="lobby-join-hint"
              onChange={(event) => onJoinCodeChange(event.target.value.toUpperCase())}
            />
          </label>
          <button type="submit" className="btn primary" disabled={!codeOk || busy}>
            Rejoindre la table
          </button>
          <p className="lobby-hint join-hint" id="lobby-join-hint" aria-live="polite">
            {codeOk
              ? 'Code complet : la table peut être rejointe.'
              : `${joinCode.length}/${ROOM_CODE_LENGTH} caractères.`}
          </p>
        </form>
      )}

      {joinNotice && <p className="lobby-hint join-notice">{joinNotice}</p>}
    </div>
  );
}
