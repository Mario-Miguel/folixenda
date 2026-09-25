import { cache } from "react";
import { cookies, headers } from "next/headers";
import { createInstance } from "i18next";
import { defaultLocale, i18nOptions, isLocale, LOCALE_COOKIE, type Locale } from "./settings";

// Cookie set by the language switcher, then the browser's Accept-Language, then the default
export const getLocale = cache(async (): Promise<Locale> => {
  const fromCookie = (await cookies()).get(LOCALE_COOKIE)?.value;
  if (isLocale(fromCookie)) return fromCookie;

  const acceptLanguage = (await headers()).get("accept-language") ?? "";
  const preferred = acceptLanguage
    .split(",")
    .map((part) => part.split(";")[0].trim().slice(0, 2).toLowerCase())
    .find(isLocale);
  return preferred ?? defaultLocale;
});

// Translation function for Server Components
export const getTranslation = cache(async () => {
  const locale = await getLocale();
  const i18n = createInstance();
  await i18n.init(i18nOptions(locale));
  return { t: i18n.t, locale };
});
