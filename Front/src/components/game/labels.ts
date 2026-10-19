import type { Phase, Role } from '../../api/types';

/** Ce que chaque phase annonce, en un mot et une phrase. */
export const PHASES: Record<Phase, { emoji: string; title: string; hint: string }> = {
  lobby: { emoji: '🪑', title: 'Salle d’attente', hint: '' },
  roles: {
    emoji: '🎭',
    title: 'Distribution des rôles',
    hint: 'Chacun découvre son rôle : la partie commence.',
  },
  nuit: {
    emoji: '🌙',
    title: 'La nuit tombe',
    hint: 'Le village dort. Les rôles se réveillent un à un.',
  },
  aube: {
    emoji: '🌅',
    title: 'L’aube se lève',
    hint: 'Le village découvre ce que la nuit a laissé.',
  },
  debat: {
    emoji: '☀️',
    title: 'Le débat',
    hint: 'Qui soupçonnez-vous ? Parlez, écoutez, méfiez-vous.',
  },
  vote: { emoji: '🗳️', title: 'Le vote', hint: 'Désignez celui que le village doit éliminer.' },
  verdict: {
    emoji: '⚖️',
    title: 'Le verdict',
    hint: 'Le village découvre le visage de l’éliminé.',
  },
  fin: { emoji: '🏆', title: 'La partie est finie', hint: 'Tous les rôles sont révélés.' },
};

/** Nom lisible d'un rôle. */
export const ROLE_LABELS: Record<Role, string> = {
  villageois: 'Villageois',
  'loup-garou': 'Loup-Garou',
  voyante: 'Voyante',
  sorciere: 'Sorcière',
};

/** Emblème d'un rôle, pour la composition annoncée. */
export const ROLE_EMOJI: Record<Role, string> = {
  villageois: '🧑‍🌾',
  'loup-garou': '🐺',
  voyante: '🔮',
  sorciere: '🧪',
};

/** Petit rappel de ce que chaque rôle attend du joueur. */
export const ROLE_HINTS: Record<Role, string> = {
  villageois: 'Démasquez les loups pendant le jour, et tâchez de survivre à la nuit.',
  'loup-garou': 'Chaque nuit, dévorez un villageois. Le jour, jouez l’innocent.',
  voyante:
    'Chaque nuit, observez un joueur et découvrez son rôle. Le jour, guidez le village sans vous trahir.',
  sorciere:
    'Une potion de vie et une potion de mort, à usage unique : une seule décision par nuit, et vous connaissez la proie des loups.',
};
