type Props = {
  wolves: string[];
  wolfVotes?: Record<string, string>;
  /** Vrai la nuit, quand les choix des compères sont connus. */
  showVotes: boolean;
  you: string;
  nameOf: (id: string) => string;
};

/** Den est l'antre des loups : la liste des compères, et leurs choix la nuit. */
export default function Den({ wolves, wolfVotes, showVotes, you, nameOf }: Props) {
  return (
    <section className="panel wolves">
      <h2>
        L’antre <span className="count">(vos compères)</span>
      </h2>

      <ul className="wolf-list">
        {wolves.map((id) => (
          <li key={id}>
            <span className="player-name">
              {nameOf(id)}
              {id === you && <span className="tag-you">vous</span>}
            </span>

            {showVotes && (
              <span className="wolf-target">
                {wolfVotes?.[id] ? `choisit ${nameOf(wolfVotes[id])}` : 'n’a pas encore choisi'}
              </span>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
