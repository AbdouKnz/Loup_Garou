import type { Note, Notes } from '../../hooks/useNotes';
import type { PlayerView } from '../../api/types';

type Props = {
  players: PlayerView[];
  you: string;
  notes: Notes;
  onCycle: (playerId: string) => void;
};

/** Libellé de la marque courante, pour le bouton. */
function currentLabel(note: Note | undefined): string {
  if (note === 'suspect') {
    return '🔴 Suspect';
  }
  if (note === 'innocent') {
    return '🟢 Innocent';
  }

  return '— Aucune';
}

/** nextLabel dit ce que le prochain clic posera. */
function nextLabel(note: Note | undefined): string {
  if (note === undefined) {
    return 'suspect';
  }
  if (note === 'suspect') {
    return 'innocent';
  }

  return 'aucune';
}

/**
 * NotesPanel est le carnet privé du joueur : ses soupçons, pour lui seul.
 *
 * Les marques restent dans le navigateur. Elles ne partent jamais vers le
 * serveur, et aucun autre joueur ne les voit : c'est un aide-mémoire, pas une
 * information de jeu.
 */
export default function NotesPanel({ players, you, notes, onCycle }: Props) {
  return (
    <details className="panel notes-panel">
      <summary>
        Mon carnet <span className="count">(privé)</span>
      </summary>

      <p className="hint">
        Vos soupçons restent sur cet appareil. Ils ne sont jamais envoyés aux autres joueurs.
      </p>

      <ul className="notes-list">
        {players.map((player) => {
          const note = notes[player.id];

          return (
            <li key={player.id}>
              <span className="notes-name">
                {player.pseudo}
                {player.id === you && <span className="tag-you">vous</span>}
                {player.dead && <span className="badge gone">mort</span>}
              </span>
              <button
                type="button"
                className={note ? `note-toggle ${note}` : 'note-toggle'}
                onClick={() => onCycle(player.id)}
                aria-label={`${player.pseudo} : ${currentLabel(note)}. Marquer comme ${nextLabel(note)}.`}
              >
                {currentLabel(note)}
              </button>
            </li>
          );
        })}
      </ul>
    </details>
  );
}
