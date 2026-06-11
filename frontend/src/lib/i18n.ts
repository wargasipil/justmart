import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { initReactI18next } from "react-i18next";

import en from "../locales/en.json";
import id from "../locales/id.json";

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      id: { translation: id },
    },
    fallbackLng: "id",
    supportedLngs: ["en", "id"],
    interpolation: { escapeValue: false },
    detection: {
      order: ["localStorage", "navigator"],
      lookupLocalStorage: "justmart_lang",
      caches: ["localStorage"],
    },
    // Re-render translation consumers when a resource bundle is overwritten at
    // runtime — the mode-aware glossary swap (catalog noun: Product vs Obat) in
    // <GlossaryBridge> overrides the `glossary` bundle once the business mode
    // resolves; without this, nested `$t(glossary.*)` strings wouldn't refresh.
    react: { bindI18nStore: "added" },
  });

export default i18n;
