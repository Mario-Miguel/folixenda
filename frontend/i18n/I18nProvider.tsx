"use client";

import { useMemo } from "react";
import { createInstance } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { i18nOptions, type Locale } from "./settings";

// The locale is decided on the server (see i18n/server.ts); a switch sets the cookie and
// refreshes, which hands a new `locale` here and rebuilds the instance
export default function I18nProvider({ locale, children }: { locale: Locale; children: React.ReactNode }) {
  const i18n = useMemo(() => {
    const instance = createInstance();
    instance.use(initReactI18next).init(i18nOptions(locale));
    return instance;
  }, [locale]);

  return <I18nextProvider i18n={i18n}>{children}</I18nextProvider>;
}
