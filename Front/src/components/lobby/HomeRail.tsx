import type { ReactNode } from 'react';

import { WolfIcon } from '../WolfIcon';
import { HomeIcon, MoonIcon, TableIcon, UsersIcon } from './icons';

type Props = {
  active: 'home' | 'tables' | 'friends';
  onHome: () => void;
  onTables: () => void;
  onFriends: () => void;
};

/**
 * HomeRail est le rail fixe de l'accueil : la marque, trois repères et la lune.
 *
 * Chaque repère mène à une zone réelle de la page. Le rail ne crée aucune
 * action : il déplace le regard et le clavier, jamais une partie.
 */
export default function HomeRail({ active, onHome, onTables, onFriends }: Props) {
  const items: Array<{
    key: Props['active'];
    label: string;
    icon: ReactNode;
    onClick: () => void;
  }> = [
    { key: 'home', label: 'Accueil', icon: <HomeIcon />, onClick: onHome },
    { key: 'tables', label: 'Tables ouvertes', icon: <TableIcon />, onClick: onTables },
    { key: 'friends', label: 'Amis', icon: <UsersIcon />, onClick: onFriends },
  ];

  return (
    <nav className="lobby-rail" aria-label="Navigation de l’accueil">
      <span className="lobby-rail-mark" aria-hidden="true">
        <WolfIcon />
      </span>

      <ul className="lobby-rail-list">
        {items.map((item) => (
          <li key={item.key}>
            <button
              type="button"
              className={item.key === active ? 'lobby-rail-link is-active' : 'lobby-rail-link'}
              aria-label={item.label}
              aria-current={item.key === active ? 'page' : undefined}
              title={item.label}
              onClick={item.onClick}
            >
              {item.icon}
            </button>
          </li>
        ))}
      </ul>

      <span className="lobby-rail-moon" aria-hidden="true">
        <MoonIcon />
      </span>
    </nav>
  );
}
