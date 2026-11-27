"use client";

import { useState, SubmitEvent } from "react";
import { useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { AlertTriangle, Building2 } from "lucide-react";
import { savePublisherProfile } from "@/lib/api/me";
import { PublisherProfile } from "@/lib/types";
import OrganizationFields, { OrgErrors, orgValuesFrom, toProfileInput, useOrgValidation } from "./OrganizationFields";
import { apiErrorMessage, Feedback, PrimaryButton, Section } from "./ui";

export default function OrganizationSection({
  profile,
  highlight,
}: {
  profile: PublisherProfile | null;
  highlight: boolean; // came here because saving it right after sign-up failed
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const validate = useOrgValidation();
  const [values, setValues] = useState(orgValuesFrom(profile));
  const [errors, setErrors] = useState<OrgErrors>({});
  const [saved, setSaved] = useState(profile !== null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setSuccess("");
    const found = validate(values);
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    setLoading(true);
    try {
      const updated = await savePublisherProfile(toProfileInput(values));
      setValues(orgValuesFrom(updated));
      setSaved(true);
      setSuccess(t("profile.saved"));
      router.refresh();
    } catch (err) {
      setError(apiErrorMessage(t, err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Section
      id="organization"
      icon={Building2}
      title={t("profile.org.title")}
      description={t("profile.org.subtitle")}
    >
      {!saved && (
        <div
          role={highlight ? "alert" : undefined}
          className="flex items-start gap-3 rounded-xl border px-4 py-3 mb-6 text-sm bg-orange-50"
          style={{ borderColor: "#ec5b13" }}
        >
          <AlertTriangle className="w-5 h-5 shrink-0" style={{ color: "#ec5b13" }} aria-hidden="true" />
          <div>
            <p className="font-semibold text-gray-900">{t("profile.org.incompleteTitle")}</p>
            <p className="text-gray-600 mt-0.5">
              {highlight ? t("profile.org.incompleteAfterSignup") : t("profile.org.incompleteBody")}
            </p>
          </div>
        </div>
      )}

      <form onSubmit={handleSubmit} className="flex flex-col gap-5">
        <OrganizationFields values={values} errors={errors} onChange={setValues} idPrefix="profile-org" />
        <Feedback error={error} success={success} />
        <div>
          <PrimaryButton loading={loading}>{loading ? t("profile.saving") : t("profile.save")}</PrimaryButton>
        </div>
      </form>
    </Section>
  );
}
