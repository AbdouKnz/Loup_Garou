import { authMessage } from './types';
import type { ClientMessage, ServerMessage } from './types';

/** État de la connexion temps réel, tel qu'affiché à l'écran. */
export type SocketStatus = 'connecting' | 'online' | 'reconnecting' | 'offline';

/** Ce qu'un WebSocket doit savoir faire : permet de le simuler dans les tests. */
export type WebSocketLike = {
  send(data: string): void;
  close(code?: number, reason?: string): void;
  onopen: ((event: unknown) => void) | null;
  onmessage: ((event: { data: unknown }) => void) | null;
  onclose: ((event: unknown) => void) | null;
  onerror: ((event: unknown) => void) | null;
};

/** Réactions du store aux événements de la connexion. */
export type SocketHandlers = {
  onStatus(status: SocketStatus): void;
  onMessage(message: ServerMessage): void;
};

/** Connexion vue par le store. */
export type Socket = {
  send(message: ClientMessage): void;
  close(): void;
};

export type SocketOptions = {
  /** Fabrique de WebSocket, injectée dans les tests. */
  create?: (url: string) => WebSocketLike;
  /** Délai initial avant une nouvelle tentative (doublé à chaque échec). */
  baseDelayMs?: number;
  /** Délai maximal entre deux tentatives. */
  maxDelayMs?: number;
};

const DEFAULT_DELAY_MS = 500;
const MAX_DELAY_MS = 8000;

/**
 * roomSocketUrl construit l'URL de la passerelle à partir de l'adresse courante.
 *
 * Le même chemin fonctionne en développement (relayé par Vite) et en production
 * (servi par Go) : aucun réglage à changer d'un environnement à l'autre.
 */
export function roomSocketUrl(code: string): string {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';

  return `${scheme}://${window.location.host}/api/rooms/${encodeURIComponent(code)}/ws`;
}

/**
 * RoomSocket est la connexion temps réel d'un joueur.
 *
 * Elle s'authentifie à chaque (re)connexion, met les trames en attente tant que
 * la connexion n'est pas prête, et retente automatiquement avec un délai
 * croissant : un onglet qui perd le réseau se rétablit tout seul.
 */
export class RoomSocket implements Socket {
  private readonly url: string;
  private readonly token: string;
  private readonly handlers: SocketHandlers;
  private readonly factory: (url: string) => WebSocketLike;
  private readonly baseDelayMs: number;
  private readonly maxDelayMs: number;

  private socket: WebSocketLike | null = null;
  private pending: ClientMessage[] = [];
  private attempt = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;
  /** Vrai seulement quand la session est ouverte et authentifiée. */
  private ready = false;

  constructor(url: string, token: string, handlers: SocketHandlers, options: SocketOptions = {}) {
    this.url = url;
    this.token = token;
    this.handlers = handlers;
    this.factory =
      options.create ?? ((target) => new WebSocket(target) as unknown as WebSocketLike);
    this.baseDelayMs = options.baseDelayMs ?? DEFAULT_DELAY_MS;
    this.maxDelayMs = options.maxDelayMs ?? MAX_DELAY_MS;

    this.connect();
  }

  /** send transmet une trame, ou la met en attente si la session n'est pas ouverte. */
  send(message: ClientMessage): void {
    if (this.closed) {
      return;
    }

    if (this.ready && this.socket) {
      this.socket.send(JSON.stringify(message));
      return;
    }

    // File d'attente : l'authentification doit partir avant toute autre trame.
    this.pending.push(message);
  }

  /** close ferme définitivement la connexion : plus aucune reconnexion. */
  close(): void {
    if (this.closed) {
      return;
    }

    this.closed = true;
    this.clearTimer();

    const socket = this.socket;
    this.socket = null;
    socket?.close(1000, 'fin de session');

    this.handlers.onStatus('offline');
  }

  /** connect ouvre une connexion, ou la relance après une coupure. */
  private connect(): void {
    if (this.closed) {
      return;
    }

    this.handlers.onStatus(this.attempt === 0 ? 'connecting' : 'reconnecting');

    this.ready = false;
    const socket = this.factory(this.url);
    this.socket = socket;

    socket.onopen = () => this.handleOpen(socket);
    socket.onmessage = (event) => this.handleMessage(event);
    socket.onclose = () => this.handleClose(socket);
    // Une erreur est toujours suivie d'une fermeture : handleClose suffit.
    socket.onerror = () => undefined;
  }

  private handleOpen(socket: WebSocketLike): void {
    if (this.socket !== socket || this.closed) {
      return;
    }

    this.attempt = 0;
    this.ready = true;
    this.handlers.onStatus('online');

    // L'authentification ouvre la session : elle précède toute autre trame.
    socket.send(JSON.stringify(authMessage(this.token)));

    const queued = this.pending;
    this.pending = [];

    for (const message of queued) {
      socket.send(JSON.stringify(message));
    }
  }

  private handleMessage(event: { data: unknown }): void {
    if (typeof event.data !== 'string') {
      return;
    }

    try {
      this.handlers.onMessage(JSON.parse(event.data) as ServerMessage);
    } catch {
      // Trame illisible : on l'ignore plutôt que de casser la connexion.
    }
  }

  private handleClose(socket: WebSocketLike): void {
    if (this.socket !== socket) {
      return;
    }

    this.socket = null;
    this.ready = false;

    if (this.closed) {
      return;
    }

    this.handlers.onStatus('reconnecting');
    this.scheduleReconnect();
  }

  private scheduleReconnect(): void {
    if (this.closed || this.timer !== null) {
      return;
    }

    const delay = Math.min(this.maxDelayMs, this.baseDelayMs * 2 ** this.attempt);
    this.attempt += 1;

    this.timer = setTimeout(() => {
      this.timer = null;
      this.connect();
    }, delay);
  }

  private clearTimer(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}
