import { Store, isMyGo, backendName, installApp, type LocalHit } from "../api";
import { appCard, emptyState, esc, notice, skeletonGrid, toast } from "../components";
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
    Store.searchLocal(query).catch(() => [] as LocalHit[]),
  ]);

  const header = `
    <div class="view-head">
      <div>
        <h1 class="view-title">Results for “${esc(query)}”</h1>
        <p class="view-sub">${res
          ? `${res.total.toLocaleString()} ${res.total === 1 ? "app" : "apps"} ${res.fromFlathub ? "on Flathub" : "in the built-in catalog"}`
          : "Flathub unreachable"}${local.length ? ` · ${local.length} from your repos` : ""}</p>
      </div>
    </div>`;

  const flathubBody = res && res.hits.length > 0
    ? `<div class="app-grid">${res.hits.map(appCard).join("")}</div>`
    : "";
  const localBody = local.length
    ? `<h2 class="section-label"><span>From your distro's package manager (${esc(backendName(local[0]!.backend))})</span></h2>
       <div class="list">${local.map(localRow).join("")}</div>`
    : "";

  const offline = res !== null && res.fromFlathub === false;
  const nothingFound = res !== null && res.hits.length === 0 && local.length === 0;
  const unavailable = res === null && local.length === 0;

  const body = unavailable
    ? emptyState("wifiOff", "Search is unavailable", isMyGo
        ? "Check your network connection and try again."
        : "Run inside U Store to see live search.")
    : nothingFound
      ? emptyState("search", "No apps found", `We couldn't find anything for “${query}”. Try a shorter or different spelling.`)
      : `${flathubBody}${localBody}`;

  root.innerHTML = `<div class="view">${offline && !localBody ? notice("Flathub unavailable — showing built-in catalog matches only.", "info") : ""}${header}${body}</div>`;

  const grid = root.querySelector<HTMLElement>(".app-grid");
  if (grid) wireCardActions(grid, navigate, onChanged);
  wireLocalActions(root, onChanged);
}

function localRow(hit: LocalHit): string {
  const letter = esc((hit.name.trim()[0] ?? "?").toUpperCase());
  return `
  <div class="list-row">
    <span class="icon-tile" aria-hidden="true">${letter}</span>
    <div class="row-main">
      <span class="row-title">${esc(hit.name)}</span>
      <div class="row-sub">${hit.summary ? esc(hit.summary) : `${esc(backendName(hit.backend))} package`}</div>
    </div>
    <span class="source-pill">${esc(backendName(hit.backend))}</span>
    <div class="row-actions">
      ${hit.installed
        ? `<span class="source-pill" style="color:var(--ok);border-color:transparent;background:var(--ok-soft)">Installed</span>`
        : `<button class="btn sm" data-local-get="${esc(hit.name)}" data-backend="${esc(hit.backend)}">${icon("download")} Install</button>`}
    </div>
  </div>`;
}

function wireLocalActions(root: HTMLElement, onChanged: () => void) {
  root.addEventListener("click", (event) => {
    const target = event.target as HTMLElement;
    const btn = target.closest<HTMLButtonElement>("[data-local-get]");
    if (!btn || btn.disabled) return;
    const name = btn.dataset.localGet!;
    const backend = btn.dataset.backend!;
    btn.disabled = true;
    btn.innerHTML = `<span class="spinner"></span>`;
    void installApp(name, backend, () => {}).then((ok) => {
      if (ok) {
        const pill = document.createElement("span");
        pill.className = "source-pill";
        pill.textContent = "Installed";
        pill.style.cssText = "color:var(--ok);border-color:transparent;background:var(--ok-soft)";
        btn.replaceWith(pill);
        toast(`${name} installed.`, "ok");
        onChanged();
      } else {
        btn.disabled = false;
        btn.innerHTML = `${icon("download")} Install`;
        toast("Install failed.", "error");
      }
    });
  });
}
