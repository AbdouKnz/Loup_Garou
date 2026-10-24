// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';

import Game from './Game';
import { useRoomStore } from '../state/room';
import { useSoundStore } from '../state/sound';
import { MAX_PLAYERS } from '../api/types';
import type { Cue } from '../audio/sounds';
import type { GameEvent, GameView, PlayerView, RoomState } from '../api/types';

/**
 * Ces tests vérifient ce que l'écran de jeu demande au moteur de son.
 *
 * On remplace la fonction « play » du store par un espion : le son réel n'est
 * jamais joué, seul le choix du repère est vérifié.
 */

const roster: PlayerView[] = [
  { id: 'p1', pseudo: 'Alice', ready: true, online: true, host: true },
  { id: 'p2', pseudo: 'Bob', ready: true, online: true, host: false },
];

/** scene assemble une salle et sa partie, comme le serveur les enverrait. */
function scene(phase: GameView['phase'], overrides: Partial<GameView> = {}) {
  const view: GameView = {
    phase,
    day: 1,
    round: 1,
    secondsLeft: 30,
    alive: roster.map((player) => player.id),
    dead: [],
    ...overrides,
  };

  const room: RoomState = {
    code: 'ABC234',
    phase: view.phase,
    hostId: 'p1',
    you: 'p1',
    maxPlayers: MAX_PLAYERS,
    canStart: false,
    players: roster,
    messages: [],
    composition: [],
    game: view,
  };

  return { room, game: view };
}

/** Repères demandés au moteur, dans l'ordre. */
const joue: Cue[] = [];
const vraiPlay = useSoundStore.getState().play;

beforeEach(() => {
  useRoomStore.setState({
    connection: 'online',
    event: null,
    eventSeq: 0,
    history: [],
    phaseKey: '',
    phaseTotal: 30,
    error: null,
  });
  joue.length = 0;
  useSoundStore.setState({ play: (cue) => joue.push(cue) });
});

afterEach(() => {
  cleanup();
  useSoundStore.setState({ play: vraiPlay });
});

describe('Game — sons', () => {
  it('ne sonne pas à l’ouverture sur une phase en cours', () => {
    render(<Game {...scene('debat')} />);

    expect(joue).toEqual([]);
  });

  it('sonne le passage d’une phase à l’autre', () => {
    const { rerender } = render(<Game {...scene('debat')} />);

    rerender(<Game {...scene('vote')} />);

    expect(joue).toContain('vote');
  });

  it('sonne la veillée qui attend le joueur', () => {
    render(<Game {...scene('nuit', { yourRole: 'voyante', yourAction: 'voir' })} />);

    expect(joue).toContain('yourTurn');
  });

  it('sonne chaque prise de parole du débat, et l’attente quand c’est à moi', () => {
    const { rerender } = render(<Game {...scene('debat', { speaker: 'p2' })} />);
    // La parole est à Bob : un repère discret, pas encore celui de l'attente.
    expect(joue).toContain('turn');
    expect(joue).not.toContain('yourTurn');

    rerender(<Game {...scene('debat', { speaker: 'p1' })} />);
    // Elle me revient : c'est le repère de l'attente.
    expect(joue).toContain('yourTurn');
  });

  it('ne rejoue pas un fait rangé avant l’ouverture, mais sonne le suivant', () => {
    const fin: GameEvent = { kind: 'fin', text: '', phase: 'fin', day: 2, round: 1 };
    useRoomStore.setState({ event: fin, eventSeq: 3 });

    render(<Game {...scene('aube')} />);
    // Le fait qui précède l'ouverture de l'écran reste muet.
    expect(joue).toEqual([]);

    act(() => {
      useRoomStore.setState({
        event: { ...fin, kind: 'victime', phase: 'aube' },
        eventSeq: 4,
      });
    });
    // Un fait qui arrive ensuite, lui, sonne.
    expect(joue).toContain('victim');
  });
});
