import { Store, backendName, updateAll, type UpdateInfo } from "../api";
import { appIcon, emptyState, esc, toast, matchesSourceFilter, sourceSegmented, wireSourceFilter, type SourceFilter } from "../components";
import { icon } from "../icons";

type Navigate = (route: string, param?: string) => void;

let sourceFilter: SourceFilter = "all";
let lastUpdates: UpdateInfo[] = [];

export async function renderUpdates(root: HTMLElement, navigate: Navigate, onChanged: () => void) {
  root.innerHTML = `<div class="view"><div class="view-head"><div><h1 class="view-title">Updates</h1><p class="view-sub">Checking every source for what's new…</p></div></div><div class="skeleton" style="height:70px;border-radius:var(--radius-lg)"></div></div>`;

  try {
    lastUpdates = await Store.updates();
  } catch {
    root.innerHTML = `<div class="view"><div class="view-head"><div><h1 class="view-title">Updates</h1></div></div>${emptyState("wifiOff", "Couldn't check for updates", "The package managers didn't answer. Try again in a moment.")}</div>`;
    return;
  }

  paint(root, navigate, onChanged);
}

function paint(root: HTMLElement, navigate: Navigate, onChanged: () => void) {
  const updates = lastUpdates;
  const filtered = updates.filter((u) => matchesSourceFilter(sourceFilter, [u.backend]));
  const flatpakCount = updates.filter((u) => u.backend === "flatpak").length;
  const nativeCount = updates.length - flatpakCount;
  const sourceWord =
    sourceFilter === "all" ? "updates" :
    sourceFilter === "flatpak" ? "Flatpak updates" :
    sourceFilter === "snap" ? "Snap updates" : "native updates";

  root.innerHTML = `
  <div class="view">
    <div class="view-head">
      <div>
        <h1 class="view-title">Updates</h1>
        <p class="view-sub">${updates.length === 0
          ? "Everything is up to date."
          : `${updates.length} pending — ${flatpakCount} Flatpak, ${nativeCount} native. Native managers update in one transaction, so everything is updated together. Showing ${filtered.length} ${esc(sourceWord)}.`}</p>
      </div>
      ${updates.length ? `<button class="btn" id="update-all">${icon("download")} Update all</button>` : ""}
    </div>
    ${updates.length === 0 ? "" : sourceSegmented("updates-source-filter", sourceFilter)}
    ${filtered.length === 0
      ? (updates.length === 0
          ? emptyState("check", "All caught up", "New versions of your apps will appear here the moment they land in Flathub or your distro's repositories.")
          : emptyState("check", "Nothing matches this source", "Try updating everything — native managers apply them in one transaction."))
      : `<div class="list" id="updates-list" style="margin-top:16px">${filtered.slice(0, 100).map(row).join("")}${filtered.length > 100 ? `<p class="view-sub" style="padding:6px 4px">+ ${filtered.length - 100} more — native managers apply them all together.</p>` : ""}</div>
         <pre class="install-console" id="update-console"></pre>
         <div class="progress-track" id="update-track"><div class="progress-fill"></div></div>`}
  </div>`;

  const allBtn = root.querySelector<HTMLButtonElement>("#update-all");
  const consoleEl = root.querySelector<HTMLElement>("#update-console");
  const track = root.querySelector<HTMLElement>("#update-track");

  wireSourceFilter(root, "updates-source-filter", (f) => {
    sourceFilter = f;
    paint(root, navigate, onChanged);
  });

  allBtn?.addEventListener("click", async () => {
    allBtn.disabled = true;
    allBtn.innerHTML = `<span class="spinner"></span> Updating…`;
    const ok = await updateAll((ev) => {
      if (ev.line) {
        if (consoleEl) {
          consoleEl.classList.add("is-visible");
          consoleEl.textContent += ev.line + "\n";
          consoleEl.scrollTop = consoleEl.scrollHeight;
        }
      }
      if (track) {
        track.classList.add("is-visible");
        const fill = track.querySelector<HTMLElement>(".progress-fill")!;
        if (ev.percent >= 0) {
          track.classList.remove("indeterminate");
          fill.style.width = `${ev.percent}%`;
        } else {
          track.classList.add("indeterminate");
        }
      }
    });
    track?.classList.remove("is-visible", "indeterminate");
    if (ok) {
      toast("Everything is up to date.", "ok");
      onChanged();
      void renderUpdates(root, navigate, onChanged);
    } else {
      allBtn.disabled = false;
      allBtn.innerHTML = `${icon("download")} Retry update all`;
      toast("Update failed or was cancelled — see the output.", "error");
    }
  });

  root.querySelectorAll<HTMLButtonElement>("[data-detail]").forEach((btn) =>
    btn.addEventListener("click", () => navigate("detail", btn.dataset.detail))
  );
}

function row(u: UpdateInfo): string {
  const iconHtml = u.icon
    ? appIcon({ id: u.id, name: u.name, icon: u.icon })
    : `<span class="icon-tile" style="background:var(--surface-2);color:var(--muted)">${esc(u.name[0]?.toUpperCase() ?? "?")}</span>`;
  const versions = u.to ? `${esc(u.from || "—")} <span class="arrow">→</span> ${esc(u.to)}` : "";
  return `
  <div class="list-row">
    ${iconHtml}
    <div class="row-main">
      <span class="row-title">${esc(u.name)}</span>
      <div class="row-sub">${versions || "Version pending"}</div>
    </div>
    <span class="source-pill ${esc(u.backend)}">${esc(backendName(u.backend))}</span>
    <div class="row-actions"><button class="btn ghost sm" data-detail="${esc(u.id)}">Details</button></div>
  </div>`;
}
