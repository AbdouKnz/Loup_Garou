import { useEffect, useRef, useState } from 'react';

import ChatPanel from '../components/ChatPanel';
import Ballot from '../components/game/Ballot';
import DayRecap from '../components/game/DayRecap';
import Den from '../components/game/Den';
import NarrationCard from '../components/game/NarrationCard';
import NightStage from '../components/game/NightStage';
import NotesPanel from '../components/game/NotesPanel';
import PhaseHeader from '../components/game/PhaseHeader';
import PhaseTransition from '../components/game/PhaseTransition';
import Potions from '../components/game/Potions';
import Reveal from '../components/game/Reveal';
import RoleCard from '../components/game/RoleCard';
import RoundTable from '../components/game/RoundTable';
import RulesPanel from '../components/game/RulesPanel';
import Scoreboard from '../components/game/Scoreboard';
import VillageBackdrop from '../components/game/VillageBackdrop';
import { isNightPhase } from '../components/game/moments';
import Vision from '../components/game/Vision';
import { PHASES, ROLE_HINTS, ROLE_LABELS } from '../components/game/labels';
import { dawnNarration } from '../components/game/narration';
import type { Narration } from '../components/game/narration';
import { cueForEvent, cueForPhase } from '../audio/sounds';
import { useAmbience } from '../hooks/useAmbience';
import { useNotes } from '../hooks/useNotes';
import { useRoomStore } from '../state/room';
import { useSoundStore } from '../state/sound';
import { useVoiceStore } from '../state/voice';
import type { GameView, NightAnswer, Phase, PlayerView, RoomState } from '../api/types';

/** Libellé de l'action proposée sur la table, selon ce qu'attend la veillée. */
const INTENT_LABELS = {
  devorer: 'Dévorer',
  voir: 'Observer',
  empoisonner: 'Empoisonner',
  voter: 'Voter',
};

/**
 * Choix retenu en attendant confirmation : les réponses d'une veillée, plus le
 * vote des loups, qui passe par la nuit et non par la veillée.
 */
type PendingAction = NightAnswer | 'devorer';

/** Verbe d'une confirmation, selon l'action choisie. */
const CONFIRM_VERBS: Record<PendingAction, string> = {
  devorer: 'Dévorer',
  voir: 'Observer',
  soigner: 'Soigner',
  empoisonner: 'Empoisonner',
  passer: 'Passer',
};

/**
 * Durée maximale pendant laquelle la carte attend la voix : un navigateur qui
 * n'achève jamais sa lecture ne doit pas retenir le récit indéfiniment.
 */
const NARRATION_MAX_MS = 20_000;

/** Le récit affiché : son texte, et la nuit à laquelle il se rattache. */
type Story = { key: string; narration: Narration };

/**
 * Game est l'écran de la partie : phase en cours, table, votes et fin de partie.
 *
 * L'écran ne décide de rien : il affiche la vue calculée par le serveur et
 * n'envoie que des intentions. Notamment, il ne devine jamais un rôle : ce que
 * le serveur ne dit pas n'existe pas ici.
 */
export default function Game({ room, game }: { room: RoomState; game: GameView }) {
  const connection = useRoomStore((state) => state.connection);
  const event = useRoomStore((state) => state.event);
  const eventSeq = useRoomStore((state) => state.eventSeq);
  const history = useRoomStore((state) => state.history);
  const phaseTotal = useRoomStore((state) => state.phaseTotal);
  const error = useRoomStore((state) => state.error);
  const sendChat = useRoomStore((state) => state.sendChat);
  const voteJour = useRoomStore((state) => state.voteJour);
  const voteNuit = useRoomStore((state) => state.voteNuit);
  const agirNuit = useRoomStore((state) => state.agirNuit);
  const terminerParole = useRoomStore((state) => state.terminerParole);
  const rejouer = useRoomStore((state) => state.rejouer);
  const leave = useRoomStore((state) => state.leave);
  const dismissError = useRoomStore((state) => state.dismissError);
  const play = useSoundStore((state) => state.play);
  const speak = useVoiceStore((state) => state.speak);
  const cancelVoice = useVoiceStore((state) => state.cancel);
  const voiceMuted = useVoiceStore((state) => state.muted);

  const left = useCountdown(game.deadline);
  /** Choix déjà fait, en attente de confirmation : la nuit se joue en un clic. */
  const [pending, setPending] = useState<{
    action: PendingAction;
    target: string;
    key: string;
  } | null>(null);
  /** Vote du jour en attente de confirmation : un clic ne suffit plus à éliminer. */
  const [pendingVote, setPendingVote] = useState<{ target: string; key: string } | null>(null);
  /** Étape où la sorcière a ouvert sa potion de mort, en attendant sa victime. */
  const [picking, setPicking] = useState<string | null>(null);
  /** Carte de rôle rouverte à la main : elle se referme avec l'étape en cours. */
  const [roleOpenKey, setRoleOpenKey] = useState<string | null>(null);
  /** Le récit affiché : il survit à l'aube tant que la voix le lit. */
  const [story, setStory] = useState<Story | null>(null);
  /** Lecture en cours — sa clé et son numéro d'ordre. Le numéro évite qu'une
   * ancienne utterance, interrompue, ne close la lecture qui l'a remplacée. */
  const [reading, setReading] = useState<{ key: string; token: number } | null>(null);
  /** Récit masqué à la main : la clé le rattache à une nuit précise. */
  const [narrationHidden, setNarrationHidden] = useState<string | null>(null);
  /** Aide de jeu ouverte par-dessus la table. */
  const [rulesOpen, setRulesOpen] = useState(false);
  const readToken = useRef(0);

  // Le fond sonore suit la phase, sans jamais trahir une information privée.
  useAmbience(game.phase);
  // Le carnet de soupçons reste sur cet appareil : il ne part jamais au serveur.
  const { notes, cycle } = useNotes(room.code);

  // Couper la voix met fin à la lecture en cours : la carte ne reste pas
  // suspendue à un récit qui ne viendra plus. Pendant l'aube, elle reste
  // affichée — le récit se lit aussi des yeux.
  const [previousMuted, setPreviousMuted] = useState(voiceMuted);
  if (previousMuted !== voiceMuted) {
    setPreviousMuted(voiceMuted);
    if (voiceMuted) {
      setReading(null);
    }
  }

  const me = room.players.find((player) => player.id === room.you);
  const isHost = room.you === room.hostId;
  const online = connection === 'online';
  const over = game.phase === 'fin';
  const dead = me?.dead === true;
  const nameOf = (id: string) => room.players.find((player) => player.id === id)?.pseudo ?? '?';

  /**
   * Le récit de l'aube : les faits publics de la nuit qui vient de s'achever.
   * Il ne sort que pendant la phase d'aube. L'échéance distingue deux aubes de
   * même jour — deux parties relancées, par exemple.
   */
  const dawnKey = `aube-${game.day}-${game.deadline ?? ''}`;
  const dawn = game.phase === 'aube' ? dawnNarration(history, game.day, game.deaths, nameOf) : null;

  const candidates = game.candidates ?? [];
  const runoff = game.round > 1 && candidates.length > 0;
  const isWolf = game.yourRole === 'loup-garou';
  const isSeer = game.yourRole === 'voyante';
  const night = game.phase === 'nuit';
  const dayVote = game.phase === 'vote';
  /**
   * Le débat est un tour de parole : le serveur dit qui l'a, et la table suit.
   * On ne déduit jamais cet ordre d'autre chose que de la vue reçue.
   */
  const debating = game.phase === 'debat';
  const speaker = debating ? (game.speaker ?? null) : null;
  const myTurn = speaker !== null && speaker === room.you;
  /**
   * Identifie la veillée en cours : la nuit change d'acteur sans changer de
   * phase, et l'échéance les distingue.
   */
  const stepKey = `${game.phase}-${game.day}-${game.round}-${game.deadline ?? ''}`;

  // Ce que le serveur attend de ce joueur à cet instant : rien, la plupart du
  // temps. La nuit, il le dit ; le jour, c'est le vote du village.
  const acting = night && !dead ? (game.yourAction ?? null) : null;
  const poisonOpen = picking === stepKey && acting === 'potions';

  // --- Sons ----------------------------------------------------------------
  // Chaque fait du narrateur a son repère sonore. On ne sonne que sur un fait
  // *nouveau* : un événement déjà rangé dans le store au moment où l'écran
  // s'ouvre (reprise d'une partie finie) ne doit pas rejouer.
  const handledSeq = useRef(eventSeq);
  useEffect(() => {
    if (eventSeq === handledSeq.current) {
      return;
    }
    handledSeq.current = eventSeq;

    const cue = event ? cueForEvent(event) : null;
    if (cue !== null) {
      play(cue);
    }
  }, [event, eventSeq, play]);

  // Le passage d'une phase à l'autre a son repère : la nuit tombe, l'aube se
  // lève… Le premier instantané est muet (on arrive sur une partie en cours).
  const previousPhase = useRef<Phase | null>(null);
  useEffect(() => {
    const cue = cueForPhase(game.phase, previousPhase.current);
    previousPhase.current = game.phase;

    if (cue !== null) {
      play(cue);
    }
  }, [game.phase, play]);

  // La veillée m'attend : le serveur le dit par « yourAction ». Le repère ne
  // vient que de cette information, personnelle, jamais d'une déduction.
  useEffect(() => {
    if (acting !== null) {
      play('yourTurn');
    }
  }, [stepKey, acting, play]);

  // Le débat change d'orateur : un repère discret à chaque prise de parole, et
  // celui de l'attente quand c'est à vous. Le silence entre deux débats remet
  // le repère à zéro, pour que le premier orateur de chaque jour sonne.
  const previousSpeaker = useRef<string | null>(null);
  useEffect(() => {
    if (speaker === null) {
      previousSpeaker.current = null;
      return;
    }
    if (speaker === previousSpeaker.current) {
      return;
    }
    previousSpeaker.current = speaker;
    play(myTurn ? 'yourTurn' : 'turn');
  }, [speaker, myTurn, play]);

  // Le récit de l'aube est lu une fois, au moment où la nuit se referme — pas
  // quand on arrive en cours de route. La carte suit la voix : elle reste à
  // l'écran tant que la lecture n'est pas finie, même après la fin de l'aube.
  const storyKey = useRef<string | null>(null);
  const dawnSpoken = useRef(game.phase === 'aube');
  useEffect(() => {
    if (game.phase !== 'aube') {
      dawnSpoken.current = false;
      return;
    }
    if (dawn === null) {
      return;
    }

    // Le récit est fixé dès la première image de l'aube : les faits de la nuit
    // sont arrivés avant l'instantané. On ne le remplace pas ensuite.
    if (storyKey.current !== dawnKey) {
      storyKey.current = dawnKey;
      setStory({ key: dawnKey, narration: dawn });
    }

    if (dawnSpoken.current) {
      return;
    }
    dawnSpoken.current = true;
    setNarrationHidden(null);

    const token = ++readToken.current;
    const started = speak(dawn.spoken, () => {
      setReading((active) => (active?.token === token ? null : active));
    });
    setReading(started ? { key: dawnKey, token } : null);
  }, [game.phase, dawnKey, dawn, speak]);

  // Garde-fou : un navigateur qui n'achève jamais sa lecture ne retient pas la
  // carte au-delà du délai.
  useEffect(() => {
    if (reading === null) {
      return;
    }

    const token = reading.token;
    const timer = window.setTimeout(() => {
      setReading((active) => (active?.token === token ? null : active));
      cancelVoice();
    }, NARRATION_MAX_MS);

    return () => window.clearTimeout(timer);
  }, [reading, cancelVoice]);

  useEffect(() => () => cancelVoice(), [cancelVoice]);

  /**
   * Intention proposée sur la table : chaque clic y répond. Un mort, un joueur
   * hors ligne ou un spectateur n'en ont aucune.
   */
  const intent: keyof typeof INTENT_LABELS | null = (() => {
    if (dead || !online) {
      return null;
    }
    if (dayVote) {
      return 'voter';
    }
    if (!night) {
      return null;
    }
    if (acting === 'devorer' || acting === 'voir') {
      return acting;
    }
    if (acting === 'potions' && poisonOpen) {
      return 'empoisonner';
    }

    return null;
  })();

  /** votable dit si ce joueur peut être désigné au tour en cours. */
  const votable = (player: PlayerView) =>
    player.id !== room.you &&
    !player.dead &&
    // La nuit, un loup ne se dévore pas lui-même ni ses compères (mêmes règles
    // que les bots).
    !(intent === 'devorer' && (game.wolves ?? []).includes(player.id)) &&
    (!runoff || candidates.includes(player.id));

  /** choose retient le choix du joueur : la nuit, il demande confirmation. */
  const choose = (target: string) => {
    if (intent === null) {
      return;
    }
    if (intent === 'voter') {
      setPendingVote({ target, key: stepKey });
      return;
    }

    setPending({ action: intent, target, key: stepKey });
  };

  /** apply envoie un choix confirmé : le vote des loups passe par la nuit. */
  const apply = (choice: { action: PendingAction; target: string }) => {
    if (choice.action === 'devorer') {
      voteNuit(choice.target);
    } else {
      agirNuit(choice.action, choice.target);
    }

    setPending(null);
  };

  // Une confirmation ne vaut que pour la veillée où elle a été demandée :
  // changer d'étape (nuit écourtée, veillée suivante) l'annule d'elle-même.
  const confirm = pending !== null && pending.key === stepKey ? pending : null;
  const voteConfirm = pendingVote !== null && pendingVote.key === stepKey ? pendingVote : null;

  /** applyVote envoie le vote confirmé, une seule fois. */
  const applyVote = () => {
    if (voteConfirm === null) {
      return;
    }

    voteJour(voteConfirm.target);
    setPendingVote(null);
  };

  // La discussion se ferme la nuit, et pour les morts jusqu'à la fin.
  const chatClosed = (() => {
    if (!online) {
      return 'Connexion perdue…';
    }
    if (dead && !over) {
      return 'Les morts ne parlent plus : ils regardent.';
    }
    if (night && !over) {
      return 'Le village dort : personne ne parle.';
    }
    if (game.phase === 'roles') {
      return 'Chacun découvre son rôle : la partie commence.';
    }
    if (debating && speaker !== null && !myTurn) {
      return `${nameOf(speaker)} a la parole : écoutez, puis ce sera à vous.`;
    }

    return null;
  })();

  // La distribution des rôles est une pause : la carte s'impose, et c'est le
  // serveur qui en donne la fin. Aucun compteur du navigateur ne décide du
  // départ de la partie.
  const dealing = game.phase === 'roles';
  const roleOpen = dealing || roleOpenKey === stepKey;

  /**
   * Sièges éclairés la nuit : les compères — que le serveur ne donne qu'aux
   * loups — et soi-même pendant sa veillée. Rien d'autre n'allume un siège :
   * ce que le serveur ne dit pas n'existe pas ici.
   */
  const awake = [
    ...(night && isWolf ? (game.wolves ?? []) : []),
    ...(night && acting !== null ? [room.you] : []),
  ];

  /** Jetons du scrutin : les voix publiques reçues par chaque cible. */
  const tally: Record<string, number> = {};
  for (const target of Object.values(game.ballot ?? {})) {
    if (target) {
      tally[target] = (tally[target] ?? 0) + 1;
    }
  }

  const leader = Object.entries(tally).sort((a, b) => b[1] - a[1])[0];
  const scored = dayVote || game.phase === 'verdict';

  /** Ce que raconte le centre de la table. */
  const center = {
    emoji: PHASES[game.phase].emoji,
    title: dealing
      ? PHASES.roles.title
      : over
        ? PHASES.fin.title
        : `${night ? 'Nuit' : 'Jour'} ${game.day}`,
    note:
      !over && scored && leader
        ? `${leader[1]} voix pour ${nameOf(leader[0])}`
        : `${game.alive.length} vivants`,
  };

  /** Cible marquée par la veillée en cours : la proie, pour la sorcière. */
  const marked = acting === 'potions' ? game.wolfTarget : undefined;

  /**
   * Le récit à l'écran : celui de l'aube en cours, ou celui que la voix lit
   * encore après la fin de l'aube. Masqué à la main, il disparaît.
   */
  const shownStory =
    story !== null &&
    narrationHidden !== story.key &&
    (game.phase === 'aube' || reading?.key === story.key)
      ? story
      : null;

  /** replay relit le récit affiché, et retient la carte jusqu'à la fin. */
  const replay = (target: Story) => {
    const token = ++readToken.current;
    const started = speak(target.narration.spoken, () => {
      setReading((active) => (active?.token === token ? null : active));
    });
    setReading(started ? { key: target.key, token } : null);
  };

  const moment = isNightPhase(game.phase) ? 'night' : 'day';

  return (
    <>
      <VillageBackdrop phase={game.phase} />
      <main
        className="game"
        data-phase={game.phase}
        data-moment={moment}
        data-role={isWolf ? 'wolf' : 'villager'}
      >
        <PhaseHeader
          code={room.code}
          phase={game.phase}
          day={game.day}
          over={over}
          dead={dead}
          secondsLeft={left}
          phaseTotal={phaseTotal}
          runoff={runoff}
          connection={connection}
          onLeave={leave}
          onRules={dealing ? undefined : () => setRulesOpen(true)}
        />

        {error && (
          <p className="alert" role="alert">
            <span>{error}</span>
            <button type="button" className="link" onClick={dismissError}>
              fermer
            </button>
          </p>
        )}

        {event && (
          <p key={eventSeq} className="scene" aria-live="polite">
            {event.text}
          </p>
        )}

        <div className="game-stage">
          <section className="table-panel" aria-label={`La table (${game.alive.length} vivants)`}>
            {dayVote && <DayRecap game={game} history={history} nameOf={nameOf} />}

            {debating && speaker !== null && (
              <p className={myTurn ? 'speaker-banner mine' : 'speaker-banner'} role="status">
                {myTurn ? (
                  <>🎤 À vous de parler — dites ce que vous avez vu.</>
                ) : (
                  <>🎤 {nameOf(speaker)} a la parole.</>
                )}
                {left !== null && <span className="count"> · {left} s</span>}
              </p>
            )}

            <RoundTable
              players={room.players}
              you={room.you}
              yourVote={game.yourVote}
              action={
                intent !== null
                  ? { label: INTENT_LABELS[intent], can: votable, onChoose: choose }
                  : undefined
              }
              awake={awake}
              speaking={speaker ?? undefined}
              marked={marked}
              votes={tally}
              notes={notes}
              center={center}
            />

            {myTurn && (
              <div className="actions vote-actions">
                <button type="button" className="btn primary" onClick={terminerParole}>
                  J’ai fini de parler
                </button>
                <span className="hint">Le débat passera au joueur suivant.</span>
              </div>
            )}

            {intent !== null && intent !== 'voir' && intent !== 'empoisonner' && (
              <div className="actions vote-actions">
                <button type="button" className="btn" onClick={() => choose('')}>
                  S’abstenir
                </button>
                {me?.voted === true && (
                  <span className="hint">
                    {game.yourVote
                      ? `Votre choix : ${nameOf(game.yourVote)}. Vous pouvez changer.`
                      : 'Vous vous êtes abstenu. Vous pouvez voter.'}
                  </span>
                )}
              </div>
            )}

            {intent === 'voir' && (
              <p className="hint">
                🔮 Choisissez un joueur : vous découvrirez son rôle, et vous seule le saurez.
              </p>
            )}

            {intent === 'empoisonner' && (
              <p className="hint">☠️ Choisissez la victime de votre potion de mort.</p>
            )}

            {confirm && (
              <div className="confirm" role="alertdialog" aria-label="Confirmer votre choix">
                <p>
                  {CONFIRM_VERBS[confirm.action]} {nameOf(confirm.target)} cette nuit ?
                </p>
                <div className="actions">
                  <button
                    type="button"
                    className="btn primary"
                    onClick={() => {
                      apply(confirm);
                      setPicking(null);
                    }}
                  >
                    Oui, {CONFIRM_VERBS[confirm.action].toLowerCase()}
                  </button>
                  <button type="button" className="btn" onClick={() => setPending(null)}>
                    Annuler
                  </button>
                </div>
              </div>
            )}

            {voteConfirm && (
              <div
                className="confirm vote-confirm"
                role="alertdialog"
                aria-label="Confirmer votre vote"
              >
                <p>
                  {voteConfirm.target
                    ? `Voter contre ${nameOf(voteConfirm.target)} ?`
                    : 'Confirmer votre abstention ?'}
                </p>
                <div className="actions">
                  <button type="button" className="btn primary" onClick={applyVote} autoFocus>
                    {voteConfirm.target ? 'Oui, voter' : 'Oui, m’abstenir'}
                  </button>
                  <button type="button" className="btn" onClick={() => setPendingVote(null)}>
                    Annuler
                  </button>
                </div>
              </div>
            )}
          </section>

          <aside className="game-side">
            <div className="game-column">
              <section className="panel role">
                <h2>Votre rôle</h2>
                <p className={isWolf ? 'role-name wolf' : 'role-name'}>
                  {game.yourRole ? ROLE_LABELS[game.yourRole] : '—'}
                </p>
                <p className="hint">{game.yourRole ? ROLE_HINTS[game.yourRole] : ''}</p>

                {!dealing && game.yourRole && (
                  <button
                    type="button"
                    className="btn small"
                    onClick={() => setRoleOpenKey(stepKey)}
                  >
                    Revoir mon rôle
                  </button>
                )}
              </section>

              <NotesPanel players={room.players} you={room.you} notes={notes} onCycle={cycle} />

              {isWolf && !over && game.wolves && game.wolves.length > 0 && (
                <Den
                  wolves={game.wolves}
                  wolfVotes={game.wolfVotes}
                  showVotes={night}
                  you={room.you}
                  nameOf={nameOf}
                />
              )}

              {isSeer && !over && <Vision vision={game.yourVision} nameOf={nameOf} />}

              {acting === 'potions' && !over && (
                <Potions
                  potions={game.yourPotions}
                  wolfTarget={game.wolfTarget}
                  picking={poisonOpen}
                  nameOf={nameOf}
                  onHeal={() =>
                    setPending({ action: 'soigner', target: game.wolfTarget ?? '', key: stepKey })
                  }
                  onPickPoison={() => setPicking(stepKey)}
                  onCancelPoison={() => setPicking(null)}
                  onPass={() => {
                    setPicking(null);
                    agirNuit('passer');
                  }}
                />
              )}

              {night && !over && acting === null && <NightStage secondsLeft={left} />}

              {runoff && !over && (
                <p className="runoff-banner" role="status">
                  ⚖️ Égalité : second tour entre {candidates.map((id) => nameOf(id)).join(' et ')}.
                  Seuls ces joueurs peuvent être désignés.
                </p>
              )}

              {game.ballot && (dayVote || game.phase === 'verdict') && (
                <Ballot
                  ballot={game.ballot}
                  players={room.players}
                  you={room.you}
                  closed={game.phase === 'verdict'}
                />
              )}

              <Reveal
                phase={game.phase}
                over={over}
                deaths={game.deaths}
                eliminated={game.eliminated}
                eliminatedRole={game.eliminatedRole}
                nameOf={nameOf}
              />

              {over && (
                <Scoreboard
                  winner={game.winner}
                  roles={game.roles}
                  players={room.players}
                  history={history}
                  isHost={isHost}
                  online={online}
                  onReplay={rejouer}
                />
              )}

              {!over && (
                <p className="hint">
                  Votre place est gardée même si vous fermez l’onglet : revenez avec le même lien.
                </p>
              )}
            </div>

            <ChatPanel
              messages={room.messages}
              onSend={sendChat}
              disabled={chatClosed !== null}
              disabledReason={chatClosed ?? undefined}
            />
          </aside>
        </div>

        <PhaseTransition phase={game.phase} day={game.day} />

        {shownStory && (
          <NarrationCard
            narration={shownStory.narration}
            muted={voiceMuted}
            onReplay={() => replay(shownStory)}
            onClose={() => {
              setNarrationHidden(shownStory.key);
              setReading(null);
              cancelVoice();
            }}
          />
        )}

        {roleOpen && game.yourRole && (
          <RoleCard
            role={game.yourRole}
            wolves={game.wolves}
            you={room.you}
            nameOf={nameOf}
            secondsLeft={dealing ? left : null}
            onClose={dealing ? undefined : () => setRoleOpenKey(null)}
          />
        )}
      </main>

      {rulesOpen && (
        <RulesPanel composition={room.composition} onClose={() => setRulesOpen(false)} />
      )}
    </>
  );
}

/**
 * useCountdown rend les secondes restantes avant une échéance, et les rafraîchit
 * quatre fois par seconde : l'affichage ne dépend pas de la fréquence des
 * instantanés du serveur.
 */
function useCountdown(deadline: string | undefined): number | null {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!deadline) {
      return;
    }

    const timer = window.setInterval(() => setNow(Date.now()), 250);

    return () => window.clearInterval(timer);
  }, [deadline]);

  if (!deadline) {
    return null;
  }

  const target = new Date(deadline).getTime();
  if (Number.isNaN(target)) {
    return null;
  }

  return Math.max(0, Math.round((target - now) / 1000));
}
