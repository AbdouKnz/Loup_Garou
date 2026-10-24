import { useState } from 'react';
import type { FormEvent } from 'react';

import type { LobbyData } from '../../hooks/useLobbyData';
import Avatar from '../game/Avatar';
import FriendProfileDialog from './FriendProfileDialog';
import { SearchIcon, TrashIcon } from './icons';
import type { Friend } from './types';

type Props = {
  data: LobbyData;
  hideTitle?: boolean;
  /** Depuis l'accueil : proposer d'inviter un ami en ligne. */
  onInvite?: (friend: Friend) => void;
  /** Amis déjà invités dans cette session, pour marquer la ligne. */
  invited?: string[];
};

/** normalize prépare une comparaison sans accent ni majuscule. */
function normalize(value: string): string {
  return value
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLocaleLowerCase('fr');
}

/**
 * FriendsSidebar liste les amis et la recherche.
 *
 * La même saisie sert à filtrer la liste (au fil de la frappe) et à chercher un
 * nouveau joueur (à la validation). Les réglages de table vivent dans le
 * dialogue de création : le panneau reste consacré aux relations.
 */
export default function FriendsSidebar({ data, hideTitle = false, onInvite, invited = [] }: Props) {
  const [profile, setProfile] = useState<Friend | null>(null);

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    data.runSearch();
  };

  const needle = normalize(data.search.trim());
  const matches = (friend: Friend) => !needle || normalize(friend.username).includes(needle);
  const onlineFriends = data.onlineFriends.filter(matches);
  const offlineFriends = data.offlineFriends.filter(matches);

  return (
    <aside className="friends-sidebar" aria-label="Amis">
      {!hideTitle && (
        <div className="friends-heading">
          <h2>Amis</h2>
          <span className="friends-total">{data.friends.length}</span>
        </div>
      )}

      <p className="friends-summary">
        <span className="live-dot" aria-hidden="true" />
        {data.onlineFriends.length} en ligne
        <span className="friends-summary-sep" aria-hidden="true">
          ·
        </span>
        {data.friends.length} au total
      </p>

      <form className="friends-search" onSubmit={submitSearch} role="search">
        <input
          id="lobby-friends-search"
          type="search"
          value={data.search}
          aria-label="Rechercher un ami à ajouter"
          placeholder="Rechercher un ami…"
          autoComplete="off"
          onChange={(event) => data.setSearch(event.target.value)}
        />
        <button type="submit" className="friends-search-submit" aria-label="Rechercher">
          <SearchIcon />
          <span className="visually-hidden">Rechercher</span>
        </button>
      </form>

      <div className="friends-scroll">
        {data.searched && (
          <section className="friends-search-results" aria-live="polite">
            <p className="friends-section-title">Résultats</p>
            {data.searchResults.length > 0 ? (
              <ul className="friend-list">
                {data.searchResults.map((friend) => (
                  <li className="friend-row" key={friend.id}>
                    <Avatar index={friend.avatarIndex} />
                    <span className="friend-identity">
                      <span className="friend-name">{friend.username}</span>
                      <span className="friend-last-seen">
                        {friend.status === 'online' ? 'En ligne' : 'Hors ligne'}
                      </span>
                    </span>
                    <button
                      type="button"
                      className="friend-action"
                      onClick={() => data.addFriend(friend)}
                      aria-label={`Ajouter ${friend.username}`}
                    >
                      Ajouter
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="friends-empty">Aucun joueur trouvé.</p>
            )}
          </section>
        )}

        <section className="friends-section" aria-labelledby="friends-online-title">
          <p className="friends-section-title" id="friends-online-title">
            En ligne <span className="friends-count">{onlineFriends.length}</span>
          </p>
          {onlineFriends.length > 0 ? (
            <ul className="friend-list">
              {onlineFriends.map((friend) => (
                <li className="friend-row" key={friend.id}>
                  <Avatar index={friend.avatarIndex} />
                  <span className="friend-identity">
                    <span className="friend-name">
                      <span className="live-dot" aria-hidden="true" />
                      {friend.username}
                    </span>
                    <span className="friend-last-seen">En ligne</span>
                  </span>
                  {onInvite && (
                    <button
                      type="button"
                      className={
                        invited.includes(friend.id)
                          ? 'friend-action is-invited'
                          : 'friend-action friend-invite'
                      }
                      onClick={() => onInvite(friend)}
                      aria-label={`Inviter ${friend.username}`}
                    >
                      {invited.includes(friend.id) ? 'Invité' : 'Inviter'}
                    </button>
                  )}
                </li>
              ))}
            </ul>
          ) : (
            <p className="friends-empty">
              {needle ? 'Aucun ami ne correspond à cette recherche.' : 'Aucun ami en ligne pour le moment.'}
            </p>
          )}
        </section>

        <section className="friends-section" aria-labelledby="friends-offline-title">
          <p className="friends-section-title" id="friends-offline-title">
            Hors ligne <span className="friends-count">{offlineFriends.length}</span>
          </p>
          {offlineFriends.length > 0 ? (
            <ul className="friend-list">
              {offlineFriends.map((friend) => (
                <li className="friend-row is-offline" key={friend.id}>
                  <Avatar index={friend.avatarIndex} />
                  <span className="friend-identity">
                    <span className="friend-name">{friend.username}</span>
                    <span className="friend-last-seen">{friend.lastSeen ?? 'récemment'}</span>
                  </span>
                  <span className="friend-actions">
                    <button
                      type="button"
                      className="friend-action"
                      onClick={() => setProfile(friend)}
                      aria-label={`Voir le profil de ${friend.username}`}
                    >
                      Profil
                    </button>
                    <button
                      type="button"
                      className="friend-remove"
                      onClick={() => data.removeFriend(friend.id)}
                      aria-label={`Retirer ${friend.username} de mes amis`}
                      title={`Retirer ${friend.username}`}
                    >
                      <TrashIcon />
                    </button>
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="friends-empty">
              {needle ? 'Aucun ami hors ligne ne correspond.' : 'Aucun ami hors ligne.'}
            </p>
          )}
        </section>
      </div>

      {profile && <FriendProfileDialog friend={profile} onClose={() => setProfile(null)} />}
    </aside>
  );
}
