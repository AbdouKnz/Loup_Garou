import { useEffect, useRef } from 'react';

import { ROLE_EMOJI, ROLE_HINTS, ROLE_LABELS } from './labels';
import type { Role } from '../../api/types';

type Props = {
  role: Role;
  /** Compères d'un loup : le serveur ne les donne qu'à eux. */
  wolves?: string[];
  you: string;
  nameOf: (id: string) => string;
  /** Secondes restantes avant que la partie ne commence, à la distribution. */
  secondsLeft?: number | null;
  /**
   * Ferme la carte. Absent pendant la distribution : là, c'est le serveur qui
   * décide quand la partie commence, et la carte disparaît avec la phase.
   */
  onClose?: () => void;
};

/**
 * RoleCard est la carte de rôle : elle s'affiche à la distribution, le temps que
 * chacun découvre ce qu'il est.
 *
 * Elle ne décide de rien. Pendant la distribution, elle ne se ferme pas : elle
 * disparaît quand la phase se termine, et c'est exactement à cet instant que la
 * partie commence. Rouverte plus tard, elle se ferme comme n'importe quel
 * panneau.
 */
export default function RoleCard({ role, wolves, you, nameOf, secondsLeft, onClose }: Props) {
  const dialog = useRef<HTMLDivElement>(null);
  const wolf = role === 'loup-garou';

  useEffect(() => {
    dialog.current?.focus();
  }, []);

  useEffect(() => {
    if (!onClose) {
      return;
    }

    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    window.addEventListener('keydown', onKey);

    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const companions = (wolves ?? []).filter((id) => id !== you);

  return (
    <div
      className="role-card-backdrop"
      onClick={
        onClose
          ? (event) => {
              if (event.target === event.currentTarget) {
                onClose();
              }
            }
          : undefined
      }
    >
      <div
        ref={dialog}
        className={wolf ? 'role-card wolf' : 'role-card'}
        role="dialog"
        aria-modal="true"
        aria-labelledby="role-card-title"
        tabIndex={-1}
      >
        <p className="role-card-eyebrow">🎭 Votre rôle</p>
        <p className="role-card-emoji" aria-hidden="true">
          {ROLE_EMOJI[role]}
        </p>
        <h2 id="role-card-title" className="role-card-name">
          {ROLE_LABELS[role]}
        </h2>
        <p className="role-card-camp">
          {wolf ? 'Vous chassez avec les loups' : 'Vous défendez le village'}
        </p>
        <p className="role-card-hint">{ROLE_HINTS[role]}</p>

        {companions.length > 0 && (
          <p className="role-card-companions">
            {companions.length === 1 ? 'Votre compère : ' : 'Vos compères : '}
            <strong>{companions.map(nameOf).join(', ')}</strong>
          </p>
        )}

        {secondsLeft !== null && secondsLeft !== undefined && (
          <p className="role-card-count">
            La partie commence dans <strong>{secondsLeft} s</strong>
          </p>
        )}

        {onClose && (
          <button type="button" className="btn" onClick={onClose}>
            Fermer
          </button>
        )}
      </div>
    </div>
  );
}
