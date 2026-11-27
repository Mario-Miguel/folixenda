import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { AlertTriangle } from "lucide-react";
import { getTranslation } from "@/i18n/server";
import { ApiError } from "@/lib/api/api";
import { getMe } from "@/lib/api/me";
import { auth } from "@/lib/better-auth/server";
import { getServerApiToken } from "@/lib/better-auth/server-token";
import { MeResponse } from "@/lib/types";
import AccountSection from "./AccountSection";
import OrganizationSection from "./OrganizationSection";
import NotificationsSection from "./NotificationsSection";
import SecuritySection from "./SecuritySection";
import DangerZone from "./DangerZone";

async function getCurrentSession() {
  try {
    return await auth.api.getSession({ headers: await headers() });
  } catch {
    return null;
  }
}

export default async function ProfilePage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const session = await getCurrentSession();
  const token = session ? await getServerApiToken() : null;
  if (!session || !token) redirect("/login?redirect=/profile");

  const { t, locale } = await getTranslation();
  const { completeOrg } = await searchParams;

  // Better Auth owns name, email, role and creation date; the Go API owns the rest
  let me: MeResponse | null = null;
  try {
    me = await getMe(token);
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login?redirect=/profile");
    console.error(err);
  }

  const { user } = session;
  const role = user.role === "admin" || user.role === "publisher" ? user.role : "consumer";
  const memberSince = new Date(user.createdAt).toLocaleDateString(locale, {
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 py-8 flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">{t("profile.title")}</h1>
        <p className="text-sm text-gray-500 mt-1">{t("profile.subtitle")}</p>
      </div>

      <AccountSection name={user.name} email={user.email} role={role} memberSince={memberSince} />

      {me === null && (
        <div role="alert" className="flex items-start gap-3 bg-red-50 text-red-700 rounded-2xl px-5 py-4 text-sm">
          <AlertTriangle className="w-5 h-5 shrink-0" aria-hidden="true" />
          {t("profile.loadError")}
        </div>
      )}

      {me && role === "publisher" && (
        <OrganizationSection profile={me.publisherProfile} highlight={completeOrg === "1"} />
      )}

      {me && <NotificationsSection initial={me.notifications} isPublisher={role === "publisher"} />}

      <SecuritySection />

      <DangerZone isPublisher={role === "publisher"} />
    </div>
  );
}
