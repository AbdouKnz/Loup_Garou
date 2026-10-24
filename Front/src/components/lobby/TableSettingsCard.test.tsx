// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import { useLobbyData } from '../../hooks/useLobbyData';
import TableSettingsCard from './TableSettingsCard';

function Harness({ onCreate }: { onCreate: () => void }) {
  const data = useLobbyData();

  return (
    <>
      <TableSettingsCard
        settings={data.settings}
        busy={false}
        onCreate={onCreate}
        onPlayersChange={data.setPlayers}
        onRoleToggle={data.toggleRole}
        onNightTimerChange={data.setNightTimer}
        onDebateTimerChange={data.setDebateTimer}
        onPrivateChange={data.setPrivate}
      />
      <output data-testid="players">{data.settings.players}</output>
    </>
  );
}

afterEach(cleanup);

describe('TableSettingsCard', () => {
  it('modifie le nombre de joueurs dans les bornes', () => {
    render(<Harness onCreate={() => {}} />);

    expect(screen.getByTestId('players').textContent).toBe('8');

    fireEvent.click(screen.getByRole('button', { name: 'Ajouter un joueur' }));
    expect(screen.getByTestId('players').textContent).toBe('9');

    fireEvent.click(screen.getByRole('button', { name: 'Retirer un joueur' }));
    expect(screen.getByTestId('players').textContent).toBe('8');
  });

  it('active et désactive les rôles sans les bloquer', () => {
    render(<Harness onCreate={() => {}} />);

    const chasseur = screen.getByRole('button', { name: 'Chasseur' });
    expect(chasseur.getAttribute('aria-pressed')).toBe('false');

    fireEvent.click(chasseur);
    expect(chasseur.getAttribute('aria-pressed')).toBe('true');

    fireEvent.click(chasseur);
    expect(chasseur.getAttribute('aria-pressed')).toBe('false');
  });

  it('valide les durées à la sortie du champ', () => {
    render(<Harness onCreate={() => {}} />);

    const night = screen.getByLabelText('Nuit en secondes') as HTMLInputElement;
    fireEvent.change(night, { target: { value: '120' } });
    fireEvent.blur(night);

    expect(night.value).toBe('120');

    fireEvent.change(night, { target: { value: '999' } });
    fireEvent.blur(night);

    expect(night.value).toBe('180');
  });

  it('affiche un code quand la table devient privée', () => {
    const { container } = render(<Harness onCreate={() => {}} />);
    const toggle = screen.getByRole('switch', { name: /Table privée/ });

    expect(toggle.getAttribute('aria-checked')).toBe('false');
    fireEvent.click(toggle);

    expect(toggle.getAttribute('aria-checked')).toBe('true');
    expect(container.querySelector('.lobby-private-code code')?.textContent).toMatch(
      /^[A-Z2-9]{6}$/,
    );
  });

  it('appelle la création depuis le bouton de la table', () => {
    const onCreate = vi.fn();
    render(<Harness onCreate={onCreate} />);

    fireEvent.click(screen.getByRole('button', { name: 'Créer une table' }));

    expect(onCreate).toHaveBeenCalledTimes(1);
  });
});
