// bots.mjs — remplit une partie avec des joueurs automatiques.
//
// La partie demande au moins 6 joueurs : cet outil en fait entrer quelques-uns
// dans une salle existante pour pouvoir jouer seul, en attendant des humains.
//
//   node tools/bots.mjs --room ABC234 --count 5
//   node tools/bots.mjs --room ABC234 --count 5 --url http://localhost:8080
//
// Les bots ne sont pas l'hôte : c'est vous qui lancez la partie depuis le
// navigateur. Ils se déclarent prêts, puis jouent chaque veillée de la nuit à
// leur tour — les loups dévorent, la voyante observe, la sorcière se sert de
// ses potions —, votent le jour, et bavardent un peu pendant le débat.
//
// Chaque bot a son compte (pseudo et mot de passe fixés ici) : le script le
// crée au premier passage, puis s'y reconnecte. Relancer le script ne duplique
// donc personne, et les comptes créés restent dans data/accounts.json.
//
// Le débat prend trente secondes par personne par défaut : pour une partie
// rapide, lancez le serveur avec des durées courtes, par exemple
//   $env:GAME_SPEAK_SECONDS = 15; $env:GAME_NIGHT_SECONDS = 8; $env:GAME_NIGHT_STEP_SECONDS = 5; go run .

const args = parseArgs(process.argv.slice(2));

const base = (args.url ?? "http://localhost:8080").replace(/\/+$/, "");
const code = args.room;
const count = Number.parseInt(args.count ?? "5", 10);
const delay = Number.parseInt(args.delay ?? "1200", 10);
const names = (args.names ?? "")
  .split(",")
  .map((name) => name.trim())
  .filter(Boolean);

if (!code || !/^[A-Z2-9]{6}$/.test(code)) {
  console.error(
    "Usage : node tools/bots.mjs --room CODE [--count 5] [--url http://localhost:8080]",
  );
  process.exit(2);
}

if (!Number.isInteger(count) || count < 1 || count > 6) {
  console.error("--count doit être un entier entre 1 et 6.");
  process.exit(2);
}

const PSEUDOS = [
  "Anatole",
  "Berthe",
  "Casimir",
  "Denise",
  "Eustache",
  "Fanny",
  "Gaspard",
];
// Les bots ont un compte, comme les humains : c'est lui qui porte leur pseudo.
// Le mot de passe est le même pour tous et n'a aucune valeur : ces comptes ne
// servent qu'à cette machine de test.
const PASSWORD = "loup-garou-bots";
const CHATTER = [
  "Je n’ai rien vu de la nuit, moi.",
  "Il y a quelque chose de louche chez quelqu’un ici…",
  "Moi je dis qu’il faut se méfier des silencieux.",
  "Je voterai pour le premier qui se tait trop longtemps.",
  "Le village doit rester uni.",
  "Quelqu’un a une idée, ou on vote au hasard ?",
];

const sockets = new Set();

// Node s'arrête dès qu'il n'a plus rien à faire : sans minuteur actif, le
// processus mourrait juste après avoir placé les bots, et la table les verrait
// tous « hors ligne ». Le minuteur se retire quand la dernière place est partie.
let placed = false;

const keepAlive = setInterval(() => {
  if (placed && sockets.size === 0) {
    clearInterval(keepAlive);
    console.log("Tous les bots sont partis.");
  }
}, 1000);

/** parseArgs lit des options --clé valeur, sans dépendance. */
function parseArgs(argv) {
  const out = {};

  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) {
      continue;
    }

    const key = token.slice(2);
    const value =
      argv[i + 1] && !argv[i + 1].startsWith("--") ? argv[i + 1] : "true";
    out[key] = value;
    if (value !== "true") {
      i += 1;
    }
  }

  return out;
}

/** log écrit une ligne préfixée par le nom du bot. */
function log(who, text) {
  const stamp = new Date().toLocaleTimeString("fr-FR");
  console.log(`${stamp}  ${who.padEnd(9)} ${text}`);
}

/** post envoie une requête JSON, avec le cookie de session s'il y en a un. */
async function post(path, body, cookie) {
  return fetch(`${base}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(cookie ? { Cookie: cookie } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

/** cookieOf extrait le cookie de session d'une réponse. */
function cookieOf(response) {
  const header = response.headers
    .getSetCookie()
    .find((value) => value.startsWith("lg_session="));

  return header ? header.split(";")[0] : "";
}

/**
 * ensureAccount crée le compte du bot, ou s'y reconnecte s'il existe déjà.
 *
 * Une seconde exécution du script retrouve donc ses comptes, et un bot qui
 * relance le script reprend la même place dans la partie.
 */
async function ensureAccount(pseudo) {
  const created = await post("/api/auth/signup", {
    username: pseudo,
    password: PASSWORD,
  });
  if (created.ok) {
    return cookieOf(created);
  }

  const login = await post("/api/auth/login", {
    username: pseudo,
    password: PASSWORD,
  });
  if (!login.ok) {
    throw new Error(
      `le pseudo « ${pseudo} » est déjà pris par un autre compte (essayez --names)`,
    );
  }

  return cookieOf(login);
}

/** join entre dans la partie au nom du compte et renvoie la session de siège. */
async function join(cookie) {
  const response = await post(`/api/rooms/${code}/join`, undefined, cookie);

  const json = await response.json().catch(() => null);

  if (!response.ok) {
    throw new Error(json?.message ?? `entrée refusée (${response.status})`);
  }

  return json;
}

/** pick choisit un élément au hasard dans une liste. */
function pick(list) {
  return list[Math.floor(Math.random() * list.length)];
}

/** playBot connecte un bot et le fait jouer jusqu'à la fin de la partie. */
async function playBot(index) {
  const pseudo =
    names[index] ?? PSEUDOS[index % PSEUDOS.length] ?? `Bot ${index + 1}`;
  const session = await join(await ensureAccount(pseudo));

  const socket = new WebSocket(
    `${base.replace(/^http/, "ws")}/api/rooms/${code}/ws`,
  );
  sockets.add(socket);

  // Un bot n'agit qu'une fois par tour : cette clé garde la trace de son
  // dernier geste, pour ne pas voter en boucle à chaque instantané.
  const acted = new Set();

  const send = (message) => socket.send(JSON.stringify(message));
  const later = (fn) => setTimeout(fn, delay + Math.random() * 600);

  socket.addEventListener("open", () => {
    send({ type: "auth", data: { token: session.token } });
  });

  socket.addEventListener("close", () => sockets.delete(socket));

  socket.addEventListener("error", () => log(pseudo, "connexion perdue"));

  socket.addEventListener("message", (event) => {
    let message;
    try {
      message = JSON.parse(event.data);
    } catch {
      return;
    }

    if (message.type === "error") {
      // Un refus n'est pas fatal : le bot attend l'instantané suivant.
      if (message.data?.code !== "rate_limited") {
        log(pseudo, `refus : ${message.data?.message ?? message.data?.code}`);
      }
      return;
    }

    if (message.type === "game.event" && message.data?.kind === "fin") {
      const won = message.data.winners === "loups" ? "les loups" : "le village";
      log(pseudo, `la partie est finie — victoire de ${won}`);
      return;
    }

    if (message.type !== "room.state") {
      return;
    }

    const state = message.data;
    const me = state.players.find((player) => player.id === state.you);
    const game = state.game;

    // En salle d'attente, il suffit de se déclarer prêt.
    if (!game) {
      if (me && !me.ready && !acted.has("ready")) {
        acted.add("ready");
        send({ type: "player.ready", data: { ready: true } });
        log(pseudo, "prêt");
      }
      return;
    }

    const alive = state.players.filter((player) => !player.dead);
    const others = alive.filter((player) => player.id !== state.you);
    const wolves = game.wolves ?? [];
    const pseudoOf = (id) =>
      state.players.find((player) => player.id === id)?.pseudo ?? "?";

    // Une clé par veillée : la nuit change d'acteur sans changer de phase, et
    // le serveur dit à chaque instant ce qu'il attend de ce joueur.
    const step = `${game.phase}-${game.day}-${game.round}-${game.yourAction ?? ""}`;
    const round = `${game.phase}-${game.day}-${game.round}`;

    if (game.phase === "nuit" && game.yourAction === "devorer") {
      // Les loups ne se dévorent pas entre eux.
      const preys = others.filter((player) => !wolves.includes(player.id));
      const target = preys.length > 0 ? pick(preys) : null;

      if (target && !acted.has(step)) {
        acted.add(step);
        later(() => {
          send({ type: "game.night.vote", data: { target: target.id } });
          log(pseudo, `loup : dévore ${target.pseudo}`);
        });
      }
      return;
    }

    if (
      game.phase === "nuit" &&
      game.yourAction === "voir" &&
      !acted.has(step)
    ) {
      // La voyante ne s'observe pas elle-même : others l'exclut déjà.
      const target = pick(others);

      if (target) {
        acted.add(step);
        later(() => {
          send({
            type: "game.night.action",
            data: { action: "voir", target: target.id },
          });
          log(pseudo, `voyante : observe ${target.pseudo}`);
        });
      }
      return;
    }

    if (
      game.phase === "nuit" &&
      game.yourAction === "potions" &&
      !acted.has(step)
    ) {
      acted.add(step);

      const potions = game.yourPotions ?? {};
      const prey = game.wolfTarget;
      // Empoisonner la proie des loups n'ajouterait rien : on vise un autre.
      const victim = others.find((player) => player.id !== prey);

      let action = { action: "passer", target: "" };
      let note = "sorcière : ne touche à rien";

      if (potions.heal && prey) {
        action = { action: "soigner", target: prey };
        note = `sorcière : sauve ${pseudoOf(prey)}`;
      } else if (potions.poison && victim) {
        action = { action: "empoisonner", target: victim.id };
        note = `sorcière : empoisonne ${victim.pseudo}`;
      }

      later(() => {
        send({ type: "game.night.action", data: action });
        log(pseudo, note);
      });
      return;
    }

    if (game.phase === "debat" && !me?.dead && !acted.has(`debat-${round}`)) {
      // Un seul bon mot par débat, et seulement quand la place est libre.
      const last = state.messages[state.messages.length - 1];
      const quiet = !last || Date.now() - new Date(last.at).getTime() > 3000;
      if (!quiet) {
        return;
      }

      acted.add(`debat-${round}`);
      later(() => {
        send({ type: "chat.send", data: { text: pick(CHATTER) } });
        log(pseudo, "prend la parole");
      });
      return;
    }

    if (
      game.phase === "vote" &&
      !me?.dead &&
      !me?.voted &&
      !acted.has(`vote-${round}`)
    ) {
      // Second tour : seuls les candidats sont votables.
      const candidates = game.candidates?.length ? game.candidates : null;
      const targets = others.filter(
        (player) => !candidates || candidates.includes(player.id),
      );

      const target = targets.length > 0 ? pick(targets) : null;

      acted.add(`vote-${round}`);
      later(() => {
        if (target) {
          send({ type: "game.vote", data: { target: target.id } });
          log(pseudo, `vote contre ${target.pseudo}`);
        } else {
          send({ type: "game.vote", data: { target: "" } });
          log(pseudo, "s’abstient");
        }
      });
    }
  });
}

async function main() {
  console.log(`→ ${count} bot(s) vers ${base}, partie ${code}`);

  for (let i = 0; i < count; i += 1) {
    try {
      await playBot(i);
    } catch (error) {
      console.error(`  bot ${i + 1} : ${error.message}`);
      process.exit(1);
    }
  }

  console.log("Bots en place. Lancez la partie depuis le navigateur (hôte).");
  console.log("Ctrl+C pour les retirer.");
  placed = true;
}

process.on("SIGINT", () => {
  clearInterval(keepAlive);
  for (const socket of sockets) {
    socket.close(1000, "fin");
  }
  console.log("\nBots retirés.");
  process.exit(0);
});

main().catch((error) => {
  console.error("Erreur fatale :", error);
  process.exit(1);
});
