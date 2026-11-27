"use client";

import { useTranslation } from "react-i18next";
import { PublisherProfile, PublisherProfileInput } from "@/lib/types";

// Client-side mirror of the Go API's validation for PUT /api/me/publisher-profile
export const ORG_NAME_MIN = 2;
export const ORG_NAME_MAX = 120;
export const PHONE_MAX = 30;
export const DESCRIPTION_MAX = 1000;
const PHONE_PATTERN = /^[0-9 +\-().]*$/;

export const INPUT_CLASS =
  "border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition";

export interface OrgValues {
  orgName: string;
  website: string;
  phone: string;
  description: string;
}

export type OrgErrors = Partial<Record<keyof OrgValues, string>>;

export function orgValuesFrom(profile: PublisherProfile | null | undefined): OrgValues {
  return {
    orgName: profile?.orgName ?? "",
    website: profile?.website ?? "",
    phone: profile?.phone ?? "",
    description: profile?.description ?? "",
  };
}

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return (url.protocol === "http:" || url.protocol === "https:") && url.hostname !== "";
  } catch {
    return false;
  }
}

// Returns translation-ready error messages, empty when the values are valid
export function useOrgValidation() {
  const { t } = useTranslation();
  return (values: OrgValues): OrgErrors => {
    const errors: OrgErrors = {};
    const orgName = values.orgName.trim();
    if (orgName.length < ORG_NAME_MIN || orgName.length > ORG_NAME_MAX)
      errors.orgName = t("profile.org.errors.orgName", { min: ORG_NAME_MIN, max: ORG_NAME_MAX });
    const website = values.website.trim();
    if (website && !isHttpUrl(website)) errors.website = t("profile.org.errors.website");
    const phone = values.phone.trim();
    if (phone.length > PHONE_MAX || !PHONE_PATTERN.test(phone))
      errors.phone = t("profile.org.errors.phone", { max: PHONE_MAX });
    if (values.description.trim().length > DESCRIPTION_MAX)
      errors.description = t("profile.org.errors.description", { max: DESCRIPTION_MAX });
    return errors;
  };
}

// Empty optional fields are left out of the request
export function toProfileInput(values: OrgValues): PublisherProfileInput {
  const optional = (value: string) => value.trim() || undefined;
  return {
    orgName: values.orgName.trim(),
    website: optional(values.website),
    phone: optional(values.phone),
    description: optional(values.description),
  };
}

interface Props {
  values: OrgValues;
  errors: OrgErrors;
  onChange: (values: OrgValues) => void;
  idPrefix?: string;
}

// The organization inputs shared by the sign-up form and the profile page
export default function OrganizationFields({ values, errors, onChange, idPrefix = "org" }: Props) {
  const { t } = useTranslation();
  const set = (key: keyof OrgValues) => (e: { target: { value: string } }) =>
    onChange({ ...values, [key]: e.target.value });

  const field = (key: keyof OrgValues) => ({
    id: `${idPrefix}-${key}`,
    value: values[key],
    onChange: set(key),
    "aria-invalid": errors[key] ? true : undefined,
    "aria-describedby": errors[key] ? `${idPrefix}-${key}-error` : undefined,
  });

  const errorText = (key: keyof OrgValues) =>
    errors[key] && (
      <p id={`${idPrefix}-${key}-error`} className="text-xs text-red-600">
        {errors[key]}
      </p>
    );

  return (
    <>
      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${idPrefix}-orgName`} className="text-sm font-medium text-gray-700">
          {t("profile.org.orgName")} <span className="text-red-500" aria-hidden="true">*</span>
        </label>
        <input
          {...field("orgName")}
          type="text"
          autoComplete="organization"
          required
          minLength={ORG_NAME_MIN}
          maxLength={ORG_NAME_MAX}
          className={INPUT_CLASS}
        />
        {errorText("orgName")}
      </div>

      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${idPrefix}-website`} className="text-sm font-medium text-gray-700">
          {t("profile.org.website")} <span className="font-normal text-gray-400">{t("profile.optional")}</span>
        </label>
        <input
          {...field("website")}
          type="url"
          autoComplete="url"
          placeholder="https://"
          className={INPUT_CLASS}
        />
        {errorText("website")}
      </div>

      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${idPrefix}-phone`} className="text-sm font-medium text-gray-700">
          {t("profile.org.phone")} <span className="font-normal text-gray-400">{t("profile.optional")}</span>
        </label>
        <input {...field("phone")} type="tel" autoComplete="tel" maxLength={PHONE_MAX} className={INPUT_CLASS} />
        {errorText("phone")}
      </div>

      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${idPrefix}-description`} className="text-sm font-medium text-gray-700">
          {t("profile.org.description")} <span className="font-normal text-gray-400">{t("profile.optional")}</span>
        </label>
        <textarea {...field("description")} rows={4} maxLength={DESCRIPTION_MAX} className={INPUT_CLASS} />
        <p className="text-xs text-gray-400 text-right">
          {values.description.length}/{DESCRIPTION_MAX}
        </p>
        {errorText("description")}
      </div>
    </>
  );
}
