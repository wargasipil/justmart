import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { useBranding } from "../queries/settings";

// Keeps the browser tab title in sync with the business-mode brand: the licensed
// shop name in pharmacy mode (apotech-style; falls back to the localized
// "Apotek"/"Pharmacy" label), else the "Justmart" retail brand. Mirrors the
// Sidebar top-left brand. Uses the PUBLIC branding query so the title is correct
// even before login (a cached brand paints instantly; index.html's static title
// shows only until React hydrates). Renders nothing.
export default function DocumentTitle() {
  const { isPharmacy, shopName } = useBranding();
  const { t } = useTranslation();
  useEffect(() => {
    document.title = isPharmacy ? shopName || t("app.pharmacyName") : t("app.name");
  }, [isPharmacy, shopName, t]);
  return null;
}
