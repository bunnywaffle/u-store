import { Store, isMyGo, backendName, type LocalHit, type AppSummary } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid, matchesSourceFilter, sourceSegmented, wireSourceFilter, type SourceFilter } from "../components";
import { wireCardActions } from "../actions";

type Navigate = (route: string, param?: string) => void;

let sourceFilter: SourceFilter = "all";
let lastData: { query: string; res: Awaited<ReturnType<typeof Store.search>> | null; local: { hits: LocalHit[]; errors: string[] } } | null = null;

export async function renderSearch(root: HTMLElement, navigate: Navigate, onChanged: () => void, query: string) {
  root.innerHTML = `
  <div class="view">
    <div class="view-head"><div><h1 class="view-title">Results for “${esc(query)}”</h1><p class="view-sub">Searching Flathub and your repositories…</p></div></div>
    ${skeletonGrid(9)}
  </div>`;

  const [res, local] = await Promise.all([
    Store.search(query).catch(() => null),
    Store.searchLocal(query).catch(() => ({ hits: [], errors: ["Couldn't query your package manager"] }) as { hits: LocalHit[]; errors: string[] }),
  ]);

  lastData = { query, res, local };
  paint(root, navigate, onChanged);
}

function paint(root: HTMLElement, navigate: Navigate, onChanged: () => void) {
  if (!lastData) return;
  const { query, res, local } = lastData;

  const localHits: LocalHit[] = local.hits ?? [];
  const localErrors: string[] = (local.errors ?? []).filter(Boolean);

  const flatCards: AppSummary[] = (res?.hits ?? []).filter((hit) => sourceFilter === "all" || sourceFilter === "flatpak");
  const localCards: AppSummary[] = localHits
    .filter((hit) => matchesSourceFilter(sourceFilter, [hit.backend]))
    .map((hit) => ({
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

  const filteredTotal = flatCards.length + localCards.length;
  const sourceWord =
    sourceFilter === "all" ? "apps" :
    sourceFilter === "flatpak" ? "Flatpak apps" :
    sourceFilter === "snap" ? "Snap apps" : "native apps";

  const header = `
    <div class="view-head">
      <div>
        <h1 class="view-title">Results for “${esc(query)}”</h1>
        <p class="view-sub">${res
          ? `${res.total.toLocaleString()} ${res.total === 1 ? "app" : "apps"} ${res.fromFlathub ? "on Flathub" : "in the built-in catalog"}`
          : "Flathub unreachable"}${localHits.length ? ` · ${localHits.length} from your repos` : ""} · showing ${filteredTotal} ${esc(sourceWord)}</p>
      </div>
      ${sourceSegmented("search-source-filter", sourceFilter)}
    </div>`;

  const flathubBody = flatCards.length
    ? `<div class="app-grid">${flatCards.map(appCard).join("")}</div>`
    : "";
  const localBody = localCards.length
    ? `<h2 class="section-label"><span>From your package manager${localCards[0] ? ` (${esc(backendName(localCards[0]!.source))})` : ""}</span></h2>
       <div class="app-grid">${localCards.map(appCard).join("")}</div>`
    : "";
  const localErrorsBody = localErrors.length
    ? notice(`Couldn't query your package manager${localErrors.length > 1 ? "s" : ""}: ${localErrors.join(" · ")}`)
    : "";

  const offline = res !== null && res.fromFlathub === false;
  const nothingFound = filteredTotal === 0;
  const unavailable = res === null && filteredTotal === 0 && sourceFilter === "all";

  const body = unavailable
    ? emptyState("wifiOff", "Search unavailable", isMyGo
        ? "Check your network connection and try again."
        : "Run inside U Store to see live search.")
    : nothingFound
      ? emptyState("search", sourceFilter === "all" ? "No apps found" : "Nothing with that source", `We couldn't find ${esc(sourceWord)} for “${esc(query)}”. Try another source or a shorter spelling.`)
      : `${flathubBody}${localBody}`;

  root.innerHTML = `<div class="view">${offline && !localBody ? notice("Flathub unavailable — showing built-in catalog matches only.", "info") : ""}${header}${localErrorsBody}${body}</div>`;

  wireSourceFilter(root, "search-source-filter", (f) => {
    sourceFilter = f;
    paint(root, navigate, onChanged);
  });
  const grids = root.querySelectorAll<HTMLElement>(".app-grid");
  grids.forEach((g) => wireCardActions(g, navigate, onChanged));
}
