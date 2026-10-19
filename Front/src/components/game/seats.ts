import type { CSSProperties } from 'react';

/**
 * seatStyle place un siège sur le pourtour de la table ronde.
 *
 * On part du haut de la table et on tourne dans le sens des aiguilles d'une
 * montre : le premier de la liste se tient face au narrateur. Deux rayons
 * distincts parce que la table est plus large que haute — c'est ce qui donne
 * l'ellipse plutôt qu'un cercle écrasé.
 */
export function seatStyle(index: number, total: number): CSSProperties {
  const seats = Math.max(1, total);
  const angle = ((-90 + (index * 360) / seats) * Math.PI) / 180;

  return {
    '--seat-x': `${(50 + SEAT_RADIUS_X * Math.cos(angle)).toFixed(2)}%`,
    '--seat-y': `${(50 + SEAT_RADIUS_Y * Math.sin(angle)).toFixed(2)}%`,
  } as CSSProperties;
}

/** Rayons de l'ellipse, en pourcentage du plateau. */
const SEAT_RADIUS_X = 43;
const SEAT_RADIUS_Y = 38;
