// The two device classes Justmart is designed for, as Storybook viewports.
//
// Every story under screens/desktop/* and screens/mobile/* pins one of these
// through `globals`, so the story IS that version of the app — the viewport
// toolbar is locked on it rather than left to whatever was last selected.
// Chakra's responsive props read real media queries inside the resized
// iframe, so a mobile story exercises the same breakpoints a phone does.
//
// Widths: 1440 is a common shop-counter monitor; 390 is a current mid-size
// phone (iPhone 12–15 / most Androids fall within ±30px of it).

export const VIEWPORTS = {
  desktop: {
    name: "Justmart desktop (1440×900)",
    styles: { width: "1440px", height: "900px" },
    type: "desktop",
  },
  mobile: {
    name: "Justmart mobile (390×844)",
    styles: { width: "390px", height: "844px" },
    type: "mobile",
  },
} as const;

export const DESKTOP = { viewport: { value: "desktop", isRotated: false } };
export const MOBILE = { viewport: { value: "mobile", isRotated: false } };

/**
 * Canvas padding for a page-only story. The preview decorator reads
 * `parameters.canvasPadding`; these mirror AppShell's <main> gutter at each
 * breakpoint, so a page reads the same with or without the shell around it.
 */
export const DESKTOP_PAGE = { canvasPadding: 6 };
export const MOBILE_PAGE = { canvasPadding: 3 };
/** A full-app screen draws its own chrome edge to edge. */
export const SCREEN = { canvasPadding: 0 };
