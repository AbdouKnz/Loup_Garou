// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';

import Composition from './Composition';
import type { Role } from '../../api/types';

afterEach(cleanup);

describe('Composition', () => {
  it('regroupe les rôles annoncés, dans l’ordre du serveur', () => {
    const roles: Role[] = [
      'loup-garou',
      'voyante',
      'sorciere',
      'villageois',
      'villageois',
      'villageois',
    ];

    render(<Composition roles={roles} />);

    const entries = screen.getAllByRole('listitem').map((item) => item.textContent);

    expect(entries).toEqual(['1🐺Loup-Garou', '1🔮Voyante', '1🧪Sorcière', '3🧑‍🌾Villageois']);
  });

  it('n’annonce rien pour une table vide', () => {
    render(<Composition roles={[]} />);

    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
  });
});
