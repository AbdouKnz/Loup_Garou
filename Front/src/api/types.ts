/**
 * Types du protocole, miroir de `Back/internal/protocol`.
 *
 * Le serveur est la seule autorité : le client n'envoie que des intentions et
 * reçoit une vue personnalisée de l'état de la partie.
 */

/**
 * Phase d'une partie : la salle d'attente, la distribution des rôles, puis le
 * cycle nuit/jour, et enfin l'écran de fin. Les valeurs sont celles du serveur.
 */
export type Phase = 'lobby' | 'roles' | 'nuit' | 'aube' | 'debat' | 'vote' | 'verdict' | 'fin';

/** Rôle distribué par le serveur. */
export type Role = 'villageois' | 'loup-garou' | 'voyante' | 'sorciere';

/**
 * Ce que la veillée en cours attend du joueur connecté : « devorer », « voir »
 * ou « potions ». Absent, il n'a rien à y faire.
 */
export type NightAction = 'devorer' | 'voir' | 'potions';

/** Ce que le joueur répond à une veillée : voir, soigner, empoisonner, passer. */
export type NightAnswer = 'voir' | 'soigner' | 'empoisonner' | 'passer';

/** Cause d'une mort de la nuit. */
export type DeathCause = 'loups' | 'poison';

/** Une observation de la voyante : ce qu'elle a vu, et quand. */
export type VisionView = {
  day: number;
  target: string;
  role: Role;
};

/** Ce qu'il reste à la sorcière : chaque potion ne sert qu'une fois. */
export type PotionsView = {
  heal: boolean;
  poison: boolean;
};

/** Un mort de la nuit et sa cause, jamais son rôle. */
export type DeathView = {
  target: string;
  cause: DeathCause;
};

/** Camp vainqueur, annoncé à la fin seulement. */
export type Winner = 'village' | 'loups';

/** Vue publique d'un joueur. */
export type PlayerView = {
  id: string;
  pseudo: string;
  ready: boolean;
  online: boolean;
  host: boolean;
  /** Mort (dévoré ou éliminé) : il ne joue plus, il regarde. */
  dead?: boolean;
  /** A déjà voté au tour courant. */
  voted?: boolean;
  /** Joueur automatique ajouté par l'hôte. */
  bot?: boolean;
};

/**
 * Vue d'une partie, calculée par le serveur pour le joueur connecté.
 *
 * Les champs privés (yourRole, wolves, wolfVotes, roles) ne sont renseignés que
 * pour qui a le droit de les voir : le client ne les déduit jamais lui-même.
 */
export type GameView = {
  phase: Phase;
  day: number;
  round: number;
  deadline?: string;
  secondsLeft: number;
  winner?: Winner;
  yourRole?: Role;
  yourVote?: string;
  alive: string[];
  dead: string[];
  /** Votants du tour courant : absent quand personne n'a encore voté. */
  voted?: string[];
  /**
   * Scrutin du jour, public : votant → cible (vide = abstention).
   * Présent pendant le vote du village et le verdict qui suit, jamais la nuit.
   */
  ballot?: Record<string, string>;
  wolves?: string[];
  wolfVotes?: Record<string, string>;
  /** Veillée en cours, réservée à son acteur. */
  yourAction?: NightAction;
  /** Carnet de la voyante, réservé à elle. */
  yourVision?: VisionView[];
  /** Potions restantes de la sorcière, réservées à elle. */
  yourPotions?: PotionsView;
  /** Proie des loups, révélée à la sorcière pendant sa veillée. */
  wolfTarget?: string;
  /** Morts de la nuit, annoncées à l'aube, sans leur rôle. */
  deaths?: DeathView[];
  eliminated?: string;
  eliminatedRole?: Role;
  tie?: boolean;
  candidates?: string[];
  /**
   * Tour de parole du débat : qui parle à cet instant, l'ordre des vivants, et
   * ceux qui ont déjà parlé. Public, sans rapport avec les rôles.
   */
  speaker?: string;
  speakers?: string[];
  spoken?: string[];
  roles?: Record<string, Role>;
};

/** Nature d'un fait de jeu annoncé par le narrateur. */
export type GameEventKind =
  | 'phase'
  | 'parole'
  | 'egalite'
  | 'victime'
  | 'sauve'
  | 'aucune-victime'
  | 'elimine'
  | 'aucune-elimination'
  | 'fin';

/** Fait de jeu, tel que le narrateur le raconte. */
export type GameEvent = {
  kind: GameEventKind;
  text: string;
  phase: Phase;
  day: number;
  round: number;
  target?: string;
  /** Cause d'une mort de la nuit. */
  cause?: DeathCause;
  role?: Role;
  winners?: Winner;
};

/** Une ligne du fil de discussion d'une partie. */
export type ChatMessage = {
  id: string;
  playerId?: string;
  pseudo: string;
  text: string;
  at: string;
  system?: boolean;
};

/** Instantané complet d'une partie, tel que le joueur connecté le voit. */
export type RoomState = {
  code: string;
  phase: Phase;
  hostId: string;
  you: string;
  maxPlayers: number;
  canStart: boolean;
  players: PlayerView[];
  messages: ChatMessage[];
  /**
   * Répartition des rôles de la table, telle qu'elle sera distribuée : annoncée
   * dès la salle d'attente, elle ne dépend que du nombre de joueurs.
   */
  composition: Role[];
  /** Présente dès que la partie a commencé, jusqu'à la revanche. */
  game?: GameView;
};

/** Résumé d'une partie, consultable sans être connecté. */
export type RoomSummary = {
  code: string;
  phase: Phase;
  players: number;
  maxPlayers: number;
};

/** Identité d'un joueur dans une partie : à conserver pour se reconnecter. */
export type Session = {
  code: string;
  playerId: string;
  token: string;
  pseudo: string;
};

/**
 * Identité du compte connecté, telle que le serveur la connaît (GET
 * /api/auth/me).
 *
 * Le pseudo est choisi à la création du compte et vaut pour toutes les parties
 * et tous les onglets.
 */
export type Account = {
  authenticated: boolean;
  /** Pseudo du compte, absent si personne n'est connecté. */
  username?: string;
  /** Fournisseurs externes utilisables sur ce serveur. */
  providers?: Providers;
  /**
   * Fournisseurs déjà rattachés à ce compte : l'interface propose d'associer les
   * autres, jamais de rattacher deux fois.
   */
  linked?: string[];
  /** Connexion externe vérifiée qui attend un pseudo. */
  pending?: PendingOAuth;
  /** Échec de la dernière connexion externe, annoncé une seule fois. */
  error?: ErrorPayload;
};

/**
 * Fournisseurs de connexion externe proposés par le serveur.
 *
 * Un fournisseur sans identifiants d'application n'est pas configuré : le
 * serveur le dit, et l'interface ne montre pas son bouton.
 */
export type Providers = {
  google: boolean;
  discord: boolean;
};

/**
 * Connexion externe dont l'identité est vérifiée : il ne manque que le pseudo.
 */
export type PendingOAuth = {
  provider: string;
  /** Pseudo proposé par le fournisseur, absent s'il n'y en a pas d'utilisable. */
  suggested?: string;
};

/** Aucun fournisseur externe : l'état d'un serveur sans identifiants. */
export const NO_PROVIDERS: Providers = { google: false, discord: false };

/** Codes d'erreur du serveur. */
export type ErrorCode =
  | 'invalid_message'
  | 'invalid_pseudo'
  | 'invalid_room_code'
  | 'room_not_found'
  | 'room_full'
  | 'not_authenticated'
  | 'unauthorized'
  | 'rate_limited'
  | 'not_implemented'
  | 'internal'
  | 'not_enough_players'
  | 'game_in_progress'
  | 'game_over'
  | 'wrong_phase'
  | 'not_a_wolf'
  | 'wrong_action'
  | 'player_dead'
  | 'bad_target'
  | 'username_taken'
  | 'bad_credentials'
  | 'weak_password'
  | 'oauth_state'
  | 'oauth_failed'
  | 'oauth_linked_elsewhere';

/** Erreur applicative transmise par le serveur. */
export type ErrorPayload = {
  code: ErrorCode;
  message: string;
};

/** Trame reçue du serveur. */
export type ServerMessage = {
  type: string;
  data?: unknown;
};

/** Trame envoyée au serveur. */
export type ClientMessage = {
  type: string;
  data?: unknown;
};

/** Limite d'historique conservée côté client (comme dans un instantané). */
export const MESSAGES_LIMIT = 50;

/** Bornes du protocole, miroir de `Back/internal/protocol`. */
export const MAX_CHAT_LENGTH = 500;
export const PSEUDO_MIN_LENGTH = 2;
export const PSEUDO_MAX_LENGTH = 20;
export const ROOM_CODE_LENGTH = 6;
export const MIN_PLAYERS = 6;
export const MAX_PLAYERS = 12;

/** Bornes d'un mot de passe, miroir de `Back/internal/accounts`. */
export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_LENGTH = 200;

/**
 * providerLabel nomme un fournisseur pour l'affichage.
 *
 * Le nom vient du serveur : l'interface ne connaît pas d'avance les
 * fournisseurs qu'un serveur donné propose.
 */
export function providerLabel(provider: string): string {
  const labels: Record<string, string> = { google: 'Google', discord: 'Discord' };

  return labels[provider] ?? provider;
}

/**
 * providerStartURL est l'adresse qui lance une connexion externe.
 *
 * Le navigateur y va directement : c'est une redirection vers le fournisseur,
 * pas un appel d'API. Le retour ramène à l'application, qui demandera le pseudo.
 */
export function providerStartURL(provider: string): string {
  return `/api/auth/${encodeURIComponent(provider)}/start`;
}

/**
 * providerLinkURL est l'adresse qui rattache un fournisseur au compte connecté.
 *
 * Comme la connexion, c'est une navigation : le retour ramène à l'application.
 * La session ouverte par le cookie autorise le rattachement ; sans elle, rien
 * n'est rattaché.
 */
export function providerLinkURL(provider: string): string {
  return `/api/auth/${encodeURIComponent(provider)}/link`;
}

export function authMessage(token: string): ClientMessage {
  return { type: 'auth', data: { token } };
}

export function chatMessage(text: string): ClientMessage {
  return { type: 'chat.send', data: { text } };
}

export function readyMessage(ready: boolean): ClientMessage {
  return { type: 'player.ready', data: { ready } };
}

export function startMessage(): ClientMessage {
  return { type: 'game.start' };
}

export function againMessage(): ClientMessage {
  return { type: 'game.again' };
}

/** voteMessage désigne le suspect du village (phase de vote). */
export function voteMessage(target: string): ClientMessage {
  return { type: 'game.vote', data: { target } };
}

/** nightVoteMessage désigne la victime des loups (phase de nuit). */
export function nightVoteMessage(target: string): ClientMessage {
  return { type: 'game.night.vote', data: { target } };
}

/**
 * nightActionMessage répond à une veillée : la voyante observe, la sorcière se
 * sert d'une potion ou passe.
 */
export function nightActionMessage(action: NightAnswer, target = ''): ClientMessage {
  return { type: 'game.night.action', data: { action, target } };
}

/** botAddMessage demande l'ajout de bots en salle d'attente (hôte). */
export function botAddMessage(count: number): ClientMessage {
  return { type: 'bot.add', data: { count } };
}

/** botRemoveMessage retire un bot ; une cible vide les retire tous (hôte). */
export function botRemoveMessage(target: string): ClientMessage {
  return { type: 'bot.remove', data: { target } };
}

/**
 * speakDoneMessage rend la parole : le débat passe au vivant suivant sans
 * attendre l'échéance. Le serveur refuse qui n'a pas la parole.
 */
export function speakDoneMessage(): ClientMessage {
  return { type: 'game.speak.done' };
}

/** isPlaying dit si l'étape décrit une partie, par opposition à la salle d'attente. */
export function isPlaying(phase: Phase): boolean {
  return phase !== 'lobby';
}
