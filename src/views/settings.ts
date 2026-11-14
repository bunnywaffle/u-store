import { Store, preferNative, setPreferNative, backendName, type SystemInfo } from "../api";
import { emptyState, esc, toast } from "../components";
import { icon } from "../icons";

export async function renderSettings(root: HTMLElement) {
  root.innerHTML = `<div class="view"><h1 class="view-title">Settings</h1><div class="skeleton" style="height:200px;border-radius:var(--radius-lg);margin-top:16px"></div></div>`;

  let info: SystemInfo | null = null;
  try {
    info = await Store.info();
  } catch {
    root.innerHTML = `<div class="view"><h1 class="view-title">Settings</h1>${emptyState("wifiOff", "Backend unavailable", "System information could not be read.")}</div>`;
    return;
  }

  const prefer = preferNative();
  const nativeName = info.nativeBackendId ? backendName(info.nativeBackendId) : null;

  root.innerHTML = `
  <div class="view">
    <div class="view-head">
      <div>
        <h1 class="view-title">Settings</h1>
        <p class="view-sub">How U Store installs and updates apps on this machine.</p>
      </div>
    </div>

    <div class="settings-grid">

      <section class="panel">
        <h2 class="panel-title">${icon("tool")} System</h2>
        <dl class="kv">
          <dt>Distribution</dt><dd>${esc(info.prettyName)} <span style="color:var(--faint);font-family:var(--mono);font-size:.8em">(${esc(info.distroId)})</span></dd>
          <dt>Architecture</dt><dd>${esc(info.arch)}</dd>
          <dt>Preferred source</dt><dd id="pref-source-label">${prefer && nativeName ? `${esc(nativeName)} (native) first` : "Flatpak first"}</dd>
        </dl>
      </section>

      <section class="panel">
        <h2 class="panel-title">${icon("package")} Package backends</h2>
        <table class="backend-table">
          <thead><tr><th>Source</th><th>Type</th><th>Status</th><th>What it's for</th></tr></thead>
          <tbody>
            ${info.backends.map((b) => `
              <tr>
                <td style="font-weight:650">${esc(b.name)}</td>
                <td style="color:var(--muted)">${b.kind === "universal" ? "Universal" : "Native"}</td>
                <td>${b.available ? '<span class="pill-ok">Ready</span>' : '<span class="pill-off">Missing</span>'}</td>
                <td style="color:var(--muted)">${esc(b.description)}</td>
              </tr>`).join("")}
          </tbody>
        </table>
        <p style="margin:12px 2px 0;font-size:.8rem;color:var(--muted);line-height:1.5">Flatpak is the default on every distro because it behaves identically everywhere. Native packages are preferred when you enable the option below — they integrate more deeply with the desktop.</p>
      </section>

      <section class="panel">
        <h2 class="panel-title">${icon("shieldCheck")} Flathub</h2>
        <div class="pref-row" style="border-top:0;padding-top:0">
          <div class="pref-text">
            <span class="pref-name">Flathub remote ${info.flathubEnabled ? '<span class="pill-ok" style="margin-left:6px">Enabled</span>' : '<span class="pill-off" style="margin-left:6px">Not added</span>'}</span>
            <span class="pref-desc">Flathub is the app store behind "Flatpak". Without its remote, Flatpak installs have nowhere to come from.</span>
          </div>
          ${info.flathubEnabled ? "" : `<button class="btn" id="enable-flathub">Enable Flathub</button>`}
        </div>
      </section>

      <section class="panel">
        <h2 class="panel-title">${icon("gear")} Appearance</h2>
        <div class="pref-row" style="border-top:0;padding-top:0">
          <div class="pref-text">
            <span class="pref-name">Theme</span>
            <span class="pref-desc">Follow your desktop's light or dark mode, or pin one. Dark is used when your system doesn't say — and by default, as in every serious store.</span>
          </div>
          <div class="segmented" id="theme-picker" role="radiogroup" aria-label="Theme">
            <button role="radio" data-theme="dark" aria-checked="${info.theme === "dark"}" class="${info.theme === "dark" ? "is-active" : ""}">Dark</button>
            <button role="radio" data-theme="light" aria-checked="${info.theme === "light"}" class="${info.theme === "light" ? "is-active" : ""}">Light</button>
            <button role="radio" data-theme="system" aria-checked="${info.theme !== "dark" && info.theme !== "light"}" class="${info.theme !== "dark" && info.theme !== "light" ? "is-active" : ""}">System</button>
          </div>
        </div>
      </section>

      <section class="panel">
        <h2 class="panel-title">${icon("gear")} Behaviour</h2>
        <div class="pref-row" style="border-top:0;padding-top:0">
          <div class="pref-text">
            <label class="pref-name" for="prefer-native">Prefer native packages</label>
            <span class="pref-desc">When an app exists in your distro's repositories${nativeName ? ` (${esc(nativeName)})` : ""}, install that build instead of the Flatpak. Smaller disks, deeper desktop integration.</span>
          </div>
          <label class="switch"><input type="checkbox" id="prefer-native" ${prefer ? "checked" : ""} ${nativeName ? "" : "disabled title=\"No native package manager detected\""} /><span class="track"></span><span class="thumb"></span></label>
        </div>
      </section>

      <section class="panel">
        <h2 class="panel-title">${icon("info")} About</h2>
        <dl class="kv">
          <dt>Version</dt><dd>U Store 0.1.0</dd>
          <dt>License</dt><dd>MIT</dd>
          <dt>Sources</dt><dd>Flathub (flatpak) ${info.nativeBackendId ? ", your distro's repositories, " : ""}and Snap when present</dd>
          <dt>Built with</dt><dd>Go · MyGo — a native app, not a web page in disguise</dd>
        </dl>
        <p style="margin:14px 2px 0;font-size:.8rem;color:var(--muted)">U Store works on every Linux distribution because it talks to each one's package manager the same way — natively, or through Flathub where nothing matches.</p>
      </section>

    </div>
  </div>`;

  root.querySelector("#enable-flathub")?.addEventListener("click", async (event) => {
    const btn = event.currentTarget as HTMLButtonElement;
    btn.disabled = true;
    btn.innerHTML = `<span class="spinner"></span> Enabling…`;
    try {
      await Store.enableFlathub();
      toast("Flathub is ready to install apps.", "ok");
      void renderSettings(root);
    } catch {
      btn.disabled = false;
      btn.textContent = "Retry";
      toast("Couldn't add Flathub. Check your network connection.", "error");
    }
  });

  root.querySelector<HTMLInputElement>("#prefer-native")?.addEventListener("change", (event) => {
    const checked = (event.target as HTMLInputElement).checked;
    setPreferNative(checked);
    toast(checked ? "Native packages will be preferred on Get." : "Flatpak will be preferred on Get.", "ok");
  });

  root.querySelectorAll<HTMLButtonElement>("#theme-picker button").forEach((btn) =>
    btn.addEventListener("click", async () => {
      const mode = btn.dataset.theme!;
      try {
        await Store.setTheme(mode);
      } catch (err) {
        toast(`Couldn't set the theme: ${String((err as Error)?.message ?? err)}`, "error");
        return;
      }
      root.querySelectorAll<HTMLButtonElement>("#theme-picker button").forEach((b) => {
        b.classList.toggle("is-active", b === btn);
        b.setAttribute("aria-checked", String(b === btn));
      });
      toast(mode === "system" ? "Theme will follow your desktop." : `${mode === "dark" ? "Dark" : "Light"} theme applied.`, "ok");
    })
  );
}
