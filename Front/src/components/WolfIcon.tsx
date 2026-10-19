/**
 * WolfIcon est la marque du jeu : une tête de loup dessinée au trait.
 *
 * Elle est décorative — le nom « Loup-Garou » l'accompagne toujours — donc elle
 * se tait pour les lecteurs d'écran. La couleur vient de `currentColor`.
 */
export function WolfIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M4 3.5 8.2 5.3a9 9 0 0 1 7.6 0L20 3.5l-.8 4.4a8.3 8.3 0 0 1 1.3 4.4c0 4.6-3.8 8.3-8.5 8.3S3.5 17 3.5 12.3a8.3 8.3 0 0 1 1.3-4.4L4 3.5Z" />
      <path d="M9.3 11.4h.01M14.7 11.4h.01" strokeWidth="2.6" />
      <path d="M10.6 15.3h2.8l-1.4 1.5-1.4-1.5Z" />
    </svg>
  );
}
