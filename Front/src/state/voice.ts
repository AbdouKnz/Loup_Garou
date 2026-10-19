import { create } from 'zustand';

import { createVoiceEngine } from '../audio/voice';
import type { VoiceEngine } from '../audio/voice';

/** Accès au stockage local du réglage de voix : séparé pour être simulable. */
export type VoiceStorage = {
  loadMuted(): boolean;
  saveMuted(muted: boolean): void;
};

const VOICE_KEY = 'loupgarou.voice.muted';

/** createLocalVoiceStorage range la préférence dans le navigateur. */
export function createLocalVoiceStorage(): VoiceStorage {
  const available = typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';

  return {
    loadMuted() {
      return available && window.localStorage.getItem(VOICE_KEY) === '1';
    },

    saveMuted(muted) {
      if (available) {
        window.localStorage.setItem(VOICE_KEY, muted ? '1' : '0');
      }
    },
  };
}

export type VoiceStoreState = {
  /** Vrai quand le joueur a coupé la narration vocale : le choix est mémorisé. */
  muted: boolean;
  /** setMuted coupe ou rétablit la voix, et s'en souvient. */
  setMuted(muted: boolean): void;
  /** toggleMuted inverse l'état courant. */
  toggleMuted(): void;
  /** speak fait lire un récit, sauf si la voix est coupée ; onEnd est appelé
   * quand la lecture s'achève. */
  speak(text: string, onEnd?: () => void): boolean;
  /** cancel interrompt le récit en cours. */
  cancel(): void;
};

/** createVoiceStore construit un store isolé : un par onglet, un par test. */
export function createVoiceStore(storage: VoiceStorage, engine: VoiceEngine) {
  // La préférence mémorisée vaut dès l'ouverture : le moteur la connaît avant
  // qu'un seul récit ne soit lu.
  const muted = storage.loadMuted();
  engine.setMuted(muted);

  return create<VoiceStoreState>((set, get) => ({
    muted,

    setMuted(value) {
      engine.setMuted(value);
      storage.saveMuted(value);
      set({ muted: value });
    },

    toggleMuted: () => get().setMuted(!get().muted),

    speak: (text, onEnd) => engine.speak(text, onEnd),

    cancel: () => engine.cancel(),
  }));
}

/** Store de l'application : la voix, partagée par les écrans. */
export const useVoiceStore = createVoiceStore(createLocalVoiceStorage(), createVoiceEngine());
