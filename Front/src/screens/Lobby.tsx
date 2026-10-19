import { useEffect, useRef, useState } from 'react';

import ChatPanel from '../components/ChatPanel';
import ConnectionBadge from '../components/ConnectionBadge';
import PlayerList from '../components/PlayerList';
import SoundToggle from '../components/SoundToggle';
import Composition from '../components/game/Composition';
import { MIN_PLAYERS } from '../api/types';
import { useAccountStore } from '../state/account';
import { useRoomStore } from '../state/room';
import { useSoundStore } from '../state/sound';
import type { RoomState } from '../api/types';

/** Lobby est la salle d'attente : joueurs, prêts, lancement et discussion. */
export default function Lobby({ room }: { room: RoomState }) {
  const connection = useRoomStore((state) => state.connection);
  const error = useRoomStore((state) => state.error);
  const sendChat = useRoomStore((state) => state.sendChat);
  const toggleReady = useRoomStore((state) => state.toggleReady);
  const startGame = useRoomStore((state) => state.startGame);
  const ajouterBots = useRoomStore((state) => state.ajouterBots);
  const retirerBot = useRoomStore((state) => state.retirerBot);
  const retirerTousLesBots = useRoomStore((state) => state.retirerTousLesBots);
  const leave = useRoomStore((state) => state.leave);
  const dismissError = useRoomStore((state) => state.dismissError);
  const logout = useAccountStore((state) => state.logout);
  const play = useSoundStore((state) => state.play);

  const [copied, setCopied] = useState(false);
  const [botCount, setBotCount] = useState(() => Math.max(1, MIN_PLAYERS - room.players.length));

  const online = connection === 'online';
  const me = room.players.find((player) => player.id === room.you);
  const isHost = room.you === room.hostId;

  const bots = room.players.filter((player) => player.bot);
  const seats = Math.max(0, room.maxPlayers - room.players.length);
  const missing = Math.max(0, MIN_PLAYERS - room.players.length);

  // Un joueur prend place : un petit repère, pour que la salle reste vivante.
  // Seule l'arrivée sonne ; les départs se lisent dans la liste.
  const previousSeated = useRef(room.players.length);
  useEffect(() => {
    if (room.players.length > previousSeated.current) {
      play('join');
    }
    previousSeated.current = room.players.length;
  }, [room.players.length, play]);

  const addBots = () => {
    const count = Math.min(Math.max(1, botCount), seats);
    if (count > 0) {
      ajouterBots(count);
    }
  };

  const copyLink = async () => {
    const link = `${window.location.origin}/partie/${room.code}`;

    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Presse-papiers indisponible : le code reste visible à l'écran.
      setCopied(false);
    }
  };

  return (
    <main className="card wide">
      <header className="lobby-header">
        <div>
          <p className="eyebrow">Salle d’attente</p>
          <h1 className="room-code">{room.code}</h1>
          <p className="subtitle">
            Partagez ce code : vos amis le saisissent sur la page d’accueil.
          </p>
        </div>

        <div className="lobby-tools">
          <ConnectionBadge status={connection} />
          <SoundToggle />
          <button type="button" className="btn" onClick={copyLink}>
            {copied ? 'Lien copié !' : 'Copier le lien'}
          </button>
          <button type="button" className="btn danger" onClick={leave}>
            Quitter
          </button>
          <button
            type="button"
            className="btn"
            onClick={() => {
              void logout();
              leave();
            }}
          >
            Se déconnecter
          </button>
        </div>
      </header>

      {error && (
        <p className="alert" role="alert">
          <span>{error}</span>
          <button type="button" className="link" onClick={dismissError}>
            fermer
          </button>
        </p>
      )}

      <section className="panel">
        <h2>
          Joueurs{' '}
          <span className="count">
            ({room.players.length}/{room.maxPlayers})
          </span>
        </h2>

        <PlayerList
          players={room.players}
          you={room.you}
          onRemoveBot={isHost ? retirerBot : undefined}
        />

        <div className="actions">
          <button
            type="button"
            className={me?.ready ? 'btn' : 'btn primary'}
            disabled={!online}
            onClick={toggleReady}
          >
            {me?.ready ? 'Annuler « prêt »' : 'Je suis prêt'}
          </button>

          {isHost && (
            <button
              type="button"
              className="btn primary"
              disabled={!online || !room.canStart}
              onClick={startGame}
            >
              Démarrer la partie
            </button>
          )}
        </div>

        {isHost && !room.canStart && (
          <p className="hint">
            Il faut au moins {MIN_PLAYERS} joueurs pour lancer la partie ({room.players.length} pour
            le moment).
          </p>
        )}
      </section>

      <section className="panel">
        <h2>
          Composition <span className="count">(annoncée d’avance)</span>
        </h2>
        <p className="hint">
          Voici les rôles qui seront distribués au hasard. Chacun le sait : personne ne pourra
          prétendre le contraire.
        </p>
        <Composition roles={room.composition} />
      </section>

      {isHost && (
        <section className="panel">
          <h2>
            Bots <span className="count">(entraînement)</span>
          </h2>
          <p className="hint">
            Ajoutez des joueurs automatiques pour tester une partie sans réunir {MIN_PLAYERS}{' '}
            personnes.
          </p>

          <div className="actions">
            <label className="field inline">
              <span>Nombre</span>
              <input
                type="number"
                min={1}
                max={Math.max(1, seats)}
                value={botCount}
                disabled={!online || seats === 0}
                onChange={(event) => setBotCount(Number(event.target.value))}
              />
            </label>

            <button
              type="button"
              className="btn primary"
              disabled={!online || seats === 0}
              onClick={addBots}
            >
              Ajouter des bots
            </button>

            {missing > 0 && seats > 0 && (
              <button
                type="button"
                className="btn"
                disabled={!online}
                onClick={() => ajouterBots(missing)}
              >
                Compléter à {MIN_PLAYERS}
              </button>
            )}

            {bots.length > 0 && (
              <button
                type="button"
                className="btn danger"
                disabled={!online}
                onClick={retirerTousLesBots}
              >
                Retirer tous les bots
              </button>
            )}
          </div>
        </section>
      )}

      <ChatPanel messages={room.messages} onSend={sendChat} disabled={!online} />
    </main>
  );
}
