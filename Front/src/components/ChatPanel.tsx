import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';

import { MAX_CHAT_LENGTH } from '../api/types';
import type { ChatMessage } from '../api/types';

type Props = {
  messages: ChatMessage[];
  onSend(text: string): void;
  /** Vrai quand la discussion est indisponible (connexion perdue, nuit, mort). */
  disabled: boolean;
  /** Explication affichée dans le champ quand la discussion est fermée. */
  disabledReason?: string;
};

/** ChatPanel affiche le fil de discussion et permet d'écrire. */
export default function ChatPanel({ messages, onSend, disabled, disabledReason }: Props) {
  const [draft, setDraft] = useState('');
  const logRef = useRef<HTMLOListElement | null>(null);

  // Le fil reste collé au bas, comme dans une messagerie. On fait défiler le
  // journal lui-même, jamais la page : un message ne doit pas emporter la table
  // et le fond du village hors de l'écran.
  useEffect(() => {
    const log = logRef.current;
    if (log) {
      log.scrollTop = log.scrollHeight;
    }
  }, [messages.length]);

  const submit = (event: FormEvent) => {
    event.preventDefault();

    const text = draft.trim();
    if (text === '') {
      return;
    }

    onSend(text);
    setDraft('');
  };

  return (
    <section className="panel chat">
      <h2>Discussion</h2>

      <ol className="chat-log" ref={logRef}>
        {messages.length === 0 && <li className="chat-empty">Personne n’a encore parlé.</li>}

        {messages.map((message) => (
          <li key={message.id} className={message.system ? 'chat-line system' : 'chat-line'}>
            <span className="chat-author">{message.pseudo}</span>
            <span className="chat-text">{message.text}</span>
            <time className="chat-time" dateTime={message.at}>
              {heure(message.at)}
            </time>
          </li>
        ))}
      </ol>

      <form className="chat-form" onSubmit={submit}>
        <input
          type="text"
          value={draft}
          maxLength={MAX_CHAT_LENGTH}
          placeholder={disabled ? (disabledReason ?? 'Connexion perdue…') : 'Votre message'}
          aria-label="Votre message"
          onChange={(event) => setDraft(event.target.value)}
          disabled={disabled}
        />
        <button type="submit" className="btn primary" disabled={disabled || draft.trim() === ''}>
          Envoyer
        </button>
      </form>
    </section>
  );
}

/** heure formate l'horodatage d'un message pour l'affichage. */
function heure(at: string): string {
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) {
    return '';
  }

  return date.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
}
