/* Shared install/remove flows, used by every view.
   Grid buttons pass no track/console; the detail page passes both. */

import { installApp, removeApp, type InstallEvent } from "./api";
import { toast } from "./components";

export type FlowHooks = {
  /** Called after a successful install/remove — views refresh their data. */
  onChanged?: () => void;
  /** Optional progress bar element (detail view). */
  track?: HTMLElement | null;
  /** Optional mono console element for raw output (detail view). */
  consoleEl?: HTMLElement | null;
  /** Optional status line element (detail view). */
  statusEl?: HTMLElement | null;
};

function setBusy(btn: HTMLButtonElement, label: string) {
  btn.disabled = true;
  btn.classList.add("is-busy");
  btn.innerHTML = `<span class="spinner"></span><span>${label}</span>`;
}

function progressLabel(ev: InstallEvent): string {
  if (ev.percent >= 0) return `${Math.floor(ev.percent)}%`;
  return ev.message ?? "Working…";
}

function updateTrack(track: HTMLElement | null | undefined, ev: InstallEvent) {
  if (!track) return;
  track.classList.add("is-visible");
  const fill = track.querySelector<HTMLElement>(".progress-fill")!;
  if (ev.percent >= 0) {
    track.classList.remove("indeterminate");
    fill.style.width = `${ev.percent}%`;
  } else {
    track.classList.add("indeterminate");
  }
}

function appendConsole(consoleEl: HTMLElement | null | undefined, line: string | undefined) {
  if (!consoleEl || !line) return;
  consoleEl.classList.add("is-visible");
  consoleEl.textContent += line + "\n";
  consoleEl.scrollTop = consoleEl.scrollHeight;
}

/** Install an app, driving button/console/track. Resolves true on success. */
export async function runInstall(btn: HTMLButtonElement, id: string, backend: string, hooks: FlowHooks = {}): Promise<boolean> {
  setBusy(btn, "…");
  const { track, consoleEl, statusEl } = hooks;
  const ok = await installApp(id, backend, (ev) => {
    if (ev.type === "start" || ev.type === "progress") {
      btn.querySelector("span:last-child")!.textContent = progressLabel(ev);
      if (statusEl && ev.message) statusEl.textContent = ev.message;
      updateTrack(track, ev);
    } else if (ev.type === "line") {
      appendConsole(consoleEl, ev.line);
      updateTrack(track, ev);
    }
  });
  btn.disabled = false;
  btn.classList.remove("is-busy");
  if (ok) {
    if (track) { track.classList.remove("is-visible", "indeterminate"); }
    hooks.onChanged?.();
    return true;
  }
  if (track) track.classList.remove("is-visible", "indeterminate");
  toast("Install failed — see the output below.", "error");
  return false;
}

/** Remove an app. Resolves true on success. */
export async function runRemove(btn: HTMLButtonElement, id: string, backend: string, hooks: FlowHooks = {}): Promise<boolean> {
  setBusy(btn, "…");
  const { track, consoleEl, statusEl } = hooks;
  const ok = await removeApp(id, backend, (ev) => {
    if (ev.type === "start" || ev.type === "progress") {
      btn.querySelector("span:last-child")!.textContent = progressLabel(ev);
      if (statusEl && ev.message) statusEl.textContent = ev.message;
      updateTrack(track, ev);
    } else if (ev.type === "line") {
      appendConsole(consoleEl, ev.line);
      updateTrack(track, ev);
    }
  });
  btn.disabled = false;
  btn.classList.remove("is-busy");
  if (ok) {
    if (track) track.classList.remove("is-visible", "indeterminate");
    hooks.onChanged?.();
    return true;
  }
  if (track) track.classList.remove("is-visible", "indeterminate");
  toast("Remove failed — see the output below.", "error");
  return false;
}

/** Wire delegated clicks for [data-open] and [data-get] inside root. */
export function wireCardActions(
  root: HTMLElement,
  navigate: (route: string, param?: string) => void,
  onChanged: () => void
) {
  root.addEventListener("click", (event) => {
    const target = event.target as HTMLElement;
    const getBtn = target.closest<HTMLButtonElement>("[data-get]");
    if (getBtn) {
      event.preventDefault();
      event.stopPropagation();
      const id = getBtn.dataset.get!;
      const backend = getBtn.dataset.backend ?? "flatpak";
      if (getBtn.classList.contains("installed")) {
        navigate("detail", id);
        return;
      }
      void runInstall(getBtn, id, backend, {
        onChanged: () => {
          getBtn.classList.add("installed");
          getBtn.textContent = "Installed";
          toast("Installed — find it in your application menu.", "ok");
          onChanged();
        },
      });
      return;
    }
    const card = target.closest<HTMLElement>("[data-open]");
    if (card) {
      navigate("detail", card.dataset.open!);
    }
  });

  root.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    const target = event.target as HTMLElement;
    if (target.closest("button")) return; // buttons handle themselves
    const card = target.closest<HTMLElement>("[data-open]");
    if (card) {
      event.preventDefault();
      navigate("detail", card.dataset.open!);
    }
  });
}
