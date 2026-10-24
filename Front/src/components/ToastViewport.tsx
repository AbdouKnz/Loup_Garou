import { useEffect } from 'react';

import { useToastStore } from '../state/toast';
import type { Toast } from '../state/toast';
import { CheckIcon, CloseIcon } from './lobby/icons';

/** Durée d'affichage d'une confirmation : assez pour être lue, pas plus. */
const TOAST_MS = 4200;

function ToastLine({ toast }: { toast: Toast }) {
  const dismiss = useToastStore((state) => state.dismiss);

  useEffect(() => {
    const timer = window.setTimeout(() => dismiss(toast.id), TOAST_MS);

    return () => window.clearTimeout(timer);
  }, [dismiss, toast.id]);

  return (
    <li className={`toast toast-${toast.tone}`} role="status">
      <span className="toast-icon" aria-hidden="true">
        <CheckIcon />
      </span>
      <span className="toast-message">{toast.message}</span>
      <button
        type="button"
        className="toast-close"
        aria-label="Masquer le message"
        onClick={() => dismiss(toast.id)}
      >
        <CloseIcon />
      </button>
    </li>
  );
}

/**
 * ToastViewport affiche les confirmations locales au-dessus de la coquille.
 * Le conteneur reste monté : les lecteurs d'écran annoncent chaque arrivée.
 */
export default function ToastViewport() {
  const toasts = useToastStore((state) => state.toasts);

  return (
    <ul className="toast-viewport" aria-live="polite" aria-atomic="false">
      {toasts.map((toast) => (
        <ToastLine key={toast.id} toast={toast} />
      ))}
    </ul>
  );
}
