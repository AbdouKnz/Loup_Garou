import { useSoundStore } from '../state/sound';

/**
 * SoundToggle coupe ou rétablit les sons.
 *
 * La préférence est mémorisée dans le navigateur : elle survit au
 * rafraîchissement, comme le choix d'une langue.
 */
export default function SoundToggle() {
  const muted = useSoundStore((state) => state.muted);
  const toggleMuted = useSoundStore((state) => state.toggleMuted);

  const label = muted ? 'Activer les sons' : 'Couper les sons';

  return (
    <button
      type="button"
      className="btn sound-toggle"
      onClick={toggleMuted}
      aria-pressed={!muted}
      title={label}
    >
      <span aria-hidden="true">{muted ? '🔇' : '🔊'}</span>
      <span className="visually-hidden">{label}</span>
    </button>
  );
}
