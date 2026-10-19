# 🐺 Loup-Garou

Jeu du **Loup-Garou** jouable dans le navigateur : **React** côté front, **Go** côté back.

> **Aucune application desktop.** C'est une application web : on lance le serveur, le navigateur
> s'ouvre, c'est tout. Rien à installer sur le poste, rien à compiler côté utilisateur.

> **État : étape M6 — la table immersive.** Avant de jouer, on crée un **compte** (pseudo et mot
> de passe) : le pseudo vient de ce compte, et la session est un cookie `HttpOnly` qui survit au
> redémarrage du serveur. Une partie se crée, se partage par son code, réunit **au moins 6
> joueurs**, puis se joue d'un cycle complet : distribution des rôles, nuit, aube, débat, vote,
> verdict, jusqu'à la victoire du village ou des loups. La partie occupe **toute la fenêtre** : la
> nuit n'est plus un bloc, elle se déroule en **veillées**, une par rôle qui se réveille — la
> **Voyante** observe un joueur et découvre son rôle, les **Loups-Garous** désignent leur proie, la
> **Sorcière** peut sauver la victime ou en empoisonner une autre. Le débat est un **tour de
> parole** — chacun parle à son tour, dans l'ordre de la table — et l'aube s'ouvre sur une **carte
> de récit lue à voix haute**, qui reste jusqu'au dernier mot. Un **voile** marque le passage du
> jour à la nuit et de la nuit au jour. Repères sonores et voix sont **synthétisés par le
> navigateur** — aucun fichier audio — et se coupent séparément. Le vote du jour se suit **en
> direct**, et la fin de partie révèle les rôles.
>
> Besoin de joueurs pour essayer seul ? L'hôte ajoute des **bots** d'un clic dans la salle
> d'attente (voir « Tester avec des bots »).
>
> Le compte peut aussi se confier à **Google** ou **Discord** : le fournisseur vérifie l'identité,
> puis le joueur choisit son pseudo — dans les trois cas, c'est le même compte et le même pseudo.
> Un même joueur garde **un seul compte** : il associe un second fournisseur depuis l'accueil, ou
> le rattachement se fait tout seul quand le fournisseur garantit un e-mail déjà connu.
> Ces connexions s'activent en déclarant l'application chez les fournisseurs (voir
> « Configuration ») ; sans identifiants, leurs boutons restent masqués.

## Prérequis

- **Go** 1.27 ou plus récent
- **Node.js** 24 ou plus récent (npm 11+)

## Démarrage

### Mode 1 — une seule commande (le plus simple)

```powershell
.\start.ps1
```

Construit l'interface (`Front/dist`), lance **un seul processus Go** qui sert l'application
et l'API sur **http://localhost:8080**, puis ouvre le navigateur. `Ctrl+C` pour arrêter.

### Mode 2 — développement avec hot reload

```powershell
.\dev.ps1
```

Lance le backend (`Back/`, port 8080) et le frontend (`Front/`, Vite) dans deux fenêtres,
**détecte l'URL réellement servie** et ouvre le navigateur automatiquement.

Si le port 5173 est déjà pris, Vite en choisit un autre (5174, 5175…) : pas besoin de
regarder le terminal, `dev.ps1` trouve la bonne URL et l'ouvre pour vous.

### Mode 3 — deux terminaux

```bash
# Terminal 1 — serveur Go (Back/) sur http://localhost:8080
cd Back
go run .

# Terminal 2 — interface React (Front/) sur http://localhost:5173
cd Front
npm install
npm run dev
```

Puis ouvrez l'URL affichée par Vite. Le proxy Vite relaie `/api` vers le serveur Go, **y compris
les montées en WebSocket** : le temps réel fonctionne à l'identique dans les deux modes.

Vérification directe du backend :

```bash
curl http://localhost:8080/healthz
```

### Essai à deux

1. Ouvrez l'application, créez un **compte** (pseudo et mot de passe — ou Google, ou Discord, si
   le serveur les propose) puis cliquez **Créer une partie** : un code à 6 caractères s'affiche
   (par exemple `ABC234`).
2. Dans un **second navigateur** (ou une fenêtre de navigation privée), créez un **autre compte**,
   cliquez **J'ai un code** et saisissez ce code.
3. Les deux joueurs se voient en direct : arrivées, départs, état « prêt », discussion.
4. Un **F5** dans un onglet le reconnecte à sa place et à son compte ; il apparaît quelques
   secondes « hors ligne » chez les autres, puis de nouveau en ligne.

> Deux onglets d'un **même** navigateur partagent la session de compte : le second prend la place
> du premier (qui est renvoyé à l'accueil). Pour jouer à deux sur une même machine, utilisez donc
> deux navigateurs différents, ou une fenêtre privée.
>
> À noter aussi : `http://localhost:5173` (mode développement) et `http://localhost:8080`
> (mode `start.ps1`) sont deux origines distinctes — pratique pour jouer deux rôles sans
> navigateur supplémentaire.

## Jouer une partie

Une fois **au moins 6 joueurs** dans la salle (12 au maximum), l'hôte appuie sur
**Démarrer la partie**. Le système fait office de narrateur : l'hôte joue comme tout le monde.

La partie s'ouvre sur la **distribution des rôles** : une **pause de 3 s** pendant laquelle chacun
découvre sa **carte de rôle** en plein écran, et où personne ne peut parler. C'est le **serveur**
qui en donne la fin — la nuit ne commence qu'à cet instant — et la carte disparaît d'elle-même. La
carte reste consultable pendant toute la partie avec **Revoir mon rôle**.

Puis le cycle :

1. **Nuit** — les rôles se réveillent **un à un**, chacun à son tour, et personne d'autre ne voit
   rien :
   - la **Voyante** observe un joueur et découvre son rôle ; elle seule le sait, et son carnet la
     suit de nuit en nuit ;
   - les **Loups-Garous** désignent ensemble leur victime. Seuls les loups voient l'antre, les
     choix de leurs compères et le second tour éventuel ; on peut confirmer ou s'abstenir ;
   - la **Sorcière** apprend qui les loups ont désigné, et décide : **soigner** la victime,
     **empoisonner** un autre joueur, ou ne rien faire. Chaque potion ne sert **qu'une fois**, et
     une seule par nuit.
   La victime ne meurt **qu'à la fin de la nuit**, une fois toutes les veillées passées : la
   sorcière a le temps de la sauver.
2. **Aube** — les morts de la nuit sont annoncées **sans leur rôle**, chacune avec sa cause : la
   morsure des loups, ou la potion de la sorcière. Si les loups ont frappé et que la victime a été
   sauvée, le village l'apprend — sans savoir qui était visé. Une **carte de récit** résume la nuit
   au centre de l'écran, et la **voix du narrateur** la lit en même temps ; elle **reste affichée
   jusqu'à la fin de la lecture**, même après l'aube. On peut la masquer, la réécouter, ou couper la
   voix d'un bouton — un réglage distinct des sons, et mémorisé.
3. **Débat** — c'est un **tour de parole** : chaque vivant parle à son tour, dans l'ordre de la
   table, avec une durée (30 s par défaut) que le bouton « J'ai fini de parler » abrège. Pendant son
   tour, l'orateur est le **seul** à pouvoir écrire ; les autres écoutent. Quand le dernier a parlé,
   le vote s'ouvre.
4. **Vote** — chacun désigne un suspect, ou s'abstient. Le scrutin est **public et en direct** :
   tout le monde voit qui a voté pour qui, les voix s'accumulent par cible et les abstentions
   sont affichées.
5. **Verdict** — l'éliminé du jour est annoncé **avec son rôle** (« c'était un Loup-Garou »), le
   scrutin restant affiché à côté.
6. Retour à la nuit, jusqu'à la **fin** : le village gagne quand tous les loups sont morts, les
   loups gagnent quand ils sont aussi nombreux que les villageois vivants. Tous les rôles sont
   alors révélés, avec le **fil de la partie** (qui est mort, quand et comment), et l'hôte peut
   relancer une partie.

En cas d'**égalité**, un **second tour** oppose les seuls joueurs à égalité (les votes repartent
de zéro, et l'écran l'annonce) ; une seconde égalité ne tue personne. Les loups connaissent la
même règle entre eux : partagés une première fois, ils revotent ; partagés encore, la nuit sera
tranquille.

Les **morts** ne votent plus et ne peuvent plus parler : ils regardent la fin de la partie.

Le vote de la nuit, lui, **reste secret** : les loups eux-mêmes ne voient pas de « scrutin »,
seulement les choix de leurs compères. Le carnet de la voyante et les potions de la sorcière ne
sortent jamais de leur vue.

La **composition** de la table est annoncée dès la salle d'attente : elle ne dépend que du nombre
de joueurs, et personne ne peut donc prétendre le contraire. Ce sont les **places** qui sont
tirées au sort, pas les rôles.

### Tester avec des bots

Pas besoin d'être six pour essayer : **l'hôte ajoute des bots depuis la salle d'attente**
(panneau « Bots », visible de lui seul). Indiquez un nombre et cliquez **Ajouter des bots**, ou
**Compléter à 6** d'un clic. Un bot se déclare prêt tout seul, joue sa veillée comme un vrai
joueur (il dévore, observe ou se sert de ses potions), vote le jour, et bavarde un peu pendant le
débat. Le bouton **Retirer tous les bots** vide la table.

C'est le moyen le plus simple de jouer une partie complète seul pour tester le gameplay.

Un script reste disponible pour les tests automatiques et les tables à plusieurs : il range des
joueurs automatiques dans une salle existante, par le même protocole qu'un vrai client.

```bash
# Laissez tourner le backend et l'interface, puis, à la racine du projet :
node tools/bots.mjs --room ABC234 --count 5
```

Remplacez `ABC234` par le code affiché dans votre salon. `--count` va de 1 à 6 ; `--url` change
l'adresse du serveur (`--url http://localhost:8080`, la valeur par défaut) ; `--delay` règle la
réactivité des bots (en millisecondes) ; `--names Albert,Berthe` impose les pseudos. Gardez le
terminal ouvert : les bots se déconnectent si le script s'arrête.

Chaque bot joue avec **son compte** : le script le crée au premier passage (les comptes restent
dans `Back/data/accounts.json`) puis s'y reconnecte. Relancer le script ne duplique donc personne,
et un compte déjà pris par un humain est signalé — changez alors de pseudo avec `--names`.

Pour des parties courtes, réduisez les durées côté serveur, par exemple :

```powershell
$env:GAME_ROLES_SECONDS = 5; $env:GAME_SPEAK_SECONDS = 15; $env:GAME_NIGHT_SECONDS = 8; $env:GAME_NIGHT_STEP_SECONDS = 5; go run .
```

## Structure du projet

```
Loup_Garou/
├── Back/                     backend Go (module Go indépendant)
│   ├── go.mod / go.sum
│   ├── main.go               point d'entrée HTTP + arrêt propre
│   ├── internal/
│   │   ├── config/           configuration (PORT, WEB_DIST, ALLOWED_ORIGINS, GAME_*, chemins)
│   │   ├── protocol/         fil JSON : trames, limites, validation, codes d'erreur
│   │   ├── accounts/         comptes : pseudo, mot de passe (PBKDF2), fournisseurs
│   │   ├── session/          sessions de compte : jetons opaques, seule l'empreinte est écrite
│   │   ├── oauth/            connexions externes : state + PKCE, fournisseurs décrits, identité
│   │   ├── filestore/        lecture/écriture JSON atomique (temporaire + renommage)
│   │   ├── game/             moteur pur (état + commande → nouvel état), sans I/O
│   │   ├── room/             salles : acteur par salle + registre, pilote le temps
│   │   ├── ws/               passerelle WebSocket (transport uniquement)
│   │   └── httpapi/          routes REST, JSON, cookies, connexions externes, service SPA + tests
│   ├── data/                 comptes et sessions (créé au premier lancement, à ne pas versionner)
│   ├── .env.example          modèle de configuration locale (copier en .env, non versionné)
│   └── bin/                  binaire produit (gitignoré)
├── Front/                    interface React (Vite + TypeScript)
│   ├── src/
│   │   ├── api/              client REST, connexion WebSocket, types du protocole
│   │   ├── state/            store Zustand (session, état de la salle, partie)
│   │   ├── components/       discussion, badge de connexion, liste des joueurs
│   │   │   │                  marque du loup, icônes de la connexion (SVG en ligne)
│   │   │   └── game/         écran de jeu éclaté : phase, table, scrutin, antre, carnet
│   │   ├── screens/          connexion (page d'atterrissage), accueil, salle d'attente, écran de jeu
│   │   └── App.tsx           aiguillage connexion / accueil / salle d'attente / partie
│   └── dist/                 build de production (gitignoré)
├── tools/
│   └── bots.mjs              bots pour remplir une table et jouer seul
├── dev.ps1                   développement : backend + front + navigateur
├── start.ps1                 mode simple : build + un seul serveur sur :8080
├── Makefile                  raccourcis (optionnel, nécessite `make`)
└── README.md
```

## Commandes

| Où | Commande | Rôle |
|---|---|---|
| `Back/` | `go run .` | Lancer le serveur |
| `Back/` | `go build ./...` | Compiler le backend |
| `Back/` | `go vet ./...` | Analyse statique Go |
| `Back/` | `gofmt -l .` | Vérifier le formatage Go |
| `Back/` | `go test ./...` | Tests Go (moteur, salles, protocole) |
| `Front/` | `npm run dev` | Serveur de développement Vite |
| `Front/` | `npm run build` | Build de production (typecheck + bundle) |
| `Front/` | `npm test` | Tests Vitest (store, protocole, écran de jeu) |
| `Front/` | `npm run lint` | ESLint |
| `Front/` | `npm run format` | Prettier |

Les commandes Go se lancent depuis `Back/` : `Front/node_modules` contient au hasard des fichiers
`.go` de bibliothèques tierces, qui ne font pas partie du projet et ne doivent pas être analysés.

Avec `make` installé : `make run`, `make check` (vet + tests + lint + build).

## API

Les appels qui touchent à une partie demandent un **compte** : le pseudo vient de l'identité, plus
d'un corps de requête. L'autorisation passe par le cookie de session `lg_session` (posé par
l'inscription ou la connexion, `HttpOnly`, prolongé à chaque visite).

| Méthode | Chemin | Rôle |
|---|---|---|
| `GET` | `/healthz`, `/api/health` | Sonde de santé |
| `POST` | `/api/auth/signup` | Créer un compte — corps `{ "username": "Alice", "password": "…" }` |
| `POST` | `/api/auth/login` | Se connecter — même corps |
| `POST` | `/api/auth/logout` | Fermer la session de compte |
| `GET` | `/api/auth/me` | Qui est connecté : `{ authenticated, username?, providers, linked, pending?, error? }` |
| `GET` | `/api/auth/{provider}/start` | Commencer une connexion chez Google ou Discord (redirection) |
| `GET` | `/api/auth/{provider}/link` | Rattacher Google ou Discord au compte connecté (redirection) |
| `GET` | `/api/auth/{provider}/callback` | Retour du fournisseur (redirection) — l'interface ne l'appelle jamais elle-même |
| `POST` | `/api/auth/complete` | Choisir le pseudo d'une connexion externe — corps `{ "username": "Alice" }` |
| `POST` | `/api/rooms` | Créer une partie (compte requis) |
| `GET` | `/api/rooms/{code}` | Résumé d'une partie (sans compte) |
| `POST` | `/api/rooms/{code}/join` | Entrer dans une partie (compte requis) |
| `GET` | `/api/rooms/{code}/ws` | Passerelle WebSocket (temps réel) |

La création et l'entrée renvoient une **session de siège** : `{ code, playerId, token, pseudo }`. Le
jeton est le secret de cette place ; il n'est transmis qu'à cet instant, et sert ensuite à ouvrir le
WebSocket. Rejouer l'entrée avec le même compte rend **la même place** : un rafraîchissement du
navigateur ne duplique personne.

Les erreurs de l'API du jeu sont toujours de la forme
`{ "code": "code_machine", "message": "phrase pour le joueur" }`, avec un statut HTTP cohérent
(400, 401, 403, 404, 409, 429, 501). Côté comptes : `username_taken` (409), `bad_credentials` (401),
`weak_password` (400), et `rate_limited` (429) après dix tentatives de connexion par minute. Côté
connexions externes : `oauth_state` (400) quand la tentative n'est plus là ou n'est pas la nôtre,
`oauth_failed` (400) quand le fournisseur refuse ou répond de travers, `oauth_linked_elsewhere`
(409) quand l'identité est déjà rattachée à un autre compte, `not_implemented` (501) quand aucun
identifiant n'est configuré.

Le retour du fournisseur fait exception : le navigateur y arrive par une **redirection**, il en
repart par une **redirection**. Un échec ne s'y affiche donc pas — il est mis de côté et annoncé
une seule fois par `GET /api/auth/me` : l'écran de connexion le lit au chargement, et un joueur déjà
connecté (échec d'un rattachement) le lit lui aussi.

### Protocole WebSocket

Le jeton ne circule **jamais dans l'URL** : la première trame reçue doit être `auth`.

Client → serveur :

| Trame | Données | Rôle |
|---|---|---|
| `auth` | `{ token }` | Ouvrir la session (première trame, obligatoire) |
| `chat.send` | `{ text }` | Envoyer un message |
| `player.ready` | `{ ready }` | Se déclarer prêt ou non |
| `game.start` | — | Lancer la partie (hôte, 6 joueurs minimum) |
| `game.again` | — | Relancer une partie après la fin (hôte) |
| `game.night.vote` | `{ target }` | Loup : désigner la victime de la nuit (`target` vide = abstention) |
| `game.night.action` | `{ action, target }` | Veillée d'un rôle : `voir` (voyante), `soigner`, `empoisonner` ou `passer` (sorcière) |
| `game.vote` | `{ target }` | Voter contre un suspect (`target` vide = abstention) |
| `bot.add` | `{ count }` | Hôte : ajouter des bots en salle d'attente (plafonné aux places) |
| `bot.remove` | `{ target }` | Hôte : retirer un bot (`target` vide = tous les bots) |
| `ping` | — | Vérifier la connexion |

Serveur → client :

| Trame | Données | Rôle |
|---|---|---|
| `auth.ok` | `{ playerId }` | Session confirmée |
| `room.state` | instantané complet vu **par ce joueur** | Joueurs, phase, historique, partie |
| `chat.message` | `{ id, pseudo, text, at, system? }` | Nouveau message |
| `game.event` | `{ kind, text, phase, day, round, target?, cause?, role?, winners? }` | Fait de jeu à narrer (bandeau, rôle révélé) |
| `error` | `{ code, message }` | Erreur applicative |
| `pong` | — | Réponse au `ping` |

La connexion est maintenue par des `ping`/`pong` et refermée si le client ne s'authentifie pas
dans les 10 secondes.

Le champ `game` de `room.state` est la vue **personnalisée** de la partie : le rôle, les loups et
leurs votes ne sont présents que pour le destinataire autorisé. Pendant la nuit, un non-loup ne
voit ni le tour de vote en cours ni les hésitations des loups.

La partie s'ouvre sur la phase `roles` — la distribution. `game.yourRole` y est déjà renseigné pour
chacun, `yourAction` est vide, le chat est fermé et aucun vote n'est accepté : il n'y a qu'à lire.
C'est le moteur qui la clôt à son échéance (3 s par défaut), et la salle ne peut pas l'écourter :
une vague de bots ne referme pas la distribution.

La nuit, `game.yourAction` dit à chaque joueur ce que la veillée en cours attend de lui
(`devorer`, `voir`, `potions`) : absent, il n'a rien à y faire. Trois champs ne sortent jamais de
la vue de leur porteur : `yourVision` (le carnet de la voyante, nuit après nuit), `yourPotions`
(ce qu'il reste à la sorcière) et `wolfTarget` (la proie des loups, révélée à la sorcière pendant
sa veillée seulement). `game.deaths` liste les morts de la nuit avec leur cause (`loups`,
`poison`), annoncées à l'aube — jamais avec leur rôle.

Pendant le vote du village et le verdict qui suit, `game.ballot` expose le **scrutin public**
(`{ votant: cible }`, une cible vide valant abstention). Ce champ est absent la nuit : le moteur
ne le remplit que pendant le vote du jour.

## Décisions techniques

- **Le serveur est la seule autorité.** Le client n'envoie que des intentions ; le serveur renvoie
  à chaque joueur une **vue personnalisée** de l'état (`SnapshotFor(playerID)`), et nulle part
  ailleurs. Les informations privées (rôle, loups, votes de la nuit) ne passent que par ce chemin :
  le rôle d'un joueur ne sort jamais du filtre `game.View`.
- **Un acteur par salle** : une goroutine détient l'état et ne le modifie qu'en réponse à des
  commandes reçues par canal. Aucun verrou, ordre des événements garanti, et les tests pilotent
  une salle sans réseau.
- **Diffusion par instantané complet**, pas par différences : après une reconnexion, l'état est
  correct par construction.
- **Deux couches d'identité** : le **compte** (cookie `lg_session`, REST) dit qui parle ; le
  **jeton de siège** (`localStorage`, première trame `auth`) dit quelle place. La salle ignore les
  comptes : un même compte qui entre deux fois dans la même partie retrouve la même place, parce
  que le serveur cherche la place par identifiant de compte avant d'en créer une.
- **Reprise après F5** : `{ code, playerId, token }` est conservé dans `localStorage` ; le client
  se reconnecte seul, la reconnexion WebSocket est automatique (délai doublant, de 0,5 s à 8 s).
- **Une seule session par joueur** : rouvrir la partie dans un second onglet *reprend la place*.
  La première connexion reçoit l'explication puis est **refermée** — un onglet abandonné ne peut
  donc pas continuer à jouer dans le dos du joueur qui a repris la place. La session mémorisée
  n'est pas effacée pour autant : les onglets d'un même navigateur partagent le stockage, et
  l'effacer déconnecterait celui qui vient de reprendre la main.
- **Transport séparé du jeu** : le paquet `ws` ne connaît que le transport, le paquet `room` ne
  connaît ni HTTP ni WebSocket, le paquet `protocol` décrit le fil et valide les entrées.
- **Moteur de jeu pur** (`Back/internal/game`) : `état + commande → nouvel état`, sans I/O ni
  horloge, testable par tables et par simulation. La salle pilote le temps (`armTimer`/`stopTimer`)
  et transmet les commandes ; le moteur ne connaît ni HTTP ni WebSocket.
- **Aucune fuite la nuit** : pendant la nuit, les hésitations des loups (second tour, votes en
  cours) ne sont visibles que des loups ; le village ne reçoit qu'un tour neutre. Ce que la nuit
  produit de secret — le carnet de la voyante (`yourVision`), les potions restantes de la sorcière
  (`yourPotions`) et la proie des loups (`wolfTarget`) — n'est renseigné que dans la vue de son
  porteur, et seulement le temps de sa veillée. Le **scrutin du jour** est public — c'est une
  règle, pas une fuite — mais il n'est rempli que pendant le vote du village : la nuit, `ballot`
  est absent pour tout le monde. Un test de l'invariant vérifie, sur des centaines de parties
  simulées, qu'aucune vue ne révèle un rôle au mauvais destinataire.
- **Interface de jeu décomposée** (`Front/src/components/game/`) : chaque morceau de l'écran de
  jeu a son fichier (phase, table, scrutin, antre, bilan). La phase en cours est portée par
  `data-phase`, et les couleurs viennent de variables CSS : l'ambiance change sans que le
  composant ait à savoir *pourquoi*. Le client ne déduit jamais un rôle : il n'affiche que ce que
  le serveur lui envoie.
- **Table ronde et carte de rôle** : les joueurs sont assis autour d'une ellipse (positions
  calculées en pourcentage du plateau, repli en liste à deux colonnes sous 760 px). Chaque siège
  est un bouton quand — et seulement quand — le serveur a dit que ce joueur pouvait agir : le
  libellé reste « Voter », « Dévorer », « Observer » ou « Empoisonner » suivi du pseudo. La nuit,
  seuls les sièges éveillés s'éclairent : les compères d'un loup, et soi-même pendant sa veillée.
  La carte de rôle, elle, est pilotée par la seule phase `roles` : aucun compteur du navigateur ne
  décide du départ de la partie.
- **Repères sonores sans fichier audio** : chaque moment du jeu a son petit dessin de tons,
  fabriqué à la volée par **Web Audio** (oscillateurs et enveloppes courtes) : aucun binaire à
  livrer, aucune dépendance. Le son se coupe d'un bouton, mémorisé, et reste silencieux tant que
  le navigateur n'a pas reçu de geste — ou s'il n'expose pas l'API.
- **Narration vocale sans dépendance** : le récit de l'aube est lu par la **synthèse vocale du
  navigateur** (Web Speech, `fr-FR`), pas par un fichier audio — aucun binaire à livrer. La voix a
  son propre réglage, mémorisé dans `localStorage`, **séparé du son** : couper les repères sonores
  laisse le récit, et inversement. Elle ne démarre qu'au passage de la nuit à l'aube — pas quand on
  rejoint une aube en cours — et un navigateur sans synthèse reste simplement silencieux. La carte
  suit la voix : elle reste à l'écran **jusqu'à la fin de la lecture**, avec un plafond de 20 s si
  le navigateur n'achève jamais la phrase.
- **Transitions sans bibliothèque** : un voile plein écran (un dégradé et deux `@keyframes`)
  accompagne la tombée de la nuit et le lever du jour. Il ne capte aucun clic, reste **sous la
  carte de narration**, ne se déclenche que sur une vraie bascule de phase — pas entre deux
  veillées — et disparaît entièrement sous `prefers-reduced-motion`.
- **Page d'atterrissage** : l'écran de connexion (`Front/src/screens/Auth.tsx`) occupe toute la
  fenêtre — la marque du loup (`components/WolfIcon.tsx`, aussi favicon `public/wolf.svg`) en haut à
  gauche, la carte d'authentification au centre. C'est du **CSS maison** : dégradés, carte de verre
  et faisceaux qui font le tour de la bordure (`@keyframes`), sans Tailwind ni librairie d'animation.
  Les pictogrammes des champs et des fournisseurs sont des **SVG en ligne** (`components/icons.tsx`),
  et les libellés des champs restent associés à leurs `<input>` (masqués à l'œil, lisibles par les
  lecteurs d'écran). Une connexion réussie fait passer l'aiguillage d'`App` à l'accueil — créer ou
  rejoindre une partie.
- **Librairie WebSocket** : [`github.com/coder/websocket`](https://github.com/coder/websocket),
  montée en WebSocket vérifiée par origine, taille de trame bornée (8 Kio).
- **Limites** : 12 joueurs, pseudo de 2 à 20 caractères, mot de passe de 8 à 200 caractères,
  messages de 500 caractères, 5 messages par tranche de 5 secondes, 100 messages conservés par
  salle, joueur hors ligne retiré après 5 minutes, salle vide fermée après 30 minutes.
- **Authentification** : un **compte obligatoire** pour entrer dans une salle. Deux couches
  distinctes : le **cookie de session** (`lg_session`, `HttpOnly`, `SameSite=Lax`, `Secure` en
  HTTPS) autorise les appels REST ; le **jeton de siège** remis à la création ou à l'entrée d'une
  partie ouvre le WebSocket. La salle ne connaît que des places, elle ignore les comptes.
- **Comptes** : mot de passe haché en **PBKDF2-SHA256** (600 000 itérations, sel de 16 octets,
  comparaison à temps constant) ; le pseudo est unique sans tenir compte de la casse. Sessions de
  compte à **expiration glissante** (7 jours), dont **seule l'empreinte SHA-256** est écrite : le
  fichier volé ne permet pas de se faire passer pour quelqu'un. Comptes et sessions **survivent au
  redémarrage** (`Back/data/`) ; les parties, non — elles vivent en mémoire.
- **Connexion externe** : un seul moteur (`Back/internal/oauth`) mène les deux fournisseurs, qui ne
  sont que des **descriptions** — points d'entrée, portée, émetteur, et la façon de lire leur
  réponse. Au départ, le serveur tire une tentative, un `state` et un vérificateur **PKCE** ; le
  navigateur ne reçoit que le défi (`S256`), jamais le vérificateur. Au retour, le `state` doit
  être celui de la tentative — c'est ce qui empêche un retour fabriqué — puis le code est échangé
  en `form-urlencoded` (Discord l'exige) et l'identité est **demandée au fournisseur** plutôt que
  lue dans un jeton : aucune signature n'est vérifiée localement, donc aucune clé publique à
  gérer. L'émetteur annoncé, quand le fournisseur le donne (RFC 9207, Discord le fait), doit être
  le sien. L'identité vérifiée ne nomme personne : **le pseudo est toujours choisi sur le site**,
  et pré-rempli avec ce que le fournisseur propose (prénom pour Google, nom affiché pour Discord).
  Un compte externe n'a **pas de mot de passe** : seul le fournisseur peut le rouvrir. Un même
  joueur garde **un seul compte** : connecté, il **associe** un second fournisseur depuis l'accueil
  (`GET /api/auth/{provider}/link`) ; et si une identité inconnue annonce un **e-mail vérifié** déjà
  rattaché à un compte, elle rejoint ce compte au lieu d'en ouvrir un second. Seul un e-mail que le
  fournisseur garantit est utilisé — une adresse non vérifiée ne prouve rien. L'interface lit les
  fournisseurs déjà rattachés dans `linked` (GET /api/auth/me) et ne propose que les autres.

## Configuration

Variables d'environnement (toutes optionnelles) :

Elles peuvent être posées dans le shell, ou regroupées dans un fichier **`Back/.env`** que le
serveur lit automatiquement au démarrage (avant d'appliquer la configuration). Une variable déjà
présente dans l'environnement n'est **jamais** écrasée : la ligne de commande prime sur le fichier.
Le format est simple — `CLE=valeur`, une par ligne, `#` pour un commentaire. `Back/.env.example`
sert de modèle (copiez-le en `Back/.env`) ; `.env` n'est pas versionné.

| Variable | Défaut | Rôle |
|---|---|---|
| `PORT` | `8080` | Port du serveur Go |
| `WEB_DIST` | `Front/dist` | Dossier de l'interface construite (mode `start.ps1`) |
| `ALLOWED_ORIGINS` | `localhost:*,127.0.0.1:*` | Origines supplémentaires acceptées en WebSocket |
| `PUBLIC_URL` | `http://localhost:$PORT` | Adresse publique du site : cookie `Secure` en HTTPS, adresses de retour déclarées aux fournisseurs |
| `FRONT_URL` | `PUBLIC_URL` | Adresse de l'interface : où le navigateur revient après une connexion externe. En développement avec Vite : `http://localhost:5173` |
| `ACCOUNTS_FILE` | `Back/data/accounts.json` | Fichier des comptes |
| `SESSIONS_FILE` | `Back/data/sessions.json` | Fichier des sessions ouvertes |
| `SESSION_TTL_DAYS` | `7` | Durée d'une session, prolongée à chaque visite |
| `GOOGLE_CLIENT_ID` | — | Identifiant d'application Google (bouton masqué sans lui) |
| `GOOGLE_CLIENT_SECRET` | — | Secret d'application Google |
| `DISCORD_CLIENT_ID` | — | Identifiant d'application Discord (bouton masqué sans lui) |
| `DISCORD_CLIENT_SECRET` | — | Secret d'application Discord |
| `SHOW_OAUTH_BUTTONS` | — | Développement : affiche les deux boutons Google/Discord même sans identifiants (ils mènent alors à « non disponible ») |
| `GAME_ROLES_SECONDS` | `3` | Durée de la distribution des rôles, avant la première nuit |
| `GAME_NIGHT_SECONDS` | `60` | Durée de la veillée des loups |
| `GAME_NIGHT_STEP_SECONDS` | `25` | Durée des veillées solitaires (voyante, sorcière) |
| `GAME_REVEAL_SECONDS` | `8` | Durée des révélations (aube, verdict) |
| `GAME_SPEAK_SECONDS` | `30` | Temps de parole de chaque vivant pendant le débat (le débat dure donc ce temps par joueur) |
| `GAME_VOTE_SECONDS` | `60` | Durée du vote |
| `GAME_MIN_PLAYERS` | `6` | Nombre minimal de joueurs |
| `GAME_MAX_PLAYERS` | `12` | Nombre maximal de joueurs |
| `GAME_ROLES` | `voyante,sorciere` | Rôles distribués en plus des loups et des villageois (`aucun` pour n'en distribuer aucun) |
| `GAME_BOT_DELAY_SECONDS` | `2` | Temps de réflexion des bots avant chaque vague (`0` = immédiat) |

Les variables `GAME_*_SECONDS` sont précieuses pour tester : par exemple
`GAME_SPEAK_SECONDS=20 GAME_NIGHT_SECONDS=20 go run .` donne des parties rapides.

`WEB_DIST` est relatif au dossier **courant** : si vous lancez le serveur depuis `Back/`, indiquez
un chemin absolu (`WEB_DIST=D:\Abdou\Loup_Garou\Front\dist`), sinon la page d'accueil répond
« interface web non construite ».

L'origine de la requête est toujours acceptée ; `ALLOWED_ORIGINS` sert au développement, quand
l'interface est servie par Vite sur un autre port que le serveur Go.

### Activer Google et Discord

Les connexions externes restent invisibles tant que l'application n'est pas déclarée chez les
fournisseurs. Une seule adresse de retour est à enregistrer chez chacun :

- Google : `<PUBLIC_URL>/api/auth/google/callback`
- Discord : `<PUBLIC_URL>/api/auth/discord/callback`

Côté **Google** : console Google Cloud → « API et services » → « Écran de consentement », puis
« Identifiants » → « Créer des identifiants » → « ID client OAuth » de type *application web*.
L'identifiant et le secret vont dans `GOOGLE_CLIENT_ID` et `GOOGLE_CLIENT_SECRET` — le plus simple
étant de les écrire dans **`Back/.env`**, lu au démarrage.

Côté **Discord** : [portail des développeurs](https://discord.com/developers/applications) →
votre application → « OAuth2 ». L'identifiant et le secret vont dans `DISCORD_CLIENT_ID` et
`DISCORD_CLIENT_SECRET`, là encore dans `Back/.env`.

Trois points à garder en tête :

- `PUBLIC_URL` doit être l'adresse **vue par le navigateur**, exactement celle qui est déclarée
  chez le fournisseur : `http://localhost:8080` sur une machine seule, l'adresse HTTPS du site en
  ligne. Une barre finale est sans effet.
- Les deux fournisseurs acceptent une adresse de retour en `http://localhost` : on peut donc
  essayer en local avant de mettre le site en ligne.
- Un fournisseur sans identifiants n'est pas proposé : son bouton reste masqué et ses routes
  répondent `not_implemented`. Rien à désactiver pour l'ignorer.

Pour **voir l'interface sans rien déclarer** chez les fournisseurs, lancez le serveur avec
`SHOW_OAUTH_BUTTONS=1` : les deux boutons apparaissent sur l'écran de connexion, même si cliquer
répond « non disponible ». C'est un réglage de développement, éteint par défaut :

```powershell
$env:SHOW_OAUTH_BUTTONS = '1'; .\start.ps1
```

### Développement avec Vite (`dev.ps1`)

En mode `dev.ps1`, l'interface est servie par **Vite** sur `http://localhost:5173` alors que l'API
répond sur `http://localhost:8080`. Le retour du fournisseur arrive sur `PUBLIC_URL`, c'est-à-dire
sur l'API — puis le serveur doit renvoyer le navigateur vers l'interface. Indiquez-lui où elle est :

```ini
# Back/.env
FRONT_URL=http://localhost:5173
```

Sans ce réglage, `FRONT_URL` vaut `PUBLIC_URL` (`http://localhost:8080`) : le navigateur revient sur
le port de l'API, qui n'a pas d'interface construite en développement et répond
« interface web non construite ». L'`redirect_uri` déclarée chez les fournisseurs, elle, ne change
pas : elle reste `<PUBLIC_URL>/api/auth/{provider}/callback`, donc une seule adresse à enregistrer
pour le développement **et** la production.

## Feuille de route

- [x] **M0** — squelette : `Back/` (Go) + `Front/` (React), lint, tests de base, lancement
- [x] **M1** — salles, lobby, WebSocket, chat, reprise après rafraîchissement
- [x] **M2** — moteur de jeu : cycle nuit/jour, vote, victoire, rôles de base + UI minimale jouable
- [x] **M3** — UI de jeu : ambiance par phase, décompte, avatars, scrutin public en direct,
      second tour annoncé, mode spectateur, bilan de fin de partie
- [x] **M4.1** — rôles supplémentaires : veillées de la nuit (voyante, sorcière), potions, morts
      multiples, composition annoncée en salle d'attente, distribution des rôles en pause (carte
      de rôle), table ronde
- [x] **M4.2** — comptes : inscription/connexion sur le site, session persistée, pseudo issu du
      compte, compte obligatoire pour jouer
- [x] **M4.3** — connexion avec **Google** et **Discord** (moteur OAuth commun, `state` + PKCE,
      vérification de l'émetteur, choix du pseudo dans les trois cas)
- [x] **M4.4** — un seul compte par joueur : rattachement explicite d'un second fournisseur, et
      rattachement automatique au retour d'une identité dont l'e-mail est **vérifié**
- [x] **M5.1** — repères sonores **synthétisés** avec Web Audio (phase, veillée, vote, événements),
      sans fichier audio, coupés d'un bouton mémorisé
- [x] **M6.1** — **tours de parole** : un seul orateur à la fois (30 s, ou « J'ai fini de parler »),
      et le chat lui est réservé pendant son tour
- [x] **M6.2** — **table plein écran** : la partie occupe la fenêtre, la table ronde au centre, les
      panneaux autour
- [x] **M6.3** — **récit de l'aube** : une carte résume la nuit et la **voix du narrateur** la lit,
      avec sa propre coupure, mémorisée
- [x] **M6.4** — **transitions jour/nuit** : un voile traverse l'écran quand la nuit tombe et quand
      le jour se lève
- [ ] **M7** — Docker : une image et un volume pour les comptes
