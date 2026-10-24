// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';

import { loadNotes, useNotes } from './useNotes';

/**
 * Le carnet local cycle entre suspect, innocent et aucune marque, se relit au
 * même code de partie, et ne mélange jamais deux tables.
 */

describe('useNotes', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(cleanup);

  it('cycle suspect → innocent → aucune, et persiste', () => {
    const { result } = renderHook(() => useNotes('ABC234'));

    expect(result.current.notes).toEqual({});

    act(() => result.current.cycle('p2'));
    expect(result.current.notes.p2).toBe('suspect');

    act(() => result.current.cycle('p2'));
    expect(result.current.notes.p2).toBe('innocent');

    act(() => result.current.cycle('p2'));
    expect(result.current.notes.p2).toBeUndefined();
    expect(loadNotes('ABC234')).toEqual({});
  });

  it('relit le carnet du même code et isole les autres parties', () => {
    window.localStorage.setItem('loupgarou.notes.ABC234', JSON.stringify({ p2: 'suspect' }));

    const first = renderHook(() => useNotes('ABC234'));
    const second = renderHook(() => useNotes('XYZ789'));

    expect(first.result.current.notes).toEqual({ p2: 'suspect' });
    expect(second.result.current.notes).toEqual({});
  });

  it('ignore un carnet illisible sans faire échouer la partie', () => {
    window.localStorage.setItem('loupgarou.notes.ABC234', '{pas du json');

    expect(loadNotes('ABC234')).toEqual({});
  });
});
