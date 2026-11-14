import { Store, isMyGo, installBackendFor, type DiscoverResult, type AppSummary } from "../api";
import { appCard, appIcon, emptyState, esc, notice, skeletonGrid, verifiedBadge } from "../components";
import { icon } from "../icons";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

export async function renderDiscover(root: HTMLElement, navigate: Navigate, onChanged: () => void) {
  root.innerHTML = `<div class="view"><div class="view-head"><div><h1 class="view-title">Discover</h1><p class="view-sub">Handpicked for your system and trending everywhere right now.</p></div></div>${skeletonGrid(10)}</div>`;

  let data: DiscoverResult | null = null;
  try {
    data = await Store.discover();
  } catch (err) {
    root.innerHTML = `<div class="view">${notice(isMyGo ? "Couldn't reach the backend. Is the app fully up to date?" : "You're previewing this page in a browser — run inside the U Store app to install real apps.", "error")}${emptyState("wifiOff", "Nothing to show yet", "Check your network connection and try again.")}</div>`;
    return;
  }

  const featured = data.featured;
  const hero = featured[0] ?? data.trending[0];
  const rest = featured.slice(1, 7);
  const trending = data.trending.slice(0, 12);
  const offline = !data.fromFlathub;

  root.innerHTML = `
  <div class="view">
    ${offline ? notice("Flathub is unreachable — showing the built-in catalog. Connecting will restore live search and installs.", "info") : ""}
    ${hero ? `
    <section class="hero">
      <div class="hero-text">
        <span class="hero-kicker">Featured app</span>
        <h1 class="hero-title">${esc(hero.name)}</h1>
        <p class="hero-summary">${esc(hero.summary)}</p>
        <div class="hero-actions">
          <button class="btn on-hero" id="hero-get" data-get="${esc(hero.id)}" data-backend="${esc(hero.source)}">${hero.installed ? "Open" : "Get"} — free</button>
          <button class="btn hero-ghost" id="hero-learn">Learn more</button>
        </div>
      </div>
      <img class="hero-icon" src="${esc(hero.icon)}" alt="${esc(hero.name)} icon" width="128" height="128" data-fallback-id="${esc(hero.id)}" data-fallback-letter="${esc(hero.name[0] ?? "?").toUpperCase()}" data-fallback-hue="#3c3630" referrerpolicy="no-referrer" />
    </section>` : ""}

    ${trending.length ? `
    <h2 class="section-label"><span>Trending on Flathub</span></h2>
    <div class="app-grid">${trending.map(appCard).join("")}</div>` : ""}

    ${rest.length ? `
    <h2 class="section-label"><span>Editors' picks</span><button class="see-more" id="see-browse">Browse everything →</button></h2>
    <ol class="pick-list">${rest.map((app, i) => pickRow(app, i)).join("")}</ol>` : ""}

    <h2 class="section-label"><span>Categories</span></h2>
    <div class="chip-row">${data.categories.map((c) => `<button class="chip" data-cat="${esc(c.id)}">${icon(c.icon)}${esc(c.name)}</button>`).join("")}</div>
  </div>`;

  // Hero Get: when already installed the button launches the app;
  // otherwise the delegated card action runs the install.
  const heroGet = root.querySelector<HTMLButtonElement>("#hero-get");
  if (heroGet && hero?.installed) {
    heroGet.removeAttribute("data-get");
    heroGet.addEventListener("click", () => void Store.launch(hero.id, hero.source).catch(() => navigate("detail", hero.id)));
  }
  root.querySelector("#hero-learn")?.addEventListener("click", () => hero && navigate("detail", hero.id));
  root.querySelector("#see-browse")?.addEventListener("click", () => navigate("browse"));
  root.querySelectorAll<HTMLButtonElement>("[data-cat]").forEach((chip) =>
    chip.addEventListener("click", () => navigate("browse", chip.dataset.cat))
  );

  wireCardActions(root, navigate, onChanged);
}

/** A numbered editorial row for Editors' picks — deliberately a different
    structure from the card grids, like a store's printed chart. */
function pickRow(app: AppSummary, index: number): string {
  const verified = app.verified ? verifiedBadge : "";
  const installed = app.installed;
  const backend = installBackendFor(app);
  return `
  <li class="pick-row" role="button" tabindex="0" data-open="${esc(app.id)}" aria-label="${esc(app.name)}">
    <span class="pick-rank">${String(index + 1).padStart(2, "0")}</span>
    ${appIcon(app)}
    <div class="row-main">
      <span class="row-title">${esc(app.name)} ${verified}</span>
      <div class="row-sub">${esc(app.summary)}</div>
    </div>
    <div class="row-actions">
      <button class="get-btn${installed ? " installed" : ""}" data-get="${esc(app.id)}" data-backend="${esc(backend)}">${installed ? "Installed" : "Get"}</button>
    </div>
  </li>`;
}
