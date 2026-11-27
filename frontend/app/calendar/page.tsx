"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight, Map as MapIcon, CalendarDays } from "lucide-react";
import CategoryFilter from "@/components/CategoryFilter";
import LoadError from "@/components/LoadError";
import MapView from "@/components/MapView";
import { Event } from "@/lib/types";
import { CATEGORIES, categoryColor } from "@/lib/categories";
import { monthRange, startOfMonth, toISODateStr } from "@/lib/dates";
import { useEvents } from "@/lib/hooks/useEvents";
import { formatMonthYear, weekdayNames } from "@/i18n/format";

export default function CalendarPage() {
  const { t, i18n } = useTranslation();
  const [viewDate, setViewDate] = useState(() => startOfMonth(new Date()));
  const [selectedCategory, setSelectedCategory] = useState("All");
  const [showMap, setShowMap] = useState(false);
  const [selectedEventId, setSelectedEventId] = useState<string | null>(null);

  // Only the visible month, already filtered by category in the API
  const { events: monthEvents, error, retry } = useEvents({ ...monthRange(viewDate), category: selectedCategory });

  const year = viewDate.getFullYear();
  const month = viewDate.getMonth();
  const firstDay = new Date(year, month, 1).getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();

  const prevMonth = () => setViewDate(new Date(year, month - 1, 1));
  const nextMonth = () => setViewDate(new Date(year, month + 1, 1));

  const cells = useMemo(() => {
    const byDate = new Map<string, Event[]>();
    for (const e of monthEvents) byDate.set(e.date, [...(byDate.get(e.date) ?? []), e]);

    const result: { day: number | null; dateStr: string | null; events: Event[] }[] = [];

    for (let i = 0; i < firstDay; i++) {
      result.push({ day: null, dateStr: null, events: [] });
    }

    for (let d = 1; d <= daysInMonth; d++) {
      const dateStr = toISODateStr(new Date(year, month, d));
      const events = (byDate.get(dateStr) ?? []).sort((a, b) => a.startTime.localeCompare(b.startTime));
      result.push({ day: d, dateStr, events });
    }

    return result;
  }, [year, month, firstDay, daysInMonth, monthEvents]);

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Tab bar */}
      <div className="flex items-center gap-1 mb-6 border-b border-gray-200">
        {[
          { href: "/calendar", label: t("common.calendar"), active: true },
          { href: "/", label: t("nav.discover"), active: false },
          { href: "/my-events", label: t("nav.myTickets"), active: false },
        ].map(({ href, label, active }) => (
          <Link
            key={href}
            href={href}
            className={`px-5 py-3 text-sm font-medium border-b-2 transition-colors -mb-px ${
              active ? "border-primary text-primary" : "border-transparent text-gray-500 hover:text-gray-800"
            }`}
            style={active ? { borderColor: "#ec5b13", color: "#ec5b13" } : undefined}
          >
            {label}
          </Link>
        ))}
      </div>

      {/* Category filter */}
      <div className="mb-6">
        <CategoryFilter selected={selectedCategory} onChange={setSelectedCategory} />
      </div>

      {/* View toggle */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <button
            onClick={prevMonth}
            aria-label={t("common.previousMonth")}
            className="w-8 h-8 flex items-center justify-center rounded-full border border-gray-200 hover:bg-gray-50 transition-colors text-gray-600"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <h2 className="text-lg font-bold text-gray-900 min-w-[160px] text-center">
            {formatMonthYear(year, month, i18n.language)}
          </h2>
          <button
            onClick={nextMonth}
            aria-label={t("common.nextMonth")}
            className="w-8 h-8 flex items-center justify-center rounded-full border border-gray-200 hover:bg-gray-50 transition-colors text-gray-600"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>

        <div className="flex rounded-lg border border-gray-200 overflow-hidden">
          <button
            onClick={() => setShowMap(false)}
            className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium transition-colors ${
              !showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
            }`}
            style={!showMap ? { backgroundColor: "#ec5b13" } : undefined}
          >
            <CalendarDays className="w-4 h-4" />
            {t("common.calendar")}
          </button>
          <button
            onClick={() => setShowMap(true)}
            className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium transition-colors ${
              showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
            }`}
            style={showMap ? { backgroundColor: "#ec5b13" } : undefined}
          >
            <MapIcon className="w-4 h-4" />
            {t("common.map")}
          </button>
        </div>
      </div>

      {error ? (
        <LoadError onRetry={retry} />
      ) : showMap ? (
        <MapView
          events={monthEvents}
          selectedEventId={selectedEventId}
          onSelectEvent={setSelectedEventId}
          className="h-[32rem]"
        />
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          {/* Day headers */}
          <div className="grid grid-cols-7 border-b border-gray-100">
            {weekdayNames(i18n.language, "short").map((day, i) => (
              <div key={i} className="py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wide">
                {day}
              </div>
            ))}
          </div>

          {/* Calendar grid */}
          <div className="grid grid-cols-7 divide-x divide-gray-50">
            {cells.map(({ day, dateStr, events }, i) => (
              <div
                key={dateStr ?? `empty-${i}`}
                className={`min-h-28 p-2 border-b border-gray-50 ${
                  !day ? "bg-gray-50/50" : "hover:bg-orange-50/30 transition-colors"
                }`}
              >
                {day && (
                  <>
                    <span className="text-sm font-medium text-gray-700 block mb-1.5">{day}</span>
                    <div className="space-y-1">
                      {events.slice(0, 2).map((event) => (
                        <Link key={event.id} href={`/events/${event.id}`} className="block">
                          <div
                            className="text-xs px-1.5 py-0.5 rounded text-white truncate font-medium hover:opacity-90 transition-opacity"
                            style={{ backgroundColor: categoryColor(event.category) }}
                          >
                            {event.title}
                          </div>
                        </Link>
                      ))}
                      {events.length > 2 && (
                        <span className="text-xs text-gray-400 font-medium">{t("common.moreCount", { count: events.length - 2 })}</span>
                      )}
                    </div>
                  </>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Map toggle: show legend below when in map mode */}
      {!showMap && !error && (
        <div className="mt-4 flex flex-wrap gap-3">
          {CATEGORIES.map((label) => (
            <div key={label} className="flex items-center gap-1.5">
              <div className="w-3 h-3 rounded-sm" style={{ backgroundColor: categoryColor(label) }} />
              <span className="text-xs text-gray-500">{t(`categories.${label}`)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
