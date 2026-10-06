import { useEffect } from "react";

import { useAppTitle, useBranding } from "../queries/settings";

// Keeps the browser tab in sync with the configured brand: the TITLE is the app
// title set in Settings ▸ General, falling back to the built-in brand for the
// active business mode, and the ICON is the same mark the sidebar shows (a
// store in retail, a pill in pharmacy mode). Uses the PUBLIC branding query so
// both are correct even before login (a cached brand paints instantly;
// index.html's static title and retail icon show only until React hydrates).
// Renders nothing.
export default function DocumentTitle() {
  const title = useAppTitle();
  const { isPharmacy } = useBranding();
  useEffect(() => {
    document.title = title;
  }, [title]);
  useEffect(() => {
    const icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (icon) icon.href = isPharmacy ? "/favicon-pharmacy.svg" : "/favicon.svg";
  }, [isPharmacy]);
  return null;
}
