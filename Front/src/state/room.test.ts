import { describe, expect, it, vi } from 'vitest';

import type { Socket, SocketHandlers } from '../api/socket';
import type { Transport } from '../api/transport';
import type { ClientMessage, RoomState, RoomSummary, Session } from '../api/types';
import { ApiError } from '../api/client';
import { createRoomStore, HISTORY_LIMIT } from './room';
import type { Storage } from './room';

/** Faux transport : aucune requête ne part, le test pilote tout. */
class FakeTransport implements Transport {
  createCalls = 0;
  joinCalls: string[] = [];
  fetchCalls: string[] = [];
  handlers: SocketHandlers[] = [];
  sockets: Array<{ sent: ClientMessage[]; closed: boolean }> = [];
  failWith: Error | null = null;
  roomExists = true;
  session: Session = { code: 'ABC234', playerId: 'P1', token: 'jeton', pseudo: 'Alice' };

  createRoom(): Promise<Session> {
    this.createCalls += 1;
    if (this.failWith) {
      return Promise.reject(this.failWith);
    }

    return Promise.resolve({ ...this.session });
  }

  joinRoom(code: string): Promise<Session> {
    this.joinCalls.push(code);
    if (this.failWith) {
      return Promise.reject(this.failWith);
    }

    return Promise.resolve({ ...this.session, code });
  }

  open(_session: Session, handlers: SocketHandlers): Socket {
    const handle: { sent: ClientMessage[]; closed: boolean } = { sent: [], closed: false };
    this.sockets.push(handle);
    this.handlers.push(handlers);

    return {
      send: (message) => handle.sent.push(message),
      close: () => {
        handle.closed = true;
      },
    };
  }

  fetchRoom(code: string): Promise<RoomSummary> {
    this.fetchCalls.push(code);

    if (!this.roomExists) {
      return Promise.reject(new ApiError('room_not_found', 'Cette partie a été fermée.'));
    }

    return Promise.resolve({ code, phase: 'lobby', players: 1, maxPlayers: 12 });
  }

  /** Dernier état de la connexion ouverte. */
  get lastSocket() {
    return this.sockets[this.sockets.length - 1];
  }

  /** Dernier jeu de réactions, pour simuler le serveur. */
  get lastHandlers() {
    return this.handlers[this.handlers.length - 1];
  }
}

/** Stockage en mémoire, à la place du localStorage du navigateur. */
function memoryStorage(initial: Session | null = null): Storage {
  let session = initial;

  return {
    loadSession: () => session,
    saveSession: (value) => {
      session = value;
    },
    clearSession: () => {
      session = null;
    },
  };
}

function roomState(overrides: Partial<RoomState> = {}): RoomState {
  return {
    code: 'ABC234',
    phase: 'lobby',
    hostId: 'P1',
    you: 'P1',
    canStart: false,
    maxPlayers: 12,
    players: [{ id: 'P1', pseudo: 'Alice', ready: false, online: true, host: true }],
    messages: [],
    composition: [],
    ...overrides,
  };
}

function setup(initialSession: Session | null = null, onAuthLost?: () => void) {
  const transport = new FakeTransport();
  const storage = memoryStorage(initialSession);

  return { transport, storage, store: createRoomStore({ transport, storage, onAuthLost }) };
}

describe('createRoomStore', () => {
  it('crée une partie, mémorise la session et ouvre la connexion', async () => {
    const { transport, storage, store } = setup();

    await store.getState().createRoom();

    expect(transport.createCalls).toBe(1);
    expect(store.getState().session?.code).toBe('ABC234');
    expect(storage.loadSession()?.playerId).toBe('P1');
    expect(store.getState().connection).toBe('connecting');

    transport.lastHandlers.onStatus('online');
    expect(store.getState().connection).toBe('online');

    transport.lastHandlers.onMessage({ type: 'room.state', data: roomState() });
    expect(store.getState().room?.code).toBe('ABC234');
  });

  it('affiche l’erreur d’API sans ouvrir de connexion', async () => {
    const { transport, store } = setup();
    transport.failWith = new Error('La partie est complète (12 joueurs).');

    await store.getState().createRoom();

    expect(store.getState().error).toBe('La partie est complète (12 joueurs).');
    expect(store.getState().session).toBeNull();
    expect(store.getState().busy).toBe(false);
    expect(transport.sockets).toHaveLength(0);
  });

  it('signale une session de compte perdue', async () => {
    let lost = 0;
    const { transport, store } = setup(null, () => {
      lost += 1;
    });
    transport.failWith = new ApiError('not_authenticated', 'Connectez-vous pour jouer.');

    await store.getState().createRoom();

    expect(lost).toBe(1);
    expect(store.getState().error).toBe('Connectez-vous pour jouer.');
  });

  it('rejoint une partie par son code', async () => {
    const { transport, store } = setup();

    await store.getState().joinRoom('abc234');

    expect(transport.joinCalls).toEqual(['abc234']);
    expect(store.getState().session?.code).toBe('abc234');
  });

  it('ajoute les messages sans doublon, puis remplace par l’instantané', async () => {
    const { transport, store } = setup();
    await store.getState().createRoom();

    transport.lastHandlers.onMessage({ type: 'room.state', data: roomState() });

    const message = {
      id: 'M1',
      playerId: 'P2',
      pseudo: 'Bob',
      text: 'Coucou',
      at: '2026-10-17T20:00:00Z',
    };

    transport.lastHandlers.onMessage({ type: 'chat.message', data: message });
    transport.lastHandlers.onMessage({ type: 'chat.message', data: message });

    expect(store.getState().room?.messages).toHaveLength(1);

    // Un nouvel instantané fait autorité : il ne duplique rien.
    transport.lastHandlers.onMessage({
      type: 'room.state',
      data: roomState({ messages: [message] }),
    });

    expect(store.getState().room?.messages).toHaveLength(1);
  });

  it('oublie la session quand la partie n’existe plus', async () => {
    const { transport, storage, store } = setup();
    await store.getState().createRoom();

    transport.lastHandlers.onMessage({
      type: 'error',
      data: { code: 'room_not_found', message: 'La partie a été fermée (salle vide).' },
    });

    expect(store.getState().session).toBeNull();
    expect(store.getState().room).toBeNull();
    expect(storage.loadSession()).toBeNull();
    expect(transport.lastSocket.closed).toBe(true);
    expect(store.getState().error).toBe('La partie a été fermée (salle vide).');
  });

  it('rend l’accueil quand la place est reprise ailleurs, sans oublier la session', async () => {
    const { transport, storage, store } = setup();
    await store.getState().createRoom();

    transport.lastHandlers.onMessage({
      type: 'error',
      data: { code: 'unauthorized', message: 'Cette place a été reprise depuis un autre onglet.' },
    });

    expect(store.getState().session).toBeNull();
    expect(store.getState().room).toBeNull();
    expect(transport.lastSocket.closed).toBe(true);

    // La session reste mémorisée : les onglets partagent le stockage, l'effacer
    // déconnecterait l'onglet qui a repris la place.
    expect(storage.loadSession()).not.toBeNull();
  });

  it('garde la partie sur une erreur passagère', async () => {
    const { transport, store } = setup();
    await store.getState().createRoom();

    transport.lastHandlers.onMessage({
      type: 'error',
      data: { code: 'rate_limited', message: 'Doucement !' },
    });

    expect(store.getState().session).not.toBeNull();
    expect(store.getState().error).toBe('Doucement !');

    store.getState().dismissError();
    expect(store.getState().error).toBeNull();
  });

  it('transmet le chat et l’état « prêt »', async () => {
    const { transport, store } = setup();
    await store.getState().createRoom();

    transport.lastHandlers.onMessage({ type: 'room.state', data: roomState() });

    store.getState().sendChat('   ');
    store.getState().sendChat('  salut  ');
    store.getState().toggleReady();
    store.getState().startGame();

    expect(transport.lastSocket.sent).toEqual([
      { type: 'chat.send', data: { text: 'salut' } },
      { type: 'player.ready', data: { ready: true } },
      { type: 'game.start' },
    ]);
  });

  it('quitte la partie en tout oubliant', async () => {
    const { transport, storage, store } = setup();
    await store.getState().createRoom();

    store.getState().leave();

    expect(store.getState().session).toBeNull();
    expect(store.getState().room).toBeNull();
    expect(store.getState().connection).toBe('idle');
    expect(storage.loadSession()).toBeNull();
    expect(transport.lastSocket.closed).toBe(true);
  });

  it('reprend la session mémorisée après un rafraîchissement', () => {
    const saved: Session = { code: 'ABC234', playerId: 'P1', token: 'jeton', pseudo: 'Alice' };
    const { transport, store } = setup(saved);

    expect(store.getState().session).toEqual(saved);

    store.getState().resume();
    expect(transport.sockets).toHaveLength(1);

    // Un second appel ne doit pas ouvrir une deuxième connexion.
    store.getState().resume();
    expect(transport.sockets).toHaveLength(1);
  });

  it('rend l’accueil quand la partie a fermé pendant la reconnexion', async () => {
    const saved: Session = { code: 'ABC234', playerId: 'P1', token: 'jeton', pseudo: 'Alice' };
    const { transport, storage, store } = setup(saved);
    transport.roomExists = false;

    store.getState().resume();
    transport.lastHandlers.onStatus('reconnecting');

    await vi.waitFor(() => {
      expect(store.getState().session).toBeNull();
    });

    expect(transport.fetchCalls).toEqual(['ABC234']);
    expect(store.getState().error).toBe('Cette partie a été fermée.');
    expect(storage.loadSession()).toBeNull();
  });

  it('garde la session quand la partie existe toujours', async () => {
    const saved: Session = { code: 'ABC234', playerId: 'P1', token: 'jeton', pseudo: 'Alice' };
    const { transport, store } = setup(saved);

    store.getState().resume();
    transport.lastHandlers.onStatus('reconnecting');

    await vi.waitFor(() => {
      expect(transport.fetchCalls).toEqual(['ABC234']);
    });

    expect(store.getState().session).not.toBeNull();
    expect(store.getState().connection).toBe('reconnecting');
  });
});

describe('partie', () => {
  /** session en jeu, avec la connexion ouverte. */
  async function enJeu() {
    const context = setup();
    await context.store.getState().createRoom();

    return context;
  }

  it('transmet les intentions de jeu au serveur', async () => {
    const { transport, store } = await enJeu();

    store.getState().startGame();
    store.getState().voteNuit('P3');
    store.getState().voteJour('P4');
    store.getState().voteJour('');
    store.getState().terminerParole();
    store.getState().rejouer();

    expect(transport.lastSocket.sent).toEqual([
      { type: 'game.start' },
      { type: 'game.night.vote', data: { target: 'P3' } },
      { type: 'game.vote', data: { target: 'P4' } },
      { type: 'game.vote', data: { target: '' } },
      { type: 'game.speak.done' },
      { type: 'game.again' },
    ]);
  });

  it('transmet les intentions de bots au serveur', async () => {
    const { transport, store } = await enJeu();

    store.getState().ajouterBots(5);
    store.getState().retirerBot('B1');
    store.getState().retirerTousLesBots();

    expect(transport.lastSocket.sent).toEqual([
      { type: 'bot.add', data: { count: 5 } },
      { type: 'bot.remove', data: { target: 'B1' } },
      { type: 'bot.remove', data: { target: '' } },
    ]);
  });

  it('remplace l’état par l’instantané reçu, partie comprise', async () => {
    const { transport, store } = await enJeu();

    transport.lastHandlers.onMessage({
      type: 'room.state',
      data: roomState({
        phase: 'nuit',
        canStart: false,
        game: {
          phase: 'nuit',
          day: 1,
          round: 1,
          secondsLeft: 42,
          yourRole: 'loup-garou',
          alive: ['P1', 'P2'],
          dead: [],
          voted: ['P1'],
          wolves: ['P1'],
          wolfVotes: { P1: 'P2' },
        },
        players: [
          { id: 'P1', pseudo: 'Alice', ready: false, online: true, host: true, voted: true },
        ],
      }),
    });

    expect(store.getState().room?.phase).toBe('nuit');
    expect(store.getState().room?.game?.yourRole).toBe('loup-garou');
    expect(store.getState().room?.game?.wolfVotes).toEqual({ P1: 'P2' });
  });

  it('retient le dernier fait de jeu pour le bandeau', async () => {
    const { transport, store } = await enJeu();

    transport.lastHandlers.onMessage({
      type: 'game.event',
      data: {
        kind: 'victime',
        text: 'La nuit a été cruelle : Bob ne s’est pas réveillé.',
        phase: 'aube',
        day: 1,
        round: 1,
        target: 'P2',
      },
    });

    expect(store.getState().event?.kind).toBe('victime');
    expect(store.getState().event?.target).toBe('P2');
  });

  it('accumule les faits de jeu, bornés, pour le bilan', async () => {
    const { transport, store } = await enJeu();

    const fait = (day: number, text: string) => {
      transport.lastHandlers.onMessage({
        type: 'game.event',
        data: { kind: 'victime', text, phase: 'aube', day, round: 1, target: 'P2' },
      });
    };

    for (let day = 1; day <= HISTORY_LIMIT + 5; day += 1) {
      fait(day, `Nuit ${day}.`);
    }

    const history = store.getState().history;
    expect(history).toHaveLength(HISTORY_LIMIT);
    // Ce sont les derniers faits qui restent, pas les premiers.
    expect(history[history.length - 1].text).toBe(`Nuit ${HISTORY_LIMIT + 5}.`);
  });

  it('annonce la prise de parole sans encombrer le bilan', async () => {
    const { transport, store } = await enJeu();

    transport.lastHandlers.onMessage({
      type: 'game.event',
      data: {
        kind: 'parole',
        text: '🎤 Bob prend la parole (30 s).',
        phase: 'debat',
        day: 1,
        round: 1,
        target: 'P2',
      },
    });

    // Le bandeau réagit au fait…
    expect(store.getState().event?.kind).toBe('parole');
    // …mais le bilan, lui, garde les faits de la partie, pas les tours de parole.
    expect(store.getState().history).toEqual([]);
  });

  it('remet le bilan à zéro en revenant en salle d’attente', async () => {
    const { transport, store } = await enJeu();

    transport.lastHandlers.onMessage({
      type: 'room.state',
      data: roomState({
        phase: 'nuit',
        game: { phase: 'nuit', day: 1, round: 1, secondsLeft: 58, alive: ['P1'], dead: [] },
      }),
    });
    transport.lastHandlers.onMessage({
      type: 'game.event',
      data: { kind: 'phase', text: 'La nuit tombe.', phase: 'nuit', day: 1, round: 1 },
    });
    expect(store.getState().history).toHaveLength(1);

    transport.lastHandlers.onMessage({ type: 'room.state', data: roomState() });

    expect(store.getState().history).toEqual([]);
  });

  it('retient la durée d’une phase pour l’anneau de progression', async () => {
    const { transport, store } = await enJeu();

    const etat = (round: number, secondsLeft: number) =>
      transport.lastHandlers.onMessage({
        type: 'room.state',
        data: roomState({
          phase: 'vote',
          game: { phase: 'vote', day: 1, round, secondsLeft, alive: ['P1'], dead: [] },
        }),
      });

    etat(1, 58);
    etat(1, 41);
    expect(store.getState().phaseTotal).toBe(58);

    // Un second tour repart de sa propre durée.
    etat(2, 30);
    expect(store.getState().phaseTotal).toBe(30);
  });

  it('affiche un refus de règle sans quitter la partie', async () => {
    const { transport, store } = await enJeu();

    transport.lastHandlers.onMessage({
      type: 'error',
      data: { code: 'player_dead', message: 'Les morts ne parlent plus : ils regardent.' },
    });

    expect(store.getState().error).toBe('Les morts ne parlent plus : ils regardent.');
    expect(store.getState().session).not.toBeNull();
  });

  it('oublie le fait de jeu en quittant', async () => {
    const { transport, store } = await enJeu();

    transport.lastHandlers.onMessage({
      type: 'game.event',
      data: { kind: 'phase', text: 'La nuit tombe.', phase: 'nuit', day: 1, round: 1 },
    });
    expect(store.getState().event).not.toBeNull();

    store.getState().leave();
    expect(store.getState().event).toBeNull();
  });
});
