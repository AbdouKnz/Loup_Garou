import { useRef, useState } from 'react';

import { ROOM_CODE_LENGTH } from '../api/types';
import { useLobbyData } from '../hooks/useLobbyData';
import { useMediaQuery } from '../hooks/useMediaQuery';
import { useAccountStore } from '../state/account';
import { useRoomStore } from '../state/room';
import { useToastStore } from '../state/toast';
import ActionCards from '../components/lobby/ActionCards';
import AnnouncementBanner from '../components/lobby/AnnouncementBanner';
import CreateTableDialog from '../components/lobby/CreateTableDialog';
import HomeRail from '../components/lobby/HomeRail';
import LobbyFrame from '../components/lobby/LobbyFrame';
import OpenTablesList from '../components/lobby/OpenTablesList';
import { MoonIcon } from '../components/lobby/icons';
import type { Friend, LobbyAction, OpenTable } from '../components/lobby/types';

/** Repères d'ambiance : aucune présence globale n'est exposée par le serveur. */
const VEILLEE = { joueurs: 21, nuit: '07' };

/** codeDepuisUrl lit /partie/CODE dans l'adresse : c'est le lien de partage. */
function codeDepuisUrl(): string {
  const match = /^\/partie\/([A-Za-z0-9]+)\/?$/.exec(window.location.pathname);

  return match ? match[1].toUpperCase() : '';
}

/**
 * Home est l'accueil pré-partie : la marque, les actions, les tables ouvertes et
 * le panneau d'amis. La salle d'attente d'une partie reste dans Lobby.
 */
export default function Home() {
  const username = useAccountStore((state) => state.me?.username ?? '');
  const linked = useAccountStore((state) => state.me?.linked ?? []);
  const providers = useAccountStore((state) => state.providers);
  const logout = useAccountStore((state) => state.logout);
  const busy = useRoomStore((state) => state.busy);
  const error = useRoomStore((state) => state.error);
  const createRoom = useRoomStore((state) => state.createRoom);
  const joinRoom = useRoomStore((state) => state.joinRoom);
  const dismissError = useRoomStore((state) => state.dismissError);
  const pushToast = useToastStore((state) => state.push);

  const data = useLobbyData();
  const narrow = useMediaQuery('(max-width: 1023px)');
  const mainRef = useRef<HTMLElement>(null);

  const initialCode = codeDepuisUrl();
  const [active, setActive] = useState<LobbyAction>(() => (initialCode ? 'join' : 'create'));
  const [joinOpen, setJoinOpen] = useState(() => Boolean(initialCode));
  const [joinCode, setJoinCode] = useState(initialCode);
  const [joinNotice, setJoinNotice] = useState('');
  const [friendsOpen, setFriendsOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<'create' | 'friends' | null>(null);
  const [section, setSection] = useState<'home' | 'tables' | 'friends'>('home');
  const [joined, setJoined] = useState<string[]>([]);

  const scrollToTop = () => {
    mainRef.current?.scrollTo?.({ top: 0, behavior: 'smooth' });
  };

  const submitCreate = () => {
    setActive('create');
    void createRoom();
  };

  const requestCreate = () => {
    setSection('home');
    setActive('create');
    setFriendsOpen(false);
    setJoinOpen(false);
    setDialogMode('create');
  };

  const requestFriends = () => {
    setSection('friends');
    setActive('friends');
    setFriendsOpen(false);
    setJoinOpen(false);
    data.setPrivate(true);
    setDialogMode('friends');
  };

  const requestJoin = () => {
    setSection('home');
    setActive('join');
    setJoinNotice('');
    setJoinOpen(true);
  };

  const submitJoin = () => {
    if (joinCode.trim().length !== ROOM_CODE_LENGTH) {
      return;
    }

    void joinRoom(joinCode);
  };

  const updateJoinCode = (code: string) => {
    setJoinCode(code);
    setJoinNotice('');
  };

  const joinTable = (table: OpenTable) => {
    setSection('home');
    setActive('join');
    setJoinOpen(true);
    setJoinCode(table.code);
    setJoinNotice(
      `« ${table.name} » est une table de démonstration : son code est prérempli pour l’essai.`,
    );
    setJoined((current) => (current.includes(table.id) ? current : [...current, table.id]));
    pushToast(`Table « ${table.name} » rejointe pour l’essai.`);
    scrollToTop();
  };

  const inviteFriend = (friend: Friend) => {
    data.toggleInvite(friend.id);
    pushToast(
      data.invited.includes(friend.id)
        ? `${friend.username} retiré des invitations.`
        : `Invitation envoyée à ${friend.username}.`,
      data.invited.includes(friend.id) ? 'info' : 'success',
    );
  };

  const goHome = () => {
    const code = codeDepuisUrl();
    setActive(code ? 'join' : 'create');
    setJoinOpen(Boolean(code));
    setJoinNotice('');
    setFriendsOpen(false);
    setDialogMode(null);
    setSection('home');
    scrollToTop();
  };

  const goTables = () => {
    setSection('tables');
    document.getElementById('home-tables')?.scrollIntoView?.({ behavior: 'smooth', block: 'start' });
  };

  const goFriends = () => {
    setSection('friends');

    if (narrow) {
      setFriendsOpen(true);
      return;
    }

    document.getElementById('lobby-friends-search')?.focus();
  };

  const prepareTable = () => {
    setActive('create');
    setFriendsOpen(false);
    setJoinOpen(false);
    setJoinNotice('');
    setDialogMode('create');
  };

  return (
    <>
      <LobbyFrame
        username={username}
        providers={providers}
        linked={linked}
        onlineFriends={data.onlineFriends.length}
        friendsOpen={friendsOpen}
        onLogo={goHome}
        onOpenFriends={() => setFriendsOpen((open) => !open)}
        onCloseFriends={() => setFriendsOpen(false)}
        onLogout={() => void logout()}
        data={data}
        narrow={narrow}
        mainRef={mainRef}
        onInviteFriend={inviteFriend}
        invitedFriends={data.invited}
        rail={
          narrow ? undefined : (
            <HomeRail
              active={section}
              onHome={goHome}
              onTables={goTables}
              onFriends={goFriends}
            />
          )
        }
      >
        <div className="lobby-main-inner lobby-home-inner">
          <section className="home-hero" aria-labelledby="home-title">
            <div className="home-hero-copy">
              <p className="eyebrow">
                <span className="home-eyebrow-rule" aria-hidden="true" />
                Le village avant la nuit
              </p>
              <h1 className="lobby-title" id="home-title">
                Loup-Garou
              </h1>
              <p className="lobby-subtitle">
                Créez votre table, choisissez les rôles et retrouvez vos amis pour une veillée
                mémorable.
              </p>

              <div className="home-hero-stats">
                <span className="home-live">
                  <span className="live-dot" aria-hidden="true" />
                  {VEILLEE.joueurs} joueurs éveillés
                </span>
                <span className="home-night">Nuit {VEILLEE.nuit}</span>
              </div>
            </div>

            <div className="home-hero-visual" aria-hidden="true">
              <span className="home-moon-glow" />
              <span className="home-moon">
                <MoonIcon />
              </span>
              <span className="home-stars">
                <i />
                <i />
                <i />
                <i />
                <i />
                <i />
              </span>
              <span className="home-hero-caption">Le village dort. Les loups veillent.</span>
            </div>
          </section>

          {error && (
            <p className="alert lobby-alert" role="alert">
              <span>{error}</span>
              <button type="button" className="link" onClick={dismissError}>
                fermer
              </button>
            </p>
          )}

          <ActionCards
            active={active}
            busy={busy}
            joinOpen={joinOpen}
            joinCode={joinCode}
            joinNotice={joinNotice}
            onSelect={setActive}
            onCreate={requestCreate}
            onJoin={requestJoin}
            onFriends={requestFriends}
            onJoinCodeChange={updateJoinCode}
            onJoinSubmit={submitJoin}
          />

          <OpenTablesList
            tables={data.openTables}
            busy={busy}
            joined={joined}
            onJoin={joinTable}
          />

          <AnnouncementBanner announcement={data.announcement} onAction={prepareTable} />
        </div>
      </LobbyFrame>

      {dialogMode && (
        <CreateTableDialog
          mode={dialogMode}
          data={data}
          busy={busy}
          error={error ?? undefined}
          username={username}
          onCreate={submitCreate}
          onClose={() => setDialogMode(null)}
          onDismissError={dismissError}
        />
      )}
    </>
  );
}
