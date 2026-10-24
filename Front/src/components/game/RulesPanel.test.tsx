// @vitest-environment jsdom
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import RulesPanel from './RulesPanel';

/**
 * L'aide de jeu ne détaille que les rôles annoncés par la salle, reste
 * utilisable au clavier, et rend le focus à l'ouvreur en se refermant.
 */

describe('RulesPanel', () => {
  afterEach(cleanup);

  it('détaille les rôles de la composition et ferme sur Échap', () => {
    const onClose = vi.fn();

    render(<RulesPanel composition={['villageois', 'loup-garou']} onClose={onClose} />);

    expect(screen.getByRole('dialog', { name: /Règles du village/ })).toBeTruthy();
    expect(screen.getByText('Villageois')).toBeTruthy();
    expect(screen.getByText('Loup-Garou')).toBeTruthy();
    expect(screen.queryByText('Voyante')).toBeNull();

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('montre tous les rôles quand la salle n’en annonce aucun', () => {
    render(<RulesPanel composition={[]} onClose={() => {}} />);

    expect(screen.getByText('Villageois')).toBeTruthy();
    expect(screen.getByText('Loup-Garou')).toBeTruthy();
    expect(screen.getByText('Voyante')).toBeTruthy();
    expect(screen.getByText('Sorcière')).toBeTruthy();
  });

  it('garde le focus dans le panneau et le rend à l’ouvreur', () => {
    function Harness() {
      const [open, setOpen] = useState(false);

      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>
            Ouvrir l’aide
          </button>
          {open && <RulesPanel composition={[]} onClose={() => setOpen(false)} />}
        </>
      );
    }

    render(<Harness />);

    const opener = screen.getByRole('button', { name: 'Ouvrir l’aide' });
    opener.focus();
    fireEvent.click(opener);

    const close = screen.getByRole('button', { name: 'Fermer' });
    expect(document.activeElement).toBe(close);

    // Shift+Tab depuis le premier élément boucle vers le dernier.
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true });
    expect(document.activeElement).toBe(screen.getByRole('slider'));

    // Tab depuis le dernier élément revient au premier.
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(document.activeElement).toBe(close);

    fireEvent.click(close);
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.activeElement).toBe(opener);
  });
});
