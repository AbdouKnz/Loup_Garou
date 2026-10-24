/** Types de l'accueil pré-partie : amis, tables ouvertes et réglages. */

export type FriendStatus = 'online' | 'offline';

export type Friend = {
  id: string;
  username: string;
  avatarIndex: number;
  status: FriendStatus;
  /** Dernière connexion, en clair. Absent pour un ami en ligne. */
  lastSeen?: string;
};

export type OpenTableStatus = 'waiting' | 'in_progress';

export type OpenTable = {
  id: string;
  code: string;
  name: string;
  host: string;
  players: number;
  maxPlayers: number;
  status: OpenTableStatus;
};

export type LobbyRole = 'voyante' | 'sorciere' | 'chasseur' | 'cupidon';

export type LobbySettings = {
  players: number;
  roles: Record<LobbyRole, boolean>;
  /** Durée de la nuit, en secondes. */
  nightTimer: number;
  /** Temps de parole par joueur pendant le débat, en secondes. */
  debateTimer: number;
  privateTable: boolean;
  /** Aperçu local du code d'invitation, tant que la table n'existe pas. */
  privateCode?: string;
};

export type LobbyAnnouncement = {
  id: string;
  title: string;
  text: string;
  actionLabel: string;
};

export type LobbyAction = 'create' | 'join' | 'friends';
