import { Store, isMyGo } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid } from "../components";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

export async function renderSearch(root: HTMLElement, navigate: Navigate, onChanged: () => void, query: string) {
  root.innerHTML = `
  <div class="view">
    <div class="view-head"><div><h1 class="view-title">Results for “${esc(query)}”</h1><p class="view-sub">Searching Flathub…</p></div></div>
    ${skeletonGrid(9)}
  </div>`;

  try {
    const res = await Store.search(query);
    const header = `
      <div class="view-head">
        <div>
          <h1 class="view-title">Results for “${esc(query)}”</h1>
          <p class="view-sub">${res.total.toLocaleString()} ${res.total === 1 ? "app" : "apps"} ${res.fromFlathub ? "on Flathub" : "in the built-in catalog (offline)"}</p>
        </div>
      </div>`;
    const body = res.hits.length === 0
      ? emptyState("search", "No apps found", `We couldn't find anything for “${query}”. Try a shorter or different spelling.`)
      : `<div class="app-grid">${res.hits.map(appCard).join("")}</div>`;
    root.innerHTML = `<div class="view">${res.fromFlathub ? "" : notice("Offline — showing built-in catalog matches only.", "info")}${header}${body}</div>`;
    const grid = root.querySelector<HTMLElement>(".app-grid");
    if (grid) wireCardActions(grid, navigate, onChanged);
  } catch {
    root.innerHTML = `<div class="view">${isMyGo
      ? emptyState("wifiOff", "Search is unavailable", "Check your network connection and try again.")
      : emptyState("info", "Browser preview", "Live search needs the app backend. Run inside U Store.")}</div>`;
  }
}
