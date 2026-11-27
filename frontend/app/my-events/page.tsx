import Link from "next/link";
import { redirect } from "next/navigation";
import { Bookmark } from "lucide-react";
import MyEventsView, { EventDay } from "@/components/MyEventsView";
import { getTranslation } from "@/i18n/server";
import { getSavedEvents } from "@/lib/api/userSavedEvents";
import { getServerApiToken } from "@/lib/better-auth/server-token";
import { SavedEvent } from "@/lib/types";
import { compareSessions, nextSession } from "@/lib/dates";

// Upcoming events grouped by the day of their next session (the one in progress, or the next to
// start), sorted by that session's start. A multi-day event stays upcoming until its last session
// ends; events whose last session has ended are left out.
function groupUpcomingByDay(events: SavedEvent[]): EventDay[] {
  // Event times are Spanish wall-clock times, so compare against "now" in Europe/Madrid:
  // correct whatever time zone the server runs in
  const now = new Date();
  const upcoming = events
    .map((event) => ({ event, session: nextSession(event, now) }))
    .filter((x): x is { event: SavedEvent; session: NonNullable<typeof x.session> } => !!x.session)
    .sort((a, b) => compareSessions(a.session, b.session));

  const days: EventDay[] = [];
  for (const { event, session } of upcoming) {
    const last = days.at(-1);
    if (last?.date === session.date) last.events.push(event);
    else days.push({ date: session.date, events: [event] });
  }
  return days;
}

export default async function MyEventsPage() {
  // Saved events belong to a user: send anonymous visitors to log in first
  const token = await getServerApiToken();
  if (!token) redirect("/login?redirect=/my-events");

  const { t } = await getTranslation();

  let days: EventDay[] | null = null;
  try {
    const { savedEvents } = await getSavedEvents(token);
    days = groupUpcomingByDay(savedEvents);
  } catch (err) {
    console.error(err);
  }

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      <h1 className="text-2xl font-bold text-gray-900 mb-8">{t("nav.myEvents")}</h1>

      {days === null ? (
        <p className="text-center py-20 text-gray-400 font-medium">{t("myEvents.loadError")}</p>
      ) : days.length === 0 ? (
        <div className="text-center py-20 text-gray-400">
          <Bookmark className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p className="font-medium">{t("myEvents.noSavedEvents")}</p>
          <Link href="/" className="inline-block text-sm mt-1 hover:underline" style={{ color: "#ec5b13" }}>
            {t("myEvents.browseEvents")}
          </Link>
        </div>
      ) : (
        <MyEventsView days={days} />
      )}
    </div>
  );
}
