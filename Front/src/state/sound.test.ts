import { describe, expect, it } from 'vitest';

import { createSoundStore } from './sound';
import type { SoundStorage } from './sound';
import type { Cue, SoundEngine } from '../audio/sounds';

/** Espionne le moteur : on vérifie ce qui lui est demandé, pas le son réel. */
function fakeEngine() {
  const requested = { plays: [] as Cue[], mutes: [] as boolean[], unlocks: 0 };

  const engine: SoundEngine = {
    play: (cue) => requested.plays.push(cue),
    setMuted: (muted) => requested.mutes.push(muted),
    unlock: () => {
      requested.unlocks += 1;
    },
  };

  return { engine, requested };
}

/** Stockage en mémoire : reflète le contenu du navigateur. */
function fakeStorage(initial = false): SoundStorage & { muted: boolean } {
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

describe('createSoundStore', () => {
  it('applique la préférence mémorisée dès l’ouverture', () => {
    const { engine, requested } = fakeEngine();

    const store = createSoundStore(fakeStorage(true), engine);

    expect(store.getState().muted).toBe(true);
    expect(requested.mutes).toEqual([true]);
  });

  it('coupe le son, s’en souvient et le dit au moteur', () => {
    const { engine, requested } = fakeEngine();
    const storage = fakeStorage(false);
    const store = createSoundStore(storage, engine);

    store.getState().setMuted(true);

    expect(store.getState().muted).toBe(true);
    expect(storage.muted).toBe(true);
    expect(requested.mutes).toEqual([false, true]);
  });

  it('inverse l’état au clic sur le bouton', () => {
    const store = createSoundStore(fakeStorage(false), fakeEngine().engine);

    store.getState().toggleMuted();

    expect(store.getState().muted).toBe(true);
  });

  it('transmet les repères et le déblocage au moteur', () => {
    const { engine, requested } = fakeEngine();
    const store = createSoundStore(fakeStorage(false), engine);

    store.getState().play('night');
    store.getState().unlock();

    expect(requested.plays).toEqual(['night']);
    expect(requested.unlocks).toBe(1);
  });
});
