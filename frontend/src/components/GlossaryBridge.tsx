import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import en from "../locales/en.json";
import id from "../locales/id.json";
import { useAuth } from "../lib/auth";
import { useBusinessMode } from "../queries/settings";

// Source glossaries straight from the bundled locale JSON: `glossary` is the
// retail/default catalog noun (referenced by every product-noun string via
// i18next `$t(glossary.*)` nesting); `glossaryPharmacy` is the pharmacy override.
const BUNDLES: Record<string, { glossary: object; glossaryPharmacy: object }> = {
  en: { glossary: en.glossary, glossaryPharmacy: en.glossaryPharmacy },
  id: { glossary: id.glossary, glossaryPharmacy: id.glossaryPharmacy },
};

// GlossaryBridge swaps the catalog-noun glossary by business mode (Product →
// Medicine / Produk → Obat in pharmacy mode, restored to the file default in
// retail). It overwrites the `glossary` resource bundle so all
// `$t(glossary.*)` references resolve mode-aware. i18n.ts sets
// `react.bindI18nStore: "added"`, so the overwrite re-renders every translation
// consumer. Mode is license-driven (changes only on restart) — this fires once
// the mode query resolves; a brief flash of the retail noun on cold load is
// acceptable (same posture as the theme flash). Renders nothing.
export default function GlossaryBridge() {
  const { user } = useAuth();
  const { isPharmacy } = useBusinessMode(!!user); // skip the authed RPC pre-login
  const { i18n } = useTranslation();
  useEffect(() => {
    for (const lng of Object.keys(BUNDLES)) {
      const b = BUNDLES[lng];
      i18n.addResourceBundle(
        lng,
        "translation",
        { glossary: isPharmacy ? b.glossaryPharmacy : b.glossary },
        true, // deep merge
        true, // overwrite existing keys
      );
    }
  }, [isPharmacy, i18n]);
  return null;
}
