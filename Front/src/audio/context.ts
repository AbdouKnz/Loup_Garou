/**
 * Carte son partagée par les repères sonores et l'ambiance.
 *
 * Un navigateur limite le nombre de cartes son d'une page, et les deux moteurs
 * n'ont aucune raison d'en ouvrir une chacun. La carte naît au premier geste
 * qui en a besoin ; tant qu'aucune API Web Audio n'existe, les moteurs restent
 * silencieux sans jamais échouer.
 */

type AudioContextConstructor = new () => AudioContext;

let shared: AudioContext | null = null;

/** audioContextConstructor trouve l'API audio du navigateur, s'il en a une. */
function audioContextConstructor(): AudioContextConstructor | undefined {
  if (typeof window === 'undefined') {
    return undefined;
  }

  const legacy = window as unknown as { webkitAudioContext?: AudioContextConstructor };

  return window.AudioContext ?? legacy.webkitAudioContext;
}

/** getAudioContext renvoie la carte son, ou null si le navigateur n'en offre pas. */
export function getAudioContext(): AudioContext | null {
  if (shared) {
    return shared;
  }

  const Constructor = audioContextConstructor();
  if (!Constructor) {
    return null;
  }

  try {
    shared = new Constructor();
  } catch {
    return null;
  }

  return shared;
}

/** unlockAudio relance une carte son suspendue par le navigateur. */
export function unlockAudio(): void {
  const ctx = getAudioContext();
  if (ctx && ctx.state === 'suspended') {
    void ctx.resume();
  }
}

/**
 * resetAudioContextForTests oublie la carte son partagée.
 *
 * Les tests qui simulent l'API audio doivent repartir d'une ardoise vierge :
 * sans cela, une carte son créée dans un test resterait collée au suivant.
 */
export function resetAudioContextForTests(): void {
  shared = null;
}
