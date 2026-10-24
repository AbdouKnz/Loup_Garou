import { useRef } from 'react';

import AmbienceToggle from '../AmbienceToggle';
import SoundToggle from '../SoundToggle';
import VoiceToggle from '../VoiceToggle';
import { useFocusTrap } from '../../hooks/useFocusTrap';
import { useAmbienceStore } from '../../state/ambience';
import { ROLE_DETAILS, ROLE_EMOJI, ROLE_LABELS } from './labels';
import type { Role } from '../../api/types';

type Props = {
  /** Rôles annoncés dans la salle : le panneau ne détaille que ceux-là. */
  composition: Role[];
  onClose: () => void;
};

/** Ordre de lecture des rôles, du plus commun au plus particulier. */
const ROLE_ORDER: Role[] = ['villageois', 'loup-garou', 'voyante', 'sorciere'];

/**
 * RulesPanel est l'aide de jeu : le cycle, les rôles, le vote et les réglages.
 *
 * Il ne lit que ce que la salle annonce déjà — la composition des rôles est
 * publique — et ne révèle jamais qui porte quoi. Il s'ouvre depuis la salle
 * d'attente comme depuis la partie.
 */
export default function RulesPanel({ composition, onClose }: Props) {
  const dialog = useRef<HTMLDivElement>(null);
  useFocusTrap(dialog, onClose);

  const volume = useAmbienceStore((state) => state.volume);
  const setVolume = useAmbienceStore((state) => state.setVolume);

  const present = new Set(composition);
  const roles =
    composition.length > 0 ? ROLE_ORDER.filter((role) => present.has(role)) : ROLE_ORDER;

  return (
    <div
      className="rules-backdrop"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <div
        ref={dialog}
        className="rules-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="rules-title"
        tabIndex={-1}
      >
        <header className="rules-head">
          <div>
            <p className="eyebrow">Aide de jeu</p>
            <h2 id="rules-title">Règles du village</h2>
          </div>
          <button type="button" className="btn" onClick={onClose}>
            Fermer
          </button>
        </header>

        <section className="rules-section">
          <h3>But du jeu</h3>
          <p>
            Le village gagne quand plus aucun loup n’est vivant. Les loups gagnent dès qu’ils sont
            aussi nombreux que les villageois : le village ne peut plus les mettre en minorité.
          </p>
        </section>

        <section className="rules-section">
          <h3>Le cycle</h3>
          <ol className="rules-cycle">
            <li>
              La nuit tombe : la Voyante observe, les Loups désignent une proie, puis la Sorcière
              peut intervenir.
            </li>
            <li>L’aube révèle les morts de la nuit, sans leur rôle.</li>
            <li>
              Le débat donne la parole à chaque vivant, dans l’ordre de la table. Chacun peut
              terminer son tour quand il a fini.
            </li>
            <li>
              Le vote élimine un suspect. En cas d’égalité, un second tour départage les joueurs à
              égalité ; s’il reste une égalité, personne n’est éliminé.
            </li>
            <li>Le verdict révèle le rôle de l’éliminé, puis la nuit revient.</li>
          </ol>
        </section>

        <section className="rules-section">
          <h3>Les rôles en jeu</h3>
          <div className="rules-roles">
            {roles.map((role) => {
              const detail = ROLE_DETAILS[role];
              return (
                <article
                  key={role}
                  className={role === 'loup-garou' ? 'rule-role wolf' : 'rule-role'}
                >
                  <header className="rule-role-head">
                    <span className="rule-role-emoji" aria-hidden="true">
                      {ROLE_EMOJI[role]}
                    </span>
                    <div>
                      <h4>{ROLE_LABELS[role]}</h4>
                      <p className="rule-role-camp">
                        {detail.camp === 'loups' ? 'Camp des loups' : 'Camp du village'}
                      </p>
                    </div>
                  </header>
                  <p>
                    <strong>Objectif :</strong> {detail.goal}
                  </p>
                  <p>
                    <strong>La nuit :</strong> {detail.night}
                  </p>
                  <p>
                    <strong>Le jour :</strong> {detail.day}
                  </p>
                  <p className="rule-role-tip">{detail.tip}</p>
                </article>
              );
            })}
          </div>
        </section>

        <section className="rules-section">
          <h3>Informations publiques et privées</h3>
          <p>
            Tout le monde voit les vivants, les morts, les votes et le tour de parole. Les rôles
            restent secrets jusqu’à la fin. Seuls les loups voient leur antre, la Voyante son
            carnet, et la Sorcière la proie des loups.
          </p>
        </section>

        <section className="rules-section">
          <h3>Son et confort</h3>
          <div className="rules-audio">
            <div className="rules-audio-row">
              <span>Sons du jeu</span>
              <SoundToggle />
            </div>
            <div className="rules-audio-row">
              <span>Narration vocale</span>
              <VoiceToggle />
            </div>
            <div className="rules-audio-row">
              <span>Ambiance</span>
              <AmbienceToggle />
            </div>
            <label className="field rules-volume">
              <span>Volume de l’ambiance</span>
              <input
                type="range"
                min="0"
                max="100"
                step="1"
                value={Math.round(volume * 100)}
                onChange={(event) => setVolume(Number(event.target.value) / 100)}
              />
            </label>
          </div>
        </section>
      </div>
    </div>
  );
}
