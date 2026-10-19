/** NightStage occupe le village pendant que les rôles de la nuit se réveillent. */
export default function NightStage({ secondsLeft }: { secondsLeft: number | null }) {
  return (
    <section className="panel night-stage">
      <h2>🌙 Le village s’endort…</h2>
      <p className="hint">
        Les rôles de la nuit se réveillent un à un. Vous ne saurez rien avant l’aube : patientez, ou
        surveillez la discussion.
      </p>
      {secondsLeft !== null && (
        <p className="night-count">
          La nuit s’achève dans <strong>{secondsLeft} s</strong>.
        </p>
      )}
    </section>
  );
}
