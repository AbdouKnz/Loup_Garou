import { useEffect } from 'react';

import ConnectionBadge from './components/ConnectionBadge';
import Auth from './screens/Auth';
import Game from './screens/Game';
import Home from './screens/Home';
import Lobby from './screens/Lobby';
import { useAccountStore } from './state/account';
import { useRoomStore } from './state/room';
import { useSoundStore } from './state/sound';

/**
 * App aiguille entre la connexion, l'accueil, la salle d'attente et la partie.
 *
 * Aucun routeur n'est nécessaire : l'identité du compte, l'état de la session
 * de siège et la phase annoncée par le serveur suffisent. Le lien /partie/CODE
 * est lu par l'écran d'accueil pour pré-remplir le code.
 */
export default function App() {
  const accountReady = useAccountStore((state) => state.ready);
  const me = useAccountStore((state) => state.me);
  const loadAccount = useAccountStore((state) => state.load);

  const session = useRoomStore((state) => state.session);
  const room = useRoomStore((state) => state.room);
  const connection = useRoomStore((state) => state.connection);
  const resume = useRoomStore((state) => state.resume);
  const unlockSound = useSoundStore((state) => state.unlock);

  // La carte son ne s'ouvre qu'après un geste du joueur : c'est la règle des
  // navigateurs. On écoute donc le premier clic, la première touche de clavier.
  useEffect(() => {
    const unlock = () => unlockSound();
    window.addEventListener('pointerdown', unlock, { once: true });
    window.addEventListener('keydown', unlock, { once: true });

    return () => {
      window.removeEventListener('pointerdown', unlock);
      window.removeEventListener('keydown', unlock);
    };
  }, [unlockSound]);

  // Reprise automatique après un rafraîchissement de page (F5).
  useEffect(() => {
    resume();
  }, [resume]);

  // Qui est connecté ? Le cookie de session le dit, lui seul.
  useEffect(() => {
    void loadAccount();
  }, [loadAccount]);

  // Changer d'écran remet la page en haut : on quitte une salle d'attente ou une
  // table déjà défilée, et le nouvel écran doit s'ouvrir sur son en-tête.
  const screen = !session
    ? 'accueil'
    : !room
      ? 'reprise'
      : room.phase === 'lobby' || !room.game
        ? 'salle'
        : 'partie';
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [screen]);

  if (!accountReady) {
    return (
      <main className="card">
        <p className="eyebrow">Loup-Garou</p>
        <h1>Connexion…</h1>
        <p className="subtitle">Un instant, le temps de retrouver votre place.</p>
      </main>
    );
  }

  if (!me) {
    return <Auth />;
  }

  if (!session) {
    return <Home />;
  }

  if (!room) {
    return (
      <main className="card">
        <p className="eyebrow">Partie {session.code}</p>
        <h1>Connexion…</h1>
        <p className="subtitle">
          Reprise de votre place
          {connection === 'reconnecting' ? ', le serveur est injoignable' : ''}…
        </p>
        <p>
          <ConnectionBadge status={connection} />
        </p>
      </main>
    );
  }

  if (room.phase === 'lobby' || !room.game) {
    return <Lobby room={room} />;
  }

  return <Game room={room} game={room.game} />;
}
