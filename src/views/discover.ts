import { Store, isMyGo, formatInstalls, type DiscoverResult } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid } from "../components";
import { icon } from "../icons";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

export async function renderDiscover(root: HTMLElement, navigate: Navigate, onChanged: () => void) {
  root.innerHTML = `<div class="view"><div class="view-head"><div><h1 class="view-title">Discover</h1><p class="view-sub">What Flathub says people install right now.</p></div></div>${skeletonGrid(10)}</div>`;

  let data: DiscoverResult | null = null;
  try {
    data = await Store.discover();
  } catch (err) {
    root.innerHTML = `<div class="view">${notice(isMyGo ? "Couldn't reach the backend. Is the app fully up to date?" : "You're previewing this page in a browser — run inside the U Store app to install real apps.", "error")}${emptyState("wifiOff", "Nothing to show yet", "Check your network connection and try again.")}</div>`;
    return;
  }

  const live = data.fromFlathub;
  const trending = data.trending;
  const hero = trending[0];
  // Everything is one live list: the hero is #1 right now, the grid is the next twelve.
  const rest = trending.slice(1, 13);
  const more = trending.slice(13);

  const heroInstalls = hero?.installsLastMonth ?? 0;

  root.innerHTML = `
  <div class="view">
    ${!live ? notice("Flathub is unreachable — showing the built-in catalog. Connecting restores live trending.", "info") : ""}
    ${hero ? `
    <section class="hero">
      <div class="hero-text">
        <span class="hero-kicker">Most installed on Flathub right now${heroInstalls > 0 ? ` · ${formatInstalls(heroInstalls)} installs last month` : ""}</span>
        <h1 class="hero-title">${esc(hero.name)}</h1>
        <p class="hero-summary">${esc(hero.summary)}</p>
        <div class="hero-actions">
          <button class="btn on-hero" id="hero-get" data-get="${esc(hero.id)}" data-backend="${esc(hero.source)}">${hero.installed ? "Open" : "Get"} — free</button>
          <button class="btn hero-ghost" id="hero-learn">Learn more</button>
        </div>
      </div>
      <img class="hero-icon" src="${esc(hero.icon)}" alt="${esc(hero.name)} icon" width="132" height="132" data-fallback-id="${esc(hero.id)}" data-fallback-letter="${esc(hero.name[0] ?? "?").toUpperCase()}" data-fallback-hue="#3f3f3f" referrerpolicy="no-referrer" />
    </section>` : ""}

    ${rest.length ? `
    <h2 class="section-label"><span>${live ? "Most installed right now" : "Picks from the catalog"}</span><button class="see-more" id="see-browse">Browse everything →</button></h2>
    <div class="app-grid">${rest.map(appCard).join("")}</div>` : ""}

    ${more.length ? `
    <h2 class="section-label"><span>Also popular</span></h2>
    <div class="app-grid">${more.slice(0, 12).map(appCard).join("")}</div>` : ""}

    <h2 class="section-label"><span>Categories</span></h2>
    <div class="chip-row">${data.categories.map((c) => `<button class="chip" data-cat="${esc(c.id)}">${icon(c.icon)}${esc(c.name)}</button>`).join("")}</div>
  </div>`;

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
