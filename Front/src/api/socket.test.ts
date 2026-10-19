import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { RoomSocket } from './socket';
import type { WebSocketLike } from './socket';
import type { ServerMessage } from './types';

/** Faux WebSocket : le test décide quand la connexion s'ouvre et se ferme. */
class FakeSocket implements WebSocketLike {
  static instances: FakeSocket[] = [];

  readonly url: string;
  sent: string[] = [];
  closed = false;

  onopen: ((event: unknown) => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;
  onclose: ((event: unknown) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeSocket.instances.push(this);
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(): void {
    this.closed = true;
  }

  // --- aides de test ---

  open(): void {
    this.onopen?.({});
  }

  emit(message: ServerMessage): void {
    this.onmessage?.({ data: JSON.stringify(message) });
  }

  drop(): void {
    this.onclose?.({});
  }
}

/** json relit une trame envoyée par la connexion. */
function json(raw: string): ServerMessage {
  return JSON.parse(raw) as ServerMessage;
}

function create(url = 'ws://localhost/api/rooms/ABC234/ws', token = 'jeton') {
  const statuses: string[] = [];
  const messages: ServerMessage[] = [];

  const socket = new RoomSocket(
    url,
    token,
    {
      onStatus: (status) => statuses.push(status),
      onMessage: (message) => messages.push(message),
    },
    { create: (target) => new FakeSocket(target), baseDelayMs: 10, maxDelayMs: 40 },
  );

  return { socket, statuses, messages };
}

beforeEach(() => {
  FakeSocket.instances = [];
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('RoomSocket', () => {
  it('s’authentifie à l’ouverture puis envoie ce qui attendait', () => {
    const { socket, statuses } = create();

    const first = FakeSocket.instances[0];
    expect(first.url).toBe('ws://localhost/api/rooms/ABC234/ws');

    // Tant que la connexion n'est pas ouverte, les trames patientent.
    socket.send({ type: 'chat.send', data: { text: 'en attente' } });
    expect(first.sent).toHaveLength(0);

    first.open();

    expect(json(first.sent[0])).toEqual({ type: 'auth', data: { token: 'jeton' } });
    expect(json(first.sent[1])).toEqual({ type: 'chat.send', data: { text: 'en attente' } });
    expect(statuses).toEqual(['connecting', 'online']);
  });

  it('remonte les trames du serveur et ignore le bruit', () => {
    const { socket, messages } = create();
    const fake = FakeSocket.instances[0];

    fake.open();
    fake.emit({ type: 'room.state', data: { code: 'ABC234' } });
    fake.onmessage?.({ data: 'pas du JSON' });
    fake.onmessage?.({ data: new ArrayBuffer(4) });

    expect(messages).toEqual([{ type: 'room.state', data: { code: 'ABC234' } }]);

    socket.send({ type: 'ping' });
    expect(json(fake.sent.at(-1) as string)).toEqual({ type: 'ping' });
  });

  it('se reconnecte avec un délai qui double', () => {
    const { statuses } = create();
    const first = FakeSocket.instances[0];

    first.open();
    first.drop();
    expect(statuses.at(-1)).toBe('reconnecting');

    // Premier report : 10 ms.
    vi.advanceTimersByTime(10);
    expect(FakeSocket.instances).toHaveLength(2);

    // Deuxième report : 20 ms, donc rien à 10 ms.
    FakeSocket.instances[1].drop();
    vi.advanceTimersByTime(10);
    expect(FakeSocket.instances).toHaveLength(2);

    vi.advanceTimersByTime(10);
    expect(FakeSocket.instances).toHaveLength(3);
  });

  it('oublie les tentatives après une reconnexion réussie', () => {
    const { socket } = create();

    FakeSocket.instances[0].drop();
    vi.advanceTimersByTime(10);
    FakeSocket.instances[1].open();

    // Le délai repart de zéro : nouveau report à 10 ms.
    socket.send({ type: 'chat.send', data: { text: 'ok' } });
    expect(json(FakeSocket.instances[1].sent.at(-1) as string)).toEqual({
      type: 'chat.send',
      data: { text: 'ok' },
    });

    FakeSocket.instances[1].drop();
    vi.advanceTimersByTime(10);
    expect(FakeSocket.instances).toHaveLength(3);
  });

  it('ne se reconnecte plus après close', () => {
    const { socket, statuses } = create();
    const fake = FakeSocket.instances[0];

    fake.open();
    socket.close();

    expect(fake.closed).toBe(true);
    expect(statuses.at(-1)).toBe('offline');

    fake.drop();
    vi.advanceTimersByTime(1000);

    expect(FakeSocket.instances).toHaveLength(1);

    // Une trame envoyée après la fermeture est ignorée.
    socket.send({ type: 'ping' });
    expect(fake.sent).toHaveLength(1);
  });
});
