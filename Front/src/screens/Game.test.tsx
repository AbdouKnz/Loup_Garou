// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';

import Game from './Game';
import { AVATARS } from '../components/game/avatars';
import { useRoomStore } from '../state/room';
import { MAX_PLAYERS } from '../api/types';
import type { GameEvent, GameView, PlayerView, RoomState } from '../api/types';

/**
 * Ces tests vérifient l'écran de jeu, phase par phase.
 *
 * Ils partent de ce que le serveur enverrait : jamais plus. Si un test a besoin
 * d'un rôle pour passer, c'est que l'écran le déduit au lieu de le lire — et
 * c'est exactement ce qu'on ne veut pas.
 */

// Les envois d'intentions, doublés par les tests qui les vérifient.
const envois = useRoomStore.getState();

const roster: PlayerView[] = [
  { id: 'p1', pseudo: 'Alice', ready: true, online: true, host: true },
  { id: 'p2', pseudo: 'Bob', ready: true, online: true, host: false },
  { id: 'p3', pseudo: 'Chloé', ready: true, online: true, host: false },
  { id: 'p4', pseudo: 'David', ready: true, online: true, host: false },
];

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
});

afterEach(() => {
  cleanup();
  // Les intentions doublées par un test rendent la main au store réel.
  useRoomStore.setState({
    agirNuit: envois.agirNuit,
    voteNuit: envois.voteNuit,
    voteJour: envois.voteJour,
  });
});

/** scene assemble une salle et sa partie, comme le serveur les enverrait. */
function scene(game: Partial<GameView>, overrides: Partial<RoomState> = {}) {
  const view: GameView = {
    phase: 'nuit',
    day: 1,
    round: 1,
    secondsLeft: 30,
    alive: roster.map((player) => player.id),
    dead: [],
    ...game,
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
    ...overrides,
  };

  return { room, game: view };
}

describe('Game', () => {
  it('marque la phase en cours sur le plateau', () => {
    render(<Game {...scene({ phase: 'vote' })} />);

    expect(document.querySelector('[data-phase="vote"]')).toBeTruthy();
  });

  it('affiche le décompte de la phase', () => {
    const deadline = new Date(Date.now() + 30_000).toISOString();
    render(<Game {...scene({ phase: 'debat', deadline, secondsLeft: 30 })} />);

    expect(screen.getByRole('timer')).toBeTruthy();
  });

  it('met le village en scène la nuit, sans le laisser voter', () => {
    render(<Game {...scene({ phase: 'nuit', yourRole: 'villageois' })} />);

    expect(screen.getByText(/village s’endort/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Dévorer/ })).toBeNull();
  });

  it('ouvre l’antre au loup, avec ses compères et une confirmation', () => {
    render(
      <Game
        {...scene({
          phase: 'nuit',
          yourRole: 'loup-garou',
          yourAction: 'devorer',
          wolves: ['p1', 'p2'],
          wolfVotes: {},
        })}
      />,
    );

    expect(screen.getByText(/L’antre/)).toBeTruthy();

    // Le loup ne se dévore pas lui-même, ni ses compères : il reste deux proies
    // (Chloé et David).
    expect(screen.getAllByRole('button', { name: /Dévorer/ })).toHaveLength(2);
    expect(screen.queryByRole('button', { name: 'Dévorer Bob' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Dévorer Chloé' }));
    expect(screen.getByText(/Dévorer Chloé cette nuit/)).toBeTruthy();
  });

  it('n’ouvre la table qu’à l’acteur de la veillée en cours', () => {
    // Le loup est bien un loup, mais ce n'est pas sa veillée : le serveur ne
    // l'invite à rien, et la table n'offre donc aucune proie.
    render(
      <Game {...scene({ phase: 'nuit', yourRole: 'loup-garou', wolves: ['p1'], wolfVotes: {} })} />,
    );

    expect(screen.queryByRole('button', { name: /Dévorer/ })).toBeNull();
    expect(screen.getByText(/village s’endort/)).toBeTruthy();
    expect(screen.queryByText(/S’abstenir/)).toBeNull();
  });

  it('met la partie en pause le temps de découvrir son rôle', () => {
    const deadline = new Date(Date.now() + 3_000).toISOString();

    render(
      <Game
        {...scene({
          phase: 'roles',
          deadline,
          secondsLeft: 3,
          yourRole: 'loup-garou',
          wolves: ['p1', 'p2'],
        })}
      />,
    );

    // La carte s'impose, et ne se ferme pas : c'est le serveur qui clôt la
    // distribution, et la partie commence à cet instant.
    const carte = screen.getByRole('dialog', { name: /Loup-Garou/ });
    expect(within(carte).getByText(/Votre compère/)).toBeTruthy();
    expect(within(carte).getByText(/La partie commence dans/)).toBeTruthy();
    expect(within(carte).queryByRole('button')).toBeNull();

    // Personne ne joue, personne ne parle : la partie n'a pas commencé.
    expect(screen.queryByRole('button', { name: /^Dévorer/ })).toBeNull();
    expect(screen.getByPlaceholderText(/Chacun découvre son rôle/)).toBeTruthy();
  });

  it('garde la carte de rôle à portée de main après la distribution', () => {
    render(<Game {...scene({ phase: 'debat', yourRole: 'sorciere', yourAction: undefined })} />);

    // Pendant la partie, la carte se rouvre à la demande et se referme.
    expect(screen.queryByRole('dialog')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Revoir mon rôle' }));

    const carte = screen.getByRole('dialog', { name: /Sorcière/ });
    expect(within(carte).getByText(/Une potion de vie/)).toBeTruthy();
    expect(within(carte).queryByText(/La partie commence dans/)).toBeNull();

    fireEvent.click(within(carte).getByRole('button', { name: 'Fermer' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('invite la voyante à observer, et lui garde son carnet', () => {
    const agirNuit = vi.fn();
    useRoomStore.setState({ agirNuit });

    render(
      <Game
        {...scene({
          phase: 'nuit',
          yourRole: 'voyante',
          yourAction: 'voir',
          yourVision: [{ day: 1, target: 'p2', role: 'loup-garou' }],
        })}
      />,
    );

    // Son carnet : ce qu'elle a vu, et quand.
    expect(screen.getByText(/Votre carnet/)).toBeTruthy();
    expect(screen.getByText('Nuit 1', { selector: '.vision-day' })).toBeTruthy();
    expect(screen.getByText('Loup-Garou')).toBeTruthy();

    // Elle ne s'observe pas elle-même.
    expect(screen.getAllByRole('button', { name: /^Observer/ })).toHaveLength(3);
    expect(screen.queryByRole('button', { name: 'Observer Alice' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Observer Chloé' }));
    expect(screen.getByText(/Observer Chloé cette nuit/)).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /Oui, observer/ }));
    expect(agirNuit).toHaveBeenCalledWith('voir', 'p3');
  });

  it('donne à la sorcière la proie des loups, ses potions et ses choix', () => {
    const agirNuit = vi.fn();
    useRoomStore.setState({ agirNuit });

    render(
      <Game
        {...scene({
          phase: 'nuit',
          yourRole: 'sorciere',
          yourAction: 'potions',
          wolfTarget: 'p2',
          yourPotions: { heal: true, poison: true },
        })}
      />,
    );

    expect(screen.getByText(/Les loups ont désigné Bob/)).toBeTruthy();
    expect(screen.getByText(/Potion de vie — disponible/)).toBeTruthy();
    expect(screen.getByText(/Potion de mort — disponible/)).toBeTruthy();

    // Soigner la proie : le choix se confirme, comme celui des loups.
    fireEvent.click(screen.getByRole('button', { name: /Soigner Bob/ }));
    expect(screen.getByText(/Soigner Bob cette nuit/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /Oui, soigner/ }));
    expect(agirNuit).toHaveBeenCalledWith('soigner', 'p2');

    agirNuit.mockClear();

    // Empoisonner : la victime se désigne sur la table, jamais soi-même.
    fireEvent.click(screen.getByRole('button', { name: /Empoisonner…/ }));
    expect(screen.getByText(/Désignez votre victime sur la table/)).toBeTruthy();
    expect(screen.getAllByRole('button', { name: /^Empoisonner/ })).toHaveLength(3);

    fireEvent.click(screen.getByRole('button', { name: 'Empoisonner Chloé' }));
    expect(screen.getByText(/Empoisonner Chloé cette nuit/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /Oui, empoisonner/ }));
    expect(agirNuit).toHaveBeenCalledWith('empoisonner', 'p3');

    agirNuit.mockClear();

    // Ou ne rien faire : sa veillée se referme sans qu'elle y touche.
    fireEvent.click(screen.getByRole('button', { name: /Ne rien faire/ }));
    expect(agirNuit).toHaveBeenCalledWith('passer');
  });

  it('annonce chaque mort de la nuit avec sa cause', () => {
    render(
      <Game
        {...scene({
          phase: 'aube',
          yourRole: 'villageois',
          deaths: [
            { target: 'p2', cause: 'loups' },
            { target: 'p3', cause: 'poison' },
          ],
        })}
      />,
    );

    // La table, l’aube… et le récit qui reprend les mêmes faits.
    expect(screen.getAllByText('Bob').length).toBeGreaterThanOrEqual(2);

    // Les révélations de l’aube restent annoncées, carte de récit ou non.
    const annonces = screen.getAllByRole('status').map((node) => node.textContent ?? '');
    expect(annonces.some((texte) => /a été dévoré pendant la nuit/.test(texte))).toBe(true);
    expect(annonces.some((texte) => /est mort dans d’étranges circonstances/.test(texte))).toBe(
      true,
    );
  });

  it('affiche le scrutin public pendant le vote du jour', () => {
    render(
      <Game
        {...scene({
          phase: 'vote',
          yourRole: 'villageois',
          ballot: { p2: 'p3', p3: 'p3', p4: '' },
        })}
      />,
    );

    expect(screen.getByText(/Le scrutin/)).toBeTruthy();
    // Le total du scrutin, et non le rappel du centre de la table.
    expect(screen.getByText(/2 voix/, { selector: '.ballot-count' })).toBeTruthy();
    expect(screen.getByText('Bob, Chloé')).toBeTruthy();
    expect(screen.getByText(/Abstentions : David/)).toBeTruthy();
    expect(screen.getAllByRole('button', { name: /^Voter/ })).toHaveLength(3);
    expect(screen.getByRole('button', { name: 'S’abstenir' })).toBeTruthy();
  });

  it('demande confirmation avant d’envoyer le vote du jour', () => {
    const voteJour = vi.fn();
    useRoomStore.setState({ voteJour });

    render(<Game {...scene({ phase: 'vote', yourRole: 'villageois', ballot: {} })} />);

    fireEvent.click(screen.getByRole('button', { name: 'Voter Bob' }));

    expect(voteJour).not.toHaveBeenCalled();
    expect(screen.getByRole('alertdialog', { name: 'Confirmer votre vote' })).toBeTruthy();
    expect(screen.getByText('Voter contre Bob ?')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }));
    expect(voteJour).not.toHaveBeenCalled();
    expect(screen.queryByRole('alertdialog')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Voter Bob' }));
    fireEvent.click(screen.getByRole('button', { name: 'Oui, voter' }));

    expect(voteJour).toHaveBeenCalledTimes(1);
    expect(voteJour).toHaveBeenCalledWith('p2');
  });

  it('confirme aussi l’abstention', () => {
    const voteJour = vi.fn();
    useRoomStore.setState({ voteJour });

    render(<Game {...scene({ phase: 'vote', yourRole: 'villageois', ballot: {} })} />);

    fireEvent.click(screen.getByRole('button', { name: 'S’abstenir' }));

    expect(voteJour).not.toHaveBeenCalled();
    expect(screen.getByText(/Confirmer votre abstention/)).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Oui, m’abstenir' }));
    expect(voteJour).toHaveBeenCalledWith('');
  });

  it('ouvre l’aide de jeu depuis la table', () => {
    render(
      <Game
        {...scene({
          phase: 'debat',
          yourRole: 'villageois',
          speaker: 'p2',
          speakers: ['p1', 'p2'],
          spoken: [],
        })}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Règles du jeu' }));

    expect(screen.getByRole('dialog', { name: /Règles du village/ })).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Fermer' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('restreint le second tour aux joueurs en ballotage', () => {
    render(
      <Game
        {...scene({
          phase: 'vote',
          round: 2,
          tie: true,
          candidates: ['p2'],
          yourRole: 'villageois',
          ballot: {},
        })}
      />,
    );

    expect(screen.getByText(/second tour entre Bob/)).toBeTruthy();
    expect(screen.getAllByRole('button', { name: /^Voter/ })).toHaveLength(1);
  });

  it('laisse un mort regarder sans parler', () => {
    const players = roster.map((player) =>
      player.id === 'p1' ? { ...player, dead: true } : player,
    );

    render(
      <Game {...scene({ phase: 'debat', yourRole: 'villageois', dead: ['p1'] }, { players })} />,
    );

    expect(screen.getByText(/Vous êtes mort/)).toBeTruthy();
    expect(screen.getByPlaceholderText(/Les morts ne parlent plus/)).toBeTruthy();
  });

  it('met l’orateur du débat en vedette et lui réserve la parole', () => {
    const { rerender } = render(
      <Game
        {...scene({
          phase: 'debat',
          yourRole: 'villageois',
          speaker: 'p2',
          speakers: ['p1', 'p2'],
          spoken: [],
        })}
      />,
    );

    // Bob a la parole : la table l'annonce, et la discussion m'est fermée.
    expect(screen.getByText(/Bob a la parole/)).toBeTruthy();
    expect(screen.getByPlaceholderText(/Bob a la parole/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: /fini de parler/ })).toBeNull();

    // La parole me revient : je peux écrire, et la rendre quand j'ai fini.
    rerender(
      <Game
        {...scene({
          phase: 'debat',
          yourRole: 'villageois',
          speaker: 'p1',
          speakers: ['p1', 'p2'],
          spoken: [],
        })}
      />,
    );

    expect(screen.getByText(/À vous de parler/)).toBeTruthy();
    expect(screen.getByRole('button', { name: /fini de parler/ })).toBeTruthy();
  });

  it('dévoile les rôles et le fil de la partie à la fin', () => {
    useRoomStore.setState({
      history: [
        {
          kind: 'victime',
          text: 'Bob a été dévoré.',
          phase: 'nuit',
          day: 1,
          round: 1,
          target: 'p2',
        },
        {
          kind: 'elimine',
          text: 'Chloé a été éliminée.',
          phase: 'vote',
          day: 1,
          round: 1,
          target: 'p3',
          role: 'loup-garou',
        },
      ] as GameEvent[],
    });

    render(
      <Game
        {...scene({
          phase: 'fin',
          winner: 'village',
          yourRole: 'villageois',
          roles: {
            p1: 'villageois',
            p2: 'villageois',
            p3: 'loup-garou',
            p4: 'villageois',
          },
          dead: ['p2', 'p3'],
        })}
      />,
    );

    expect(screen.getByText(/Le village l’emporte/)).toBeTruthy();
    expect(screen.getByText('Loup-Garou')).toBeTruthy();
    expect(screen.getByText(/Bob a été dévoré/)).toBeTruthy();
    expect(screen.getByText(/Chloé a été éliminée/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Rejouer' })).toBeTruthy();
  });

  it('ne laisse pas un invité relancer la partie', () => {
    render(
      <Game
        {...scene(
          {
            phase: 'fin',
            winner: 'loups',
            yourRole: 'loup-garou',
            roles: { p1: 'villageois', p2: 'loup-garou', p3: 'loup-garou', p4: 'villageois' },
          },
          { you: 'p4' },
        )}
      />,
    );

    expect(screen.getByText(/Les loups ont gagné/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Rejouer' })).toBeNull();
    expect(screen.getByText(/L’hôte peut relancer/)).toBeTruthy();
  });
});

describe('avatars', () => {
  it('donne une pastille distincte à chaque place de la table', () => {
    expect(new Set(AVATARS).size).toBe(AVATARS.length);
    expect(AVATARS.length).toBeGreaterThanOrEqual(MAX_PLAYERS);
  });
});
