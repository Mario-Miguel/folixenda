"use client";

import { ReactNode } from "react";
import { TFunction } from "i18next";
import { LucideIcon } from "lucide-react";
import { ApiError } from "@/lib/api/api";

// Must match Better Auth's minPasswordLength (default 8)
export const MIN_PASSWORD_LENGTH = 8;

export function Section({
  id,
  icon: Icon,
  title,
  description,
  children,
  tone = "default",
}: {
  id: string;
  icon: LucideIcon;
  title: string;
  description?: string;
  children: ReactNode;
  tone?: "default" | "danger";
}) {
  const danger = tone === "danger";
  return (
    <section
      aria-labelledby={`${id}-title`}
      className={`bg-white rounded-2xl shadow-sm border p-6 sm:p-8 ${danger ? "border-red-200" : "border-gray-100"}`}
    >
      <div className="flex items-start gap-3 mb-6">
        <div
          className={`w-9 h-9 rounded-full flex items-center justify-center shrink-0 ${danger ? "bg-red-50" : "bg-orange-50"}`}
        >
          <Icon className="w-5 h-5" style={{ color: danger ? "#dc2626" : "#ec5b13" }} aria-hidden="true" />
        </div>
        <div>
          <h2 id={`${id}-title`} className={`text-lg font-bold ${danger ? "text-red-700" : "text-gray-900"}`}>
            {title}
          </h2>
          {description && <p className="text-sm text-gray-500 mt-0.5">{description}</p>}
        </div>
      </div>
      {children}
    </section>
  );
}

export function Feedback({ error, success }: { error?: string; success?: string }) {
  if (error)
    return (
      <p role="alert" className="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
        {error}
      </p>
    );
  if (success)
    return (
      <p role="status" className="text-sm text-green-700 bg-green-50 rounded-lg px-3 py-2">
        {success}
      </p>
    );
  return null;
}

export function PrimaryButton({ loading, children, disabled }: { loading: boolean; children: ReactNode; disabled?: boolean }) {
  return (
    <button
      type="submit"
      disabled={loading || disabled}
      aria-busy={loading}
      className="px-5 py-2.5 rounded-lg text-sm font-semibold text-white transition disabled:opacity-60"
      style={{ backgroundColor: "#ec5b13" }}
    >
      {children}
    </button>
  );
}

export function Toggle({
  id,
  label,
  description,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  description?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <p id={`${id}-label`} className="text-sm font-medium text-gray-900">
          {label}
        </p>
        {description && (
          <p id={`${id}-description`} className="text-xs text-gray-500 mt-0.5">
            {description}
          </p>
        )}
      </div>
      <button
        type="button"
        role="switch"
        id={id}
        aria-checked={checked}
        aria-labelledby={`${id}-label`}
        aria-describedby={description ? `${id}-description` : undefined}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-orange-200 ${
          checked ? "" : "bg-gray-200"
        }`}
        style={checked ? { backgroundColor: "#ec5b13" } : undefined}
      >
        <span
          aria-hidden="true"
          className={`inline-block h-5 w-5 rounded-full bg-white shadow transition-transform ${
            checked ? "translate-x-5" : "translate-x-0.5"
          }`}
        />
      </button>
    </div>
  );
}

// Errors from the Go API (axios interceptor wraps them in ApiError)
export function apiErrorMessage(t: TFunction, err: unknown): string {
  const status = err instanceof ApiError ? err.status : 0;
  switch (status) {
    case 401:
      return t("profile.errors.unauthorized");
    case 403:
      return t("profile.errors.forbidden");
    case 400:
      return t("profile.errors.badRequest");
    default:
      return t("profile.errors.generic");
  }
}

// Errors returned by Better Auth's client ({ status, code })
export function authErrorMessage(t: TFunction, error: { status?: number; code?: string }): string {
  switch (error.code) {
    case "INVALID_PASSWORD":
      return t("profile.errors.wrongPassword");
    case "PASSWORD_TOO_SHORT":
      return t("signup.passwordTooShort", { min: MIN_PASSWORD_LENGTH });
    case "PASSWORD_TOO_LONG":
      return t("profile.errors.passwordTooLong");
    case "SESSION_EXPIRED":
      return t("profile.errors.unauthorized");
  }
  if (error.status === 401) return t("profile.errors.unauthorized");
  if (error.status === 403) return t("profile.errors.forbidden");
  if (error.status === 400) return t("profile.errors.badRequest");
  return t("profile.errors.generic");
}
