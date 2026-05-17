import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import { localeManager } from "@/localeManager";

// TASK-W-029: Load translations from the shared Vue locale source.
// Only React-specific (dynamic) keys are kept in separate files.
import sharedEnUS from "@/locales/en-US.json";
import sharedZhCN from "@/locales/zh-CN.json";
import sharedEsES from "@/locales/es-ES.json";
import sharedJaJP from "@/locales/ja-JP.json";
import sharedViVN from "@/locales/vi-VN.json";

// React-only dynamic keys (~10-15KB each instead of ~200KB duplicates)
import reactEnUS from "@/react/locales/dynamic/en-US.json";
import reactZhCN from "@/react/locales/dynamic/zh-CN.json";
import reactEsES from "@/react/locales/dynamic/es-ES.json";
import reactJaJP from "@/react/locales/dynamic/ja-JP.json";
import reactViVN from "@/react/locales/dynamic/vi-VN.json";

const STORAGE_KEY_LANGUAGE = "bb.language";

function getLocale(): string {
  const stored = localStorage.getItem(STORAGE_KEY_LANGUAGE) ?? "";
  if (stored) {
    try {
      const parsed = JSON.parse(stored);
      if (typeof parsed === "string" && parsed) return parsed;
    } catch {
      if (stored) return stored;
    }
  }
  const nav = navigator.language;
  const mapping: Record<string, string> = {
    en: "en-US",
    ja: "ja-JP",
    es: "es-ES",
    vi: "vi-VN",
  };
  return mapping[nav] ?? (nav.includes("-") ? nav : "en-US");
}

// Merge shared (Vue-compatible) translations with React-specific dynamic keys
const resources = {
  "en-US": {
    translation: sharedEnUS,
    react: reactEnUS,
  },
  "zh-CN": {
    translation: sharedZhCN,
    react: reactZhCN,
  },
  "es-ES": {
    translation: sharedEsES,
    react: reactEsES,
  },
  "ja-JP": {
    translation: sharedJaJP,
    react: reactJaJP,
  },
  "vi-VN": {
    translation: sharedViVN,
    react: reactViVN,
  },
};

const i18n: import("i18next").i18n = i18next.createInstance();

export const i18nReady = i18n.use(initReactI18next).init({
  resources,
  lng: getLocale(),
  fallbackLng: "en-US",
  ns: ["translation", "react"],
  defaultNS: "translation",
  interpolation: {
    escapeValue: false,
  },
  initImmediate: false,
}).then(() => {
  localeManager.subscribe(async (newLocale) => {
    if (i18next.language !== newLocale) {
      await i18next.changeLanguage(newLocale);
    }
  });
});

export default i18n;

