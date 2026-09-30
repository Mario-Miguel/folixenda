"use client";

import { useState, SubmitEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { Ticket } from "lucide-react";
import { signUp } from "@/lib/better-auth/client";

// Must match Better Auth's minPasswordLength (default 8)
const MIN_PASSWORD_LENGTH = 8;

const INPUT_CLASS =
  "border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition";

export default function SignupPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    if (password !== confirmPassword) {
      setError(t("signup.passwordMismatch"));
      return;
    }

    setLoading(true);
    // Better Auth signs the new user in automatically
    const { error } = await signUp.email({ name: name.trim(), email: email.trim(), password });
    if (error) {
      setError(signupErrorMessage(error.code));
      setLoading(false);
      return;
    }
    router.push("/");
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
      <div className="w-full max-w-sm">
        <div className="flex flex-col items-center gap-2 mb-8">
          <Ticket className="w-7 h-7" style={{ color: "#ec5b13" }} />
          <h1 className="text-2xl font-bold text-gray-900">{t("signup.title")}</h1>
          <p className="text-sm text-gray-500">{t("signup.subtitle")}</p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-white rounded-2xl shadow-sm border border-gray-100 p-8 flex flex-col gap-5"
        >
          <div className="flex flex-col gap-1.5">
            <label htmlFor="name" className="text-sm font-medium text-gray-700">
              {t("signup.name")}
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
              className={INPUT_CLASS}
            />
            <p className="text-xs text-gray-400">{t("signup.passwordHint", { min: MIN_PASSWORD_LENGTH })}</p>
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

          {error && <p className="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">{error}</p>}

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
