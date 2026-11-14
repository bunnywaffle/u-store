import { Store, installLocalPackage, formatSize, type ProgressListener } from "../api";
import { emptyState, esc, toast } from "../components";
import { icon } from "../icons";

type Navigate = (route: string, param?: string) => void;
type Back = () => void;

// Flat, muted colors per package kind — drawn from the app's own palette.
const KIND_TILES: Record<string, { color: string; label: string }> = {
  deb: { color: "#3f3f3f", label: "DEB" },
  rpm: { color: "#3f3f3f", label: "RPM" },
  arch: { color: "#3f3f3f", label: "ARC" },
  apk: { color: "#3f3f3f", label: "APK" },
  snap: { color: "#3f3f3f", label: "SNP" },
  flatpak: { color: "#3f3f3f", label: "FLT" },
  appimage: { color: "#3f3f3f", label: "IMG" },
  unknown: { color: "#3f3f3f", label: "?" },
};

const SUPPORTED_KINDS = Object.keys(KIND_TILES).filter((k) => k !== "unknown");

export async function renderLocal(root: HTMLElement, navigate: Navigate, path: string | undefined, back: Back) {
  if (!path) {
    renderPicker(root, navigate, back);
    return;
  }

  root.innerHTML = `<div class="view"><div class="skeleton" style="height:190px;border-radius:var(--radius-xl)"></div></div>`;

  let p;
  try {
    p = await Store.inspectLocalPackage(path);
  } catch (err) {
    root.innerHTML = `<div class="view"><button class="detail-back" id="back">${icon("arrowLeft")} Back</button>${emptyState("x", "Couldn't read that file", String((err as Error)?.message ?? err))}</div>`;
    root.querySelector("#back")?.addEventListener("click", back);
    return;
  }

  const tile = KIND_TILES[p.kind] ?? KIND_TILES["unknown"]!;
  const prettyName = p.name || p.fileName;
  const installable = p.supported;

  root.innerHTML = `
  <div class="view">
    <button class="detail-back" id="back">${icon("arrowLeft")} Back</button>

    <section class="detail-hero">
      <span class="icon-tile detail" style="background:${tile.color};font-size:1.7rem;font-weight:800;letter-spacing:0.05em">${esc(tile.label)}</span>
      <div class="detail-info">
        <h1 class="detail-name">${esc(prettyName)}</h1>
        <span class="detail-dev">${esc(describeKind(p))}</span>
        <div class="detail-actions">
          <button class="btn ${installable ? "" : "ghost"}" id="local-install" ${installable ? "" : "disabled"}>${installable ? `Install${p.kind === "appimage" ? " into ~/Applications" : ""}` : "Not installable"}</button>
        </div>
        <p class="row-sub" id="local-status" style="margin:0;min-height:1.2em"></p>
        <div class="progress-track" id="local-track"><div class="progress-fill"></div></div>
        <pre class="install-console" id="local-console" aria-live="polite"></pre>
      </div>
    </section>

    <div class="meta-row">
      <div class="meta-item"><span class="meta-k">File</span><span class="meta-v" style="word-break:break-all">${esc(p.fileName)}</span></div>
      <div class="meta-item"><span class="meta-k">Size</span><span class="meta-v">${formatSize(p.size)}</span></div>
      ${p.version ? `<div class="meta-item"><span class="meta-k">Version</span><span class="meta-v">${esc(p.version)}</span></div>` : ""}
      <div class="meta-item"><span class="meta-k">Format</span><span class="meta-v">${esc(p.kindName)}</span></div>
    </div>

    ${p.summary ? `<div class="desc"><p>${esc(p.summary)}</p></div>` : ""}

    <div class="panel" style="margin-top:16px">
      <h2 class="panel-title">${icon(installable ? "shieldCheck" : "info")} ${installable ? "What will happen" : "Can't install this file"}</h2>
      ${installable ? `
        <p style="margin:0 0 10px;font-size:.88rem;line-height:1.55;color:var(--fg)">${esc(p.action)}</p>
        <pre class="install-console is-visible" style="color:var(--muted);margin-top:0">${esc(p.command)}</pre>
        ${p.needsAuth ? `<p style="margin:10px 0 0;font-size:.8rem;color:var(--muted)">You'll be asked to authenticate (polkit) — your password never touches this app.</p>` : `<p style="margin:10px 0 0;font-size:.8rem;color:var(--muted)">No administrator password needed.</p>`}
      ` : `
        <p style="margin:0;font-size:.88rem;line-height:1.55;color:var(--fg)">${esc(p.reason)}</p>
      `}
    </div>

    <p class="row-sub" style="margin-top:18px;padding:0 2px">Tip: you can also drag a package file onto the U Store window from anywhere.</p>
  </div>`;

  root.querySelector("#back")!.addEventListener("click", back);

  const installBtn = root.querySelector<HTMLButtonElement>("#local-install")!;
  const track = root.querySelector<HTMLElement>("#local-track")!;
  const consoleEl = root.querySelector<HTMLElement>("#local-console")!;
  const statusEl = root.querySelector<HTMLElement>("#local-status")!;

  const onProgress: ProgressListener = (ev) => {
    if (ev.type === "start" || ev.type === "progress") {
      if (ev.message) statusEl.textContent = ev.message;
      setTrack(track, ev.percent);
    } else if (ev.type === "line") {
      consoleEl.textContent += (ev.line ?? "") + "\n";
      consoleEl.scrollTop = consoleEl.scrollHeight;
      setTrack(track, ev.percent);
    } else if (ev.type === "done") {
      statusEl.textContent = ev.message ?? "Installed.";
      track.classList.remove("is-visible", "indeterminate");
    } else if (ev.type === "error" && ev.message) {
      statusEl.textContent = ev.message;
    }
  };

  installBtn.addEventListener("click", async () => {
    installBtn.disabled = true;
    installBtn.innerHTML = `<span class="spinner"></span> Installing…`;
    const ok = await installLocalPackage(p.path, onProgress);
    if (ok) {
      installBtn.className = "btn ghost";
      installBtn.innerHTML = `${icon("check")} Installed`;
      installBtn.disabled = true;
      toast(`${prettyName} installed.`, "ok");
    } else {
      installBtn.disabled = false;
      installBtn.innerHTML = "Retry install";
      toast("Install failed — see the output above.", "error");
    }
  });
}

function renderPicker(root: HTMLElement, _navigate: Navigate, back: Back) {
  root.innerHTML = `
  <div class="view">
    <button class="detail-back" id="back">${icon("arrowLeft")} Back</button>
    <div class="empty" style="padding-top:36px">
      <div class="empty-icon">${icon("download")}</div>
      <h3>Install a package file</h3>
      <p>Got a <strong>.deb</strong>, <strong>.rpm</strong>, Arch <strong>.pkg.tar.*</strong>, <strong>.apk</strong>, <strong>.snap</strong>, <strong>.flatpak</strong> or an <strong>.AppImage</strong>? U Store checks it's for this machine, shows you exactly what it will run, and installs it.</p>
      <div class="drop-zone" id="drop-zone">Drop a package file onto this window<br /><span style="color:var(--faint);font-size:.78rem">or</span></div>
      <button class="btn" id="choose-file">${icon("download")} Choose file…</button>
      <div class="chip-row" style="justify-content:center;margin-top:8px">${SUPPORTED_KINDS.map((k) => `<span class="source-pill ${k}">${esc(KIND_TILES[k]!.label)}</span>`).join("")}</div>
    </div>
  </div>`;

  root.querySelector("#back")!.addEventListener("click", back);
  root.querySelector("#choose-file")!.addEventListener("click", async () => {
    try {
      const p = await Store.pickLocalPackage();
      if (p.path) {
        const back2 = () => renderPicker(root, _navigate, back);
        void renderLocal(root, _navigate, p.path, back2);
      }
    } catch (err) {
      toast(`Couldn't open the file picker: ${String((err as Error)?.message ?? err)}`, "error");
    }
  });
}

function setTrack(track: HTMLElement, percent: number) {
  track.classList.add("is-visible");
  const fill = track.querySelector<HTMLElement>(".progress-fill")!;
  if (percent >= 0) {
    track.classList.remove("indeterminate");
    fill.style.width = `${percent}%`;
  } else {
    track.classList.add("indeterminate");
  }
}

function describeKind(p: { kindName: string }): string {
  return p.kindName;
}
