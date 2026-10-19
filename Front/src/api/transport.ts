import {
  completeAccount,
  createRoom,
  fetchAccount,
  fetchRoom,
  joinRoom,
  login,
  logout,
  signup,
} from './client';
import { RoomSocket, roomSocketUrl } from './socket';
import type { Socket, SocketHandlers } from './socket';
import type { Account, RoomSummary, Session } from './types';

/**
 * Transport : ce que le store attend de la couche réseau.
 *
 * Cette frontière permet de tester toute la logique du store (création,
 * messages, erreurs) sans serveur ni navigateur.
 */
export type Transport = {
  /** createRoom ouvre une partie au nom du compte connecté. */
  createRoom(): Promise<Session>;
  /** joinRoom fait entrer le compte connecté dans une partie existante. */
  joinRoom(code: string): Promise<Session>;
  /** fetchRoom dit si une partie existe encore : une poignée de main WebSocket
   *  refusée ne donne pas cette information au navigateur. */
  fetchRoom(code: string): Promise<RoomSummary>;
  open(session: Session, handlers: SocketHandlers): Socket;
};

/** httpTransport branche le store sur l'API HTTP et la passerelle WebSocket. */
export function httpTransport(): Transport {
  return {
    createRoom,
    joinRoom,
    fetchRoom,
    open: (session, handlers) =>
      new RoomSocket(roomSocketUrl(session.code), session.token, handlers),
  };
}

/** AccountApi : ce que le store de compte attend de la couche réseau. */
export type AccountApi = {
  me(): Promise<Account>;
  signup(username: string, password: string): Promise<Account>;
  login(username: string, password: string): Promise<Account>;
  /** complete choisit le pseudo d'une connexion externe (Google, Discord). */
  complete(username: string): Promise<Account>;
  logout(): Promise<void>;
};

/** httpAccountApi branche le store de compte sur l'API HTTP. */
export function httpAccountApi(): AccountApi {
  return { me: fetchAccount, signup, login, complete: completeAccount, logout };
}
