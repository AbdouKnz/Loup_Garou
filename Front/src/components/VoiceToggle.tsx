import { useVoiceStore } from '../state/voice';

/**
 * VoiceToggle coupe ou rétablit la narration vocale.
 *
 * C'est un réglage distinct du son : les repères sonores peuvent rester actifs
 * pendant que la voix se tait, et inversement. Le choix est mémorisé dans le
 * navigateur, comme celui du son.
 */
export default function VoiceToggle() {
  const muted = useVoiceStore((state) => state.muted);
  const toggleMuted = useVoiceStore((state) => state.toggleMuted);

  const label = muted ? 'Activer la narration vocale' : 'Couper la narration vocale';

  return (
    <button
      type="button"
      className="btn sound-toggle voice-toggle"
      onClick={toggleMuted}
      aria-pressed={!muted}
      title={label}
    >
      <span aria-hidden="true">{muted ? '🤐' : '🗣️'}</span>
      <span className="visually-hidden">{label}</span>
    </button>
  );
}
