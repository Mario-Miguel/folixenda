"use client";

import Link from "next/link";
import { MapPin, Clock, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Event } from "@/lib/types";
import EventImage from "@/components/EventImage";
import SaveEventButton from "@/components/SaveEventButton";
import { categoryBadgeClass } from "@/lib/categories";
import { DayBadge, NextDayNote, useEventWhen } from "@/components/EventSchedule";

interface EventCardProps {
  event: Event;
  // List row layout (no image)
  compact?: boolean;
  // Media card only: false shows just image, title, date/time and the action (map popup)
  detailed?: boolean;
  // Media card only: note that the event's location is an approximation
  approximateLocation?: boolean;
  // Compact only: clicking the card selects it (e.g. to highlight its map pin);
  // only the "+" button opens the event page
  selected?: boolean;
  onSelect?: () => void;
  // Overrides the event page link (e.g. to add ?from=... for the back button)
  href?: string;
  // Detailed media card only: whether the current user saved the event. When given, a working
  // bookmark button is shown; the parent fetches this once for the whole list.
  saved?: boolean;
  // Day being viewed ("YYYY-MM-DD"): show that day's session times (and "Day 2 of 3" for
  // multi-day events) instead of the event's overall dates
  day?: string;
}

function formatShortDate(date: string | undefined, locale: string): string {
  if (!date) return "";
  // Parse as local date: "YYYY-MM-DD" alone would be read as UTC midnight
  const d = new Date(`${date}T00:00:00`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString(locale, { weekday: "short", month: "short", day: "numeric" });
}

export default function EventCard({
  event,
  compact = false,
  detailed = true,
  approximateLocation = false,
  selected = false,
  onSelect,
  href: hrefOverride,
  saved,
  day,
}: EventCardProps) {
  const { t, i18n } = useTranslation();
  const categoryColor = categoryBadgeClass(event.category);
  const href = hrefOverride ?? `/events/${event.id}`;
  const when = useEventWhen(event, day);

  if (compact) {
    const title = (
      <h3
        className="font-semibold text-gray-900 text-sm truncate group-hover:text-primary transition-colors"
        style={{ ["--tw-text-opacity" as string]: "1" }}
      >
        {event.title}
      </h3>
    );

    const card = (
      <div
        className={`bg-white rounded-xl p-4 border hover:border-orange-200 hover:shadow-md transition-all cursor-pointer group ${
          selected ? "border-orange-300 bg-orange-50/40" : "border-gray-100"
        }`}
        onClick={onSelect}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${categoryColor}`}>
                {t(`categories.${event.category}`)}
              </span>
              <DayBadge label={when.dayBadge} />
            </div>
            {onSelect ? (
              // Keyboard/screen-reader target for selecting; mouse users can click anywhere on the card
              <button type="button" aria-pressed={selected} className="block w-full text-left min-w-0">
                {title}
              </button>
            ) : (
              title
            )}
            <div className="flex items-center gap-1 mt-1 text-xs text-gray-500 min-w-0">
              <Clock className="w-3 h-3 shrink-0" />
              <span className="truncate">
                {when.range && <span className="font-medium text-gray-700">{when.range} · </span>}
                {when.time} <NextDayNote show={when.nextDay} />
              </span>
            </div>
            <div className="flex items-center gap-1 mt-0.5 text-xs text-gray-500 truncate">
              <MapPin className="w-3 h-3 shrink-0" />
              <span className="truncate">{event.venue}</span>
            </div>
          </div>
          <div className="flex flex-col items-end gap-2 shrink-0">
            <span className="text-sm font-bold text-gray-900">
              {event.price === 0 ? t("common.free") : t("common.price", { price: event.price })}
            </span>
            {onSelect ? (
              <Link
                href={href}
                aria-label={t("common.details")}
                onClick={(e) => e.stopPropagation()}
                className="w-7 h-7 flex items-center justify-center rounded-full bg-orange-50 text-primary hover:bg-orange-100 transition-colors"
                style={{ color: "#ec5b13" }}
              >
                <Plus className="w-4 h-4" />
              </Link>
            ) : (
              <button
                className="w-7 h-7 flex items-center justify-center rounded-full bg-orange-50 text-primary hover:bg-orange-100 transition-colors"
                style={{ color: "#ec5b13" }}
              >
                <Plus className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>
      </div>
    );

    return onSelect ? card : <Link href={href}>{card}</Link>;
  }

  const mediaHeight = detailed ? "h-40" : "h-32";
  // Day view: that day's date and times; otherwise the event's date (or date range for multi-day)
  const dateLabel = when.range ?? formatShortDate(day ?? event.date, i18n.language);
  const whenText = [dateLabel, when.time].filter(Boolean).join(" · ");

  return (
    // h-full + flex column: cards in a grid row share the same height, actions pinned to the bottom
    <Link href={href} className="block h-full no-underline">
      <div className="h-full flex flex-col bg-white rounded-2xl overflow-hidden border border-gray-100 hover:shadow-lg transition-all cursor-pointer group">
        {/* Media */}
        <div className={`${mediaHeight} shrink-0 relative bg-gradient-to-br from-orange-100 to-orange-200`}>
          <EventImage
            src={event.imageUrl}
            alt={event.title}
            className="absolute inset-0 h-full w-full object-cover"
            fallback={
              <div className="absolute inset-0 flex items-center justify-center">
                <span className="text-4xl opacity-30">🎵</span>
              </div>
            }
          />
          {detailed && (
            <>
              {saved !== undefined && (
                // The card is a link: keep bookmark clicks from navigating to the event
                <div
                  className="absolute top-3 right-3 flex gap-2"
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                  }}
                >
                  <SaveEventButton eventId={event.id} saved={saved} variant="icon" />
                </div>
              )}
              <div className="absolute bottom-3 left-3">
                <span className={`text-xs font-medium px-2.5 py-1 rounded-full bg-white/90 ${categoryColor}`}>
                  {t(`categories.${event.category}`)}
                </span>
              </div>
            </>
          )}
        </div>

        {/* Content */}
        <div className="px-4 pt-3">
          <h3
            className="font-bold text-gray-900 leading-snug line-clamp-2 group-hover:text-primary transition-colors"
            style={{ ["--tw-text-opacity" as string]: "1" }}
          >
            {event.title}
          </h3>

          {whenText && (
            <div className="flex items-center gap-1 text-xs text-gray-500 mt-2 min-w-0">
              <Clock className="w-3.5 h-3.5 shrink-0" />
              <span className="min-w-0">
                {whenText} <NextDayNote show={when.nextDay} />
              </span>
              <DayBadge label={when.dayBadgeShort} />
            </div>
          )}
          {detailed && (
            <div className="flex items-center gap-1 text-xs text-gray-500 mt-1">
              <MapPin className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{event.venue}</span>
            </div>
          )}
          {approximateLocation && (
            <div className="flex items-center gap-1 text-xs text-gray-400 mt-1">
              <MapPin className="w-3.5 h-3.5 shrink-0" />
              <span>{t("common.approximateLocation")}</span>
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="mt-auto flex items-center justify-between px-4 pt-2 pb-3">
          {detailed && (
            <span className="font-bold text-gray-900">
              {event.price === 0 ? t("common.free") : t("common.fromPrice", { price: event.price })}
            </span>
          )}
          <span className="-mx-2 rounded px-2 py-1.5 text-[13px] font-semibold uppercase tracking-wide text-primary group-hover:bg-orange-50 transition-colors">
            {t("common.details")}
          </span>
        </div>
      </div>
    </Link>
  );
}
