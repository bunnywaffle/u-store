/* API layer: thin, resilient wrappers around the bound Go service.
   Everything the UI calls lives here so views stay dumb and testable. */

import { Channel } from "mygo-runtime";
import { Store, type AppSummary, type InstallEvent } from "./mygo";

export { Store };
export type {
  AppDetails, AppSummary, BackendInfo, Category, DiscoverResult,
  InstalledApp, InstalledSource, InstallEvent, LocalPackage, SearchResult, SystemInfo, UpdateInfo,
} from "./mygo";

export const isMyGo = typeof window !== "undefined" && "mygo" in window;

/** localStorage key for the "prefer native packages over Flatpak" setting. */
const PREF_KEY = "us.preferNative";

export function preferNative(): boolean {
  try {
    return localStorage.getItem(PREF_KEY) === "1";
  } catch {
    return false;
  }
}

export function setPreferNative(value: boolean) {
  try {
    localStorage.setItem(PREF_KEY, value ? "1" : "0");
  } catch {
    /* private mode — preference just won't persist */
  }
}

/** Pick the backend a Get button should use for this app on this machine. */
export function installBackendFor(app: Pick<AppSummary, "source" | "sources">): string {
  const sources = app.sources ?? ["flatpak"];
  if (preferNative()) {
    const native = sources.find((s) => s !== "flatpak" && s !== "snap");
    if (native) return native;
  }
  return sources.includes(app.source) ? app.source : sources[0]!;
}

/** True when the machine has a native backend this app can use. */
export function nativeOptionFor(app: Pick<AppSummary, "sources">): string | undefined {
  return (app.sources ?? []).find((s) => s !== "flatpak" && s !== "snap");
}

export function snapOptionFor(app: Pick<AppSummary, "sources">): boolean {
  return (app.sources ?? []).includes("snap");
}

/** Human label for a backend id. */
export function backendName(id: string): string {
  const names: Record<string, string> = {
    flatpak: "Flatpak", snap: "Snap", apt: "APT", dnf: "DNF",
    pacman: "pacman", zypper: "Zypper", apk: "apk",
  };
  return names[id] ?? id;
}

export function formatInstalls(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}

export function formatDate(unix: number): string {
  if (!unix) return "Unknown";
  const d = new Date(unix * 1000);
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

/** Tiny allowlist-based HTML sanitizer for flathub descriptions. */
export function sanitizeHTML(html: string): string {
  const allowed = new Set(["P", "UL", "OL", "LI", "B", "STRONG", "I", "EM", "A", "BR", "CODE", "PRE", "SPAN"]);
  const doc = new DOMParser().parseFromString(html, "text/html");
  const walk = (node: Element) => {
    for (const child of [...node.children]) {
      walk(child);
      if (!allowed.has(child.tagName)) {
        const parent = child.parentNode!;
        while (child.firstChild) parent.insertBefore(child.firstChild, child);
        parent.removeChild(child);
        continue;
      }
      for (const attr of [...child.attributes]) {
        const name = attr.name.toLowerCase();
        const ok = child.tagName === "A" ? name === "href" && /^https?:\/\//i.test(attr.value) : false;
        if (!ok) child.removeAttribute(attr.name);
      }
      if (child.tagName === "A") {
        child.setAttribute("target", "_blank");
        child.setAttribute("rel", "noreferrer noopener");
      }
    }
  };
  walk(doc.body);
  return doc.body.innerHTML;
}

export type InstallFlow = {
  /** Abort the underlying command by cancelling the context. */
  cancel: () => void;
};

export type ProgressListener = (ev: InstallEvent) => void;

/** Run Store.install and stream events. Resolves true on success. */
export async function installApp(id: string, backend: string, onProgress: ProgressListener): Promise<boolean> {
  const ch = new Channel<InstallEvent>();
  ch.onmessage = onProgress;
  try {
    await Store.install(id, backend, ch);
    return true;
  } catch {
    return false;
  }
}

/** Run Store.remove and stream events. Resolves true on success. */
export async function removeApp(id: string, backend: string, onProgress: ProgressListener): Promise<boolean> {
  const ch = new Channel<InstallEvent>();
  ch.onmessage = onProgress;
  try {
    await Store.remove(id, backend, ch);
    return true;
  } catch {
    return false;
  }
}

/** Run Store.updateAll and stream events. Resolves true on success. */
export async function updateAll(onProgress: ProgressListener): Promise<boolean> {
  const ch = new Channel<InstallEvent>();
  ch.onmessage = onProgress;
  try {
    await Store.updateAll(ch);
    return true;
  } catch {
    return false;
  }
}

/** Install a local package file, streaming events. Resolves true on success. */
export async function installLocalPackage(path: string, onProgress: ProgressListener): Promise<boolean> {
  const ch = new Channel<InstallEvent>();
  ch.onmessage = onProgress;
  try {
    await Store.installLocal(path, ch);
    return true;
  } catch {
    return false;
  }
}

/** Human-readable file size. */
export function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n >= 10 || i === 0 ? Math.round(n) : n.toFixed(1)} ${units[i]}`;
}
