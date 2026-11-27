import Link from "next/link";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { ShieldAlert } from "lucide-react";
import { getTranslation } from "@/i18n/server";
import { auth } from "@/lib/better-auth/server";
import AdminDashboard from "./AdminDashboard";

async function getCurrentSession() {
  try {
    return await auth.api.getSession({ headers: await headers() });
  } catch {
    return null;
  }
}

export default async function AdminPage() {
  // Anonymous visitors log in first and come back here
  const session = await getCurrentSession();
  if (!session) redirect("/login?redirect=/admin");

  // Logged-in users without the admin role get an access denied screen
  if (session.user.role !== "admin") {
    const { t } = await getTranslation();
    return (
      <div className="min-h-[80vh] flex items-center justify-center px-4">
        <div className="w-full max-w-sm bg-white rounded-2xl shadow-sm border border-gray-100 p-8 text-center">
          <ShieldAlert className="w-10 h-10 mx-auto mb-4" style={{ color: "#ec5b13" }} />
          <h1 className="text-xl font-bold text-gray-900 mb-2">{t("admin.access")}</h1>
          <p className="text-sm text-gray-500 mb-6">{t("admin.noAdminAccess")}</p>
          <Link href="/" className="text-sm font-medium hover:underline" style={{ color: "#ec5b13" }}>
            {t("admin.backHome")}
          </Link>
        </div>
      </div>
    );
  }

  return <AdminDashboard name={session.user.name || session.user.email} currentUserId={session.user.id} />;
}
