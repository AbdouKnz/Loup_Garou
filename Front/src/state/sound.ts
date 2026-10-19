import { create } from 'zustand';

import { createSoundEngine } from '../audio/sounds';
import type { Cue, SoundEngine } from '../audio/sounds';

/** Accès au stockage local du réglage de son : séparé pour être simulable. */
export type SoundStorage = {
  loadMuted(): boolean;
  saveMuted(muted: boolean): void;
};

const MUTE_KEY = 'loupgarou.sound.muted';

/** createLocalSoundStorage range la préférence dans le navigateur. */
export function createLocalSoundStorage(): SoundStorage {
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
  };
}

export type SoundStoreState = {
  /** Vrai quand le joueur a coupé le son : la préférence est mémorisée. */
  muted: boolean;
  /** setMuted coupe ou rétablit le son, et s'en souvient. */
  setMuted(muted: boolean): void;
  /** toggleMuted inverse l'état courant. */
  toggleMuted(): void;
  /** play joue un repère, sauf si le son est coupé. */
  play(cue: Cue): void;
  /** unlock débloque la carte son au premier geste du joueur. */
  unlock(): void;
};

/** createSoundStore construit un store isolé : un par onglet, un par test. */
export function createSoundStore(storage: SoundStorage, engine: SoundEngine) {
  // La préférence mémorisée vaut dès l'ouverture : le moteur la connaît avant
  // qu'un seul repère ne soit joué.
  const muted = storage.loadMuted();
  engine.setMuted(muted);

  return create<SoundStoreState>((set, get) => ({
    muted,

    setMuted(value) {
      engine.setMuted(value);
      storage.saveMuted(value);
      set({ muted: value });
    },

    toggleMuted: () => get().setMuted(!get().muted),

    play: (cue) => engine.play(cue),

    unlock: () => engine.unlock(),
  }));
}

/** Store de l'application : le son, partagé par les écrans. */
export const useSoundStore = createSoundStore(createLocalSoundStorage(), createSoundEngine());
