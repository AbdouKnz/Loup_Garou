// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';

import Game from './Game';
import { useRoomStore } from '../state/room';
import { useVoiceStore } from '../state/voice';
import { MAX_PLAYERS } from '../api/types';
import type { GameEvent, GameView, PlayerView, RoomState } from '../api/types';

/**
 * Ces tests vérifient le récit de l'aube : la carte qui résume la nuit, et ce
 * que l'écran demande à la voix.
 *
 * On remplace « speak » et « cancel » du store par des espions : le moteur réel
 * n'est jamais appelé, seul le choix du texte et la durée d'affichage de la
 * carte sont vérifiés.
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
    alive: ['p2'],
    dead: ['p1'],
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

/** La nuit qui vient de s'achever, telle que le narrateur l'a publiée. */
const night: GameEvent[] = [
  {
    kind: 'victime',
    text: '🐺 Bob a été dévoré pendant la nuit.',
    phase: 'nuit',
    day: 1,
    round: 1,
    target: 'p2',
    cause: 'loups',
  },
  { kind: 'phase', text: '🌅 Le jour se lève sur le village.', phase: 'aube', day: 1, round: 1 },
];

const aube = () => scene('aube', { deaths: [{ target: 'p2', cause: 'loups' }] });

const speak = vi.fn<(text: string, onEnd?: () => void) => boolean>();
const cancel = vi.fn();
const vraiSpeak = useVoiceStore.getState().speak;
const vraiCancel = useVoiceStore.getState().cancel;

/** terminerLecture dit que la voix a fini : le rappel du n-ième appel se tait. */
function terminerLecture(appel = 0) {
  const onEnd = speak.mock.calls[appel]?.[1];
  act(() => onEnd?.());
}

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
  speak.mockReset();
  speak.mockReturnValue(true);
  useVoiceStore.setState({ muted: false, speak, cancel });
  cancel.mockClear();
});

afterEach(() => {
  cleanup();
  // Les tests coupent parfois la voix : on la rétablit pour le suivant, moteur
  // compris, et on rend les vrais `speak`/`cancel` au store.
  useVoiceStore.getState().setMuted(false);
  useVoiceStore.setState({ speak: vraiSpeak, cancel: vraiCancel });
});

describe('Game — récit de l’aube', () => {
  it('affiche la carte et lit la nuit qui vient de s’achever', () => {
    useRoomStore.setState({ history: night });
    const { rerender } = render(<Game {...scene('nuit')} />);

    rerender(<Game {...aube()} />);

    expect(screen.getByText('Nuit 1')).toBeTruthy();
    expect(screen.getByText(/Bob a été dévoré/)).toBeTruthy();
    expect(speak).toHaveBeenCalledWith(
      'Le jour se lève sur le village. Bob a été dévoré pendant la nuit.',
      expect.any(Function),
    );
  });

  it('ne lit pas le récit quand on arrive en cours d’aube', () => {
    useRoomStore.setState({ history: night });

    render(<Game {...aube()} />);

    expect(screen.getByText('Nuit 1')).toBeTruthy();
    expect(speak).not.toHaveBeenCalled();
  });

  it('garde le récit à l’écran tant que la voix lit', () => {
    useRoomStore.setState({ history: night });
    const { rerender } = render(<Game {...scene('nuit')} />);
    rerender(<Game {...aube()} />);

    // L'aube se referme, mais la lecture continue : la carte reste.
    rerender(<Game {...scene('debat')} />);
    expect(screen.getByText('Nuit 1')).toBeTruthy();

    terminerLecture();

    expect(screen.queryByText('Nuit 1')).toBeNull();
  });

  it('retire la carte à la fin de l’aube quand la voix ne lit pas', () => {
    useRoomStore.setState({ history: night });
    speak.mockReturnValue(false);
    const { rerender } = render(<Game {...scene('nuit')} />);
    rerender(<Game {...aube()} />);

    rerender(<Game {...scene('debat')} />);

    expect(screen.queryByText('Nuit 1')).toBeNull();
  });

  it('cesse d’attendre la voix au bout du délai', () => {
    vi.useFakeTimers();
    try {
      useRoomStore.setState({ history: night });
      const { rerender } = render(<Game {...scene('nuit')} />);
      rerender(<Game {...aube()} />);
      rerender(<Game {...scene('debat')} />);
      expect(screen.getByText('Nuit 1')).toBeTruthy();

      act(() => {
        vi.advanceTimersByTime(20_000);
      });

      expect(screen.queryByText('Nuit 1')).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('met fin à la lecture quand la voix est coupée', () => {
    useRoomStore.setState({ history: night });
    const { rerender } = render(<Game {...scene('nuit')} />);
    rerender(<Game {...aube()} />);
    rerender(<Game {...scene('debat')} />);
    expect(screen.getByText('Nuit 1')).toBeTruthy();

    act(() => useVoiceStore.getState().setMuted(true));

    expect(screen.queryByText('Nuit 1')).toBeNull();
  });

  it('masque la carte et fait taire la voix', () => {
    useRoomStore.setState({ history: night });
    const { rerender } = render(<Game {...scene('nuit')} />);
    rerender(<Game {...aube()} />);

    fireEvent.click(screen.getByRole('button', { name: /Masquer/ }));

    expect(screen.queryByText('Nuit 1')).toBeNull();
    expect(cancel).toHaveBeenCalled();
  });

  it('relit le récit à la demande', () => {
    useRoomStore.setState({ history: night });
    const { rerender } = render(<Game {...scene('nuit')} />);
    rerender(<Game {...aube()} />);

    fireEvent.click(screen.getByRole('button', { name: /Réécouter/ }));

    expect(speak).toHaveBeenCalledTimes(2);
  });

  it('désactive la relecture quand la voix est coupée', () => {
    useVoiceStore.setState({ muted: true });
    useRoomStore.setState({ history: night });

    render(<Game {...aube()} />);

    const replay = screen.getByRole('button', { name: /Réécouter/ }) as HTMLButtonElement;
    expect(replay.disabled).toBe(true);
  });
});
