"use client";

import { useState, SubmitEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { Ticket } from "lucide-react";
import { signIn } from "@/lib/better-auth/client";

// Only follow same-site paths, so ?redirect= can't send users to another domain
function safeRedirect(target: string | null): string {
  return target && /^\/(?![/\\])/.test(target) ? target : "/";
}

export default function LoginPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setLoading(true);
    const { error } = await signIn.email({ email: email.trim(), password });
    if (error) {
      setError(error.status === 401 ? t("login.wrongCredentials") : t("login.genericError"));
      setLoading(false);
      return;
    }
    router.push(safeRedirect(new URLSearchParams(window.location.search).get("redirect")));
    router.refresh();
  }

  return (
    <div className="min-h-[80vh] flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="flex flex-col items-center gap-2 mb-8">
          <Ticket className="w-7 h-7" style={{ color: "#ec5b13" }} />
          <h1 className="text-2xl font-bold text-gray-900">{t("login.title")}</h1>
          <p className="text-sm text-gray-500">{t("login.subtitle")}</p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-white rounded-2xl shadow-sm border border-gray-100 p-8 flex flex-col gap-5"
        >
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
              autoFocus
              className="border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-sm font-medium text-gray-700">
              {t("login.password")}
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition"
            />
          </div>

          {error && <p className="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">{error}</p>}

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 rounded-lg text-sm font-semibold text-white transition disabled:opacity-60"
            style={{ backgroundColor: "#ec5b13" }}
          >
            {loading ? t("login.signingIn") : t("login.signIn")}
          </button>
        </form>

        <p className="text-sm text-gray-500 text-center mt-6">
          {t("login.noAccount")}{" "}
          <Link href="/signup" className="font-semibold hover:underline" style={{ color: "#ec5b13" }}>
            {t("login.createAccount")}
          </Link>
        </p>
      </div>
    </div>
  );
}
