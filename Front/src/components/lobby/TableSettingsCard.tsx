import { useId, useState } from 'react';

import {
  MAX_DEBATE_TIMER,
  MAX_LOBBY_PLAYERS,
  MAX_NIGHT_TIMER,
  MIN_DEBATE_TIMER,
  MIN_LOBBY_PLAYERS,
  MIN_NIGHT_TIMER,
} from './constants';
import { LockIcon } from '../icons';
import { ChevronIcon, MinusIcon, PlusIcon, SettingsIcon } from './icons';
import type { LobbyRole, LobbySettings } from './types';

type Props = {
  settings: LobbySettings;
  busy: boolean;
  onCreate: () => void;
  onPlayersChange: (players: number) => void;
  onRoleToggle: (role: LobbyRole) => void;
  onNightTimerChange: (seconds: number) => void;
  onDebateTimerChange: (seconds: number) => void;
  onPrivateChange: (privateTable: boolean) => void;
  /** Le dialogue mobile porte déjà son propre bouton de validation. */
  showSubmit?: boolean;
  collapsible?: boolean;
  /** Les réglages avancés s'ouvrent à la demande depuis la création. */
  defaultOpen?: boolean;
};

const ROLES: Array<{ key: LobbyRole; label: string }> = [
  { key: 'voyante', label: 'Voyante' },
  { key: 'sorciere', label: 'Sorcière' },
  { key: 'chasseur', label: 'Chasseur' },
  { key: 'cupidon', label: 'Cupidon' },
];

/**
 * TimerField laisse saisir une durée sans la contraindre à chaque frappe.
 *
 * La valeur n'est validée qu'à la sortie du champ ou sur Entrée : on peut donc
 * taper « 120 » sans voir le champ revenir à sa borne basse après le « 1 ».
 */
function TimerField({
  label,
  value,
  min,
  max,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (seconds: number) => void;
}) {
  const [draft, setDraft] = useState<string | null>(null);
  const shown = draft ?? String(value);

  const commit = () => {
    if (draft === null) {
      return;
    }

    if (draft.trim() === '') {
      setDraft(null);
      return;
    }

    const parsed = Number(draft);

    if (!Number.isFinite(parsed)) {
      setDraft(null);
      return;
    }

    onChange(Math.min(max, Math.max(min, parsed)));
    setDraft(null);
  };

  return (
    <label className="lobby-timer">
      <span className="setting-label">{label}</span>
      <span className="lobby-timer-input">
        <input
          type="number"
          min={min}
          max={max}
          step={5}
          value={shown}
          aria-label={`${label} en secondes`}
          onChange={(event) => setDraft(event.target.value)}
          onBlur={commit}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.currentTarget.blur();
            }
          }}
        />
        <span aria-hidden="true">s</span>
      </span>
    </label>
  );
}

/**
 * TableSettingsCard regroupe les réglages locaux d'une future table.
 *
 * Les valeurs sont encore simulées : elles préparent l'interface sans modifier
 * la partie réelle, créée par le même `createRoom()` que l'ancien accueil.
 */
export default function TableSettingsCard({
  settings,
  busy,
  onCreate,
  onPlayersChange,
  onRoleToggle,
  onNightTimerChange,
  onDebateTimerChange,
  onPrivateChange,
  showSubmit = true,
  collapsible = true,
  defaultOpen = true,
}: Props) {
  const [open, setOpen] = useState(defaultOpen);
  const headingId = useId();
  const bodyId = useId();
  const visible = collapsible ? open : true;

  return (
    <section className="table-settings" aria-labelledby={headingId}>
      {collapsible ? (
        <button
          type="button"
          className="table-settings-head"
          aria-expanded={open}
          aria-controls={bodyId}
          onClick={() => setOpen((current) => !current)}
        >
          <span className="table-settings-heading" id={headingId}>
            <SettingsIcon />
            Réglages de la table
          </span>
          <ChevronIcon className={open ? 'settings-chevron open' : 'settings-chevron'} />
        </button>
      ) : (
        <div className="table-settings-head is-static">
          <span className="table-settings-heading" id={headingId}>
            <SettingsIcon />
            Réglages de la table
          </span>
        </div>
      )}

      {visible && (
        <div className="table-settings-body" id={bodyId}>
          <div className="setting-players">
            <span className="setting-label" id={`${bodyId}-players`}>
              Joueurs
            </span>
            <div className="lobby-stepper" role="group" aria-labelledby={`${bodyId}-players`}>
              <button
                type="button"
                aria-label="Retirer un joueur"
                disabled={settings.players <= MIN_LOBBY_PLAYERS}
                onClick={() => onPlayersChange(settings.players - 1)}
              >
                <MinusIcon />
              </button>
              <output aria-live="polite">{settings.players}</output>
              <button
                type="button"
                aria-label="Ajouter un joueur"
                disabled={settings.players >= MAX_LOBBY_PLAYERS}
                onClick={() => onPlayersChange(settings.players + 1)}
              >
                <PlusIcon />
              </button>
            </div>
          </div>

          <fieldset className="lobby-fieldset">
            <legend className="setting-label">Rôles</legend>
            <div className="role-chips">
              {ROLES.map((role) => (
                <button
                  key={role.key}
                  type="button"
                  className="role-chip"
                  aria-pressed={settings.roles[role.key]}
                  onClick={() => onRoleToggle(role.key)}
                >
                  {role.label}
                </button>
              ))}
            </div>
          </fieldset>

          <div className="settings-timers">
            <TimerField
              label="Nuit"
              value={settings.nightTimer}
              min={MIN_NIGHT_TIMER}
              max={MAX_NIGHT_TIMER}
              onChange={onNightTimerChange}
            />
            <TimerField
              label="Débat"
              value={settings.debateTimer}
              min={MIN_DEBATE_TIMER}
              max={MAX_DEBATE_TIMER}
              onChange={onDebateTimerChange}
            />
          </div>

          <button
            type="button"
            role="switch"
            className="lobby-switch"
            aria-checked={settings.privateTable}
            onClick={() => onPrivateChange(!settings.privateTable)}
          >
            <span className="lobby-switch-copy">
              <span className="setting-label">
                <LockIcon />
                Table privée
              </span>
              <span className="lobby-switch-hint">Seuls les joueurs invités peuvent entrer.</span>
            </span>
            <span className="lobby-switch-track" aria-hidden="true">
              <span />
            </span>
          </button>

          {settings.privateTable && settings.privateCode && (
            <div className="lobby-private-code">
              <span>Code d’invitation</span>
              <code>{settings.privateCode}</code>
            </div>
          )}

          {showSubmit && (
            <>
              <button
                type="button"
                className="btn primary lobby-create"
                disabled={busy}
                onClick={onCreate}
              >
                Créer une table
              </button>
              {busy && <p className="lobby-hint">Un instant…</p>}
            </>
          )}
        </div>
      )}
    </section>
  );
}
