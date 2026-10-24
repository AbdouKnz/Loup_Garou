/**
 * Sons du jeu, synthétisés à la volée avec l'API Web Audio.
 *
 * Aucun fichier audio n'est livré : chaque repère est une petite suite de tons
 * dessinée ici, ce qui garde le projet sans dépendance ni binaire à versionner.
 * Le moteur ne joue rien tant que le navigateur n'y consent pas : la carte son
 * se crée au premier geste du joueur, et tout est silencieux jusque-là.
 *
 * Le choix d'un repère ne dépend que de ce que ce joueur a le droit de voir :
 * les événements et la phase qu'il reçoit. Un son ne peut donc jamais trahir un
 * rôle que le serveur n'a pas montré.
 */

import type { GameEvent, Phase } from '../api/types';
import { getAudioContext, unlockAudio } from './context';

/** Repère sonore : chaque moment du jeu a le sien. */
export type Cue =
  | 'roles'
  | 'night'
  | 'dawn'
  | 'debate'
  | 'vote'
  | 'verdict'
  | 'tie'
  | 'victim'
  | 'saved'
  | 'eliminated'
  | 'gameOver'
  | 'yourTurn'
  | 'turn'
  | 'join';

/** Une note du dessin sonore : fréquence, moment et durée. */
type Tone = {
  /** Fréquence en hertz. */
  freq: number;
  /** Décalage du début, en secondes, depuis l'instant de lecture. */
  at?: number;
  /** Durée de la note, en secondes. */
  duration: number;
  /** Forme d'onde (sinus, triangle…). */
  wave?: OscillatorType;
  /** Volume maximal de la note (0–1). */
  gain?: number;
};

/** Le dessin de chaque repère, volontairement bref et discret. */
const CUES: Record<Cue, Tone[]> = {
  roles: [
    { freq: 523.25, duration: 0.22, wave: 'triangle', gain: 0.08 },
    { freq: 659.25, at: 0.18, duration: 0.22, wave: 'triangle', gain: 0.07 },
    { freq: 783.99, at: 0.36, duration: 0.36, wave: 'triangle', gain: 0.06 },
  ],
  night: [
    { freq: 392.0, duration: 0.5, gain: 0.08 },
    { freq: 261.63, at: 0.28, duration: 0.8, gain: 0.07 },
  ],
  dawn: [
    { freq: 392.0, duration: 0.3, wave: 'triangle', gain: 0.07 },
    { freq: 523.25, at: 0.22, duration: 0.5, wave: 'triangle', gain: 0.06 },
  ],
  debate: [
    { freq: 587.33, duration: 0.16, gain: 0.06 },
    { freq: 587.33, at: 0.2, duration: 0.16, gain: 0.06 },
  ],
  vote: [
    { freq: 523.25, duration: 0.12, wave: 'square', gain: 0.045 },
    { freq: 523.25, at: 0.18, duration: 0.12, wave: 'square', gain: 0.045 },
  ],
  verdict: [
    { freq: 196.0, duration: 1.1, gain: 0.09 },
    { freq: 293.66, at: 0.02, duration: 1.0, gain: 0.045 },
  ],
  tie: [
    { freq: 440.0, duration: 0.18, wave: 'triangle', gain: 0.06 },
    { freq: 440.0, at: 0.26, duration: 0.18, wave: 'triangle', gain: 0.06 },
  ],
  victim: [
    { freq: 130.81, duration: 0.5, wave: 'sawtooth', gain: 0.07 },
    { freq: 98.0, at: 0.08, duration: 0.7, wave: 'sawtooth', gain: 0.05 },
  ],
  saved: [
    { freq: 523.25, duration: 0.2, gain: 0.07 },
    { freq: 659.25, at: 0.16, duration: 0.4, gain: 0.06 },
  ],
  eliminated: [
    { freq: 392.0, duration: 0.22, wave: 'triangle', gain: 0.07 },
    { freq: 293.66, at: 0.2, duration: 0.22, wave: 'triangle', gain: 0.07 },
    { freq: 196.0, at: 0.4, duration: 0.5, wave: 'triangle', gain: 0.06 },
  ],
  gameOver: [
    { freq: 523.25, duration: 0.24, wave: 'triangle', gain: 0.08 },
    { freq: 659.25, at: 0.2, duration: 0.24, wave: 'triangle', gain: 0.08 },
    { freq: 783.99, at: 0.4, duration: 0.24, wave: 'triangle', gain: 0.08 },
    { freq: 1046.5, at: 0.6, duration: 0.7, wave: 'triangle', gain: 0.07 },
  ],
  yourTurn: [{ freq: 880.0, duration: 0.18, gain: 0.06 }],
  // Le débat change d'orateur : un repère discret, pour suivre sans lire.
  turn: [{ freq: 659.25, duration: 0.12, wave: 'triangle', gain: 0.05 }],
  join: [{ freq: 740.0, duration: 0.1, gain: 0.045 }],
};

/**
 * cueForEvent traduit un fait du narrateur en repère sonore.
 *
 * Les nouvelles sans relief — le rappel d'une phase, une nuit tranquille, une
 * égalité sans conséquence — restent muettes : mieux vaut trop peu de sons que
 * du bruit à chaque ligne.
 */
export function cueForEvent(event: GameEvent): Cue | null {
  switch (event.kind) {
    case 'victime':
      return 'victim';
    case 'sauve':
      return 'saved';
    case 'elimine':
      return 'eliminated';
    case 'egalite':
      return 'tie';
    case 'fin':
      return 'gameOver';
    default:
      return null;
  }
}

/**
 * cueForPhase traduit un changement de phase en repère sonore.
 *
 * Le premier appel (aucune phase précédente) est muet : arriver sur une partie
 * en cours ne doit pas déclencher un son venu de nulle part. La fin de partie
 * est annoncée par l'événement « fin », pas par la phase : on ne la double pas.
 */
export function cueForPhase(phase: Phase, previous: Phase | null): Cue | null {
  if (previous === null || phase === previous) {
    return null;
  }

  switch (phase) {
    case 'roles':
      return 'roles';
    case 'nuit':
      return 'night';
    case 'aube':
      return 'dawn';
    case 'debat':
      return 'debate';
    case 'vote':
      return 'vote';
    case 'verdict':
      return 'verdict';
    default:
      return null;
  }
}

/** Ce qu'un moteur de son doit savoir faire : le rend injectable dans les tests. */
export type SoundEngine = {
  /** play joue un repère, sauf si le son est coupé. */
  play(cue: Cue): void;
  /** setMuted coupe ou rétablit le son. */
  setMuted(muted: boolean): void;
  /** unlock se charge au premier geste du joueur pour débloquer l'audio. */
  unlock(): void;
};

/**
 * createSoundEngine construit le moteur de son réel.
 *
 * Rien n'est alloué avant le premier repère : la carte son naît à la demande,
 * et un navigateur qui n'expose pas l'API Web Audio laisse simplement le moteur
 * silencieux — le jeu, lui, continue.
 */
export function createSoundEngine(): SoundEngine {
  let muted = false;

  return {
    unlock() {
      unlockAudio();
    },

    setMuted(value) {
      muted = value;
    },

    play(cue) {
      if (muted) {
        return;
      }

      const ctx = getAudioContext();
      if (!ctx) {
        return;
      }

      // Un navigateur peut suspendre la carte son tant qu'aucun geste n'a eu
      // lieu : on la relance, sans attendre, au moment de jouer.
      if (ctx.state === 'suspended') {
        void ctx.resume();
      }

      const start = ctx.currentTime;

      for (const tone of CUES[cue]) {
        const oscillator = ctx.createOscillator();
        const volume = ctx.createGain();
        const from = start + (tone.at ?? 0);
        const to = from + tone.duration;
        const peak = tone.gain ?? 0.07;

        oscillator.type = tone.wave ?? 'sine';
        oscillator.frequency.value = tone.freq;

        // Enveloppe courte : attaque douce, extinction douce, jamais de clic.
        volume.gain.setValueAtTime(0.0001, from);
        volume.gain.exponentialRampToValueAtTime(peak, from + 0.012);
        volume.gain.exponentialRampToValueAtTime(0.0001, to);

        oscillator.connect(volume).connect(ctx.destination);
        oscillator.start(from);
        oscillator.stop(to + 0.02);
      }
    },
  };
}
