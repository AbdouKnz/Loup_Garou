import type { CSSProperties } from 'react';

import AmbienceToggle from '../AmbienceToggle';
import ConnectionBadge from '../ConnectionBadge';
import SoundToggle from '../SoundToggle';
import VoiceToggle from '../VoiceToggle';
import { PHASES } from './labels';
import { momentForPhase } from './moments';
import type { ConnectionStatus } from '../../state/room';
import type { Phase } from '../../api/types';

type Props = {
  code: string;
  phase: Phase;
  day: number;
  over: boolean;
  dead: boolean;
  /** Secondes restantes avant l'échéance, ou null quand il n'y en a pas. */
  secondsLeft: number | null;
  /** Durée totale de la phase, pour dessiner l'anneau de progression. */
  phaseTotal: number;
  runoff: boolean;
  connection: ConnectionStatus;
  onLeave: () => void;
  /** Ouvre l'aide de jeu ; absent pendant la distribution des rôles. */
  onRules?: () => void;
};

/**
 * PhaseHeader annonce la phase en cours : titre, décompte et état du réseau.
 *
 * L'anneau se remplit à mesure que la phase s'écoule, et passe à l'orange sous
 * dix secondes pour que la fin d'un vote ne surprenne personne.
 */
export default function PhaseHeader({
  code,
  phase,
  day,
  over,
  dead,
  secondsLeft,
  phaseTotal,
  runoff,
  connection,
  onLeave,
  onRules,
}: Props) {
  const label = PHASES[phase];
  // La distribution n'est ni un jour ni une nuit : le titre de la phase suffit.
  const timing = phase === 'roles' ? null : phase === 'nuit' ? `Nuit ${day}` : `Jour ${day}`;
  const moment = momentForPhase(phase);
  const progress =
    secondsLeft !== null && phaseTotal > 0 ? Math.min(1, secondsLeft / phaseTotal) : 0;
  const urgent = secondsLeft !== null && secondsLeft <= 10 && !over;

  return (
    <header className="lobby-header">
      <div>
        <p className="eyebrow">Partie {code}</p>
        <h1>
          {label.emoji} {label.title} {!over && timing && <span className="count">· {timing}</span>}
          {runoff && <span className="badge tie">second tour</span>}
        </h1>
        <p className="subtitle">{dead && !over ? 'Vous êtes mort : vous regardez.' : label.hint}</p>
      </div>

      <div className="lobby-tools">
        <ConnectionBadge status={connection} />
        {!over && moment && (
          <span
            className={`moment-badge ${moment}`}
            aria-label={moment === 'night' ? 'Il fait nuit' : 'Il fait jour'}
          >
            <span aria-hidden="true">{moment === 'night' ? '🌙' : '☀️'}</span>
            {moment === 'night' ? 'Nuit' : 'Jour'}
          </span>
        )}
        <SoundToggle />
        <VoiceToggle />
        <AmbienceToggle />
        {onRules && (
          <button
            type="button"
            className="btn sound-toggle rules-toggle"
            onClick={onRules}
            title="Règles du jeu"
            aria-label="Règles du jeu"
          >
            <span aria-hidden="true">?</span>
          </button>
        )}
        {secondsLeft !== null && !over && (
          <span
            className={urgent ? 'timer-ring urgent' : 'timer-ring'}
            style={{ '--progress': `${Math.round(progress * 100)}%` } as CSSProperties}
            role="timer"
            aria-label={`Temps restant : ${secondsLeft} secondes`}
          >
            <span>{secondsLeft}</span>
          </span>
        )}
        {over && (
          <button type="button" className="btn danger" onClick={onLeave}>
            Quitter
          </button>
        )}
      </div>
    </header>
  );
}
