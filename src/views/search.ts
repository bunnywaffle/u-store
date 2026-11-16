import { Store, isMyGo, backendName, type LocalHit, type AppSummary } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid } from "../components";
import { icon } from "../icons";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

export async function renderSearch(root: HTMLElement, navigate: Navigate, onChanged: () => void, query: string) {
  root.innerHTML = `
  <div class="view">
    <div class="view-head"><div><h1 class="view-title">Results for “${esc(query)}”</h1><p class="view-sub">Searching Flathub and your repositories…</p></div></div>
    ${skeletonGrid(9)}
  </div>`;

  const [res, local] = await Promise.all([
    Store.search(query).catch(() => null),
    Store.searchLocal(query).catch(() => ({ hits: [], errors: [`Couldn't reach the package manager`] }) as { hits: LocalHit[]; errors: string[] }),
  ]);

  const localHits: LocalHit[] = local.hits ?? [];
  const localErrors: string[] = (local.errors ?? []).filter(Boolean);

  // Native hits become real cards in the same grid — same look, same behavior.
  const localCards: AppSummary[] = localHits.map((hit) => ({
    id: `native:${hit.backend}:${hit.name}`,
    name: hit.name,
    summary: hit.summary || `${backendName(hit.backend)} package`,
    icon: hit.icon ?? "",
    categories: [],
    verified: false,
    source: hit.backend,
    sources: [hit.backend],
    nativePackage: hit.name,
    installed: hit.installed,
    featured: false,
  }));

  const header = `
    <div class="view-head">
      <div>
        <h1 class="view-title">Results for “${esc(query)}”</h1>
        <p class="view-sub">${res
          ? `${res.total.toLocaleString()} ${res.total === 1 ? "app" : "apps"} ${res.fromFlathub ? "on Flathub" : "in the built-in catalog"}`
          : "Flathub unreachable"}${localCards.length ? ` · ${localCards.length} from your repos` : ""}</p>
      </div>
    </div>`;

  const flathubBody = res && res.hits.length > 0
    ? `<div class="app-grid">${res.hits.map(appCard).join("")}</div>`
    : "";
  const localBody = localCards.length
    ? `<h2 class="section-label"><span>From your package manager (${esc(backendName(localCards[0]!.source))})</span></h2>
       <div class="app-grid">${localCards.map(appCard).join("")}</div>`
    : "";
  const localErrorsBody = localErrors.length
    ? notice(`Couldn't query your package manager${localErrors.length > 1 ? "s" : ""}: ${localErrors.join(" · ")}`)
    : "";

  const offline = res !== null && res.fromFlathub === false;
  const nothingFound = res !== null && res.hits.length === 0 && localCards.length === 0;
  const unavailable = res === null && localCards.length === 0;

  const body = unavailable
    ? emptyState("wifiOff", "Search unavailable", isMyGo
        ? "Check your network connection and try again."
        : "Run inside U Store to see live search.")
    : nothingFound
      ? emptyState("search", "No apps found", `We couldn't find anything for “${query}”. Try a shorter or different spelling.`)
      : `${flathubBody}${localBody}`;

  root.innerHTML = `<div class="view">${offline && !localBody ? notice("Flathub unavailable — showing built-in catalog matches only.", "info") : ""}${header}${localErrorsBody}${body}</div>`;

  const grid = root.querySelectorAll<HTMLElement>(".app-grid");
  grid.forEach((g) => wireCardActions(g, navigate, onChanged));
}
