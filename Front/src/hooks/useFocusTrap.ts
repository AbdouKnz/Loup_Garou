import { useEffect, useRef } from 'react';
import type { RefObject } from 'react';

/** Éléments qui peuvent recevoir le focus dans une boîte de dialogue. */
const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

/**
 * useFocusTrap garde le clavier dans une boîte de dialogue.
 *
 * À l'ouverture, le focus va au premier élément utile — ou à la boîte
 * elle-même quand elle n'en a aucun, comme la carte de rôle pendant la
 * distribution. Tab et Shift+Tab bouclent, Échap ferme, et le focus revient à
 * l'élément qui avait ouvert la boîte.
 */
export function useFocusTrap<T extends HTMLElement>(
  ref: RefObject<T | null>,
  onClose?: () => void,
): void {
  const closeRef = useRef(onClose);

  // La fermeture peut changer entre deux rendus ; la référence suit sans
  // réenregistrer l'écouteur de clavier.
  useEffect(() => {
    closeRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    const node = ref.current;
    if (!node) {
      return;
    }

    const previous =
      document.activeElement instanceof HTMLElement && document.activeElement !== document.body
        ? document.activeElement
        : null;

    const focusables = (): HTMLElement[] =>
      Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
        (element) =>
          !element.hasAttribute('disabled') && element.getAttribute('aria-hidden') !== 'true',
      );

    const first = focusables()[0] ?? node;
    first.focus();

    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (closeRef.current) {
          event.stopPropagation();
          closeRef.current();
        }
        return;
      }

      if (event.key !== 'Tab') {
        return;
      }

      const items = focusables();
      if (items.length === 0) {
        event.preventDefault();
        node.focus();
        return;
      }

      const firstItem = items[0];
      const lastItem = items[items.length - 1];

      if (event.shiftKey && document.activeElement === firstItem) {
        event.preventDefault();
        lastItem.focus();
      } else if (!event.shiftKey && document.activeElement === lastItem) {
        event.preventDefault();
        firstItem.focus();
      }
    };

    document.addEventListener('keydown', onKey);

    return () => {
      document.removeEventListener('keydown', onKey);
      if (previous && document.contains(previous)) {
        previous.focus();
      }
    };
  }, [ref]);
}
