/**
 * Ambiance du village, synthétisée à la volée avec l'API Web Audio.
 *
 * Aucun fichier audio n'est livré : le fond est un drone discret, un souffle de
 * vent filtré et une note très douce qui change de couleur avec la phase. Le
 * moteur ne démarre qu'après un geste du joueur, et reste silencieux si le
 * navigateur n'expose pas Web Audio.
 */

import { getAudioContext, unlockAudio } from './context';

/** Les quatre couleurs de l'ambiance, choisies par la phase. */
export type AmbienceMood = 'mystery' | 'night' | 'day' | 'tension';

/** Ce qu'un moteur d'ambiance doit savoir faire, pour le rendre injectable. */
export type AmbienceEngine = {
  /** start lance le fond, ou le relance s'il s'était arrêté. */
  start(): void;
  /** stop coupe le fond en douceur et libère ses nœuds. */
  stop(): void;
  /** setMood change la couleur du fond, sans coupure. */
  setMood(mood: AmbienceMood): void;
  /** setMuted coupe ou rétablit le fond, sans oublier le volume choisi. */
  setMuted(muted: boolean): void;
  /** setVolume règle le volume du fond, entre 0 et 1. */
  setVolume(volume: number): void;
  /** pause met le fond en veille, par exemple quand l'onglet est caché. */
  pause(): void;
  /** resume sort le fond de veille. */
  resume(): void;
  /** unlock débloque la carte son au premier geste du joueur. */
  unlock(): void;
};

/** Le dessin de chaque ambiance : fréquences du drone, du vent et du scintillement. */
type MoodPreset = {
  drone: number;
  wind: number;
  shimmer: number;
};

const MOODS: Record<AmbienceMood, MoodPreset> = {
  // Le village s'éveille : chaleur, souffle large, note claire.
  day: { drone: 110, wind: 620, shimmer: 330 },
  // La nuit : grave, souffle bas, note lointaine.
  night: { drone: 55, wind: 280, shimmer: 220 },
  // La distribution des rôles : entre deux, un peu inquiétant.
  mystery: { drone: 82.5, wind: 420, shimmer: 277.18 },
  // Le vote et le verdict : plus tendu, un battement dans le grave.
  tension: { drone: 65, wind: 340, shimmer: 233.08 },
};

const DEFAULT_VOLUME = 0.3;

/** clampVolume borne un volume entre 0 et 1, et retombe sur la valeur par défaut. */
function clampVolume(value: number): number {
  if (!Number.isFinite(value)) {
    return DEFAULT_VOLUME;
  }

  return Math.min(1, Math.max(0, value));
}

/** createAmbienceEngine construit le moteur d'ambiance réel. */
export function createAmbienceEngine(): AmbienceEngine {
  let ctx: AudioContext | null = null;
  let master: GainNode | null = null;
  let drone: OscillatorNode | null = null;
  let droneGain: GainNode | null = null;
  let wind: AudioBufferSourceNode | null = null;
  let windGain: GainNode | null = null;
  let windFilter: BiquadFilterNode | null = null;
  let shimmer: OscillatorNode | null = null;
  let shimmerGain: GainNode | null = null;

  let muted = false;
  let volume = DEFAULT_VOLUME;
  let mood: AmbienceMood = 'day';
  let playing = false;
  let paused = false;

  /** target renvoie le gain que le fond doit atteindre maintenant. */
  const target = (): number => (muted || paused ? 0 : volume);

  /** ramp fait glisser un paramètre vers une valeur, sans clic. */
  const ramp = (param: AudioParam | null | undefined, value: number, at: number): void => {
    if (!param) {
      return;
    }

    param.cancelScheduledValues(at);
    param.setValueAtTime(param.value, at);
    param.linearRampToValueAtTime(Math.max(0, value), at + 0.7);
  };

  /** apply applique le mode, le volume et l'état de veille au graphe courant. */
  const apply = (): void => {
    if (!ctx) {
      return;
    }

    const at = ctx.currentTime;
    const preset = MOODS[mood];

    if (drone) {
      drone.frequency.setTargetAtTime(preset.drone, at, 1.1);
    }
    if (windFilter) {
      windFilter.frequency.setTargetAtTime(preset.wind, at, 1.1);
    }
    if (shimmer) {
      shimmer.frequency.setTargetAtTime(preset.shimmer, at, 1.1);
    }

    ramp(master?.gain, target(), at);
  };

  /** build crée le graphe au premier son ; sans Web Audio, il ne fait rien. */
  const build = (): void => {
    if (master) {
      return;
    }

    ctx = getAudioContext();
    if (!ctx) {
      return;
    }

    unlockAudio();
    if (ctx.state === 'suspended') {
      void ctx.resume();
    }

    master = ctx.createGain();
    master.gain.setValueAtTime(0, ctx.currentTime);
    master.connect(ctx.destination);

    // Le drone : la note grave qui porte toute l'ambiance.
    drone = ctx.createOscillator();
    drone.type = 'sine';
    drone.frequency.setValueAtTime(MOODS[mood].drone, ctx.currentTime);
    droneGain = ctx.createGain();
    droneGain.gain.setValueAtTime(0.12, ctx.currentTime);
    drone.connect(droneGain).connect(master);
    drone.start();

    // Le vent : un bruit blanc très filtré, bouclé sur deux secondes.
    const buffer = ctx.createBuffer(1, Math.floor(ctx.sampleRate * 2), ctx.sampleRate);
    const data = buffer.getChannelData(0);
    for (let index = 0; index < data.length; index += 1) {
      data[index] = Math.random() * 2 - 1;
    }

    wind = ctx.createBufferSource();
    wind.buffer = buffer;
    wind.loop = true;
    windFilter = ctx.createBiquadFilter();
    windFilter.type = 'lowpass';
    windFilter.frequency.setValueAtTime(MOODS[mood].wind, ctx.currentTime);
    windFilter.Q.setValueAtTime(0.7, ctx.currentTime);
    windGain = ctx.createGain();
    windGain.gain.setValueAtTime(0.055, ctx.currentTime);
    wind.connect(windFilter).connect(windGain).connect(master);
    wind.start();

    // Le scintillement : une note haute, à peine audible, qui donne de l'air.
    shimmer = ctx.createOscillator();
    shimmer.type = 'triangle';
    shimmer.frequency.setValueAtTime(MOODS[mood].shimmer, ctx.currentTime);
    shimmerGain = ctx.createGain();
    shimmerGain.gain.setValueAtTime(0.014, ctx.currentTime);
    shimmer.connect(shimmerGain).connect(master);
    shimmer.start();
  };

  return {
    start() {
      build();
      if (!ctx) {
        return;
      }

      playing = true;
      paused = false;
      unlockAudio();
      if (ctx.state === 'suspended') {
        void ctx.resume();
      }
      apply();
    },

    stop() {
      playing = false;
      if (!ctx) {
        return;
      }

      const at = ctx.currentTime;
      if (master) {
        master.gain.cancelScheduledValues(at);
        master.gain.setValueAtTime(master.gain.value, at);
        master.gain.linearRampToValueAtTime(0, at + 0.5);
      }

      const stopAt = at + 0.55;
      for (const node of [drone, wind, shimmer]) {
        if (node) {
          try {
            node.stop(stopAt);
          } catch {
            // Le nœud était déjà arrêté : rien à faire.
          }
        }
      }

      drone = null;
      droneGain = null;
      wind = null;
      windGain = null;
      windFilter = null;
      shimmer = null;
      shimmerGain = null;
      master = null;
    },

    setMood(next) {
      mood = next;
      if (playing) {
        apply();
      }
    },

    setMuted(value) {
      muted = value;
      if (playing) {
        apply();
      }
    },

    setVolume(value) {
      volume = clampVolume(value);
      if (playing) {
        apply();
      }
    },

    pause() {
      paused = true;
      if (playing) {
        apply();
      }
    },

    resume() {
      paused = false;
      if (playing) {
        apply();
      }
    },

    unlock() {
      unlockAudio();
    },
  };
}
