import type { CSSProperties } from 'react';

/**
 * Emojis d'avatar : un par place de la table (12 joueurs au plus).
 *
 * Les pastilles viennent de la place à table, pas du joueur : deux joueurs ont
 * donc toujours deux pastilles distinctes, et une pastille ne change pas tant
 * que la table ne bouge pas.
 */
export const AVATARS = ['🦊', '🐻', '🐼', '🐨', '🐯', '🦁', '🐮', '🐷', '🐸', '🐵', '🐔', '🦉'];

/** avatarEmoji donne l'emoji d'une place à table. */
export function avatarEmoji(index: number): string {
  return AVATARS[index % AVATARS.length];
}

/** avatarStyle donne la teinte d'une place à table, répartie sur le cercle. */
export function avatarStyle(index: number): CSSProperties {
  return { '--avatar-hue': Math.round((index * 360) / AVATARS.length) } as CSSProperties;
}
