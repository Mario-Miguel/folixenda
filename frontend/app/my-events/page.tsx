import Link from "next/link";
import { redirect } from "next/navigation";
import { Bookmark } from "lucide-react";
import MyEventsView, { EventDay } from "@/components/MyEventsView";
import { getTranslation } from "@/i18n/server";
import { getSavedEvents } from "@/lib/api/userSavedEvents";
import { getServerApiToken } from "@/lib/better-auth/server-token";
import { SavedEvent } from "@/lib/types";

// An event counts as past once its end time has gone by (server local time)
function hasEnded(event: SavedEvent, now: Date) {
  return new Date(`${event.date}T${event.endTime || "23:59"}`) < now;
}

// Upcoming events sorted by date and start time, grouped by day
function groupUpcomingByDay(events: SavedEvent[]): EventDay[] {
  const now = new Date();
  const upcoming = events
    .filter((e) => !hasEnded(e, now))
    .sort((a, b) => a.date.localeCompare(b.date) || a.startTime.localeCompare(b.startTime));

  const days: EventDay[] = [];
  for (const event of upcoming) {
    const last = days.at(-1);
    if (last?.date === event.date) last.events.push(event);
    else days.push({ date: event.date, events: [event] });
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
