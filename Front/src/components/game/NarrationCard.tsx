import type { Narration } from './narration';

type Props = {
  narration: Narration;
  /** Vrai quand la narration vocale est coupée : la relecture est alors offerte
   * mais désactivée, pour ne pas promettre un son qui ne viendra pas. */
  muted: boolean;
  onReplay: () => void;
  onClose: () => void;
};

/**
 * NarrationCard est le récit de l'aube : une carte qui résume la nuit et que la
 * voix lit en même temps.
 *
 * Son texte vient des faits publics du narrateur, jamais d'une déduction. Elle
 * n'est pas une région vivante — les révélations de l'aube sont déjà annoncées
 * par la scène — pour ne pas faire parler deux lecteurs d'écran à la fois.
 */
export default function NarrationCard({ narration, muted, onReplay, onClose }: Props) {
  return (
    <aside className="narration-card" aria-labelledby="narration-title">
      <p className="narration-eyebrow">
        <span aria-hidden="true">🎙️</span> Narration
      </p>

      <h2 id="narration-title" className="narration-title">
        {narration.title}
      </h2>

      <ul className="narration-lines">
        {narration.lines.map((line, index) => (
          <li key={index}>{line}</li>
        ))}
      </ul>

      <div className="actions">
        <button
          type="button"
          className="btn small"
          onClick={onReplay}
          disabled={muted}
          title={muted ? 'La narration vocale est coupée.' : 'Lire le récit à voix haute.'}
        >
          🔁 Réécouter
        </button>
        <button type="button" className="btn small" onClick={onClose}>
          Masquer
        </button>
      </div>
    </aside>
  );
}
