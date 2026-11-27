"use client";

import { useState, SubmitEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { Building2, Ticket, User as UserIcon } from "lucide-react";
import { clearApiToken, signUp } from "@/lib/better-auth/client";
import { safeRedirect } from "@/lib/better-auth/redirect";
import { savePublisherProfile } from "@/lib/api/me";
import OrganizationFields, {
  INPUT_CLASS,
  OrgErrors,
  orgValuesFrom,
  toProfileInput,
  useOrgValidation,
} from "@/app/profile/OrganizationFields";

// Must match Better Auth's minPasswordLength (default 8)
const MIN_PASSWORD_LENGTH = 8;

// Read by databaseHooks.user.create.before in lib/better-auth/server.ts, which is
// the only place that decides the role. The client never sends `role` itself.
type AccountType = "consumer" | "publisher";

const ACCOUNT_TYPES = [
  { value: "consumer", icon: UserIcon, title: "signup.accountType.consumer", hint: "signup.accountType.consumerHint" },
  { value: "publisher", icon: Building2, title: "signup.accountType.publisher", hint: "signup.accountType.publisherHint" },
] as const;

export default function SignupPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const validateOrg = useOrgValidation();
  const [accountType, setAccountType] = useState<AccountType>("consumer");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [org, setOrg] = useState(orgValuesFrom(null));
  const [orgErrors, setOrgErrors] = useState<OrgErrors>({});
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const isPublisher = accountType === "publisher";

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    if (password !== confirmPassword) {
      setError(t("signup.passwordMismatch"));
      return;
    }
    if (isPublisher) {
      const errors = validateOrg(org);
      setOrgErrors(errors);
      if (Object.keys(errors).length > 0) return;
    }

    setLoading(true);
    // Better Auth signs the new user in automatically. `accountType` isn't a user field, so it
    // goes through fetchOptions.body (typed as Record<string, any>), which the client merges
    // into the JSON body; /sign-up/email's body schema accepts extra keys.
    const { error } = await signUp.email({
      name: name.trim(),
      email: email.trim(),
      password,
      fetchOptions: { body: { accountType } },
    });
    if (error) {
      setError(signupErrorMessage(error.code));
      setLoading(false);
      return;
    }

    // Drop any JWT cached for a previous session so the next API call gets one for this user
    clearApiToken();

    if (!isPublisher) {
      router.push(safeRedirect(new URLSearchParams(window.location.search).get("redirect")));
      router.refresh();
      return;
    }

    // Future: publishing will be paid. This is where the checkout step would go,
    // before (or right after) saving the organization data.
    try {
      await savePublisherProfile(toProfileInput(org));
      router.push("/publisher");
    } catch (err) {
      // The account exists already: let them finish the organization data from their profile
      console.error(err);
      router.push("/profile?completeOrg=1");
    }
    router.refresh();
  }

  function signupErrorMessage(code: string | undefined) {
    switch (code) {
      case "USER_ALREADY_EXISTS":
      case "USER_ALREADY_EXISTS_USE_ANOTHER_EMAIL":
        return t("signup.emailTaken");
      case "PASSWORD_TOO_SHORT":
        return t("signup.passwordTooShort", { min: MIN_PASSWORD_LENGTH });
      case "INVALID_EMAIL":
        return t("signup.invalidEmail");
      default:
        return t("login.genericError");
    }
  }

  return (
    <div className="min-h-[80vh] flex items-center justify-center px-4 py-8">
      <div className="w-full max-w-md">
        <div className="flex flex-col items-center gap-2 mb-8">
          <Ticket className="w-7 h-7" style={{ color: "#ec5b13" }} />
          <h1 className="text-2xl font-bold text-gray-900">{t("signup.title")}</h1>
          <p className="text-sm text-gray-500">
            {isPublisher ? t("signup.subtitlePublisher") : t("signup.subtitle")}
          </p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-white rounded-2xl shadow-sm border border-gray-100 p-8 flex flex-col gap-5"
        >
          <fieldset className="flex flex-col gap-2">
            <legend className="text-sm font-medium text-gray-700 mb-2">{t("signup.accountType.label")}</legend>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {ACCOUNT_TYPES.map(({ value, icon: Icon, title, hint }) => {
                const selected = accountType === value;
                return (
                  <label
                    key={value}
                    className={`relative flex flex-col gap-1 rounded-xl border-2 p-4 cursor-pointer transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-orange-200 ${
                      selected ? "bg-orange-50" : "border-gray-100 hover:border-gray-200"
                    }`}
                    style={selected ? { borderColor: "#ec5b13" } : undefined}
                  >
                    <input
                      type="radio"
                      name="accountType"
                      value={value}
                      checked={selected}
                      onChange={() => setAccountType(value)}
                      className="sr-only"
                    />
                    <Icon className="w-5 h-5" style={{ color: selected ? "#ec5b13" : "#9ca3af" }} aria-hidden="true" />
                    <span className="text-sm font-semibold text-gray-900">{t(title)}</span>
                    <span className="text-xs text-gray-500">{t(hint)}</span>
                  </label>
                );
              })}
            </div>
            <p className="mt-2 text-xs text-gray-500">{t("signup.accountType.permanent")}</p>
          </fieldset>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="name" className="text-sm font-medium text-gray-700">
              {isPublisher ? t("signup.contactName") : t("signup.name")}
            </label>
            <input
              id="name"
              type="text"
              autoComplete="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              autoFocus
              className={INPUT_CLASS}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="email" className="text-sm font-medium text-gray-700">
              {t("login.email")}
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className={INPUT_CLASS}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-sm font-medium text-gray-700">
              {t("login.password")}
            </label>
            <input
              id="password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={MIN_PASSWORD_LENGTH}
              aria-describedby="password-hint"
              className={INPUT_CLASS}
            />
            <p id="password-hint" className="text-xs text-gray-400">
              {t("signup.passwordHint", { min: MIN_PASSWORD_LENGTH })}
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="confirmPassword" className="text-sm font-medium text-gray-700">
              {t("signup.confirmPassword")}
            </label>
            <input
              id="confirmPassword"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              className={INPUT_CLASS}
            />
          </div>

          {isPublisher && (
            <fieldset className="flex flex-col gap-5 border-t border-gray-100 pt-5">
              <legend className="sr-only">{t("signup.orgSection")}</legend>
              <div>
                <p className="text-sm font-semibold text-gray-900">{t("signup.orgSection")}</p>
                <p className="text-xs text-gray-500 mt-0.5">{t("signup.orgSectionHint")}</p>
              </div>
              <OrganizationFields values={org} errors={orgErrors} onChange={setOrg} idPrefix="signup-org" />
            </fieldset>
          )}

          {error && (
            <p role="alert" className="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 rounded-lg text-sm font-semibold text-white transition disabled:opacity-60"
            style={{ backgroundColor: "#ec5b13" }}
          >
            {loading ? t("signup.creating") : t("signup.create")}
          </button>
        </form>

        <p className="text-sm text-gray-500 text-center mt-6">
          {t("signup.haveAccount")}{" "}
          <Link href="/login" className="font-semibold hover:underline" style={{ color: "#ec5b13" }}>
            {t("login.signIn")}
          </Link>
        </p>
      </div>
    </div>
  );
}
