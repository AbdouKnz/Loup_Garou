import type { ReactNode, RefObject } from 'react';

import type { Providers } from '../../api/types';
import type { LobbyData } from '../../hooks/useLobbyData';
import type { Friend } from './types';
import ToastViewport from '../ToastViewport';
import FriendsDrawer from './FriendsDrawer';
import FriendsSidebar from './FriendsSidebar';
import TopBar from './TopBar';

type Props = {
  username: string;
  providers: Providers;
  linked: string[];
  onlineFriends: number;
  friendsOpen: boolean;
  onOpenFriends: () => void;
  onCloseFriends: () => void;
  onLogout: () => void;
  /** Absent en salle d'attente : la marque reste alors un simple repère. */
  onLogo?: () => void;
  data: LobbyData;
  narrow: boolean;
  mainRef?: RefObject<HTMLElement | null>;
  mainClassName?: string;
  /** Rail fixe de l'accueil ; la salle d'attente n'en a pas. */
  rail?: ReactNode;
  /** Invitation directe depuis le panneau d'amis (accueil seulement). */
  onInviteFriend?: (friend: Friend) => void;
  invitedFriends?: string[];
  children: ReactNode;
};

/**
 * LobbyFrame est l'ossature commune de l'accueil et de la salle d'attente :
 * en-tête, panneau d'amis et zone principale. Elle garantit que les deux
 * écrans gardent exactement la même navigation.
 */
export default function LobbyFrame({
  username,
  providers,
  linked,
  onlineFriends,
  friendsOpen,
  onOpenFriends,
  onCloseFriends,
  onLogout,
  onLogo,
  data,
  narrow,
  mainRef,
  mainClassName,
  rail,
  onInviteFriend,
  invitedFriends,
  children,
}: Props) {
  const classes = ['lobby-shell'];

  if (rail) {
    classes.push('lobby-shell-rail');
  }

  return (
    <div className={classes.join(' ')}>
      <TopBar
        username={username}
        providers={providers}
        linked={linked}
        onlineFriends={onlineFriends}
        friendsOpen={friendsOpen}
        onLogo={onLogo}
        onOpenFriends={onOpenFriends}
        onLogout={onLogout}
      />

      {rail}

      {!narrow && (
        <div className="lobby-sidebar">
          <FriendsSidebar
            data={data}
            onInvite={onInviteFriend}
            invited={invitedFriends}
          />
        </div>
      )}

      <main className={mainClassName ? `lobby-main ${mainClassName}` : 'lobby-main'} ref={mainRef}>
        {children}
      </main>

      {narrow && friendsOpen && (
        <FriendsDrawer
          data={data}
          onClose={onCloseFriends}
          onInvite={onInviteFriend}
          invited={invitedFriends}
        />
      )}

      <ToastViewport />
    </div>
  );
}
