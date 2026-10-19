// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { createVoiceEngine } from './voice';

const speak = vi.fn();
const cancel = vi.fn();

/** Une utterance factice : jsdom n'implémente pas la synthèse vocale. */
class FakeUtterance {
  text: string;
  lang = '';
  rate = 1;
  pitch = 1;
  voice: SpeechSynthesisVoice | null = null;
  onstart: (() => void) | null = null;
  onend: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(text: string) {
    this.text = text;
  }
}

/** lastUtterance renvoie l'utterance confiée au moteur par le dernier appel. */
function lastUtterance(): FakeUtterance {
  return speak.mock.calls.at(-1)?.[0] as unknown as FakeUtterance;
}

beforeEach(() => {
  Object.defineProperty(window, 'speechSynthesis', {
    configurable: true,
    value: { speak, cancel, getVoices: () => [] },
  });
  Object.defineProperty(window, 'SpeechSynthesisUtterance', {
    configurable: true,
    value: FakeUtterance,
  });
  speak.mockClear();
  cancel.mockClear();
});

afterEach(() => {
  delete (window as unknown as Record<string, unknown>).speechSynthesis;
  delete (window as unknown as Record<string, unknown>).SpeechSynthesisUtterance;
});

describe('createVoiceEngine', () => {
  it('lit le récit en français, en remplaçant la voix en cours', () => {
    const started = createVoiceEngine().speak('  Le jour se lève.  ');

    expect(started).toBe(true);
    expect(cancel).toHaveBeenCalled();
    expect(speak).toHaveBeenCalledTimes(1);

    const utterance = lastUtterance();
    expect(utterance.text).toBe('Le jour se lève.');
    expect(utterance.lang).toBe('fr-FR');

    // La voix démarre : le délai de garde est levé.
    utterance.onstart?.();
  });

  it('prévient une seule fois la fin de la lecture', () => {
    const onEnd = vi.fn();
    createVoiceEngine().speak('Le jour se lève.', onEnd);
    const utterance = lastUtterance();

    utterance.onstart?.();
    expect(onEnd).not.toHaveBeenCalled();

    utterance.onend?.();
    utterance.onerror?.();
    expect(onEnd).toHaveBeenCalledTimes(1);
  });

  it('prévient aussi quand la voix échoue', () => {
    const onEnd = vi.fn();
    createVoiceEngine().speak('Le jour se lève.', onEnd);

    lastUtterance().onerror?.();

    expect(onEnd).toHaveBeenCalledTimes(1);
  });

  it('libère la lecture si la voix ne démarre jamais', () => {
    vi.useFakeTimers();
    try {
      const onEnd = vi.fn();
      createVoiceEngine().speak('Le jour se lève.', onEnd);

      expect(onEnd).not.toHaveBeenCalled();

      vi.advanceTimersByTime(1500);

      expect(onEnd).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('se tait quand la narration est coupée', () => {
    const engine = createVoiceEngine();
    engine.setMuted(true);

    const started = engine.speak('Le jour se lève.');

    expect(started).toBe(false);
    expect(speak).not.toHaveBeenCalled();
    expect(cancel).toHaveBeenCalled();
  });

  it('ne lit pas un texte vide', () => {
    expect(createVoiceEngine().speak('   ')).toBe(false);
    expect(speak).not.toHaveBeenCalled();
  });

  it('reste muet sans API de synthèse, sans jamais échouer', () => {
    delete (window as unknown as Record<string, unknown>).speechSynthesis;

    const engine = createVoiceEngine();

    expect(() => {
      engine.speak('Le jour se lève.');
      engine.cancel();
      engine.setMuted(true);
    }).not.toThrow();
    expect(speak).not.toHaveBeenCalled();
  });
});
