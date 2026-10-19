// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';

import PhaseTransition, { PHASE_TRANSITION_MS } from './PhaseTransition';
import { PHASES } from './labels';

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('PhaseTransition', () => {
  it('ne montre rien en arrivant sur une partie en cours', () => {
    render(<PhaseTransition phase="nuit" day={1} />);

    expect(screen.queryByText(PHASES.nuit.title)).toBeNull();
  });

  it('voile l’écran quand la nuit tombe', () => {
    const { rerender } = render(<PhaseTransition phase="roles" day={1} />);

    rerender(<PhaseTransition phase="nuit" day={1} />);

    expect(screen.getByText(PHASES.nuit.title)).toBeTruthy();
    const veil = screen.getByText(PHASES.nuit.title).closest('.phase-transition');
    expect(veil?.getAttribute('aria-hidden')).toBe('true');
  });

  it('voile l’écran quand le jour se lève', () => {
    const { rerender } = render(<PhaseTransition phase="nuit" day={1} />);

    rerender(<PhaseTransition phase="aube" day={1} />);

    expect(screen.getByText(PHASES.aube.title)).toBeTruthy();
  });

  it('ne voile pas les autres phases du jour', () => {
    const { rerender } = render(<PhaseTransition phase="aube" day={1} />);

    rerender(<PhaseTransition phase="debat" day={1} />);

    expect(screen.queryByText(PHASES.debat.title)).toBeNull();
  });

  it('retire le voile au bout de la durée', () => {
    vi.useFakeTimers();
    const { rerender } = render(<PhaseTransition phase="roles" day={1} />);
    rerender(<PhaseTransition phase="nuit" day={1} />);
    expect(screen.getByText(PHASES.nuit.title)).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(PHASE_TRANSITION_MS);
    });

    expect(screen.queryByText(PHASES.nuit.title)).toBeNull();
  });

  it('rejoue le voile à la nuit suivante', () => {
    const { rerender } = render(<PhaseTransition phase="nuit" day={1} />);
    rerender(<PhaseTransition phase="aube" day={1} />);
    rerender(<PhaseTransition phase="debat" day={1} />);
    rerender(<PhaseTransition phase="vote" day={1} />);
    rerender(<PhaseTransition phase="verdict" day={1} />);

    rerender(<PhaseTransition phase="nuit" day={2} />);

    expect(screen.getByText(PHASES.nuit.title)).toBeTruthy();
  });
});
