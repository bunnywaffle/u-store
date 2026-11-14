import { Store, isMyGo, type Category } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid } from "../components";
import { icon } from "../icons";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

let categories: Category[] | null = null;

export function resetBrowseCache() {
  categories = null;
}

export async function renderBrowse(root: HTMLElement, navigate: Navigate, onChanged: () => void, categoryId?: string) {
  if (!categories) {
    root.innerHTML = `<div class="view"><h1 class="view-title">Browse</h1>${skeletonGrid(8)}</div>`;
    try {
      const discover = await Store.discover();
      categories = discover.categories;
    } catch {
      categories = [
        { id: "internet", name: "Internet", icon: "globe", query: "browser" },
        { id: "communication", name: "Communication", icon: "chat", query: "messenger" },
        { id: "media", name: "Media & Video", icon: "play", query: "video player" },
        { id: "graphics", name: "Graphics & Photo", icon: "brush", query: "image editor" },
        { id: "office", name: "Office & Docs", icon: "doc", query: "office" },
        { id: "development", name: "Development", icon: "code", query: "developer" },
        { id: "games", name: "Games", icon: "gamepad", query: "game" },
        { id: "system", name: "System Tools", icon: "tool", query: "system utility" },
        { id: "security", name: "Security", icon: "shield", query: "password" },
      ];
    }
  }

  const active = categoryId ? categories.find((c) => c.id === categoryId) : undefined;

  root.innerHTML = `
  <div class="view">
    <div class="view-head">
      <div>
        <h1 class="view-title">${active ? esc(active.name) : "Browse"}</h1>
        <p class="view-sub">${active ? "Live results from Flathub, sorted by what people install" : "Every category we curate, one click away."}</p>
      </div>
      ${active ? `<button class="btn ghost sm" id="all-categories">${icon("grid")} All categories</button>` : ""}
    </div>
    <div class="chip-row">${categories.map((c) => `<button class="chip${c.id === categoryId ? " is-active" : ""}" data-cat="${esc(c.id)}">${icon(c.icon)}${esc(c.name)}</button>`).join("")}</div>
    <div id="browse-results" style="margin-top:18px">${active ? skeletonGrid(9) : `<div class="cat-grid">${categories.map(catTile).join("")}</div>`}</div>
  </div>`;

  root.querySelectorAll<HTMLButtonElement>("[data-cat]").forEach((chip) =>
    chip.addEventListener("click", () => navigate("browse", chip.dataset.cat))
  );
  root.querySelector("#all-categories")?.addEventListener("click", () => navigate("browse"));
  root.querySelectorAll<HTMLButtonElement>("[data-tile]").forEach((tile) =>
    tile.addEventListener("click", () => navigate("browse", tile.dataset.tile))
  );

  if (active) {
    const results = root.querySelector<HTMLElement>("#browse-results")!;
    try {
      const res = await Store.search(active.query);
      results.innerHTML = res.hits.length === 0
        ? emptyState("search", "Nothing found", "Flathub has no apps for this category right now.")
        : `<div class="app-grid">${res.hits.map(appCard).join("")}</div>`;
      if (!res.fromFlathub) results.insertAdjacentHTML("afterbegin", notice("Offline — showing built-in catalog matches only.", "info"));
      wireCardActions(results, navigate, onChanged);
    } catch {
      results.innerHTML = isMyGo
        ? emptyState("wifiOff", "Couldn't load this category", "Check your network connection and try again.")
        : emptyState("info", "Browser preview", "Category browsing needs the app backend. Run inside U Store.");
    }
  }
}

function catTile(c: Category): string {
  return `<button class="cat-tile" data-tile="${esc(c.id)}"><span class="cat-icon">${icon(c.icon)}</span><span class="cat-name">${esc(c.name)}</span></button>`;
}
