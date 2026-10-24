import { create } from 'zustand';

import { createAmbienceEngine } from '../audio/ambience';
import type { AmbienceEngine, AmbienceMood } from '../audio/ambience';

/** Accès au stockage local des réglages d'ambiance : séparé pour être simulable. */
export type AmbienceStorage = {
  loadMuted(): boolean;
  saveMuted(muted: boolean): void;
  loadVolume(): number;
  saveVolume(volume: number): void;
};

const MUTE_KEY = 'loupgarou.ambience.muted';
const VOLUME_KEY = 'loupgarou.ambience.volume';
const DEFAULT_VOLUME = 0.3;

/** clampVolume borne un volume entre 0 et 1. */
export function clampVolume(value: number): number {
  if (!Number.isFinite(value)) {
    return DEFAULT_VOLUME;
  }

  return Math.min(1, Math.max(0, value));
}

/** createLocalAmbienceStorage range les préférences dans le navigateur. */
export function createLocalAmbienceStorage(): AmbienceStorage {
  const available = typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';

  return {
    loadMuted() {
      return available && window.localStorage.getItem(MUTE_KEY) === '1';
    },

    saveMuted(muted) {
      if (available) {
        window.localStorage.setItem(MUTE_KEY, muted ? '1' : '0');
      }
    },

    loadVolume() {
      if (!available) {
        return DEFAULT_VOLUME;
      }

      const stored = Number.parseFloat(window.localStorage.getItem(VOLUME_KEY) ?? '');
      return Number.isFinite(stored) ? clampVolume(stored) : DEFAULT_VOLUME;
    },

    saveVolume(volume) {
      if (available) {
        window.localStorage.setItem(VOLUME_KEY, String(clampVolume(volume)));
      }
    },
  };
}

export type AmbienceStoreState = {
  /** Vrai quand le joueur a coupé l'ambiance : la préférence est mémorisée. */
  muted: boolean;
  /** Volume du fond, entre 0 et 1. */
  volume: number;
  /** Couleur actuellement demandée au moteur. */
  mood: AmbienceMood;
  setMuted(muted: boolean): void;
  toggleMuted(): void;
  setVolume(volume: number): void;
  setMood(mood: AmbienceMood): void;
  start(): void;
  stop(): void;
  unlock(): void;
  pause(): void;
  resume(): void;
};

/** createAmbienceStore construit un store isolé : un par test, un par onglet. */
export function createAmbienceStore(storage: AmbienceStorage, engine: AmbienceEngine) {
  // La préférence mémorisée vaut dès l'ouverture. Le moteur la connaît avant
  // qu'un seul son ne soit joué, et garde le volume choisi pour plus tard.
  const muted = storage.loadMuted();
  const volume = clampVolume(storage.loadVolume());
  engine.setMuted(muted);
  engine.setVolume(volume);

  return create<AmbienceStoreState>((set, get) => ({
    muted,
    volume,
    mood: 'day',

    setMuted(value) {
      engine.setMuted(value);
      storage.saveMuted(value);
      set({ muted: value });
    },

    toggleMuted: () => get().setMuted(!get().muted),

    setVolume(value) {
      const next = clampVolume(value);
      engine.setVolume(next);
      storage.saveVolume(next);
      set({ volume: next });
    },

    setMood(mood) {
      engine.setMood(mood);
      set({ mood });
    },

    start: () => engine.start(),
    stop: () => engine.stop(),
    unlock: () => engine.unlock(),
    pause: () => engine.pause(),
    resume: () => engine.resume(),
  }));
}

/** Store de l'application : l'ambiance, partagée par les écrans. */
export const useAmbienceStore = createAmbienceStore(
  createLocalAmbienceStorage(),
  createAmbienceEngine(),
);
