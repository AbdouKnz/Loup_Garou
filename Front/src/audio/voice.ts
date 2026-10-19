/**
 * Voix du narrateur, confiée à la synthèse vocale du navigateur.
 *
 * Aucun fichier audio ni dépendance : on s'appuie sur l'API Web Speech quand
 * elle existe. Un navigateur qui ne l'expose pas — ou l'environnement de test —
 * laisse simplement la narration muette, sans jamais bloquer le jeu.
 *
 * La voix est un réglage à part du son : couper les repères sonores ne coupe
 * pas le récit, et inversement.
 */

/** Ce qu'un moteur de voix doit savoir faire : injectable dans les tests. */
export type VoiceEngine = {
  /**
   * speak lit un texte, en remplaçant le récit en cours.
   *
   * Renvoie vrai si la lecture a réellement démarré. onEnd est appelé quand
   * elle s'achève — ou échoue —, une seule fois.
   */
  speak(text: string, onEnd?: () => void): boolean;
  /** cancel interrompt le récit en cours. */
  cancel(): void;
  /** setMuted coupe ou rétablit la voix. */
  setMuted(muted: boolean): void;
};

/**
 * Délai accordé à la synthèse pour démarrer.
 *
 * Passé ce cap, on considère que le navigateur n'a pas lu — politique
 * d'autoplay, voix absente — et le récit est déclaré terminé : l'interface ne
 * reste pas suspendue à une voix qui ne viendra jamais.
 */
const START_GRACE_MS = 1500;

/** synthesis renvoie l'API de synthèse du navigateur, si elle existe. */
function synthesis(): SpeechSynthesis | null {
  if (typeof window === 'undefined' || typeof window.speechSynthesis === 'undefined') {
    return null;
  }

  return window.speechSynthesis;
}

/**
 * createVoiceEngine construit le moteur réel.
 *
 * Rien n'est demandé au navigateur avant le premier récit. L'API est relue à
 * chaque appel : un navigateur sans voix reste silencieux, et les tests
 * peuvent la remplacer sans recharger le module.
 */
export function createVoiceEngine(): VoiceEngine {
  let muted = false;

  return {
    setMuted(value) {
      muted = value;
      if (value) {
        synthesis()?.cancel();
      }
    },

    cancel() {
      synthesis()?.cancel();
    },

    speak(text, onEnd) {
      const clean = text.trim();
      if (muted || clean === '') {
        return false;
      }

      const synth = synthesis();
      if (!synth || typeof SpeechSynthesisUtterance === 'undefined') {
        return false;
      }

      // Une seule voix à la fois : un nouveau récit remplace le précédent.
      synth.cancel();

      const utterance = new SpeechSynthesisUtterance(clean);
      utterance.lang = 'fr-FR';
      utterance.rate = 1;
      utterance.pitch = 1;

      // Une voix française donne le bon accent ; à défaut, la langue suffit.
      const voices = typeof synth.getVoices === 'function' ? synth.getVoices() : [];
      const french = voices.find((voice) => voice.lang.toLowerCase().startsWith('fr'));
      if (french) {
        utterance.voice = french;
      }

      let done = false;
      let grace: ReturnType<typeof setTimeout> | undefined;
      const stopGrace = () => {
        if (grace !== undefined) {
          clearTimeout(grace);
          grace = undefined;
        }
      };
      const finish = () => {
        if (done) {
          return;
        }
        done = true;
        stopGrace();
        onEnd?.();
      };

      utterance.onstart = stopGrace;
      utterance.onend = finish;
      utterance.onerror = finish;

      // Sans « onstart » dans le délai, la voix n'a pas démarré : on libère.
      grace = setTimeout(finish, START_GRACE_MS);

      synth.speak(utterance);

      return true;
    },
  };
}
