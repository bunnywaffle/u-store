import { Store, isMyGo, type AppSummary, type Category } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid } from "../components";
import { icon } from "../icons";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

let categories: Category[] | null = null;
let page = 1;

export function resetBrowseCache() {
  categories = null;
  page = 1;
}

export async function renderBrowse(root: HTMLElement, navigate: Navigate, onChanged: () => void, categoryId?: string) {
  if (!categories) {
    root.innerHTML = `<div class="view"><h1 class="view-title">Browse</h1>${skeletonGrid(8)}</div>`;
    try {
      const discover = await Store.discover();
      categories = discover.categories;
    } catch {
      categories = [
        { id: "network",     name: "Internet & Network", icon: "globe", query: "network" },
        { id: "audiovideo",  name: "Audio & Video",      icon: "play",  query: "video" },
        { id: "graphics",    name: "Graphics & Photo",   icon: "brush", query: "photo" },
        { id: "office",      name: "Office & Documents", icon: "doc",   query: "office" },
        { id: "development", name: "Development",        icon: "code",  query: "developer" },
        { id: "game",        name: "Games",              icon: "gamepad",query: "game" },
        { id: "system",      name: "System Tools",       icon: "tool",  query: "system utility" },
        { id: "utility",     name: "Utilities",          icon: "tool",  query: "utility" },
        { id: "science",     name: "Science",            icon: "tool",  query: "science" },
        { id: "education",   name: "Education",          icon: "doc",   query: "education" },
      ];
    }
  }

  const active = categoryId ? categories.find((c) => c.id === categoryId) : undefined;
  page = 1;

  root.innerHTML = `
  <div class="view">
    <div class="view-head">
      <div>
        <h1 class="view-title">${active ? esc(active.name) : "Browse"}</h1>
        <p class="view-sub">${active ? "Everything Flathub lists in this category, most-installed first" : "Every category we curate, one click away."}</p>
      </div>
      ${active ? `<button class="btn ghost sm" id="all-categories">${icon("grid")} All categories</button>` : ""}
    </div>
    <div class="chip-row">${categories.map((c) => `<button class="chip${c.id === categoryId ? " is-active" : ""}" data-cat="${esc(c.id)}">${icon(c.icon)}${esc(c.name)}</button>`).join("")}</div>
    <div id="browse-results" style="margin-top:18px">${active ? skeletonGrid(9) : `<div class="cat-grid">${categories.map(catTile).join("")}</div>`}</div>
    ${active ? `<div style="display:flex;justify-content:center;margin-top:22px"><button class="btn ghost" id="load-more" hidden>Load more <span id="load-more-count"></span></button></div>` : ""}
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
    const loadMore = root.querySelector<HTMLButtonElement>("#load-more")!;
    const loadMoreCount = root.querySelector<HTMLElement>("#load-more-count")!;

    const appendPage = async (pageNum: number) => {
      const res = await Store.searchCategory(active.id, pageNum);
      if (pageNum === 1) {
        results.innerHTML = res.hits.length === 0
          ? emptyState("search", "Nothing here", "No apps are listed in this category right now.")
          : `<div class="app-grid">${res.hits.map(appCard).join("")}</div>`;
      } else {
        const grid = results.querySelector<HTMLElement>(".app-grid");
        if (grid) grid.insertAdjacentHTML("beforeend", res.hits.map(appCard).join(""));
      }
      const loaded = results.querySelectorAll(".app-card").length;
      const total = res.total;
      loadMoreCount.textContent = `· ${loaded.toLocaleString()} of ${total.toLocaleString()}`;
      loadMore.hidden = loaded >= total || !res.fromFlathub;
      if (pageNum === 1 && !res.fromFlathub) results.insertAdjacentHTML("afterbegin", notice("Offline — showing built-in catalog matches only.", "info"));
      if (pageNum === 1) wireCardActions(results, navigate, onChanged);
    };

    loadMore.addEventListener("click", async () => {
      loadMore.disabled = true;
      page += 1;
      await appendPage(page).catch(() => {});
      loadMore.disabled = false;
    });

    try {
      await appendPage(1);
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
