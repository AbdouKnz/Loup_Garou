import { create } from 'zustand';

/** Un message bref, affiché puis oublié : rien n'est conservé côté serveur. */
export type Toast = {
  id: number;
  message: string;
  tone: 'success' | 'info';
};

export type ToastStoreState = {
  toasts: Toast[];
  /** push ajoute un message et renvoie son identifiant. */
  push(message: string, tone?: Toast['tone']): number;
  /** dismiss retire un message, par exemple après son délai d'affichage. */
  dismiss(id: number): void;
  clear(): void;
};

let nextID = 1;

/**
 * Store des confirmations locales. Il ne porte aucune règle du jeu : seulement
 * le retour visuel d'une action déjà faite (invitation envoyée, table rejointe).
 */
export const useToastStore = create<ToastStoreState>((set) => ({
  toasts: [],

  push(message, tone = 'success') {
    const id = nextID;
    nextID += 1;

    set((state) => ({ toasts: [...state.toasts, { id, message, tone }].slice(-4) }));

    return id;
  },

  dismiss(id) {
    set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) }));
  },

  clear() {
    set({ toasts: [] });
  },
}));
