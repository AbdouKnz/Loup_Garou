import { ROLE_EMOJI, ROLE_LABELS } from './labels';
import type { Role } from '../../api/types';

type Props = {
  /** Rôles annoncés par le serveur, dans l'ordre où ils seront distribués. */
  roles: Role[];
};

/**
 * Composition annonce la répartition des rôles, avant même qu'elle soit
 * distribuée : elle ne dépend que du nombre de joueurs, et la table la connaît
 * d'avance. Qui portera quoi ne se décide qu'au lancement.
 */
export default function Composition({ roles }: Props) {
  const counts = new Map<Role, number>();
  for (const role of roles) {
    counts.set(role, (counts.get(role) ?? 0) + 1);
  }

  return (
    <ul className="composition">
      {[...counts.entries()].map(([role, count]) => (
        <li
          key={role}
          className={role === 'loup-garou' ? 'composition-entry wolf' : 'composition-entry'}
        >
          <span className="composition-count">{count}</span>
          <span aria-hidden="true">{ROLE_EMOJI[role]}</span>
          <span className="composition-label">{ROLE_LABELS[role]}</span>
        </li>
      ))}
    </ul>
  );
}
