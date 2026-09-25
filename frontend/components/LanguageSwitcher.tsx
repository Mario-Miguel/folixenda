"use client";

import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { setLocale } from "@/i18n/actions";
import { locales } from "@/i18n/settings";

export default function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function change(locale: string) {
    startTransition(async () => {
      await setLocale(locale);
      router.refresh();
    });
  }

  return (
    <div
      role="group"
      aria-label={t("nav.language")}
      className={`flex rounded-lg border border-gray-200 overflow-hidden text-xs font-semibold ${pending ? "opacity-60" : ""}`}
    >
      {locales.map((locale) => {
        const active = i18n.language === locale;
        return (
          <button
            key={locale}
            onClick={() => change(locale)}
            disabled={active || pending}
            aria-pressed={active}
            className={`px-2 py-1 uppercase transition-colors ${active ? "text-white" : "text-gray-500 hover:bg-gray-50"}`}
            style={active ? { backgroundColor: "#ec5b13" } : undefined}
          >
            {locale}
          </button>
        );
      })}
    </div>
  );
}
