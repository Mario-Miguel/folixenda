import type { InitOptions } from "i18next";
import en from "./locales/en.json";
import es from "./locales/es.json";

export const locales = ["es", "en"] as const;
export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "es";
export const LOCALE_COOKIE = "NEXT_LOCALE";

export const resources = {
  es: { translation: es },
  en: { translation: en },
} as const;

export function isLocale(value: string | undefined | null): value is Locale {
  return locales.includes(value as Locale);
}

// Shared by the server and client instances so both render identical markup
export function i18nOptions(lng: Locale): InitOptions {
  return {
    lng,
    resources,
    fallbackLng: defaultLocale,
    supportedLngs: [...locales],
    interpolation: { escapeValue: false }, // React already escapes
    initAsync: false, // resources are bundled, so init synchronously
  };
}
