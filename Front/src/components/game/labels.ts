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

/** Ce que la règle détaille de chaque rôle, au-delà du simple rappel. */
export type RoleDetail = {
  camp: 'village' | 'loups';
  goal: string;
  night: string;
  day: string;
  tip: string;
};

/**
 * ROLE_DETAILS est le contenu du panneau de règles.
 *
 * Le type est indexé sur Role : ajouter un rôle au protocole obligera à écrire
 * sa règle ici, plutôt que de laisser un trou dans l'aide de jeu.
 */
export const ROLE_DETAILS: Record<Role, RoleDetail> = {
  villageois: {
    camp: 'village',
    goal: 'Survivre et démasquer les loups.',
    night: 'Vous dormez : aucune action, mais vous restez une cible.',
    day: 'Débattez, écoutez les autres, puis votez contre le suspect le plus probable.',
    tip: 'Votre vote est votre seule arme. Gardez vos soupçons pour vous, mais ne suivez pas un vote sans raison.',
  },
  'loup-garou': {
    camp: 'loups',
    goal: 'Dévorer les villageois jusqu’à ce que les loups soient aussi nombreux qu’eux.',
    night: 'Avec vos compères, désignez une victime. La Sorcière peut annuler l’attaque.',
    day: 'Faites-vous passer pour un villageois et orientez les votes loin de votre meute.',
    tip: 'Ne votez pas tous la même personne : un scrutin trop net trahit une entente.',
  },
  voyante: {
    camp: 'village',
    goal: 'Identifier les loups et guider le village.',
    night: 'Observez un joueur : vous découvrez son rôle, et vous seule le voyez.',
    day: 'Partagez ce que vous savez sans vous exposer trop tôt.',
    tip: 'Une révélation trop franche vous désigne aux loups : gardez une carte en réserve.',
  },
  sorciere: {
    camp: 'village',
    goal: 'Protéger le village et éliminer un loup.',
    night:
      'Vous connaissez la proie des loups. Une potion de vie ou de mort par nuit, chacune à usage unique.',
    day: 'Votez avec le village et gardez le secret de vos potions.',
    tip: 'La potion de vie annule une attaque ; la potion de mort tue sans révéler le rôle.',
  },
};
