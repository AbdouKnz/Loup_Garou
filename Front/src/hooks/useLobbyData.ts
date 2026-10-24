import { useMemo, useState } from 'react';

import type {
  Friend,
  LobbyAnnouncement,
  LobbyRole,
  LobbySettings,
  OpenTable,
} from '../components/lobby/types';
import {
  MAX_DEBATE_TIMER,
  MAX_LOBBY_PLAYERS,
  MAX_NIGHT_TIMER,
  MIN_DEBATE_TIMER,
  MIN_LOBBY_PLAYERS,
  MIN_NIGHT_TIMER,
} from '../components/lobby/constants';

/** Amis de démonstration, en attendant la vraie liste du serveur. */
const MOCK_FRIENDS: Friend[] = [
  { id: 'f-camille', username: 'Camille', avatarIndex: 2, status: 'online' },
  { id: 'f-nassim', username: 'Nassim', avatarIndex: 4, status: 'online' },
  { id: 'f-lou', username: 'Lou', avatarIndex: 7, status: 'online' },
  { id: 'f-ines', username: 'Inès', avatarIndex: 10, status: 'offline', lastSeen: 'il y a 2 h' },
  { id: 'f-marc', username: 'Marc', avatarIndex: 11, status: 'offline', lastSeen: 'hier' },
  { id: 'f-sacha', username: 'Sacha', avatarIndex: 1, status: 'offline', lastSeen: 'il y a 3 j' },
];

/** Annuaire local pour la recherche : ces joueurs ne sont pas encore amis. */
const MOCK_DIRECTORY: Friend[] = [
  { id: 'd-jade', username: 'Jade', avatarIndex: 3, status: 'online' },
  { id: 'd-elias', username: 'Élias', avatarIndex: 5, status: 'online' },
  { id: 'd-noa', username: 'Noa', avatarIndex: 8, status: 'offline', lastSeen: 'il y a 1 h' },
];

/** Tables ouvertes de démonstration ; les identifiants restent locaux. */
const MOCK_TABLES: OpenTable[] = [
  {
    id: 't-clairiere',
    code: 'BRLUNE',
    name: 'La clairière',
    host: 'Camille',
    players: 4,
    maxPlayers: 12,
    status: 'waiting',
  },
  {
    id: 't-loups',
    code: 'NUIT89',
    name: 'Les loups veillent',
    host: 'Nassim',
    players: 9,
    maxPlayers: 12,
    status: 'in_progress',
  },
  {
    id: 't-veillee',
    code: 'VEILLE',
    name: 'La longue veillée',
    host: 'Lou',
    players: 6,
    maxPlayers: 16,
    status: 'waiting',
  },
  {
    id: 't-menhir',
    code: 'MENHIR',
    name: 'Le menhir gris',
    host: 'Sacha',
    players: 11,
    maxPlayers: 12,
    status: 'in_progress',
  },
];

const DEFAULT_SETTINGS: LobbySettings = {
  players: 8,
  roles: { voyante: true, sorciere: true, chasseur: false, cupidon: false },
  nightTimer: 90,
  debateTimer: 60,
  privateTable: false,
};

const MOCK_ANNOUNCEMENT: LobbyAnnouncement = {
  id: 'welcome',
  title: 'Le village vous attend',
  text: 'Créez une table, choisissez vos rôles et invitez vos amis pour commencer la veillée.',
  actionLabel: 'Préparer une table',
};

const CODE_ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';

/** newPrivateCode donne un aperçu local du code qui sera demandé au serveur. */
function newPrivateCode(): string {
  let code = '';

  for (let index = 0; index < 6; index += 1) {
    code += CODE_ALPHABET[Math.floor(Math.random() * CODE_ALPHABET.length)];
  }

  return code;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export type LobbyData = ReturnType<typeof useLobbyData>;

/**
 * useLobbyData rassemble les données de démonstration de l'accueil.
 *
 * Tout ce qui devra venir du serveur (amis, tables, réglages) est regroupé ici :
 * remplacer les constantes par des appels API ne touchera pas aux composants.
 */
export function useLobbyData() {
  const [friends, setFriends] = useState<Friend[]>(MOCK_FRIENDS);
  const [search, setSearch] = useState('');
  const [searched, setSearched] = useState(false);
  const [searchResults, setSearchResults] = useState<Friend[]>([]);
  const [settings, setSettings] = useState<LobbySettings>(DEFAULT_SETTINGS);
  const [invited, setInvited] = useState<string[]>([]);

  const onlineFriends = useMemo(
    () => friends.filter((friend) => friend.status === 'online'),
    [friends],
  );
  const offlineFriends = useMemo(
    () => friends.filter((friend) => friend.status === 'offline'),
    [friends],
  );

  const runSearch = () => {
    const query = search.trim().toLocaleLowerCase('fr');

    if (!query) {
      setSearched(false);
      setSearchResults([]);
      return;
    }

    const alreadyFriends = new Set(friends.map((friend) => friend.id));
    setSearched(true);
    setSearchResults(
      MOCK_DIRECTORY.filter(
        (candidate) =>
          !alreadyFriends.has(candidate.id) &&
          candidate.username.toLocaleLowerCase('fr').includes(query),
      ),
    );
  };

  const addFriend = (friend: Friend) => {
    setFriends((current) =>
      current.some((item) => item.id === friend.id) ? current : [...current, friend],
    );
    setSearchResults((current) => current.filter((item) => item.id !== friend.id));
    setSearch('');
    setSearched(false);
  };

  const removeFriend = (id: string) => {
    setFriends((current) => current.filter((friend) => friend.id !== id));
    setInvited((current) => current.filter((friendID) => friendID !== id));
  };

  const setPlayers = (players: number) => {
    setSettings((current) => ({
      ...current,
      players: clamp(players, MIN_LOBBY_PLAYERS, MAX_LOBBY_PLAYERS),
    }));
  };

  const toggleRole = (role: LobbyRole) => {
    setSettings((current) => ({
      ...current,
      roles: { ...current.roles, [role]: !current.roles[role] },
    }));
  };

  const setNightTimer = (seconds: number) => {
    setSettings((current) => ({
      ...current,
      nightTimer: clamp(seconds, MIN_NIGHT_TIMER, MAX_NIGHT_TIMER),
    }));
  };

  const setDebateTimer = (seconds: number) => {
    setSettings((current) => ({
      ...current,
      debateTimer: clamp(seconds, MIN_DEBATE_TIMER, MAX_DEBATE_TIMER),
    }));
  };

  const setPrivate = (privateTable: boolean) => {
    setSettings((current) => ({
      ...current,
      privateTable,
      privateCode: privateTable ? (current.privateCode ?? newPrivateCode()) : undefined,
    }));
  };

  const togglePrivate = () => setPrivate(!settings.privateTable);

  const toggleInvite = (id: string) => {
    setInvited((current) =>
      current.includes(id) ? current.filter((friendID) => friendID !== id) : [...current, id],
    );
  };

  return {
    friends,
    onlineFriends,
    offlineFriends,
    search,
    searched,
    searchResults,
    setSearch,
    runSearch,
    addFriend,
    removeFriend,
    settings,
    setPlayers,
    toggleRole,
    setNightTimer,
    setDebateTimer,
    setPrivate,
    togglePrivate,
    invited,
    toggleInvite,
    openTables: MOCK_TABLES,
    announcement: MOCK_ANNOUNCEMENT,
  };
}
