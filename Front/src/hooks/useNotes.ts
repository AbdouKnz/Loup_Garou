import { useCallback, useEffect, useState } from 'react';

/** Une marque du carnet privé : suspect, ou innocent. */
export type Note = 'suspect' | 'innocent';

/** Les marques d'une partie, rangées par identifiant de joueur. */
export type Notes = Record<string, Note>;

const PREFIX = 'loupgarou.notes.';

/** noteKey nomme l'entrée locale d'une partie. */
function noteKey(code: string): string {
  return `${PREFIX}${code}`;
}

/** cycleNote passe d'une marque à la suivante, puis à aucune. */
export function cycleNote(note: Note | undefined): Note | undefined {
  if (note === undefined) {
    return 'suspect';
  }
  if (note === 'suspect') {
    return 'innocent';
  }

  return undefined;
}

/** loadNotes relit le carnet d'une partie, ou repart d'un carnet vide. */
export function loadNotes(code: string): Notes {
  if (typeof window === 'undefined' || typeof window.localStorage === 'undefined') {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(noteKey(code));
    if (!raw) {
      return {};
    }

    const parsed: unknown = JSON.parse(raw);
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {};
    }

    const notes: Notes = {};
    for (const [id, value] of Object.entries(parsed)) {
      if (value === 'suspect' || value === 'innocent') {
        notes[id] = value;
      }
    }

    return notes;
  } catch {
    return {};
  }
}

/** saveNotes range le carnet local, sans jamais faire échouer la partie. */
export function saveNotes(code: string, notes: Notes): void {
  if (typeof window === 'undefined' || typeof window.localStorage === 'undefined') {
    return;
  }

  try {
    window.localStorage.setItem(noteKey(code), JSON.stringify(notes));
  } catch {
    // Stockage plein ou refusé : le carnet reste en mémoire pour cette session.
  }
}

/**
 * useNotes tient le carnet privé du joueur pour une partie.
 *
 * Le carnet ne quitte jamais le navigateur : il ne passe ni par le serveur ni
 * par les autres joueurs. La clé comprend le code de la partie, pour qu'une
 * nouvelle table ne retrouve pas les marques de la précédente.
 */
export function useNotes(code: string): { notes: Notes; cycle: (playerId: string) => void } {
  // Le code de la partie est gardé avec les notes : si la table change, le
  // carnet se recharge avant que la sauvegarde ne puisse écrire au mauvais endroit.
  const [stored, setStored] = useState(() => ({ code, notes: loadNotes(code) }));

  if (stored.code !== code) {
    setStored({ code, notes: loadNotes(code) });
  }

  useEffect(() => {
    saveNotes(stored.code, stored.notes);
  }, [stored]);

  const cycle = useCallback((playerId: string) => {
    setStored((current) => {
      const notes = { ...current.notes };
      const next = cycleNote(notes[playerId]);

      if (next === undefined) {
        delete notes[playerId];
      } else {
        notes[playerId] = next;
      }

      return { ...current, notes };
    });
  }, []);

  return { notes: stored.notes, cycle };
}
