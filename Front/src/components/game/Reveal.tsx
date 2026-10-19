import { ROLE_LABELS } from './labels';
import type { DeathCause, DeathView, Phase, Role } from '../../api/types';

type Props = {
  phase: Phase;
  over: boolean;
  /** Morts de la nuit, annoncées à l'aube. */
  deaths?: DeathView[];
  eliminated?: string;
  eliminatedRole?: Role;
  nameOf: (id: string) => string;
};

/** Ce qu'on dit d'un mort selon ce qui l'a emporté. */
const DEATH_TEXTS: Record<DeathCause, string> = {
  loups: 'a été dévoré pendant la nuit',
  poison: 'est mort dans d’étranges circonstances',
};

const DEATH_EMOJI: Record<DeathCause, string> = {
  loups: '🩸',
  poison: '☠️',
};

/**
 * Reveal met en scène les révélations : les morts de la nuit à l'aube — elles
 * peuvent être plusieurs, et chacune porte sa cause, jamais son rôle — puis
 * l'éliminé du jour au verdict, avec son rôle, lui.
 */
export default function Reveal({ phase, over, deaths, eliminated, eliminatedRole, nameOf }: Props) {
  const fallen = phase === 'aube' || over ? (deaths ?? []) : [];

  return (
    <>
      {fallen.map((death) => (
        <p key={death.target} className="reveal victim" role="status">
          {DEATH_EMOJI[death.cause]} <strong>{nameOf(death.target)}</strong>{' '}
          {DEATH_TEXTS[death.cause]}.
        </p>
      ))}

      {eliminated && (phase === 'verdict' || over) && (
        <p className="reveal verdict" role="status">
          ⚖️ <strong>{nameOf(eliminated)}</strong> a été éliminé par le village
          {eliminatedRole ? ` — c’était un ${ROLE_LABELS[eliminatedRole]}` : ''}.
        </p>
      )}
    </>
  );
}
