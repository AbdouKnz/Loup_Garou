import type { Account, ErrorPayload, RoomSummary, Session } from './types';

/** Erreur renvoyée par l'API, porteuse du code applicatif du serveur. */
export class ApiError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

/** messageErreur rend lisible n'importe quelle erreur attrapée. */
export function messageErreur(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return 'Erreur inattendue.';
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init.headers },
  });

  const payload = (await response.json().catch(() => null)) as unknown;

  if (!response.ok) {
    const failure = payload as Partial<ErrorPayload> | null;
    throw new ApiError(
      failure?.code ?? 'internal',
      failure?.message ?? `Le serveur a répondu ${response.status}.`,
    );
  }

  return payload as T;
}

/** createRoom ouvre une partie au nom du compte connecté. */
export function createRoom(): Promise<Session> {
  return request<Session>('/api/rooms', { method: 'POST' });
}

/** joinRoom fait entrer le compte connecté dans une partie existante. */
export function joinRoom(code: string): Promise<Session> {
  const normalized = code.trim().toUpperCase();

  return request<Session>(`/api/rooms/${encodeURIComponent(normalized)}/join`, {
    method: 'POST',
  });
}

/** fetchRoom vérifie qu'une partie existe et renvoie son résumé. */
export function fetchRoom(code: string): Promise<RoomSummary> {
  const normalized = code.trim().toUpperCase();

  return request<RoomSummary>(`/api/rooms/${encodeURIComponent(normalized)}`, {
    method: 'GET',
  });
}

/** fetchAccount dit qui est connecté : c'est le cookie de session qui répond. */
export function fetchAccount(): Promise<Account> {
  return request<Account>('/api/auth/me', { method: 'GET' });
}

/** signup crée un compte local et ouvre sa session. */
export function signup(username: string, password: string): Promise<Account> {
  return request<Account>('/api/auth/signup', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

/** login ouvre la session d'un compte existant. */
export function login(username: string, password: string): Promise<Account> {
  return request<Account>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

/**
 * completeAccount choisit le pseudo d'une connexion externe et ouvre la session.
 *
 * L'identité a déjà été vérifiée par le fournisseur : c'est le cookie de
 * tentative qui dit laquelle, et le serveur qui décide si elle vaut encore.
 */
export function completeAccount(username: string): Promise<Account> {
  return request<Account>('/api/auth/complete', {
    method: 'POST',
    body: JSON.stringify({ username }),
  });
}

/** logout ferme la session de compte. */
export async function logout(): Promise<void> {
  await request<null>('/api/auth/logout', { method: 'POST' });
}
