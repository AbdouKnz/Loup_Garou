import { create } from 'zustand';

import { ApiError, messageErreur } from '../api/client';
import { httpTransport } from '../api/transport';
import type { Transport } from '../api/transport';
import { useAccountStore } from './account';
import type { Socket, SocketStatus } from '../api/socket';
import {
  chatMessage,
  readyMessage,
  startMessage,
  againMessage,
  voteMessage,
  nightVoteMessage,
  nightActionMessage,
  speakDoneMessage,
  botAddMessage,
  botRemoveMessage,
  MESSAGES_LIMIT,
} from '../api/types';
import type {
  ChatMessage,
  ClientMessage,
  ErrorPayload,
  GameEvent,
  NightAnswer,
  RoomState,
  ServerMessage,
  Session,
} from '../api/types';

/** Accès au stockage local : séparé pour être simulable en test. */
export type Storage = {
  loadSession(): Session | null;
  saveSession(session: Session): void;
  clearSession(): void;
};

const SESSION_KEY = 'loupgarou.session';

/** Nombre de faits de jeu conservés pour le bilan de fin de partie. */
export const HISTORY_LIMIT = 40;

/** createLocalStorage persiste la session de siège dans le navigateur. */
export function createLocalStorage(): Storage {
  const available = typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';

  return {
    loadSession() {
      if (!available) {
        return null;
      }

      const raw = window.localStorage.getItem(SESSION_KEY);
      if (!raw) {
        return null;
      }

      try {
        const parsed = JSON.parse(raw) as Partial<Session>;
        if (parsed.code && parsed.playerId && parsed.token) {
          return {
            code: parsed.code,
            playerId: parsed.playerId,
            token: parsed.token,
            pseudo: parsed.pseudo ?? '',
          };
        }
      } catch {
        // Données illisibles : on repart de zéro plutôt que de bloquer l'écran.
      }

      return null;
    },

    saveSession(session) {
      if (available) {
        window.localStorage.setItem(SESSION_KEY, JSON.stringify(session));
      }
    },

    clearSession() {
      if (available) {
        window.localStorage.removeItem(SESSION_KEY);
      }
    },
  };
}

/** Statut affiché : l'état de la connexion, plus l'état « aucune partie ». */
export type ConnectionStatus = SocketStatus | 'idle';

/** Codes d'erreur qui invalident la session : on revient à l'accueil. */
const FATAL_CODES = new Set<string>(['room_not_found', 'not_authenticated']);

/**
 * Code renvoyé quand la même place est ouverte ailleurs.
 *
 * On revient à l'accueil sans effacer la session mémorisée : le stockage est
 * commun aux onglets d'un même navigateur, et l'effacer délogerait l'onglet qui
 * vient précisément de reprendre la place.
 */
const TAKEOVER_CODE = 'unauthorized';

export type RoomStoreState = {
  /** Identité du joueur dans la partie en cours, ou null à l'accueil. */
  session: Session | null;
  connection: ConnectionStatus;
  /** Dernier instantané reçu du serveur. */
  room: RoomState | null;
  /** Dernier fait de jeu raconté par le narrateur, pour le bandeau de scène. */
  event: GameEvent | null;
  /** Numéro d'ordre du dernier fait de jeu : l'interface s'en sert de clé. */
  eventSeq: number;
  /** Faits de jeu accumulés depuis le début de la partie, pour le bilan final. */
  history: GameEvent[];
  /** Durée totale de la phase en cours, observée sur les instantanés. */
  phaseTotal: number;
  /** Clé de la phase observée (« phase-jour-tour »), pour repérer un changement. */
  phaseKey: string;
  /** Message d'erreur à afficher, s'il y en a un. */
  error: string | null;
  /** Vrai pendant un appel réseau (création, entrée dans une partie). */
  busy: boolean;

  /** resume reprend la session mémorisée après un rafraîchissement (F5). */
  resume(): void;
  /** createRoom ouvre une partie au nom du compte connecté. */
  createRoom(): Promise<void>;
  /** joinRoom fait entrer le compte connecté dans une partie existante. */
  joinRoom(code: string): Promise<void>;
  /** leave quitte la partie et oublie la session. */
  leave(): void;
  sendChat(text: string): void;
  toggleReady(): void;
  startGame(): void;
  /** voteJour désigne un suspect (phase de vote). Vide = abstention. */
  voteJour(target: string): void;
  /** voteNuit désigne la victime des loups (phase de nuit). */
  voteNuit(target: string): void;
  /**
   * agirNuit répond à la veillée en cours : la voyante observe un joueur, la
   * sorcière se sert d'une potion ou passe.
   */
  agirNuit(action: NightAnswer, target?: string): void;
  /** terminerParole rend la parole pendant le débat (elle doit être à vous). */
  terminerParole(): void;
  /** rejouer ramène la table en salle d'attente après une partie (hôte). */
  rejouer(): void;
  /** ajouterBots invite des joueurs automatiques en salle d'attente (hôte). */
  ajouterBots(count: number): void;
  /** retirerBot retire un bot précis (hôte). */
  retirerBot(id: string): void;
  /** retirerTousLesBots vide la table de ses bots (hôte). */
  retirerTousLesBots(): void;
  dismissError(): void;
};

export type StoreDependencies = {
  transport: Transport;
  storage: Storage;
  /**
   * onAuthLost est appelé quand le serveur refuse la session de compte : le
   * cookie a expiré, ou un autre onglet s'est déconnecté. Le store ne sait rien
   * des comptes, il se contente de le signaler.
   */
  onAuthLost?: () => void;
};

/** createRoomStore construit un store isolé : un par onglet, un par test. */
export function createRoomStore(dependencies: StoreDependencies) {
  let socket: Socket | null = null;
  /** Vrai pendant une vérification « cette partie existe-t-elle encore ? ». */
  let checkingRoom = false;

  return create<RoomStoreState>((set, get) => {
    /** send transmet une trame à la salle, si la connexion existe. */
    const send = (message: ClientMessage) => {
      socket?.send(message);
    };

    /** connect ouvre la connexion d'une session, en la mémorisant si demandé. */
    const connect = (session: Session, remember: boolean) => {
      socket?.close();
      if (remember) {
        dependencies.storage.saveSession(session);
      }

      set({
        session,
        room: null,
        event: null,
        history: [],
        phaseKey: '',
        phaseTotal: 0,
        connection: 'connecting',
        error: null,
      });

      socket = dependencies.transport.open(session, {
        onStatus,
        onMessage: handleMessage,
      });
    };

    /** forget ramène à l'accueil, en oubliant la session si elle est invalide. */
    const forget = (message: string | null, clearStorage = true) => {
      socket?.close();
      socket = null;

      if (clearStorage) {
        dependencies.storage.clearSession();
      }

      set({
        session: null,
        room: null,
        event: null,
        history: [],
        phaseKey: '',
        phaseTotal: 0,
        connection: 'offline',
        error: message,
      });
    };

    /**
     * signalAuthLost prévient l'application que la session de compte n'est plus
     * valable (cookie expiré, déconnexion depuis un autre onglet).
     */
    const signalAuthLost = (error: unknown) => {
      if (error instanceof ApiError && error.code === 'not_authenticated') {
        dependencies.onAuthLost?.();
      }
    };

    /**
     * checkRoomGone demande à l'API si la partie existe encore.
     *
     * Une poignée de main WebSocket refusée ne dit pas *pourquoi* : le
     * navigateur n'expose pas le code HTTP. Sans cette vérification, une
     * session mémorisée dont la salle a fermé (serveur redémarré, partie
     * balayée) resterait indéfiniment sur « Reconnexion… ».
     */
    const checkRoomGone = async (code: string) => {
      if (checkingRoom) {
        return;
      }

      checkingRoom = true;
      try {
        await dependencies.transport.fetchRoom(code);
      } catch (error) {
        if (error instanceof ApiError && error.code === 'room_not_found') {
          forget('Cette partie a été fermée.', true);
        }
      } finally {
        checkingRoom = false;
      }
    };

    /** onStatus suit la connexion, et vérifie la partie dès qu'elle lâche. */
    const onStatus = (status: SocketStatus) => {
      set({ connection: status });

      if (status === 'reconnecting') {
        const { session } = get();
        if (session) {
          void checkRoomGone(session.code);
        }
      }
    };

    /** appendChat ajoute un message au fil, sans doublon. */
    const appendChat = (incoming: ChatMessage) => {
      const room = get().room;
      if (!room) {
        return;
      }
      if (room.messages.some((existing) => existing.id === incoming.id)) {
        return;
      }

      set({
        room: { ...room, messages: [...room.messages, incoming].slice(-MESSAGES_LIMIT) },
      });
    };

    /** handleMessage applique une trame du serveur au store. */
    const handleMessage = (message: ServerMessage) => {
      switch (message.type) {
        case 'room.state': {
          // Un instantané remplace tout l'état connu : aucune désynchronisation
          // possible, même après une reconnexion. On y observe aussi la durée de
          // la phase (le plus grand décompte vu) pour l'anneau de progression.
          const state = message.data as RoomState;
          const current = get();
          // Chaque veillée de la nuit a sa propre échéance : c'est elle qui
          // distingue deux étapes d'une même phase, et fixe la durée observée.
          const key = state.game
            ? `${state.game.phase}-${state.game.day}-${state.game.round}-${state.game.deadline ?? ''}`
            : '';
          const seen = state.game?.secondsLeft ?? 0;
          const phaseTotal =
            key !== '' && key === current.phaseKey
              ? Math.max(current.phaseTotal, seen)
              : Math.max(1, seen);

          set({
            room: state,
            error: null,
            phaseKey: key,
            phaseTotal,
            // Retour en salle d'attente : une nouvelle partie se prépare, le
            // bilan repart de zéro.
            history: key === '' ? [] : current.history,
          });
          break;
        }

        case 'chat.message':
          appendChat(message.data as ChatMessage);
          break;

        case 'game.event': {
          // Le narrateur prend la parole : l'interface peut réagir tout de suite
          // (bandeau, couleur) sans attendre l'instantané qui suit. Le numéro
          // d'ordre change à chaque fait, même si la phrase se répète. On garde
          // aussi l'historique, borné, pour le bilan de fin de partie — sauf les
          // tours de parole, qui rempliraient le bilan sans rien y apprendre.
          const event = message.data as GameEvent;
          set({
            event,
            eventSeq: get().eventSeq + 1,
            history:
              event.kind === 'parole'
                ? get().history
                : [...get().history, event].slice(-HISTORY_LIMIT),
          });
          break;
        }

        case 'error': {
          const payload = message.data as ErrorPayload;
          if (payload.code === TAKEOVER_CODE) {
            forget(payload.message, false);
            break;
          }

          if (FATAL_CODES.has(payload.code)) {
            forget(payload.message);
            break;
          }

          set({ error: payload.message });
          break;
        }

        default:
          break;
      }
    };

    const initialSession = dependencies.storage.loadSession();

    return {
      session: initialSession,
      connection: 'idle',
      room: null,
      event: null,
      eventSeq: 0,
      history: [],
      phaseKey: '',
      phaseTotal: 0,
      error: null,
      busy: false,

      resume: () => {
        const { session } = get();
        if (!session || socket) {
          return;
        }

        connect(session, false);
      },

      createRoom: async () => {
        set({ busy: true, error: null });
        try {
          const session = await dependencies.transport.createRoom();
          connect(session, true);
        } catch (error) {
          signalAuthLost(error);
          set({ error: messageErreur(error) });
        } finally {
          set({ busy: false });
        }
      },

      joinRoom: async (code) => {
        set({ busy: true, error: null });
        try {
          const session = await dependencies.transport.joinRoom(code);
          connect(session, true);
        } catch (error) {
          signalAuthLost(error);
          set({ error: messageErreur(error) });
        } finally {
          set({ busy: false });
        }
      },

      leave: () => {
        socket?.close();
        socket = null;
        dependencies.storage.clearSession();

        set({
          session: null,
          room: null,
          event: null,
          history: [],
          phaseKey: '',
          phaseTotal: 0,
          connection: 'idle',
          error: null,
        });
      },

      sendChat: (text) => {
        const clean = text.trim();
        if (clean === '') {
          return;
        }

        send(chatMessage(clean));
      },

      toggleReady: () => {
        const { room, session } = get();
        if (!room || !session) {
          return;
        }

        const me = room.players.find((player) => player.id === session.playerId);
        send(readyMessage(!me?.ready));
      },

      startGame: () => {
        send(startMessage());
      },

      voteJour: (target) => {
        send(voteMessage(target));
      },

      voteNuit: (target) => {
        send(nightVoteMessage(target));
      },

      agirNuit: (action, target = '') => {
        send(nightActionMessage(action, target));
      },

      terminerParole: () => {
        send(speakDoneMessage());
      },

      rejouer: () => {
        send(againMessage());
      },

      ajouterBots: (count) => {
        send(botAddMessage(count));
      },

      retirerBot: (id) => {
        send(botRemoveMessage(id));
      },

      retirerTousLesBots: () => {
        send(botRemoveMessage(''));
      },

      dismissError: () => set({ error: null }),
    };
  });
}

/** Store de l'application : une session par onglet. */
export const useRoomStore = createRoomStore({
  transport: httpTransport(),
  storage: createLocalStorage(),
  // Une session de compte perdue se voit ici : le store de salle le signale,
  // l'écran de connexion reprend la main.
  onAuthLost: () => {
    void useAccountStore.getState().load();
  },
});
