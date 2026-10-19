// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import Auth from './Auth';
import { useAccountStore } from '../state/account';
import { NO_PROVIDERS, PASSWORD_MIN_LENGTH } from '../api/types';

/** Les actions réelles, rendues à la fin de chaque test. */
const actions = useAccountStore.getState();

const signup = vi.fn();
const login = vi.fn();
const complete = vi.fn();

beforeEach(() => {
  signup.mockReset();
  login.mockReset();
  complete.mockReset();

  useAccountStore.setState({
    busy: false,
    error: null,
    me: null,
    ready: true,
    providers: NO_PROVIDERS,
    pending: null,
    signup,
    login,
    complete,
  });
});

afterEach(() => {
  cleanup();
  useAccountStore.setState({
    ...actions,
    busy: false,
    error: null,
    me: null,
    ready: true,
  });
});

/** soumettre remplit le formulaire et clique le bouton d'envoi. */
function soumettre(pseudo: string, password: string, label = 'Créer mon compte') {
  fireEvent.change(screen.getByLabelText('Votre pseudo'), { target: { value: pseudo } });
  fireEvent.change(screen.getByLabelText('Mot de passe'), { target: { value: password } });
  fireEvent.click(screen.getByRole('button', { name: label }));
}

describe('Auth', () => {
  it('affiche la marque du loup en haut de la page', () => {
    render(<Auth />);

    expect(screen.getByTestId('auth-brand').textContent).toContain('Loup-Garou');
  });

  it('crée un compte avec le pseudo et le mot de passe saisis', () => {
    render(<Auth />);

    soumettre('Alice', 'motdepasse');

    expect(signup).toHaveBeenCalledWith('Alice', 'motdepasse');
    expect(login).not.toHaveBeenCalled();
  });

  it('se connecte à un compte existant', () => {
    render(<Auth />);

    fireEvent.click(screen.getByRole('button', { name: 'J’ai déjà un compte' }));
    soumettre('Alice', 'motdepasse', 'Se connecter');

    expect(login).toHaveBeenCalledWith('Alice', 'motdepasse');
    expect(signup).not.toHaveBeenCalled();
  });

  it('n’envoie rien tant que le formulaire est incomplet', () => {
    render(<Auth />);

    // Un pseudo d'une lettre, puis un mot de passe trop court : le bouton reste
    // fermé, et le navigateur ne peut pas soumettre à moitié.
    fireEvent.change(screen.getByLabelText('Votre pseudo'), { target: { value: 'A' } });
    fireEvent.change(screen.getByLabelText('Mot de passe'), { target: { value: 'motdepasse' } });

    const submit = screen.getByRole('button', { name: 'Créer mon compte' });
    expect((submit as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(screen.getByLabelText('Votre pseudo'), { target: { value: 'Alice' } });
    fireEvent.change(screen.getByLabelText('Mot de passe'), {
      target: { value: 'court' },
    });
    expect((submit as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(screen.getByLabelText('Mot de passe'), {
      target: { value: 'a'.repeat(PASSWORD_MIN_LENGTH) },
    });
    expect((submit as HTMLButtonElement).disabled).toBe(false);
  });

  it('change d’onglet sans garder le mot de passe saisi', () => {
    render(<Auth />);

    fireEvent.change(screen.getByLabelText('Mot de passe'), { target: { value: 'motdepasse' } });
    fireEvent.click(screen.getByRole('button', { name: 'J’ai déjà un compte' }));
    expect((screen.getByLabelText('Mot de passe') as HTMLInputElement).value).toBe('');
  });

  it('affiche l’erreur du serveur et la referme', () => {
    useAccountStore.setState({ error: 'Ce pseudo est déjà pris. Choisissez-en un autre.' });
    render(<Auth />);

    expect(screen.getByRole('alert').textContent).toContain('Ce pseudo est déjà pris.');

    fireEvent.click(screen.getByRole('button', { name: 'fermer' }));
    expect(useAccountStore.getState().error).toBeNull();
  });

  it('désactive l’envoi pendant l’appel', () => {
    useAccountStore.setState({ busy: true });
    render(<Auth />);

    // Le pseudo et le mot de passe sont valides : seul « busy » bloque.
    fireEvent.change(screen.getByLabelText('Votre pseudo'), { target: { value: 'Alice' } });
    fireEvent.change(screen.getByLabelText('Mot de passe'), { target: { value: 'motdepasse' } });

    const submit = screen.getByRole('button', { name: 'Créer mon compte' });
    expect((submit as HTMLButtonElement).disabled).toBe(true);
  });

  it('propose les fournisseurs configurés sur le serveur', () => {
    useAccountStore.setState({ providers: { google: true, discord: false } });
    render(<Auth />);

    // Un lien, pas un bouton : la connexion est une navigation chez le
    // fournisseur, qui ramène ensuite à l'application.
    const google = screen.getByRole('link', { name: 'Continuer avec Google' });
    expect(google.getAttribute('href')).toBe('/api/auth/google/start');

    expect(screen.queryByRole('link', { name: 'Continuer avec Discord' })).toBeNull();
  });

  it('ne propose rien quand aucun fournisseur n’est configuré', () => {
    render(<Auth />);

    expect(screen.queryByRole('link', { name: /Continuer avec/ })).toBeNull();
  });

  it('demande le pseudo après une connexion externe', () => {
    useAccountStore.setState({
      providers: { google: true, discord: false },
      pending: { provider: 'google', suggested: 'Jean' },
    });
    render(<Auth />);

    // Le pseudo proposé est déjà là, et il n'y a plus rien à remplir d'autre.
    expect((screen.getByLabelText('Votre pseudo') as HTMLInputElement).value).toBe('Jean');
    expect(screen.queryByLabelText('Mot de passe')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Créer un compte' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Choisir ce pseudo' }));

    expect(complete).toHaveBeenCalledWith('Jean');
    expect(signup).not.toHaveBeenCalled();
    expect(login).not.toHaveBeenCalled();
  });

  it('n’envoie pas un pseudo trop court après une connexion externe', () => {
    useAccountStore.setState({ pending: { provider: 'discord' } });
    render(<Auth />);

    // Aucun pseudo proposé, et rien de valide saisi : le bouton reste fermé.
    const submit = screen.getByRole('button', { name: 'Choisir ce pseudo' });
    expect((submit as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(screen.getByLabelText('Votre pseudo'), { target: { value: 'A' } });
    expect((submit as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(screen.getByLabelText('Votre pseudo'), { target: { value: 'Alice' } });
    expect((submit as HTMLButtonElement).disabled).toBe(false);

    fireEvent.click(submit);
    expect(complete).toHaveBeenCalledWith('Alice');
  });
});
