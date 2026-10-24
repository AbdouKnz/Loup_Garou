import { useAmbienceStore } from '../state/ambience';

/**
 * AmbienceToggle coupe ou rétablit le fond sonore.
 *
 * C'est un réglage distinct des repères sonores et de la voix : le joueur peut
 * garder les sons du jeu tout en faisant taire l'ambiance, et inversement.
 */
export default function AmbienceToggle() {
  const muted = useAmbienceStore((state) => state.muted);
  const toggleMuted = useAmbienceStore((state) => state.toggleMuted);
  const unlock = useAmbienceStore((state) => state.unlock);

  const label = muted ? 'Activer l’ambiance' : 'Couper l’ambiance';

  return (
    <button
      type="button"
      className="btn sound-toggle ambience-toggle"
      onClick={() => {
        // Le clic est un geste : on en profite pour débloquer la carte son.
        unlock();
        toggleMuted();
      }}
      aria-pressed={!muted}
      title={label}
    >
      <span aria-hidden="true">{muted ? '🔇' : '🎵'}</span>
      <span className="visually-hidden">{label}</span>
    </button>
  );
}
