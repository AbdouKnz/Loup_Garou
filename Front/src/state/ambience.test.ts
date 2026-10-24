import { describe, expect, it } from 'vitest';

import { clampVolume, createAmbienceStore } from './ambience';
import type { AmbienceEngine, AmbienceMood } from '../audio/ambience';

/** Un moteur factice : on vérifie les ordres reçus, jamais le son produit. */
function fakeEngine() {
  const calls = {
    muted: [] as boolean[],
    volume: [] as number[],
    mood: [] as AmbienceMood[],
    start: 0,
    stop: 0,
    unlock: 0,
    pause: 0,
    resume: 0,
  };

  const engine: AmbienceEngine = {
    start() {
      calls.start += 1;
    },
    stop() {
      calls.stop += 1;
    },
    setMood(mood) {
      calls.mood.push(mood);
    },
    setMuted(muted) {
      calls.muted.push(muted);
    },
    setVolume(volume) {
      calls.volume.push(volume);
    },
    pause() {
      calls.pause += 1;
    },
    resume() {
      calls.resume += 1;
    },
    unlock() {
      calls.unlock += 1;
    },
  };

  return { engine, calls };
}

function fakeStorage(muted = false, volume = 0.3) {
  const state = { muted, volume };

  return {
    state,
    loadMuted: () => state.muted,
    saveMuted: (value: boolean) => {
      state.muted = value;
    },
    loadVolume: () => state.volume,
    saveVolume: (value: number) => {
      state.volume = value;
    },
  };
}

describe('clampVolume', () => {
  it('borne le volume et retombe sur la valeur par défaut', () => {
    expect(clampVolume(2)).toBe(1);
    expect(clampVolume(-1)).toBe(0);
    expect(clampVolume(Number.NaN)).toBe(0.3);
  });
});

describe('createAmbienceStore', () => {
  it('applique la préférence mémorisée au moteur dès l’ouverture', () => {
    const { engine, calls } = fakeEngine();
    const storage = fakeStorage(true, 0.42);

    const store = createAmbienceStore(storage, engine);

    expect(store.getState().muted).toBe(true);
    expect(store.getState().volume).toBe(0.42);
    expect(calls.muted).toEqual([true]);
    expect(calls.volume).toEqual([0.42]);
  });

  it('mémorise le mute et le volume, et pilote le moteur', () => {
    const { engine, calls } = fakeEngine();
    const storage = fakeStorage(false, 0.3);
    const store = createAmbienceStore(storage, engine);

    store.getState().toggleMuted();
    expect(store.getState().muted).toBe(true);
    expect(storage.state.muted).toBe(true);
    expect(calls.muted.at(-1)).toBe(true);

    store.getState().setVolume(2);
    expect(store.getState().volume).toBe(1);
    expect(storage.state.volume).toBe(1);

    store.getState().setMood('night');
    expect(store.getState().mood).toBe('night');
    expect(calls.mood.at(-1)).toBe('night');

    store.getState().start();
    store.getState().stop();
    store.getState().unlock();
    store.getState().pause();
    store.getState().resume();

    expect(calls.start).toBe(1);
    expect(calls.stop).toBe(1);
    expect(calls.unlock).toBe(1);
    expect(calls.pause).toBe(1);
    expect(calls.resume).toBe(1);
  });
});
