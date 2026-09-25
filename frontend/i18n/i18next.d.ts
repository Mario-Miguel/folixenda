import "i18next";
import type es from "./locales/es.json";

// Typed keys: t("missing.key") fails the type-check
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "translation";
    resources: { translation: typeof es };
  }
}
