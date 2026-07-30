import { useEffect } from "react";

import { useAppTitle } from "../queries/settings";

// Keeps the browser tab title in sync with the configured brand: the app title
// set in Settings ▸ General, falling back to the built-in brand for the active
// business mode. Uses the PUBLIC branding query so the title is correct even
// before login (a cached brand paints instantly; index.html's static title shows
// only until React hydrates). Renders nothing.
export default function DocumentTitle() {
  const title = useAppTitle();
  useEffect(() => {
    document.title = title;
  }, [title]);
  return null;
}
