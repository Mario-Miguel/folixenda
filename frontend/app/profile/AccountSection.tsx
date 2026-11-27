"use client";

import { useState, SubmitEvent } from "react";
import { useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { UserCircle } from "lucide-react";
import { authClient, clearApiToken } from "@/lib/better-auth/client";
import { INPUT_CLASS } from "./OrganizationFields";
import { authErrorMessage, Feedback, PrimaryButton, Section } from "./ui";

type Role = "admin" | "consumer" | "publisher";

const ROLE_BADGE: Record<Role, string> = {
  consumer: "bg-blue-50 text-blue-700",
  publisher: "bg-orange-50 text-orange-700",
  admin: "bg-gray-900 text-white",
};

const ROLE_LABEL = {
  consumer: "profile.account.roles.consumer",
  publisher: "profile.account.roles.publisher",
  admin: "profile.account.roles.admin",
} as const;

export default function AccountSection({
  name: initialName,
  email,
  role,
  memberSince,
}: {
  name: string;
  email: string;
  role: Role;
  memberSince: string;
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const [name, setName] = useState(initialName);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const trimmed = name.trim();

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setSuccess("");
    if (!trimmed) {
      setError(t("profile.account.nameRequired"));
      return;
    }
    setLoading(true);
    const { error } = await authClient.updateUser({ name: trimmed });
    setLoading(false);
    if (error) {
      setError(authErrorMessage(t, error));
      return;
    }
    // The JWT carries the name: get a new one so the Go profile picks it up
    clearApiToken();
    setSuccess(t("profile.saved"));
    router.refresh();
  }

  return (
    <Section id="account" icon={UserCircle} title={t("profile.account.title")}>
      <dl className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6 text-sm">
        <div>
          <dt className="text-gray-500">{t("login.email")}</dt>
          <dd className="font-medium text-gray-900 break-all mt-0.5">{email}</dd>
        </div>
        <div>
          <dt className="text-gray-500">{t("profile.account.type")}</dt>
          <dd className="mt-1">
            <span className={`inline-block px-2.5 py-0.5 rounded-full text-xs font-semibold ${ROLE_BADGE[role]}`}>
              {t(ROLE_LABEL[role])}
            </span>
            {role !== "admin" && <p className="mt-1.5 text-xs text-gray-500">{t("profile.account.typeFixed")}</p>}
          </dd>
        </div>
        <div>
          <dt className="text-gray-500">{t("profile.account.memberSince")}</dt>
          <dd className="font-medium text-gray-900 mt-0.5">{memberSince}</dd>
        </div>
      </dl>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <label htmlFor="account-name" className="text-sm font-medium text-gray-700">
            {t("signup.name")}
          </label>
          <input
            id="account-name"
            type="text"
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className={INPUT_CLASS}
          />
          <p className="text-xs text-gray-400">{t("profile.account.emailReadOnly")}</p>
        </div>
        <Feedback error={error} success={success} />
        <div>
          <PrimaryButton loading={loading} disabled={trimmed === initialName}>
            {loading ? t("profile.saving") : t("profile.save")}
          </PrimaryButton>
        </div>
      </form>
    </Section>
  );
}
