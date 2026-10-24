import { useEffect, useState } from 'react';

/**
 * useMediaQuery suit une requête CSS et se met à jour au redimensionnement.
 *
 * Le premier rendu est déjà juste quand `matchMedia` est disponible ; sans lui
 * (tests, environnements très anciens), on retombe sur la mise en page large.
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return false;
    }

    return window.matchMedia(query).matches;
  });

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }

    const media = window.matchMedia(query);
    const update = () => setMatches(media.matches);

    update();

    if (typeof media.addEventListener === 'function') {
      media.addEventListener('change', update);
      return () => media.removeEventListener('change', update);
    }

    // Anciens navigateurs : l'API `addListener` reste le seul recours.
    media.addListener(update);
    return () => media.removeListener(update);
  }, [query]);

  return matches;
}
