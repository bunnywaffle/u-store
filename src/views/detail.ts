import { Store, isMyGo, sanitizeHTML, backendName, nativeOptionFor, snapOptionFor, formatInstalls, formatDate, type AppDetails } from "../api";
import { appIcon, esc, emptyState, verifiedBadge, toast } from "../components";
import { icon } from "../icons";
import { runInstall, runRemove } from "../actions";

type Navigate = (route: string, param?: string) => void;

function openLightbox(src: string, appName: string) {
  const overlay = document.createElement("div");
  overlay.className = "lightbox";
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-modal", "true");
  overlay.setAttribute("aria-label", `${appName} screenshot`);
  overlay.innerHTML = `
    <button class="lightbox-close" aria-label="Close screenshot">${icon("x")}</button>
    <img src="${esc(src)}" alt="${esc(appName)} screenshot, enlarged" referrerpolicy="no-referrer" />`;
  const close = () => {
    document.removeEventListener("keydown", onKey, true);
    overlay.remove();
  };
  const onKey = (event: KeyboardEvent) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      close();
    }
  };
  overlay.addEventListener("click", (event) => {
    if (event.target === overlay || (event.target as HTMLElement).closest(".lightbox-close")) close();
  });
  document.addEventListener("keydown", onKey, true);
  document.body.appendChild(overlay);
  overlay.querySelector<HTMLButtonElement>(".lightbox-close")!.focus();
}

export async function renderDetail(root: HTMLElement, navigate: Navigate, onChanged: () => void, id: string, back?: () => void) {
  root.innerHTML = `<div class="view"><div class="detail-back" role="button" tabindex="0" id="back">${icon("arrowLeft")} Back</div><div class="skeleton" style="height:170px;border-radius:var(--radius-xl)"></div><div class="skeleton" style="height:120px;border-radius:var(--radius-lg);margin-top:18px"></div></div>`;
  root.querySelector("#back")!.addEventListener("click", () => (back ?? (() => navigate("discover")))());

  let app: AppDetails;
  if (id.startsWith("native:")) {
    const native = id.slice(7).split(":");
    const backend = native[0];
    const ref = native.slice(1).join(":");
    if (!backend || !ref) {
      root.innerHTML = `<div class="view"><button class="detail-back" id="back">${icon("arrowLeft")} Back</button>${emptyState("x", "Bad native package link", "")}</div>`;
      root.querySelector("#back")?.addEventListener("click", back ?? (() => navigate("discover")));
      return;
    }
    try {
      app = await Store.nativeDetails(backend, ref);
    } catch (err) {
      root.innerHTML = `<div class="view"><button class="detail-back" id="back">${icon("arrowLeft")} Back</button>${emptyState("wifiOff", "Couldn't read package info", String((err as Error)?.message ?? err))}</div>`;
      root.querySelector("#back")?.addEventListener("click", back ?? (() => navigate("discover")));
      return;
    }
  } else {
    try {
      app = await Store.appDetails(id);
    } catch {
      root.innerHTML = `<div class="view"><button class="detail-back" id="back">${icon("arrowLeft")} Back</button>${emptyState("wifiOff", "Couldn't load this app", isMyGo ? "Check your network connection and try again." : "Run inside U Store to see live app details.")}</div>`;
      root.querySelector("#back")?.addEventListener("click", () => navigate("discover"));
      return;
    }
  }

  const native = nativeOptionFor(app);
  const snap = snapOptionFor(app);
  const sources = app.sources ?? ["flatpak"];
  let selected = app.source || sources[0] || "flatpak";
  if (!sources.includes(selected)) selected = sources[0]!;

  const sourceButtons = sources.length > 1
    ? `<span class="meta-k" style="align-self:center;margin-left:4px">Install with</span><div class="segmented" id="source-picker" role="radiogroup" aria-label="Install source">
         ${sources.map((s) => `<button role="radio" aria-checked="${s === selected}" class="${s === selected ? "is-active" : ""}" data-source="${esc(s)}">${esc(backendName(s))}</button>`).join("")}
       </div>`
    : "";

  const verified = app.verified ? verifiedBadge : "";
  const downloads = app.installsLastMonth > 0 ? `<div class="meta-item"><span class="meta-k">Installs / month</span><span class="meta-v">${formatInstalls(app.installsLastMonth)}</span></div>` : "";

  root.innerHTML = `
  <div class="view">
    <button class="detail-back" id="back">${icon("arrowLeft")} Back</button>

    <section class="detail-hero">
      ${appIcon(app, "detail-icon")}
      <div class="detail-info">
        <h1 class="detail-name">${esc(app.name)} ${verified}</h1>
        <span class="detail-dev">${app.developer ? esc(app.developer) : esc(backendName(selected)) + " package"}</span>
        <div class="detail-actions">
          <button class="btn" id="action-primary">${app.installed ? "Open" : "Get"}</button>
          ${app.installed ? `<button class="btn danger" id="action-remove">${icon("trash")} Remove</button>` : ""}
          ${sourceButtons}
        </div>
        <p class="row-sub" id="action-status" style="margin:0;min-height:1.2em"></p>
        <div class="progress-track" id="action-track"><div class="progress-fill"></div></div>
        <pre class="install-console" id="action-console" aria-live="polite"></pre>
      </div>
    </section>

    <div class="meta-row">
      <div class="meta-item"><span class="meta-k">Source</span><span class="meta-v">${esc(backendName(selected))}${app.origin === "flathub" ? " · Flathub" : ""}</span></div>
      ${app.license ? `<div class="meta-item"><span class="meta-k">License</span><span class="meta-v">${esc(app.license)}</span></div>` : ""}
      ${app.updatedAt ? `<div class="meta-item"><span class="meta-k">Updated</span><span class="meta-v">${formatDate(app.updatedAt)}</span></div>` : ""}
      ${downloads}
      ${app.nativePackage && native ? `<div class="meta-item"><span class="meta-k">Native package</span><span class="meta-v" style="font-family:var(--mono);font-size:.8rem">${esc(app.nativePackage)}</span></div>` : ""}
      ${snap ? `<div class="meta-item"><span class="meta-k">Also on</span><span class="meta-v">Snap Store</span></div>` : ""}
    </div>

    ${app.screenshots?.length ? `<div class="screens" aria-label="Screenshots">${app.screenshots
        .map((src, i) => `<button class="screen" data-shot="${esc(src)}" aria-label="Screenshot ${i + 1} of ${esc(app.name)}"><img src="${esc(src)}" alt="${esc(app.name)} screenshot ${i + 1}" height="176" width="313" loading="lazy" decoding="async" referrerpolicy="no-referrer" /></button>`)
        .join("")}</div>` : ""}

    ${app.description ? `<div class="desc">${sanitizeHTML(app.description)}</div>` : ""}
    ${app.homepage || app.origin === "flathub" ? `<p style="margin:18px 4px 0">${app.homepage ? `<a class="btn ghost sm" href="${esc(app.homepage)}" target="_blank" rel="noreferrer noopener">${icon("external")} Website</a>` : ""} ${app.origin === "flathub" ? `<a class="btn ghost sm" href="https://flathub.org/apps/${esc(app.id)}" target="_blank" rel="noreferrer noopener">${icon("external")} View on Flathub</a>` : ""}</p>` : ""}
  </div>`;

  root.querySelector("#back")!.addEventListener("click", () => navigate("discover"));

  const primary = root.querySelector<HTMLButtonElement>("#action-primary")!;
  const removeBtn = root.querySelector<HTMLButtonElement>("#action-remove");
  const track = root.querySelector<HTMLElement>("#action-track");
  const consoleEl = root.querySelector<HTMLElement>("#action-console");
  const statusEl = root.querySelector<HTMLElement>("#action-status");
  const hooks = { onChanged, track, consoleEl, statusEl };

  const syncPrimaryLabel = () => {
    primary.innerHTML = app.installed ? "Open" : "Get";
  };

  primary.addEventListener("click", async () => {
    if (app.installed) {
      try {
        await Store.launch(app.id, selected);
      } catch {
        toast("Couldn't launch it — try opening it from your application menu.", "error");
      }
      return;
    }
    const ok = await runInstall(primary, app.id, selected, hooks);
    if (ok) {
      app.installed = true;
      syncPrimaryLabel();
      if (!removeBtn) {
        const actions = root.querySelector(".detail-actions")!;
        const btn = document.createElement("button");
        btn.className = "btn danger";
        btn.id = "action-remove";
        btn.innerHTML = `${icon("trash")} Remove`;
        actions.appendChild(btn);
        btn.addEventListener("click", handleRemove);
      }
      toast(`${app.name} installed.`, "ok");
    } else {
      primary.innerHTML = "Get";
    }
  });

  const handleRemove = async () => {
    if (!removeBtn) return;
    const ok = await runRemove(removeBtn, app.id, selected, hooks);
    if (ok) {
      app.installed = false;
      primary.innerHTML = "Get";
      removeBtn.remove();
      toast(`${app.name} removed.`, "ok");
    }
  };
  removeBtn?.addEventListener("click", handleRemove);

  // Source picker: swap the backend used for installs and update metadata labels.
  root.querySelectorAll<HTMLButtonElement>("#source-picker button").forEach((btn) =>
    btn.addEventListener("click", () => {
      selected = btn.dataset.source!;
      root.querySelectorAll<HTMLButtonElement>("#source-picker button").forEach((b) => {
        b.classList.toggle("is-active", b === btn);
        b.setAttribute("aria-checked", String(b === btn));
      });
    })
  );
}
