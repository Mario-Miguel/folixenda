"use client";

import { useState, SubmitEvent } from "react";
import { useTranslation } from "react-i18next";
import { Bell, Check, Info } from "lucide-react";
import { updateNotifications } from "@/lib/api/me";
import { CATEGORIES, categoryBadgeClass } from "@/lib/categories";
import { EventCategory, NotificationPreferences } from "@/lib/types";
import { apiErrorMessage, Feedback, PrimaryButton, Section, Toggle } from "./ui";

export default function NotificationsSection({
  initial,
  isPublisher,
}: {
  initial: NotificationPreferences;
  isPublisher: boolean;
}) {
  const { t } = useTranslation();
  const [prefs, setPrefs] = useState(initial);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  const set = <K extends keyof NotificationPreferences>(key: K, value: NotificationPreferences[K]) => {
    setSuccess("");
    setPrefs((p) => ({ ...p, [key]: value }));
  };

  function toggleCategory(category: EventCategory) {
    set(
      "categories",
      prefs.categories.includes(category)
        ? prefs.categories.filter((c) => c !== category)
        : [...prefs.categories, category]
    );
  }

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setSuccess("");
    setLoading(true);
    try {
      // publisherActivity is only meaningful for publishers; keep whatever is stored for the others
      setPrefs(await updateNotifications(prefs));
      setSuccess(t("profile.saved"));
    } catch (err) {
      setError(apiErrorMessage(t, err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Section
      id="notifications"
      icon={Bell}
      title={t("profile.notifications.title")}
      description={t("profile.notifications.subtitle")}
    >
      <p className="flex items-start gap-2 text-xs text-gray-500 bg-gray-50 rounded-lg px-3 py-2 mb-6">
        <Info className="w-4 h-4 shrink-0" aria-hidden="true" />
        {t("profile.notifications.comingSoon")}
      </p>

      <form onSubmit={handleSubmit} className="flex flex-col gap-5">
        <Toggle
          id="notif-saved"
          label={t("profile.notifications.savedEventReminders")}
          description={t("profile.notifications.savedEventRemindersHint")}
          checked={prefs.savedEventReminders}
          onChange={(v) => set("savedEventReminders", v)}
        />

        <div className="flex flex-col gap-3">
          <Toggle
            id="notif-categories"
            label={t("profile.notifications.categoryNews")}
            description={t("profile.notifications.categoryNewsHint")}
            checked={prefs.categoryNews}
            onChange={(v) => set("categoryNews", v)}
          />
          {prefs.categoryNews && (
            <fieldset>
              <legend className="text-xs font-medium text-gray-600 mb-2">
                {t("profile.notifications.favouriteCategories")}
              </legend>
              <div className="flex flex-wrap gap-2">
                {CATEGORIES.map((category) => {
                  const selected = prefs.categories.includes(category);
                  return (
                    <button
                      key={category}
                      type="button"
                      aria-pressed={selected}
                      onClick={() => toggleCategory(category)}
                      className={`inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold transition ring-offset-1 ${
                        selected ? `${categoryBadgeClass(category)} ring-2 ring-current` : "bg-gray-100 text-gray-500 hover:bg-gray-200"
                      }`}
                    >
                      {selected && <Check className="w-3 h-3" aria-hidden="true" />}
                      {t(`categories.${category}`)}
                    </button>
                  );
                })}
              </div>
              {prefs.categories.length === 0 && (
                <p className="text-xs text-gray-400 mt-2">{t("profile.notifications.noCategories")}</p>
              )}
            </fieldset>
          )}
        </div>

        <Toggle
          id="notif-newsletter"
          label={t("profile.notifications.newsletter")}
          description={t("profile.notifications.newsletterHint")}
          checked={prefs.newsletter}
          onChange={(v) => set("newsletter", v)}
        />

        {isPublisher && (
          <Toggle
            id="notif-publisher"
            label={t("profile.notifications.publisherActivity")}
            description={t("profile.notifications.publisherActivityHint")}
            checked={prefs.publisherActivity}
            onChange={(v) => set("publisherActivity", v)}
          />
        )}

        <Feedback error={error} success={success} />
        <div>
          <PrimaryButton loading={loading}>{loading ? t("profile.saving") : t("profile.save")}</PrimaryButton>
        </div>
      </form>
    </Section>
  );
}
