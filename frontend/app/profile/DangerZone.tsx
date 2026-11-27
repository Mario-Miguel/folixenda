"use client";

import { useState, SubmitEvent } from "react";
import { useTranslation } from "react-i18next";
import { Trash2 } from "lucide-react";
import { authClient, clearApiToken } from "@/lib/better-auth/client";
import { INPUT_CLASS } from "./OrganizationFields";
import { authErrorMessage, Feedback, Section } from "./ui";

export default function DangerZone({ isPublisher }: { isPublisher: boolean }) {
  const { t } = useTranslation();
  const confirmWord = t("profile.danger.confirmWord");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const confirmed = confirmation.trim().toLocaleUpperCase() === confirmWord.toLocaleUpperCase();

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!confirmed || !password) return;
    setError("");
    setLoading(true);
    // Future: once publishing is paid, cancel the publisher's subscription before this
    // (server side, in Better Auth's user.deleteUser.beforeDelete).
    // Better Auth verifies the password, deletes the user, its sessions and accounts and
    // clears the cookie; Postgres cascades the delete to everything the Go API stores.
    const { error } = await authClient.deleteUser({ password });
    if (error) {
      setError(authErrorMessage(t, error));
      setLoading(false);
      return;
    }
    clearApiToken();
    // Full reload so no client state (useSession, cached pages) survives the deleted account
    window.location.assign("/");
  }

  return (
    <Section
      id="danger"
      icon={Trash2}
      tone="danger"
      title={t("profile.danger.title")}
      description={t("profile.danger.subtitle")}
    >
      <div className="text-sm text-gray-600 mb-5">
        <p className="mb-2">{t("profile.danger.whatIsDeleted")}</p>
        <ul className="list-disc pl-5 space-y-1">
          <li>{t("profile.danger.items.account")}</li>
          <li>{t("profile.danger.items.saved")}</li>
          <li>{t("profile.danger.items.preferences")}</li>
          {isPublisher && <li className="font-semibold text-red-700">{t("profile.danger.items.publisher")}</li>}
        </ul>
        <p className="mt-3 font-medium text-gray-900">{t("profile.danger.irreversible")}</p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="delete-password" className="text-sm font-medium text-gray-700">
              {t("login.password")}
            </label>
            <input
              id="delete-password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className={INPUT_CLASS}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="delete-confirm" className="text-sm font-medium text-gray-700">
              {t("profile.danger.typeToConfirm", { word: confirmWord })}
            </label>
            <input
              id="delete-confirm"
              type="text"
              autoComplete="off"
              spellCheck={false}
              value={confirmation}
              onChange={(e) => setConfirmation(e.target.value)}
              required
              className={INPUT_CLASS}
            />
          </div>
        </div>
        <Feedback error={error} />
        <div>
          <button
            type="submit"
            disabled={!confirmed || !password || loading}
            aria-busy={loading}
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold text-white bg-red-600 hover:bg-red-700 transition disabled:opacity-50 disabled:hover:bg-red-600"
          >
            <Trash2 className="w-4 h-4" aria-hidden="true" />
            {loading ? t("profile.danger.deleting") : t("profile.danger.delete")}
          </button>
        </div>
      </form>
    </Section>
  );
}
