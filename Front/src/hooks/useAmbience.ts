import { useEffect, useRef } from 'react';

import { useAmbienceStore } from '../state/ambience';
import type { AmbienceMood } from '../audio/ambience';
import type { Phase } from '../api/types';

/** moodForPhase choisit la couleur de l'ambiance selon la phase. */
export function moodForPhase(phase: Phase): AmbienceMood {
  switch (phase) {
    case 'roles':
      return 'mystery';
    case 'nuit':
      return 'night';
    case 'aube':
    case 'debat':
      return 'day';
    case 'vote':
    case 'verdict':
      return 'tension';
    case 'fin':
      return 'day';
    default:
      return 'day';
  }
}

/**
 * useAmbience fait vivre le fond sonore avec la partie.
 *
 * Le navigateur n'autorise le son qu'après un geste : le premier clic ou la
 * première touche lance l'ambiance. Le mode suit ensuite la phase, l'onglet
 * caché met le fond en veille, et quitter l'écran l'arrête.
 */
export function useAmbience(phase: Phase): void {
  const muted = useAmbienceStore((state) => state.muted);
  const start = useAmbienceStore((state) => state.start);
  const stop = useAmbienceStore((state) => state.stop);
  const unlock = useAmbienceStore((state) => state.unlock);
  const setMood = useAmbienceStore((state) => state.setMood);
  const pause = useAmbienceStore((state) => state.pause);
  const resume = useAmbienceStore((state) => state.resume);

  const started = useRef(false);

  // Le mode est connu même avant le premier son : activer l'ambiance plus tard
  // ne doit pas la faire démarrer dans la couleur de la phase précédente.
  useEffect(() => {
    setMood(moodForPhase(phase));
  }, [phase, setMood]);

  // Le son ne démarre qu'après un geste. On retente au geste suivant si le
  // navigateur a suspendu la carte son entre-temps.
  useEffect(() => {
    if (muted) {
      return;
    }

    const begin = () => {
      unlock();
      if (started.current) {
        return;
      }

      started.current = true;
      start();
    };

    begin();
    window.addEventListener('pointerdown', begin);
    window.addEventListener('keydown', begin);

    return () => {
      window.removeEventListener('pointerdown', begin);
      window.removeEventListener('keydown', begin);
    };
  }, [muted, start, unlock]);

  // Réactiver l'ambiance en cours de partie la fait revenir sans attendre un
  // changement de phase.
  useEffect(() => {
    if (!muted && started.current) {
      start();
    }
  }, [muted, start]);

  // Un onglet caché ne consomme pas de CPU pour un fond que personne n'écoute.
  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        pause();
      } else {
        resume();
      }
    };

    document.addEventListener('visibilitychange', onVisibility);

    return () => document.removeEventListener('visibilitychange', onVisibility);
  }, [pause, resume]);

  // Quitter l'écran arrête le fond. Le drapeau repart à zéro pour que le
  // remontage de StrictMode en développement relance bien l'ambiance.
  useEffect(
    () => () => {
      started.current = false;
      stop();
    },
    [stop],
  );
}
