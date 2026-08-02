import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import en from "../locales/en.json";
import id from "../locales/id.json";
import { useAuth } from "../lib/auth";
import { useBusinessMode } from "../queries/settings";

// Source glossaries straight from the bundled locale JSON: `glossary` is the
// retail/default catalog noun (referenced by every product-noun string via
// i18next `$t(glossary.*)` nesting); `glossaryPharmacy` and `glossaryRestaurant`
// are the per-mode overrides (Product → Medicine / Menu item).
type Glossaries = {
  glossary: object;
  glossaryPharmacy: object;
  glossaryRestaurant: object;
};

const BUNDLES: Record<string, Glossaries> = {
  en: {
    glossary: en.glossary,
    glossaryPharmacy: en.glossaryPharmacy,
    glossaryRestaurant: en.glossaryRestaurant,
  },
  id: {
    glossary: id.glossary,
    glossaryPharmacy: id.glossaryPharmacy,
    glossaryRestaurant: id.glossaryRestaurant,
  },
};

// GlossaryBridge swaps the catalog-noun glossary by business mode (Product →
// Medicine/Obat in pharmacy, Product → Menu item/Menu in restaurant, restored to
// the file default in retail). It overwrites the `glossary` resource bundle so all
// `$t(glossary.*)` references resolve mode-aware. i18n.ts sets
// `react.bindI18nStore: "added"`, so the overwrite re-renders every translation
// consumer. Mode comes from Settings ▸ General (the save invalidates the mode
// query, so this re-fires live) — a brief flash of the retail noun on cold load
// is acceptable (same posture as the theme flash). Renders nothing.
export default function GlossaryBridge() {
  const { user } = useAuth();
  const { isPharmacy, isRestaurant } = useBusinessMode(!!user); // skip the authed RPC pre-login
  const { i18n } = useTranslation();
  useEffect(() => {
    for (const lng of Object.keys(BUNDLES)) {
      const b = BUNDLES[lng];
      const glossary = isPharmacy
        ? b.glossaryPharmacy
        : isRestaurant
          ? b.glossaryRestaurant
          : b.glossary;
      i18n.addResourceBundle(
        lng,
        "translation",
        { glossary },
        true, // deep merge
        true, // overwrite existing keys
      );
    }
  }, [isPharmacy, isRestaurant, i18n]);
  return null;
}
