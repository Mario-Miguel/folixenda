import Link from "next/link";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { Building2, ShieldCheck, Info } from "lucide-react";
import { getTranslation } from "@/i18n/server";
import { auth } from "@/lib/better-auth/server";
import { getServerApiToken } from "@/lib/better-auth/server-token";
import { getMe, getMyEvents } from "@/lib/api/me";
import { compareSessionEnds, compareSessions, hasEventEnded, lastSession, nextSession } from "@/lib/dates";
import { MeResponse, PublisherEvent } from "@/lib/types";
import PublisherDashboard from "./PublisherDashboard";

async function getCurrentSession() {
  try {
    return await auth.api.getSession({ headers: await headers() });
  } catch {
    return null;
  }
}

// Upcoming: by their next (or in-progress) session, soonest first.
// Past: by the end of their last session, most recent first.
const byNextSession = (now: Date) => (a: PublisherEvent, b: PublisherEvent) =>
  compareSessions(nextSession(a, now) ?? lastSession(a), nextSession(b, now) ?? lastSession(b));
const byLastEndDesc = (a: PublisherEvent, b: PublisherEvent) => compareSessionEnds(lastSession(b), lastSession(a));

export default async function PublisherPage() {
  const session = await getCurrentSession();
  if (!session) redirect("/login?redirect=/publisher");

  const { t } = await getTranslation();
  const role = session.user.role;

  if (role !== "publisher") {
    // Admins manage every event from /admin; consumers get an explanation of what this area is
    const isAdmin = role === "admin";
    return (
      <div className="min-h-[70vh] flex items-center justify-center px-4">
        <div className="w-full max-w-md bg-white rounded-2xl shadow-sm border border-gray-100 p-8 text-center">
          {isAdmin ? (
            <ShieldCheck className="w-10 h-10 mx-auto mb-4" style={{ color: "#ec5b13" }} />
          ) : (
            <Building2 className="w-10 h-10 mx-auto mb-4" style={{ color: "#ec5b13" }} />
          )}
          <h1 className="text-xl font-bold text-gray-900 mb-2">
            {isAdmin ? t("publisher.adminNote.title") : t("publisher.consumerCard.title")}
          </h1>
          <p className="text-sm text-gray-500 mb-6 leading-relaxed">
            {isAdmin ? t("publisher.adminNote.text") : t("publisher.consumerCard.text")}
          </p>
          {isAdmin ? (
            <Link
              href="/admin"
              className="inline-block px-4 py-2 rounded-lg text-sm font-semibold text-white"
              style={{ backgroundColor: "#ec5b13" }}
            >
              {t("publisher.adminNote.link")}
            </Link>
          ) : (
            <div className="flex flex-col sm:flex-row gap-2 justify-center">
              <Link
                href="/profile"
                className="px-4 py-2 rounded-lg text-sm font-semibold text-white"
                style={{ backgroundColor: "#ec5b13" }}
              >
                {t("publisher.consumerCard.profileLink")}
              </Link>
              <Link
                href="/my-events"
                className="px-4 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100 transition"
              >
                {t("publisher.consumerCard.myEventsLink")}
              </Link>
            </div>
          )}
        </div>
      </div>
    );
  }

  // Future: publishing will require a paid subscription. This is where the page would check the
  // account's subscription (e.g. me.user.subscriptionType) and show a "subscription required" gate
  // instead of, or above, the dashboard.

  const token = await getServerApiToken();
  if (!token) redirect("/login?redirect=/publisher");

  const [eventsResult, me] = await Promise.all([
    getMyEvents(token).catch((err) => {
      console.error(err);
      return null;
    }),
    // Only used for the organization data: if it fails, show the page without the banner
    getMe(token).catch((err): MeResponse | null => {
      console.error(err);
      return null;
    }),
  ]);

  const profile = me?.publisherProfile ?? null;
  const orgName = profile?.orgName || session.user.name || session.user.email;

  // "Now" in Europe/Madrid (event times are Spanish wall-clock times), decided on the server so
  // the split doesn't change between server render and hydration
  const now = new Date();
  const events = eventsResult?.events ?? [];
  // An event is past only once its LAST session has ended
  const upcoming = events.filter((e) => !hasEventEnded(e, now)).sort(byNextSession(now));
  const past = events.filter((e) => hasEventEnded(e, now)).sort(byLastEndDesc);

  return (
    <div className="max-w-5xl mx-auto px-6 py-8">
      {me && !profile && (
        <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-6 rounded-xl border border-orange-200 bg-orange-50 px-4 py-3">
          <Info className="w-5 h-5 shrink-0" style={{ color: "#ec5b13" }} />
          <p className="flex-1 text-sm text-gray-700">{t("publisher.profileBanner")}</p>
          <Link href="/profile" className="text-sm font-semibold hover:underline" style={{ color: "#ec5b13" }}>
            {t("publisher.profileBannerLink")}
          </Link>
        </div>
      )}

      {eventsResult === null ? (
        <>
          <h1 className="text-2xl font-bold text-gray-900 mb-8">{t("publisher.title")}</h1>
          <p className="text-center py-20 text-gray-400 font-medium">{t("publisher.loadError")}</p>
        </>
      ) : (
        <PublisherDashboard orgName={orgName} upcoming={upcoming} past={past} />
      )}
    </div>
  );
}
