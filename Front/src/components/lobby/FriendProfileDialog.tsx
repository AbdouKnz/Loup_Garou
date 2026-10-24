import { useId, useRef } from 'react';

import { useFocusTrap } from '../../hooks/useFocusTrap';
import Avatar from '../game/Avatar';
import { CloseIcon } from './icons';
import type { Friend } from './types';

type Props = {
  friend: Friend;
  onClose: () => void;
};

/** FriendProfileDialog montre la fiche locale d'un ami, sans appel réseau. */
export default function FriendProfileDialog({ friend, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  useFocusTrap(ref, onClose);

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
        className="lobby-modal-card lobby-profile-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        ref={ref}
      >
        <div className="lobby-modal-head">
          <h2 id={titleId}>Profil</h2>
          <button type="button" className="lobby-icon-button" onClick={onClose} aria-label="Fermer">
            <CloseIcon />
          </button>
        </div>

        <div className="lobby-profile-identity">
          <Avatar index={friend.avatarIndex} />
          <div>
            <p className="lobby-profile-name">{friend.username}</p>
            <p className="lobby-profile-status">
              {friend.status === 'online'
                ? 'En ligne'
                : `Dernière connexion ${friend.lastSeen ?? 'récemment'}`}
            </p>
          </div>
        </div>

        <p className="lobby-profile-text">
          Les profils détaillés et les invitations arriveront avec la liste d’amis du serveur.
        </p>

        <button type="button" className="btn lobby-modal-close" onClick={onClose}>
          Fermer
        </button>
      </div>
    </div>
  );
}
