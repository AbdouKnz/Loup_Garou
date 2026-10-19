import type { ConnectionStatus } from '../state/room';

type Look = {
  text: string;
  tone: string;
};

const LOOKS: Record<ConnectionStatus, Look> = {
  idle: { text: 'HORS LIGNE', tone: 'ko' },
  connecting: { text: 'CONNEXION…', tone: 'wait' },
  online: { text: 'EN LIGNE', tone: 'ok' },
  reconnecting: { text: 'RECONNEXION…', tone: 'wait' },
  offline: { text: 'HORS LIGNE', tone: 'ko' },
};

/** ConnectionBadge indique l'état du temps réel, d'un coup d'œil. */
export default function ConnectionBadge({ status }: { status: ConnectionStatus }) {
  const look = LOOKS[status];

  return (
    <span className={`pill ${look.tone}`} title="État de la connexion au serveur">
      {look.text}
    </span>
  );
}
