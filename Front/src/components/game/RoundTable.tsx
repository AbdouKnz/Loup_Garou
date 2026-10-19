import Avatar from './Avatar';
import { seatStyle } from './seats';
import type { PlayerView } from '../../api/types';

type ActionProps = {
  /** Libellé de l'action proposée : « Voter », « Dévorer », « Observer »… */
  label: string;
  /** can dit si ce joueur est une cible autorisée au tour courant. */
  can: (player: PlayerView) => boolean;
  onChoose: (id: string) => void;
};

type Props = {
  players: PlayerView[];
  you: string;
  /** Cible déjà désignée au tour courant, pour la mettre en avant. */
  yourVote?: string;
  /** Présent quand la table peut agir : le siège devient le bouton. */
  action?: ActionProps;
  /** Joueurs dont on sait qu'ils sont éveillés — et seulement ceux-là. */
  awake?: string[];
  /** Joueur dont c'est le tour de parole (débat) : son siège prend la vedette. */
  speaking?: string;
  /** Cible marquée par la veillée en cours (la proie, pour la sorcière). */
  marked?: string;
  /** Voix reçues par chaque cible : les jetons du scrutin. */
  votes?: Record<string, number>;
  /** Ce que raconte le centre de la table. */
  center: { emoji: string; title: string; note: string };
};

/**
 * RoundTable est la table du village : les joueurs assis autour du plateau.
 *
 * Elle ne décide de rien. Elle place les sièges, montre l'état public de chacun,
 * et — quand le serveur a dit qui pouvait agir — fait de chaque cible un bouton.
 * Un siège ne s'éclaire jamais sur une information que le serveur n'a pas
 * donnée à ce joueur : « éveillé » veut dire « compère, ou vous-même ».
 */
export default function RoundTable({
  players,
  you,
  yourVote,
  action,
  awake,
  speaking,
  marked,
  votes,
  center,
}: Props) {
  const lit = new Set(awake ?? []);

  return (
    <div className="round-table" data-count={players.length}>
      <div className="table-surface">
        <p className="table-emoji" aria-hidden="true">
          {center.emoji}
        </p>
        <p className="table-title">{center.title}</p>
        <p className="table-note">{center.note}</p>
      </div>

      <ul className="seats">
        {players.map((player, index) => (
          <Seat
            key={player.id}
            player={player}
            index={index}
            total={players.length}
            you={you}
            chosen={yourVote === player.id}
            awake={lit.has(player.id)}
            speaking={speaking === player.id}
            marked={marked === player.id}
            votes={votes?.[player.id] ?? 0}
            action={action}
          />
        ))}
      </ul>
    </div>
  );
}

type SeatProps = {
  player: PlayerView;
  index: number;
  total: number;
  you: string;
  chosen: boolean;
  awake: boolean;
  speaking: boolean;
  marked: boolean;
  votes: number;
  action?: ActionProps;
};

/** Seat est une place à table : l'avatar, le pseudo, et l'état du joueur. */
function Seat({
  player,
  index,
  total,
  you,
  chosen,
  awake,
  speaking,
  marked,
  votes,
  action,
}: SeatProps) {
  const can = action !== undefined && action.can(player);

  const classes = [
    'seat',
    player.online ? '' : 'away',
    player.dead ? 'gone' : '',
    chosen ? 'chosen' : '',
    awake ? 'awake' : '',
    speaking ? 'speaking' : '',
    marked ? 'marked' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const content = (
    <>
      <Avatar index={index} dead={player.dead} />

      <span className="player-name">
        {player.pseudo}
        {player.id === you && <span className="tag-you">vous</span>}
      </span>

      <span className="player-badges">
        {votes > 0 && (
          <span className="badge votes" aria-label={`${votes} voix`}>
            ×{votes}
          </span>
        )}
        {player.host && <span className="badge host">hôte</span>}
        {player.bot && <span className="badge bot">bot</span>}
        {speaking && <span className="badge speaking">🎤</span>}
        {player.dead && <span className="badge gone">mort</span>}
        {!player.dead && player.voted && <span className="badge voted">a voté</span>}
        {!player.online && !player.dead && <span className="badge away">hors ligne</span>}
      </span>
    </>
  );

  return (
    <li className={classes} style={seatStyle(index, total)}>
      {can ? (
        <button
          type="button"
          className="seat-body actionable"
          aria-label={`${action.label} ${player.pseudo}`}
          onClick={() => action.onChoose(player.id)}
        >
          {content}
        </button>
      ) : (
        <div className="seat-body">{content}</div>
      )}
    </li>
  );
}
