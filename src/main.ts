import "@fontsource/bricolage-grotesque/500.css";
import "@fontsource/bricolage-grotesque/600.css";
import "@fontsource/bricolage-grotesque/700.css";
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import { Store, isMyGo } from "./api";
import { icon } from "./icons";
import { installIconFallback, toast } from "./components";
import { renderDiscover } from "./views/discover";
import { renderBrowse } from "./views/browse";
import { renderSearch } from "./views/search";
import { renderDetail } from "./views/detail";
import { renderInstalled } from "./views/installed";
import { renderUpdates } from "./views/updates";
import { renderSettings } from "./views/settings";
import { renderLocal } from "./views/local";
import { onFileDrop } from "mygo-runtime";

type Navigate = (route: string, param?: string) => void;

const content = document.getElementById("content")!;
const searchInput = document.getElementById("search-input") as HTMLInputElement;
const updatesBadge = document.getElementById("updates-badge")!;

installIconFallback();

// Paint the static chrome (icons) that index.html leaves empty.
document.getElementById("search-icon")!.innerHTML = icon("search");
document.getElementById("refresh-btn")!.innerHTML = icon("refresh");
document.querySelectorAll<HTMLElement>(".side-item[data-icon]").forEach((item) => {
  item.insertAdjacentHTML("afterbegin", icon(item.dataset.icon!));
});

// ---------- Updates badge ----------

export async function refreshUpdatesBadge() {
  try {
    const updates = await Store.updates();
    updatesBadge.textContent = updates.length > 99 ? "99+" : String(updates.length);
    updatesBadge.hidden = updates.length === 0;
  } catch {
    updatesBadge.hidden = true;
  }
}

// ---------- Routing ----------

const routeRenderers: Record<string, (param?: string) => Promise<void>> = {
  discover: () => renderDiscover(content, navigate, refreshUpdatesBadge),
  browse: (param) => renderBrowse(content, navigate, refreshUpdatesBadge, param),
  search: (param) => renderSearch(content, navigate, refreshUpdatesBadge, param ?? ""),
  detail: (param) => renderDetail(content, navigate, refreshUpdatesBadge, param ?? "", goBack),
  installed: () => renderInstalled(content, navigate, refreshUpdatesBadge),
  updates: () => renderUpdates(content, navigate, refreshUpdatesBadge),
  settings: () => renderSettings(content),
  local: (param) => renderLocal(content, navigate, param, () => goBack("discover")),
};

let currentRoute = "discover";
let currentParam: string | undefined;
let navStack: { route: string; param?: string }[] = [];

type NavigateOptions = { push?: boolean };
type InternalNavigate = (route: string, param: string | undefined, opts?: NavigateOptions) => void;

const navigateInternal: InternalNavigate = (route, param, opts) => {
  if (opts?.push !== false && (route !== currentRoute || param !== currentParam)) {
    navStack.push({ route: currentRoute, param: currentParam });
    if (navStack.length > 50) navStack.shift();
  }
  currentRoute = route;
  currentParam = param;
  // settings/detail/search don't each have a sidebar entry; map them to one.
  const activeFor = route === "detail" || route === "search" ? "discover" : route;
  document.querySelectorAll<HTMLButtonElement>(".side-item[data-route]").forEach((item) =>
    item.classList.toggle("is-active", item.dataset.route === activeFor)
  );
  const render = routeRenderers[route];
  if (!render) return;
  void render(param).then(() => {
    content.scrollTop = 0;
  });
};

const navigate: Navigate = (route, param) => navigateInternal(route, param);

/** Back in navigation history, or the fallback route. */
export const goBack = (fallback = "discover") => {
  const prev = navStack.pop();
  navigateInternal(prev?.route ?? fallback, prev?.param, { push: false });
};

document.querySelectorAll<HTMLButtonElement>(".side-item[data-route]").forEach((item) =>
  item.addEventListener("click", () => navigate(item.dataset.route!))
);

// ---------- Search ----------

let searchTimer: number | undefined;

function currentSearchTarget(): string | null {
  const value = searchInput.value.trim();
  return value.length > 0 ? value : null;
}

searchInput.addEventListener("input", () => {
  window.clearTimeout(searchTimer);
  const value = searchInput.value.trim();
  if (value.length === 0) {
    if (currentRoute === "search") navigate("discover");
    return;
  }
  searchTimer = window.setTimeout(() => navigate("search", value), 250);
});

searchInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    window.clearTimeout(searchTimer);
    const value = currentSearchTarget();
    if (value) navigate("search", value);
  } else if (event.key === "Escape") {
    searchInput.value = "";
    searchInput.blur();
    if (currentRoute === "search") navigate("discover");
  }
});

window.addEventListener("keydown", (event) => {
  // "/" focuses search, like every good store.
  if (event.key === "/" && !/INPUT|TEXTAREA/.test((event.target as HTMLElement).tagName)) {
    event.preventDefault();
    searchInput.focus();
    searchInput.select();
  }
});

// ---------- Refresh button ----------

const refreshBtn = document.getElementById("refresh-btn") as HTMLButtonElement;
refreshBtn.addEventListener("click", async () => {
  refreshBtn.classList.add("is-spinning");
  await refreshUpdatesBadge();
  refreshBtn.classList.remove("is-spinning");
  toast("Update check complete.", "ok");
});

// ---------- File drops: package files land on the local-install view ----------

if (isMyGo) {
  onFileDrop(({ paths }) => {
    const packagePath = (paths ?? []).find((p) => {
      const exts = [".deb", ".rpm", ".apk", ".snap", ".flatpak", ".appimage"];
      const lower = p.toLowerCase();
      return exts.some((e) => lower.endsWith(e)) || /\.pkg\.tar\.(zst|xz|gz|zstd)$/.test(lower);
    });
    if (packagePath) {
      navigate("local", packagePath);
    } else if ((paths ?? []).length > 0) {
      toast("That file type isn't supported — the store installs .deb, .rpm, Arch packages, .apk, .snap, .flatpak and .AppImage files.", "error");
    }
  });
}

// ---------- Sidebar system card ----------

async function paintSystemCard() {
  const distro = document.getElementById("sys-distro")!;
  const backends = document.getElementById("sys-backends")!;
  try {
    const info = await Store.info();
    distro.textContent = info.prettyName || info.distroName || "Linux";
    const ready = info.backends.filter((b) => b.available).map((b) => b.name);
    backends.textContent = ready.length ? ready.join(" · ") : "No package backends found";
    backends.title = info.backends.map((b) => `${b.name}: ${b.available ? "ready" : "missing"}`).join("\n");
  } catch {
    distro.textContent = isMyGo ? "Detection failed" : "Preview mode";
    backends.textContent = isMyGo ? "backend unavailable" : "open in the app";
  }
}

// ---------- Boot ----------

void paintSystemCard();
void refreshUpdatesBadge();
navigate("discover");
