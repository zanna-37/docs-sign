import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./en";
import it from "./it";

export const SUPPORTED = ["en", "it"] as const;
export type Lang = (typeof SUPPORTED)[number];

// browserLang maps the browser's language to a supported one (default English).
export function browserLang(): Lang {
  const nav = (navigator.language || "en").toLowerCase();
  return nav.startsWith("it") ? "it" : "en";
}

void i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    it: { translation: it },
  },
  lng: browserLang(),
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  returnNull: false,
});

// applyLanguage applies a stored preference; "" / "auto" follows the browser.
export function applyLanguage(pref: string | undefined | null) {
  const lang: Lang = pref === "en" || pref === "it" ? pref : browserLang();
  if (i18n.language !== lang) {
    void i18n.changeLanguage(lang);
  }
}

export default i18n;
