"use client";

import Link from "next/link";
import { MapPin, Clock, Bookmark, BookmarkCheck, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Event } from "@/lib/types";
import EventImage from "@/components/EventImage";

export const CATEGORY_COLORS: Record<string, string> = {
  Music: "bg-purple-100 text-purple-700",
  Theater: "bg-blue-100 text-blue-700",
  Parties: "bg-pink-100 text-pink-700",
  Sports: "bg-green-100 text-green-700",
  Food: "bg-yellow-100 text-yellow-700",
  Art: "bg-rose-100 text-rose-700",
  Wellness: "bg-teal-100 text-teal-700",
};

interface EventCardProps {
  event: Event;
  // List row layout (no image)
  compact?: boolean;
  // Media card only: false shows just image, title, date/time and the action (map popup)
  detailed?: boolean;
  // Media card only: note that the event's location is an approximation
  approximateLocation?: boolean;
}

function formatShortDate(date: string): string {
  // Parse as local date: "YYYY-MM-DD" alone would be read as UTC midnight
  const d = new Date(`${date}T00:00:00`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString("en-US", { weekday: "short", month: "short", day: "numeric" });
}

export default function EventCard({
  event,
  compact = false,
  detailed = true,
  approximateLocation = false,
}: EventCardProps) {
  const { t } = useTranslation();
  const categoryColor = CATEGORY_COLORS[event.category] ?? "bg-gray-100 text-gray-600";

  if (compact) {
    return (
      <Link href={`/events/${event.id}`}>
        <div className="bg-white rounded-xl p-4 border border-gray-100 hover:border-orange-200 hover:shadow-md transition-all cursor-pointer group">
          <div className="flex items-start justify-between gap-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${categoryColor}`}>
                  {t(`categories.${event.category}`)}
                </span>
              </div>
              <h3
                className="font-semibold text-gray-900 text-sm truncate group-hover:text-primary transition-colors"
                style={{ ["--tw-text-opacity" as string]: "1" }}
              >
                {event.title}
              </h3>
              <div className="flex items-center gap-1 mt-1 text-xs text-gray-500">
                <Clock className="w-3 h-3" />
                <span>
                  {event.startTime} – {event.endTime}
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
              <button
                className="w-7 h-7 flex items-center justify-center rounded-full bg-orange-50 text-primary hover:bg-orange-100 transition-colors"
                style={{ color: "#ec5b13" }}
              >
                <Plus className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </Link>
    );
  }

  const mediaHeight = detailed ? "h-40" : "h-32";
  const when = [event.date && formatShortDate(event.date), [event.startTime, event.endTime].filter(Boolean).join(" - ")]
    .filter(Boolean)
    .join(" · ");

  return (
    // h-full + flex column: cards in a grid row share the same height, actions pinned to the bottom
    <Link href={`/events/${event.id}`} className="block h-full no-underline">
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
              <div className="absolute top-3 right-3 flex gap-2">
                <button
                  onClick={(e) => e.preventDefault()}
                  className="w-8 h-8 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm text-gray-600 hover:text-primary transition-colors"
                >
                  {event.isSaved ? (
                    <BookmarkCheck className="w-4 h-4" style={{ color: "#ec5b13" }} />
                  ) : (
                    <Bookmark className="w-4 h-4" />
                  )}
                </button>
              </div>
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

          {when && (
            <div className="flex items-center gap-1 text-xs text-gray-500 mt-2">
              <Clock className="w-3.5 h-3.5 shrink-0" />
              <span>{when}</span>
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
              <span>Approximate location</span>
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
            {t("common.getTickets")}
          </span>
        </div>
      </div>
    </Link>
  );
}
