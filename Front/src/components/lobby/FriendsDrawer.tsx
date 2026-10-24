import { useRef } from 'react';

import { useFocusTrap } from '../../hooks/useFocusTrap';
import type { LobbyData } from '../../hooks/useLobbyData';
import FriendsSidebar from './FriendsSidebar';
import { CloseIcon } from './icons';
import type { Friend } from './types';

type Props = {
  data: LobbyData;
  onClose: () => void;
  onInvite?: (friend: Friend) => void;
  invited?: string[];
};

/** FriendsDrawer donne accès aux amis sous 1024 px. */
export default function FriendsDrawer({ data, onClose, onInvite, invited }: Props) {
  const ref = useRef<HTMLElement>(null);
  useFocusTrap(ref, onClose);

  return (
    <div
      className="lobby-drawer-backdrop"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <aside
        className="friends-drawer"
        id="lobby-friends-drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="friends-drawer-title"
        ref={ref}
      >
        <div className="friends-drawer-head">
          <h2 id="friends-drawer-title">Amis</h2>
          <button type="button" className="lobby-icon-button" onClick={onClose} aria-label="Fermer">
            <CloseIcon />
          </button>
        </div>
        <FriendsSidebar data={data} hideTitle onInvite={onInvite} invited={invited} />
      </aside>
    </div>
  );
}
