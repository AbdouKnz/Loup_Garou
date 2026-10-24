import type { CSSProperties } from 'react';

/**
 * Emojis d'avatar : un par place de la table (12 joueurs au plus).
 *
 * Les pastilles viennent de la place à table, pas du joueur : deux joueurs ont
 * donc toujours deux pastilles distinctes, et une pastille ne change pas tant
 * que la table ne bouge pas.
 */
export const AVATARS = ['🦊', '🐻', '🐼', '🐨', '🐯', '🦁', '🐮', '🐷', '🐸', '🐵', '🐔', '🦉'];

/**
 * Douze nuances chaudes, du brun sombre à l'or. Elles gardent une teinte
 * distincte par place sans sortir de la palette nuit / clair de lune.
 */
const AVATAR_TONES = [8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30];

/** avatarEmoji donne l'emoji d'une place à table. */
export function avatarEmoji(index: number): string {
  return AVATARS[index % AVATARS.length];
}

/** avatarStyle donne la teinte d'une place à table, dans la palette chaude. */
export function avatarStyle(index: number): CSSProperties {
  const tone = AVATAR_TONES[index % AVATAR_TONES.length];
  return {
    '--avatar-tone': `${tone}%`,
    '--avatar-edge': `${Math.min(tone + 18, 52)}%`,
  } as CSSProperties;
}
