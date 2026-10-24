import type { GameEvent, GameView } from '../../api/types';

type Props = {
  game: GameView;
  history: GameEvent[];
  nameOf: (id: string) => string;
};

/**
 * DayRecap rappelle, juste avant le vote, ce que le jour a appris.
 *
 * Il ne lit que des faits publics : les nouvelles de la nuit déjà racontées et
 * le scrutin en cours. Aucun rôle n'y entre — le serveur ne les donne pas avant
 * le verdict. Si le fil local est vide (arrivée en cours de journée), le rappel
 * se contente de compter les vivants et les votants.
 */
export default function DayRecap({ game, history, nameOf }: Props) {
  if (game.phase !== 'vote') {
    return null;
  }

  const night = history.filter(
    (event) =>
      event.day === game.day &&
      (event.kind === 'victime' || event.kind === 'sauve' || event.kind === 'aucune-victime'),
  );

  const tally: Record<string, number> = {};
  for (const target of Object.values(game.ballot ?? {})) {
    if (target) {
      tally[target] = (tally[target] ?? 0) + 1;
    }
  }

  const leader = Object.entries(tally).sort((left, right) => right[1] - left[1])[0];
  const voted = game.voted?.length ?? Object.keys(game.ballot ?? {}).length;

  return (
    <section className="day-recap" aria-label={`Rappel du jour ${game.day}`}>
      <div className="day-recap-head">
        <h2>Rappel du jour {game.day}</h2>
        <span className="count">{game.alive.length} vivants</span>
      </div>

      {night.length > 0 ? (
        <ul>
          {night.map((event, index) => (
            <li key={`${event.kind}-${event.target ?? index}`}>{event.text}</li>
          ))}
        </ul>
      ) : (
        <p className="hint">Le fil de la nuit n’est plus affiché sur cet appareil.</p>
      )}

      <p className="hint">
        {voted} votant{voted > 1 ? 's' : ''}
        {leader ? ` · ${leader[1]} voix pour ${nameOf(leader[0])}` : ' · aucun vote pour le moment'}
      </p>
    </section>
  );
}
