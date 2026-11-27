import { notFound } from "next/navigation";
import { headers } from "next/headers";
import Link from "next/link";
import { Clock, MapPin, Calendar, ChevronLeft, Star, Building2, Globe, Phone, Pencil } from "lucide-react";
import { getEvent, getEvents } from "@/lib/api/events";
import EventCard from "@/components/EventCard";
import EventImage from "@/components/EventImage";
import { formatLongDate } from "@/i18n/format";
import { getTranslation } from "@/i18n/server";
import SaveEventButton from "@/components/SaveEventButton";
import ShareButton from "@/components/ShareButton";
import { isEventSaved } from "@/lib/api/userSavedEvents";
import { getServerApiToken } from "@/lib/better-auth/server-token";
import { auth } from "@/lib/better-auth/server";
import { Event } from "@/lib/types";
import { categoryBadgeClass } from "@/lib/categories";
import {
  endsNextDay,
  eventSessions,
  formatEventDateRange,
  formatScheduleSummary,
  formatSessionDate,
  formatSessionTime,
  hasEventEnded,
  hasSessionEnded,
  isMultiDay,
  nextSession,
  todayInEventTimeZone,
} from "@/lib/dates";
import EventGallery from "./EventGallery";

const RELATED_COUNT = 3;

async function getCurrentUserId(): Promise<string | null> {
  try {
    const session = await auth.api.getSession({ headers: await headers() });
    return session?.user.id ?? null;
  } catch {
    return null;
  }
}

// Only http(s) links are rendered, so a stored "javascript:" URL can never become a link
function safeWebsite(url?: string): string | null {
  if (!url) return null;
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.href : null;
  } catch {
    return null;
  }
}

// Pages that link here with ?from=<key>, so "back" returns to them. Whitelisted to avoid arbitrary redirects.
const BACK_LINKS = {
  "my-events": { href: "/my-events", label: "event.backToMyEvents" },
  publisher: { href: "/publisher", label: "event.backToPublisher" },
} as const;
const DEFAULT_BACK_LINK = { href: "/", label: "event.backToEvents" } as const;

export default async function EventDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const { id } = await params;
  const { from } = await searchParams;
  const backLink = (typeof from === "string" && BACK_LINKS[from as keyof typeof BACK_LINKS]) || DEFAULT_BACK_LINK;
  const { t, locale } = await getTranslation();

  let event: Event;
  try {
    event = await getEvent(id);
  } catch {
    notFound();
  }

  // Saved state of the event and of each related card, one small per-event lookup each.
  // A failing lookup shouldn't break the page: show that event as not saved.
  // Without a session nothing is saved, so skip the requests entirely.
  const token = await getServerApiToken();
  const savedState = (eventId: string) =>
    token ? isEventSaved(eventId, token).catch(() => false) : Promise.resolve(false);

  const [{ events: categoryEvents }, saved, userId] = await Promise.all([
    getEvents({ category: event.category }).catch(() => ({ events: [] as Event[] })),
    savedState(event.id),
    event.publisherId ? getCurrentUserId() : Promise.resolve(null),
  ]);
  const isOwnEvent = !!event.publisherId && event.publisherId === userId;
  const organizer = event.organizer;
  const website = safeWebsite(organizer?.website);
  const now = new Date();
  // Only events still to come (or in progress) are worth suggesting
  const related = categoryEvents
    .filter((e) => e.id !== event.id && !hasEventEnded(e, now))
    .slice(0, RELATED_COUNT);
  const relatedSaved = await Promise.all(related.map((e) => savedState(e.id)));

  const categoryColor = categoryBadgeClass(event.category);

  // Dates: every session, the one in progress / next highlighted and the past ones greyed out
  const sessions = eventSessions(event);
  const multiDay = isMultiDay(event);
  const upcomingSession = nextSession(event, now);
  // Show years only when some session falls outside the current year
  const currentYear = todayInEventTimeZone(now).slice(0, 4);
  const withYear = sessions.some((s) => s.date.slice(0, 4) !== currentYear);
  const firstSession = sessions[0];

  // Cover first (uploaded), then the scraped image; the gallery is the uploaded "gallery" images
  const images = event.images ?? [];
  const coverUrl = images.find((img) => img.kind === "cover")?.url ?? event.imageUrl;
  const gallery = images.filter((img) => img.kind === "gallery").sort((a, b) => a.position - b.position);

  return (
    <div className="max-w-4xl mx-auto px-6 py-8">
      {/* Back */}
      <Link
        href={backLink.href}
        className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-800 mb-6 transition-colors"
      >
        <ChevronLeft className="w-4 h-4" />
        {t(backLink.label)}
      </Link>

      {/* Hero */}
      <div className="rounded-2xl overflow-hidden bg-gradient-to-br from-orange-100 to-orange-300 h-64 mb-6 relative flex items-center justify-center">
        {/* Gradient background shows through when there is no image or it fails to load */}
        <EventImage
          src={coverUrl}
          alt={event.title}
          loading="eager"
          className="absolute inset-0 h-full w-full object-cover"
        />
        <div className="absolute top-4 right-4 flex gap-2">
          <ShareButton title={event.title} />
          <SaveEventButton eventId={event.id} saved={saved} variant="icon" />
        </div>
        <div className="absolute bottom-4 left-4">
          <span className={`text-xs font-semibold px-3 py-1 rounded-full bg-white/90 ${categoryColor}`}>
            {t(`categories.${event.category}`)}
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Main info */}
        <div className="lg:col-span-2 space-y-6">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 mb-2">{event.title}</h1>
            {event.artistName && (
              <p className="text-gray-500 text-sm">
                {t("event.by")} <span className="font-medium text-gray-700">{event.artistName}</span>
              </p>
            )}
          </div>

          <p className="text-gray-600 leading-relaxed">{event.description}</p>

          {/* Details */}
          <div className="space-y-3">
            <div className="flex items-start gap-3">
              <div
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "#fff3ed" }}
              >
                <Calendar className="w-5 h-5" style={{ color: "#ec5b13" }} />
              </div>
              <div>
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wide">{t("event.date")}</p>
                <p className="font-medium text-gray-800">
                  {multiDay
                    ? formatEventDateRange(event, locale, true)
                    : formatLongDate(new Date(firstSession.date + "T12:00:00"), locale, true)}
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "#fff3ed" }}
              >
                <Clock className="w-5 h-5" style={{ color: "#ec5b13" }} />
              </div>
              <div>
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wide">{t("event.time")}</p>
                <p className="font-medium text-gray-800">
                  {multiDay
                    ? formatScheduleSummary(event, locale, t("display.variableSchedule"))
                    : formatSessionTime(firstSession)}
                  {!multiDay && endsNextDay(firstSession) && (
                    <span className="ml-1.5 text-sm font-normal text-gray-500">({t("event.endsNextDay")})</span>
                  )}
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "#fff3ed" }}
              >
                <MapPin className="w-5 h-5" style={{ color: "#ec5b13" }} />
              </div>
              <div>
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wide">{t("event.venue")}</p>
                <p className="font-medium text-gray-800">{event.venue}</p>
                <p className="text-sm text-gray-500">{event.address}</p>
              </div>
            </div>
          </div>

          {/* Every session of a multi-day event */}
          {multiDay && (
            <section>
              <h3 className="font-semibold text-gray-900 mb-3">{t("event.datesAndTimes")}</h3>
              <ul className="divide-y divide-gray-100 rounded-2xl border border-gray-100 bg-white">
                {sessions.map((session) => {
                  const past = hasSessionEnded(session, now);
                  const isNext = session === upcomingSession;
                  return (
                    <li
                      key={`${session.date}-${session.startTime}`}
                      className={`flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3 ${past ? "opacity-50" : ""}`}
                    >
                      <span className={`font-medium ${past ? "text-gray-500" : "text-gray-800"}`}>
                        {formatSessionDate(session.date, locale, withYear)}
                      </span>
                      <span className="text-sm text-gray-600 tabular-nums">
                        {formatSessionTime(session)}
                        {endsNextDay(session) && (
                          <span className="ml-1.5 text-gray-400">({t("event.endsNextDay")})</span>
                        )}
                      </span>
                      {past && (
                        <span className="ml-auto text-xs font-medium px-2 py-0.5 rounded-full bg-gray-100 text-gray-500">
                          {t("event.sessionEnded")}
                        </span>
                      )}
                      {isNext && (
                        <span
                          className="ml-auto text-xs font-semibold px-2 py-0.5 rounded-full bg-orange-50"
                          style={{ color: "#ec5b13" }}
                        >
                          {t("event.nextSession")}
                        </span>
                      )}
                    </li>
                  );
                })}
              </ul>
            </section>
          )}

          {/* Perks */}
          {event.perks && event.perks.length > 0 && (
            <div>
              <h3 className="font-semibold text-gray-900 mb-3">{t("event.whatsIncluded")}</h3>
              <ul className="space-y-2">
                {event.perks.map((perk) => (
                  <li key={perk} className="flex items-center gap-2 text-sm text-gray-600">
                    <Star className="w-4 h-4 shrink-0" style={{ color: "#ec5b13" }} />
                    {perk}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Promotional gallery (uploaded images only; scraped events have none) */}
          <EventGallery images={gallery} title={event.title} />
        </div>

        {/* Ticket sidebar */}
        <div className="lg:col-span-1">
          {/* Ticket, owner and organizer cards stick together */}
          <div className="sticky top-24 space-y-4">
            <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-6">
              <div className="mb-4">
                <p className="text-xs text-gray-400 uppercase tracking-wide font-medium mb-1">{t("event.price")}</p>
                <p className="text-3xl font-bold text-gray-900">
                  {event.price === 0 ? t("common.free") : t("common.price", { price: event.price })}
                </p>
                {event.price > 0 && <p className="text-xs text-gray-400 mt-0.5">{t("event.perPerson")}</p>}
              </div>

              {/* No ticket link in the event data yet, so no "buy" button: just say where tickets are handled */}
              <p className="text-sm text-gray-500 leading-relaxed">
                {event.price === 0 ? t("event.freeEntryInfo") : t("event.ticketsInfo")}
              </p>

              <SaveEventButton eventId={event.id} saved={saved} />
            </div>

            {isOwnEvent && (
              <div className="rounded-2xl border border-orange-200 bg-orange-50 p-4">
                <p className="text-sm text-gray-700 mb-2">{t("event.youArePublisher")}</p>
                <Link
                  href={`/publisher/events/${event.id}/edit`}
                  className="inline-flex items-center gap-1.5 text-sm font-semibold hover:underline"
                  style={{ color: "#ec5b13" }}
                >
                  <Pencil className="w-4 h-4" />
                  {t("event.editInPublisher")}
                </Link>
              </div>
            )}

            {organizer && (
              <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-6">
                <p className="text-xs text-gray-400 uppercase tracking-wide font-medium mb-3">
                  {t("event.organizedBy")}
                </p>
                <div className="flex items-center gap-3 mb-3">
                  <div
                    className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
                    style={{ backgroundColor: "#fff3ed" }}
                  >
                    <Building2 className="w-5 h-5" style={{ color: "#ec5b13" }} />
                  </div>
                  <p className="font-semibold text-gray-900 break-words">{organizer.orgName}</p>
                </div>
                {organizer.description && (
                  <p className="text-sm text-gray-600 leading-relaxed mb-3 whitespace-pre-line">
                    {organizer.description}
                  </p>
                )}
                {(website || organizer.phone) && (
                  <ul className="space-y-2 text-sm">
                    {website && (
                      <li>
                        <a
                          href={website}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-2 font-medium hover:underline break-all"
                          style={{ color: "#ec5b13" }}
                        >
                          <Globe className="w-4 h-4 shrink-0" />
                          <span className="sr-only">{t("event.website")}: </span>
                          {new URL(website).host}
                        </a>
                      </li>
                    )}
                    {organizer.phone && (
                      <li>
                        <a
                          href={`tel:${organizer.phone.replace(/[^\d+]/g, "")}`}
                          className="inline-flex items-center gap-2 font-medium text-gray-700 hover:underline"
                        >
                          <Phone className="w-4 h-4 shrink-0 text-gray-400" />
                          <span className="sr-only">{t("event.phone")}: </span>
                          {organizer.phone}
                        </a>
                      </li>
                    )}
                  </ul>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Related events */}
      {related.length > 0 && (
        <section className="mt-12">
          <h2 className="text-xl font-bold text-gray-900 mb-5">
            {t("event.moreCategoryEvents", { category: t(`categories.${event.category}`) })}
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {related.map((e, i) => (
              <EventCard key={e.id} event={e} saved={relatedSaved[i]} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
