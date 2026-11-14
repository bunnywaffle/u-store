/* Inline SVG icon set (stroke = currentColor). Keeps the app dependency-free. */

const svg = (body: string, viewBox = "0 0 24 24") =>
  `<svg viewBox="${viewBox}" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${body}</svg>`;

export const icons: Record<string, string> = {
  compass: svg('<circle cx="12" cy="12" r="9"/><polygon points="15.5 8.5 13.6 13.6 8.5 15.5 10.4 10.4" fill="currentColor" stroke="none"/>'),
  grid: svg('<rect x="3.5" y="3.5" width="7" height="7" rx="2"/><rect x="13.5" y="3.5" width="7" height="7" rx="2"/><rect x="3.5" y="13.5" width="7" height="7" rx="2"/><rect x="13.5" y="13.5" width="7" height="7" rx="2"/>'),
  package: svg('<path d="M12 3 4 7.5v9L12 21l8-4.5v-9L12 3z"/><path d="M4 7.5l8 4.5 8-4.5"/><path d="M12 12v9"/><path d="M8 5.5l8 4.5"/>'),
  refresh: svg('<path d="M20 5v6h-6"/><path d="M4 19v-6h6"/><path d="M5.6 9a8 8 0 0 1 13.2-1.6L20 9"/><path d="M18.4 15a8 8 0 0 1-13.2 1.6L4 15"/>'),
  gear: svg('<circle cx="12" cy="12" r="3.2"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.03 1.56V21a2 2 0 0 1-4 0v-.09a1.7 1.7 0 0 0-1.03-1.56 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.56-1.03H3a2 2 0 0 1 0-4h.09a1.7 1.7 0 0 0 1.56-1.03 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34h0a1.7 1.7 0 0 0 1.03-1.56V3a2 2 0 0 1 4 0v.09a1.7 1.7 0 0 0 1.03 1.56 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87v0a1.7 1.7 0 0 0 1.56 1.03H21a2 2 0 0 1 0 4h-.09a1.7 1.7 0 0 0-1.51 1z"/>'),
  search: svg('<circle cx="11" cy="11" r="7"/><path d="m21 21-4.35-4.35"/>'),
  arrowLeft: svg('<path d="M15 18l-6-6 6-6"/>'),
  arrowRight: svg('<path d="M9 6l6 6-6 6"/>'),
  external: svg('<path d="M14 4h6v6"/><path d="M20 4 10 14"/><path d="M20 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h5"/>'),
  trash: svg('<path d="M4 7h16"/><path d="M9 7V5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2"/><path d="M6.5 7l1 13a2 2 0 0 0 2 1.8h5a2 2 0 0 0 2-1.8l1-13"/>'),
  download: svg('<path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M4 19h16"/>'),
  check: svg('<path d="m5 12 5 5L20 7"/>'),
  x: svg('<path d="M6 6l12 12M18 6 6 18"/>'),
  shieldCheck: svg('<path d="M12 3 5 6v5c0 4.5 3 8.5 7 10 4-1.5 7-5.5 7-10V6l-7-3z"/><path d="m9.5 12 2 2 3.5-3.5"/>'),
  info: svg('<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/>'),
  wifiOff: svg('<path d="M2 8.5a15 15 0 0 1 6-3.2M12 5c2.5 0 4.8.7 6.8 1.9M22 8.5c-2-1.4-4.4-2.3-6.8-2.6M5 12a10 10 0 0 1 4.4-2.3M15.5 11c1.6.2 3.1.9 4.5 2M8.5 15.2a5.5 5.5 0 0 1 6-0.7M12 19h.01"/><path d="m3 3 18 18"/>'),
  sparkles: svg('<path d="M12 4l1.9 4.6L18.5 10l-4.6 1.9L12 16.5l-1.9-4.6L5.5 10l4.6-1.4L12 4z"/><path d="M19 14l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8.8-2z"/>'),
  clock: svg('<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>'),

  // Category glyphs
  globe: svg('<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3a13.5 13.5 0 0 1 0 18M12 3a13.5 13.5 0 0 0 0 18"/>'),
  chat: svg('<path d="M21 12a8 8 0 0 1-8 8H5l-2 2V12a8 8 0 0 1 8-8h2a8 8 0 0 1 8 8z" transform="translate(0 0)"/>'),
  play: svg('<rect x="3" y="5" width="18" height="14" rx="3"/><path d="m10.5 9.5 4.5 2.5-4.5 2.5v-5z" fill="currentColor" stroke="none"/>'),
  brush: svg('<path d="M14.5 4.5l5 5L9 20H4v-5l10.5-10.5z"/><path d="M12.5 6.5l5 5"/>'),
  doc: svg('<path d="M7 3h7l5 5v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z"/><path d="M14 3v5h5"/><path d="M9.5 13h5M9.5 17h5M9.5 9h1.5"/>'),
  code: svg('<path d="m8 8-5 4 5 4"/><path d="m16 8 5 4-5 4"/><path d="m13 5-2 14"/>'),
  gamepad: svg('<path d="M7 8h10a5 5 0 0 1 4.9 5.9l-.8 3.6a3.5 3.5 0 0 1-5.9 1.9L14 17H10l-2.1 2.4a3.5 3.5 0 0 1-5.9-1.9l-.8-3.6A5 5 0 0 1 7 8z"/><path d="M8.5 11v3M7 12.5h3"/><circle cx="15.3" cy="11.4" r="0.9" fill="currentColor" stroke="none"/><circle cx="17.7" cy="13.8" r="0.9" fill="currentColor" stroke="none"/>'),
  tool: svg('<path d="M14.7 6.3a4.5 4.5 0 0 0-6 5.6L3 17.6a2.1 2.1 0 1 0 3 3l5.7-5.7a4.5 4.5 0 0 0 5.6-6l-3 3-2.6-.6-.6-2.6 3.6-2.4z"/>'),
  shield: svg('<path d="M12 3 5 6v5c0 4.5 3 8.5 7 10 4-1.5 7-5.5 7-10V6l-7-3z"/>'),
};


export function icon(name: string, cls = ""): string {
  const body = icons[name] ?? icons["grid"]!;
  return cls ? body.replace("<svg ", `<svg class="${cls}" `) : body;
}
