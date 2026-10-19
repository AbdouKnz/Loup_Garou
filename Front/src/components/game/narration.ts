import type { DeathCause, DeathView, GameEvent } from '../../api/types';

/** Le récit de l'aube : ce que la carte affiche, et ce que la voix lit. */
export type Narration = {
  /** Titre de la carte, sans emoji. */
  title: string;
  /** Lignes affichées, emoji compris. */
  lines: string[];
  /** Texte lu par la voix, sans emoji : elle ne doit pas les épeler. */
  spoken: string;
};

/** Ce qu'on dit d'un mort si le fil du narrateur n'est pas encore arrivé. */
const DEATH_LINES: Record<DeathCause, string> = {
  loups: 'a été dévoré pendant la nuit.',
  poison: 'est mort dans d’étranges circonstances.',
};

/** Emoji et marques de présentation, que la synthèse vocale lirait à voix haute. */
const PRESENTATION = /[\p{Extended_Pictographic}\uFE0F\u200D]/gu;

/** toSpeech prépare un texte pour la voix : sans emoji, sans espaces doubles. */
export function toSpeech(text: string): string {
  return text.replace(PRESENTATION, '').replace(/\s+/g, ' ').trim();
}

/**
 * dawnNarration compose le récit de l'aube à partir des faits publics.
 *
 * Les phrases du narrateur font foi : ce sont celles que le serveur a publiées
 * pour la nuit qui s'achève. Si le fil n'est pas encore là (reprise en cours
 * d'aube), les morts de la vue prennent le relais — jamais un rôle, seulement
 * ce que tout le monde voit.
 */
export function dawnNarration(
  history: GameEvent[],
  day: number,
  deaths: DeathView[] | undefined,
  nameOf: (id: string) => string,
): Narration {
  const dawn = history.find(
    (event) => event.kind === 'phase' && event.phase === 'aube' && event.day === day,
  );

  const night = history.filter(
    (event) =>
      event.day === day &&
      (event.kind === 'victime' || event.kind === 'sauve' || event.kind === 'aucune-victime'),
  );

  const recap =
    night.length > 0
      ? night.map((event) => event.text)
      : (deaths ?? []).map((death) => `${nameOf(death.target)} ${DEATH_LINES[death.cause]}`);

  const lines = [dawn?.text ?? '🌅 Le jour se lève sur le village.', ...recap];

  return { title: `Nuit ${day}`, lines, spoken: toSpeech(lines.join(' ')) };
}
