"use client";

import { useState, SubmitEvent } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound } from "lucide-react";
import { authClient, clearApiToken } from "@/lib/better-auth/client";
import { INPUT_CLASS } from "./OrganizationFields";
import { authErrorMessage, Feedback, MIN_PASSWORD_LENGTH, PrimaryButton, Section } from "./ui";

export default function SecuritySection() {
  const { t } = useTranslation();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setSuccess("");
    if (newPassword.length < MIN_PASSWORD_LENGTH) {
      setError(t("signup.passwordTooShort", { min: MIN_PASSWORD_LENGTH }));
      return;
    }
    if (newPassword !== confirmPassword) {
      setError(t("signup.passwordMismatch"));
      return;
    }
    setLoading(true);
    // Signs out every other device; this one gets a new session cookie
    const { error } = await authClient.changePassword({ currentPassword, newPassword, revokeOtherSessions: true });
    setLoading(false);
    if (error) {
      setError(authErrorMessage(t, error));
      return;
    }
    clearApiToken();
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    setSuccess(t("profile.security.changed"));
  }

  return (
    <Section
      id="security"
      icon={KeyRound}
      title={t("profile.security.title")}
      description={t("profile.security.subtitle")}
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <label htmlFor="current-password" className="text-sm font-medium text-gray-700">
            {t("profile.security.currentPassword")}
          </label>
          <input
            id="current-password"
            type="password"
            autoComplete="current-password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            required
            className={INPUT_CLASS}
          />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="new-password" className="text-sm font-medium text-gray-700">
              {t("profile.security.newPassword")}
            </label>
            <input
              id="new-password"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              minLength={MIN_PASSWORD_LENGTH}
              aria-describedby="new-password-hint"
              className={INPUT_CLASS}
            />
            <p id="new-password-hint" className="text-xs text-gray-400">
              {t("signup.passwordHint", { min: MIN_PASSWORD_LENGTH })}
            </p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="confirm-new-password" className="text-sm font-medium text-gray-700">
              {t("signup.confirmPassword")}
            </label>
            <input
              id="confirm-new-password"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              className={INPUT_CLASS}
            />
          </div>
        </div>
        <p className="text-xs text-gray-500">{t("profile.security.otherSessions")}</p>
        <Feedback error={error} success={success} />
        <div>
          <PrimaryButton loading={loading}>
            {loading ? t("profile.saving") : t("profile.security.change")}
          </PrimaryButton>
        </div>
      </form>
    </Section>
  );
}
