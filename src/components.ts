/* Shared UI components rendered as template strings. */

import { icon } from "./icons";
import type { AppSummary } from "./api";
import { backendName, installBackendFor } from "./api";

export const esc = (s: string) =>
  s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);

/** Deterministic flat color for letter tiles, picked from the app id hash. */
const TILE_COLORS = [
  "#3f3f3f", "#4d4d4d", "#5c5c5c", "#454545", "#585858",
  "#3a3a3a", "#525252", "#424242",
];

function tileColor(id: string): string {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) & 0xffffffff;
  return TILE_COLORS[Math.abs(h) % TILE_COLORS.length]!;
}

/** App icon <img> with a letter-tile fallback (handled by main.ts). */
export function appIcon(app: Pick<AppSummary, "id" | "name" | "icon">, cls = "app-icon"): string {
  const letter = esc((app.name.trim()[0] ?? "?").toUpperCase());
  const color = tileColor(app.id);
  if (!app.icon) return `<span class="icon-tile ${cls}" style="background:${color}">${letter}</span>`;
  return `<img class="${cls}" src="${esc(app.icon)}" alt="" width="96" height="96" loading="lazy" decoding="async" data-fallback-id="${esc(app.id)}" data-fallback-letter="${letter}" data-fallback-hue="${color}" referrerpolicy="no-referrer" />`;
}

export const verifiedBadge = `<span class="verified" title="Verified by Flathub">${icon("shieldCheck")}</span>`;

/** A card in the app grid. Actions are delegated by the views. */
export function appCard(app: AppSummary): string {
  const verified = app.verified ? verifiedBadge : "";
  const installed = app.installed;
  const backend = installBackendFor(app);
  return `
  <div class="app-card" role="button" tabindex="0" data-open="${esc(app.id)}" aria-label="${esc(app.name)}">
    ${appIcon(app)}
    <div class="app-head">
      <span class="app-name">${esc(app.name)} ${verified}</span>
      <p class="app-summary">${esc(app.summary)}</p>
    </div>
    <div class="card-foot">
      <span class="source-pill ${esc(backend)}">${esc(backendName(backend))}</span>
      <button class="get-btn${installed ? " installed" : ""}" data-get="${esc(app.id)}" data-backend="${esc(backend)}">
        ${installed ? "Installed" : "Get"}
      </button>
    </div>
  </div>`;
}

export function skeletonGrid(count = 9): string {
  return `<div class="app-grid">${'<div class="skeleton skeleton-card"></div>'.repeat(count)}</div>`;
}

export function emptyState(iconName: string, title: string, sub: string): string {
  return `<div class="empty"><div class="empty-icon">${icon(iconName)}</div><h3>${esc(title)}</h3><p>${esc(sub)}</p></div>`;
}

export function notice(text: string, kind: "error" | "info" = "error"): string {
  const glyph = kind === "error" ? icon("wifiOff") : icon("info");
  return `<div class="notice ${kind}">${glyph}<span>${esc(text)}</span></div>`;
}

/** Render a toast; kind styles the pill. */
export function toast(message: string, kind: "default" | "ok" | "error" = "default") {
  const root = document.getElementById("toast-root")!;
  const node = document.createElement("div");
  node.className = `toast${kind === "default" ? "" : ` ${kind}`}`;
  const glyph = kind === "error" ? icon("x") : kind === "ok" ? icon("check") : icon("info");
  node.innerHTML = `${glyph}<span>${esc(message)}</span>`;
  root.appendChild(node);
  window.setTimeout(() => {
    node.classList.add("leaving");
    window.setTimeout(() => node.remove(), 280);
  }, 4200);
}

/** Wire the icon-fallback listener once (capture phase). */
export function installIconFallback() {
  document.addEventListener(
    "error",
    (event) => {
      const target = event.target as HTMLElement;
      if (!(target instanceof HTMLImageElement)) return;
      const id = target.dataset.fallbackId;
      const letter = target.dataset.fallbackLetter;
      const color = target.dataset.fallbackHue ?? "#4d4d4d";
      if (!id || !letter) return;
      const tile = document.createElement("span");
      tile.className = `${target.className} icon-tile`;
      tile.style.background = color;
      tile.textContent = letter;
      target.replaceWith(tile);
    },
    true
  );
}

/* ============ Source filter (Flatpak / native / snap) ============ */

export type SourceFilter = "all" | "flatpak" | "native" | "snap";
export const SOURCE_FILTERS: { id: SourceFilter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "flatpak", label: "Flatpak" },
  { id: "native", label: "Native" },
  { id: "snap", label: "Snap" },
];

const NATIVE_BACKENDS = new Set(["apt", "dnf", "pacman", "zypper", "apk"]);

export function matchesSourceFilter(filter: SourceFilter, backends: readonly string[]): boolean {
  if (filter === "all") return true;
  if (filter === "flatpak") return backends.includes("flatpak");
  if (filter === "snap") return backends.includes("snap");
  return backends.some((b) => NATIVE_BACKENDS.has(b));
}

/** Segmented control for the All/Flatpak/Native/Snap source filter. */
export function sourceSegmented(id: string, current: SourceFilter): string {
  return `<div class="segmented" id="${id}" role="radiogroup" aria-label="Filter by source">${SOURCE_FILTERS.map(
    (f) => `<button role="radio" data-source="${f.id}" aria-checked="${f.id === current}" class="${f.id === current ? "is-active" : ""}">${f.label}</button>`
  ).join("")}</div>`;
}

/** Wire a segmented source filter: clicking a segment calls onChange with it. */
export function wireSourceFilter(root: HTMLElement, id: string, onChange: (f: SourceFilter) => void) {
  root.querySelector(`#${id}`)?.addEventListener("click", (event) => {
    const btn = (event.target as HTMLElement).closest<HTMLButtonElement>("[data-source]");
    if (!btn) return;
    const next = btn.dataset.source as SourceFilter;
    onChange(next);
  });
}
