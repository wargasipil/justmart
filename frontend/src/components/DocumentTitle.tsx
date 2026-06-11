import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { useAuth } from "../lib/auth";
import { useBusinessMode } from "../queries/settings";

// Keeps the browser tab title in sync with the business-mode brand: the licensed
// shop name in pharmacy mode (apotech-style; falls back to the localized
// "Apotek"/"Pharmacy" label), else the "Justmart" retail brand. Mirrors the
// Sidebar top-left brand. The mode query is auth-gated (skipped pre-login, so the
// title stays the static index.html "Justmart" until the mode resolves).
// Renders nothing.
export default function DocumentTitle() {
  const { user } = useAuth();
  const { isPharmacy, shopName } = useBusinessMode(!!user);
  const { t } = useTranslation();
  useEffect(() => {
    document.title = isPharmacy ? shopName || t("app.pharmacyName") : t("app.name");
  }, [isPharmacy, shopName, t]);
  return null;
}
