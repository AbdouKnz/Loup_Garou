// @vitest-environment jsdom
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import ActionCards from './ActionCards';
import type { LobbyAction } from './types';

function Harness({
  onCreate,
  onFriends,
  onJoinSubmit,
}: {
  onCreate: () => void;
  onFriends: () => void;
  onJoinSubmit: () => void;
}) {
  const [active, setActive] = useState<LobbyAction>('create');
  const [joinOpen, setJoinOpen] = useState(false);
  const [joinCode, setJoinCode] = useState('');

  return (
    <ActionCards
      active={active}
      busy={false}
      joinOpen={joinOpen}
      joinCode={joinCode}
      onSelect={setActive}
      onCreate={onCreate}
      onJoin={() => setJoinOpen(true)}
      onFriends={onFriends}
      onJoinCodeChange={setJoinCode}
      onJoinSubmit={onJoinSubmit}
    />
  );
}

afterEach(cleanup);

describe('ActionCards', () => {
  it('déclenche la création et marque la carte active', () => {
    const onCreate = vi.fn();
    const { container } = render(
      <Harness onCreate={onCreate} onFriends={vi.fn()} onJoinSubmit={vi.fn()} />,
    );

    expect(container.querySelector('.action-card.is-active .action-title')?.textContent).toBe(
      'Créer une table',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Créer une table' }));
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  it('ouvre le formulaire de code et le soumet une fois complet', () => {
    const onJoinSubmit = vi.fn();
    render(<Harness onCreate={vi.fn()} onFriends={vi.fn()} onJoinSubmit={onJoinSubmit} />);

    fireEvent.click(screen.getByRole('button', { name: 'J’ai un code' }));

    const input = screen.getByLabelText('Code de la table');
    const submit = screen.getByRole('button', { name: 'Rejoindre la table' });
    expect(submit.hasAttribute('disabled')).toBe(true);

    fireEvent.change(input, { target: { value: 'abc234' } });
    expect(submit.hasAttribute('disabled')).toBe(false);

    fireEvent.click(submit);
    expect(onJoinSubmit).toHaveBeenCalledTimes(1);
  });

  it('propose d’inviter des amis', () => {
    const onFriends = vi.fn();
    render(<Harness onCreate={vi.fn()} onFriends={onFriends} onJoinSubmit={vi.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: 'Inviter des amis' }));

    expect(onFriends).toHaveBeenCalledTimes(1);
  });
});
