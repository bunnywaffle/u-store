import { Store, backendName, type InstalledApp } from "../api";
import { appIcon, emptyState, esc, verifiedBadge, toast } from "../components";
import { icon } from "../icons";
import { runRemove } from "../actions";

type Navigate = (route: string, param?: string) => void;

export async function renderInstalled(root: HTMLElement, navigate: Navigate, onChanged: () => void) {
  root.innerHTML = `<div class="view"><div class="view-head"><div><h1 class="view-title">Installed</h1><p class="view-sub">Everything this machine has, from every source.</p></div></div><div class="skeleton" style="height:70px;border-radius:var(--radius-lg)"></div></div>`;

  let apps: InstalledApp[] = [];
  try {
    apps = await Store.installed();
  } catch {
    root.innerHTML = `<div class="view">${emptyState("wifiOff", "Couldn't read installed apps", "The backend didn't answer. Is the system package database healthy?")}</div>`;
    return;
  }

  if (apps.length === 0) {
    root.innerHTML = `<div class="view"><div class="view-head"><div><h1 class="view-title">Installed</h1><p class="view-sub">Everything this machine has, from every source.</p></div></div>${emptyState("package", "Nothing installed yet", "Apps you install from here will show up in this list, whichever source they came from.")}</div>`;
    return;
  }

  const flatpakCount = apps.filter((a) => a.sources.some((s) => s.backend === "flatpak")).length;

  root.innerHTML = `
  <div class="view">
    <div class="view-head">
      <div>
        <h1 class="view-title">Installed</h1>
        <p class="view-sub">${apps.length} ${apps.length === 1 ? "app" : "apps"}${flatpakCount ? ` · ${flatpakCount} via Flatpak` : ""}</p>
      </div>
    </div>
    <div class="list" id="installed-list">${apps.map(row).join("")}</div>
  </div>`;

  const list = root.querySelector<HTMLElement>("#installed-list")!;
  list.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    const target = event.target as HTMLElement;
    if (target.closest("button")) return;
    const row = target.closest<HTMLElement>("[data-open]");
    if (row) {
      event.preventDefault();
      navigate("detail", row.dataset.open);
    }
  });

  list.addEventListener("click", (event) => {
    const target = event.target as HTMLElement;

    const openBtn = target.closest<HTMLButtonElement>("[data-open-app]");
    if (openBtn) {
      const id = openBtn.dataset.openApp!;
      const backend = openBtn.dataset.backend!;
      void Store.launch(id, backend).catch(() => navigate("detail", id));
      return;
    }

    const removeBtn = target.closest<HTMLButtonElement>("[data-remove-app]");
    if (removeBtn) {
      // Two-click confirm: first click arms, second removes.
      if (removeBtn.dataset.armed !== "1") {
        removeBtn.dataset.armed = "1";
        removeBtn.innerHTML = `${icon("trash")} Confirm remove`;
        removeBtn.classList.add("danger");
        window.setTimeout(() => {
          if (removeBtn.isConnected && removeBtn.dataset.armed === "1") {
            removeBtn.dataset.armed = "0";
            removeBtn.innerHTML = `${icon("trash")} Remove`;
          }
        }, 3200);
        return;
      }
      removeBtn.disabled = true;
      void runRemove(removeBtn, removeBtn.dataset.removeApp!, removeBtn.dataset.backend!, {
        onChanged: () => {
          toast("Removed.", "ok");
          onChanged();
          void renderInstalled(root, navigate, onChanged);
        },
      });
      return;
    }

    const row = target.closest<HTMLElement>("[data-open]");
    if (row) navigate("detail", row.dataset.open);
  });
}

function row(app: InstalledApp): string {
  const verified = app.verified ? verifiedBadge : "";
  const backend = app.sources[0]?.backend ?? "flatpak";
  const sources = app.sources.map((s) => backendName(s.backend) + (s.scope ? ` (${s.scope})` : "")).join(" · ");
  return `
  <div class="list-row" data-open="${esc(app.id)}" role="button" tabindex="0" aria-label="${esc(app.name)}">
    ${appIcon(app)}
    <div class="row-main">
      <span class="row-title">${esc(app.name)} ${verified}</span>
      <div class="row-sub">${esc(sources)}</div>
    </div>
    <span class="source-pill ${esc(backend)}">${esc(backendName(backend))}</span>
    <div class="row-actions">
      <button class="btn ghost sm" data-open-app="${esc(app.id)}" data-backend="${esc(backend)}">Open</button>
      <button class="btn ghost sm" data-remove-app="${esc(app.id)}" data-backend="${esc(backend)}">${icon("trash")} Remove</button>
    </div>
  </div>`;
}
