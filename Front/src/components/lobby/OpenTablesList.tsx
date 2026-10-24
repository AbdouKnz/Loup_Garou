import { useId, useState } from 'react';

import type { OpenTable, OpenTableStatus } from './types';
import { CheckIcon, TableIcon, UsersIcon } from './icons';

type Filter = 'all' | OpenTableStatus;

type Props = {
  tables: OpenTable[];
  busy: boolean;
  onJoin: (table: OpenTable) => void;
  /** Tables de démonstration déjà « rejointes » dans cette session. */
  joined?: string[];
};

const FILTERS: Array<{ key: Filter; label: string }> = [
  { key: 'all', label: 'Tout' },
  { key: 'waiting', label: 'Disponibles' },
  { key: 'in_progress', label: 'En partie' },
];

/** numero met les compteurs sur deux chiffres, comme un carton d'invitation. */
function numero(value: number): string {
  return String(value).padStart(2, '0');
}

/** OpenTablesList montre les tables publiques de démonstration, filtrables. */
export default function OpenTablesList({ tables, busy, onJoin, joined = [] }: Props) {
  const [filter, setFilter] = useState<Filter>('all');
  const panelId = useId();

  const visible = filter === 'all' ? tables : tables.filter((table) => table.status === filter);

  return (
    <section className="lobby-section lobby-tables" id="home-tables" aria-labelledby="open-tables-title">
      <div className="lobby-section-head">
        <h2 className="lobby-section-title" id="open-tables-title">
          Tables ouvertes
        </h2>
        <span className="lobby-count">{numero(tables.length)}</span>

        <div className="lobby-tabs" role="tablist" aria-label="Filtrer les tables ouvertes">
          {FILTERS.map((item) => (
            <button
              key={item.key}
              type="button"
              role="tab"
              id={`${panelId}-${item.key}`}
              className={filter === item.key ? 'lobby-tab is-active' : 'lobby-tab'}
              aria-selected={filter === item.key}
              aria-controls={panelId}
              tabIndex={filter === item.key ? 0 : -1}
              onClick={() => setFilter(item.key)}
              onKeyDown={(event) => {
                if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') {
                  return;
                }

                event.preventDefault();
                const index = FILTERS.findIndex((entry) => entry.key === filter);
                const step = event.key === 'ArrowRight' ? 1 : -1;
                const next = FILTERS[(index + step + FILTERS.length) % FILTERS.length];
                setFilter(next.key);
                document.getElementById(`${panelId}-${next.key}`)?.focus();
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      <div
        className="lobby-tab-panel"
        id={panelId}
        role="tabpanel"
        aria-labelledby={`${panelId}-${filter}`}
      >
        {visible.length === 0 ? (
          <p className="open-tables-empty">
            {tables.length === 0
              ? 'Aucune table ouverte. Créez la première.'
              : 'Aucune table dans ce filtre.'}
          </p>
        ) : (
          <ul className="open-tables">
            {visible.map((table) => {
              const isJoined = joined.includes(table.id);

              return (
                <li className={isJoined ? 'open-table is-joined' : 'open-table'} key={table.id}>
                  <span className="open-table-mark" aria-hidden="true">
                    <TableIcon />
                  </span>

                  <div className="open-table-main">
                    <p className="open-table-name">{table.name}</p>
                    <p className="open-table-meta">
                      Hôte : {table.host} · {table.players}/{table.maxPlayers} joueurs
                    </p>
                  </div>

                  <span className="open-table-players" aria-hidden="true">
                    <UsersIcon />
                    {table.players}/{table.maxPlayers}
                  </span>

                  <span className={`open-table-status ${table.status}`}>
                    {table.status === 'waiting' ? 'En attente' : 'En cours'}
                  </span>

                  <button
                    type="button"
                    className={isJoined ? 'btn small open-table-join is-joined' : 'btn small open-table-join'}
                    disabled={busy || isJoined}
                    onClick={() => onJoin(table)}
                    aria-label={`Rejoindre ${table.name}`}
                  >
                    {isJoined ? (
                      <>
                        <CheckIcon />
                        Rejoint
                      </>
                    ) : (
                      'Rejoindre'
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </section>
  );
}
