import { describe, expect, it } from 'vitest';

import { createVoiceStore } from './voice';
import type { VoiceStorage } from './voice';
import type { VoiceEngine } from '../audio/voice';

/** Espionne le moteur : on vérifie ce qui lui est demandé, pas la voix réelle. */
function fakeEngine() {
  const requested = {
    speaks: [] as string[],
    ends: [] as (() => void)[],
    cancels: 0,
    mutes: [] as boolean[],
  };

  const engine: VoiceEngine = {
    speak: (text, onEnd) => {
      requested.speaks.push(text);
      if (onEnd) {
        requested.ends.push(onEnd);
      }
      return true;
    },
    cancel: () => {
      requested.cancels += 1;
    },
    setMuted: (muted) => requested.mutes.push(muted),
  };

  return { engine, requested };
}

/** Stockage en mémoire : reflète le contenu du navigateur. */
function fakeStorage(initial = false): VoiceStorage & { muted: boolean } {
  return {
    muted: initial,
    loadMuted() {
      return this.muted;
    },
    saveMuted(muted: boolean) {
      this.muted = muted;
    },
  };
}

describe('createVoiceStore', () => {
  it('applique la préférence mémorisée dès l’ouverture', () => {
    const { engine, requested } = fakeEngine();
    const store = createVoiceStore(fakeStorage(true), engine);

    expect(store.getState().muted).toBe(true);
    expect(requested.mutes).toEqual([true]);
  });

  it('coupe la voix, s’en souvient et le dit au moteur', () => {
    const { engine, requested } = fakeEngine();
    const storage = fakeStorage(false);
    const store = createVoiceStore(storage, engine);

    store.getState().setMuted(true);

    expect(store.getState().muted).toBe(true);
    expect(storage.muted).toBe(true);
    expect(requested.mutes).toEqual([false, true]);
  });

  it('inverse l’état au clic sur le bouton', () => {
    const store = createVoiceStore(fakeStorage(false), fakeEngine().engine);

    store.getState().toggleMuted();

    expect(store.getState().muted).toBe(true);
  });

  it('transmet le récit, sa fin et l’interruption au moteur', () => {
    const { engine, requested } = fakeEngine();
    const store = createVoiceStore(fakeStorage(false), engine);
    const onEnd = () => {};

    expect(store.getState().speak('Le jour se lève.', onEnd)).toBe(true);
    store.getState().cancel();

    expect(requested.speaks).toEqual(['Le jour se lève.']);
    expect(requested.ends).toEqual([onEnd]);
    expect(requested.cancels).toBe(1);
  });
});
