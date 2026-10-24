// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';

import { useLobbyData } from '../../hooks/useLobbyData';
import FriendsSidebar from './FriendsSidebar';

function Harness() {
  const data = useLobbyData();

  return <FriendsSidebar data={data} />;
}

afterEach(cleanup);

describe('FriendsSidebar', () => {
  it('affiche les amis en ligne et hors ligne sans réglages de table', () => {
    render(<Harness />);

    expect(screen.getByText('Camille')).toBeTruthy();
    expect(screen.getByText('Inès')).toBeTruthy();
    expect(document.querySelector('#friends-online-title')?.textContent).toContain('3');
    expect(document.querySelector('#friends-offline-title')?.textContent).toContain('3');
    expect(screen.queryByText('Réglages de la table')).toBeNull();
  });

  it('recherche puis ajoute un ami', () => {
    render(<Harness />);

    fireEvent.change(screen.getByLabelText('Rechercher un ami à ajouter'), {
      target: { value: 'jade' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Rechercher' }));

    expect(screen.getByText('Jade')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Ajouter Jade' }));

    expect(screen.getByText('Jade')).toBeTruthy();
    expect(screen.queryByText('Résultats')).toBeNull();
    expect(document.querySelector('#friends-online-title')?.textContent).toContain('4');
  });

  it('retire un ami depuis sa ligne', () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole('button', { name: 'Retirer Inès de mes amis' }));

    expect(screen.queryByText('Inès')).toBeNull();
    expect(document.querySelector('#friends-offline-title')?.textContent).toContain('2');
  });

  it('ouvre le profil local d’un ami hors ligne', () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole('button', { name: 'Voir le profil de Marc' }));

    const dialog = screen.getByRole('dialog', { name: 'Profil' });
    expect(within(dialog).getByText('Marc')).toBeTruthy();

    fireEvent.click(within(dialog).getAllByRole('button', { name: 'Fermer' })[0]);
    expect(screen.queryByRole('dialog', { name: 'Profil' })).toBeNull();
  });
});
