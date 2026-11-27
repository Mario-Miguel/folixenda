"use client";

import { useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
import CalendarWidget from "@/components/CalendarWidget";
import CategoryFilter from "@/components/CategoryFilter";
import EventCard from "@/components/EventCard";
import MapView from "@/components/MapView";
import { CalendarDays } from "lucide-react";
import SwitchMapWidget from "@/components/SwitchMapWidget";
import CompactEventList from "@/components/CompactEventList";
import LoadError from "@/components/LoadError";
import { formatLongDate } from "@/i18n/format";
import { monthRange, startOfMonth, toISODateStr } from "@/lib/dates";
import { useEvents } from "@/lib/hooks/useEvents";

export default function HomePage() {
  const { t, i18n } = useTranslation();
  const [selectedDate, setSelectedDate] = useState(() => new Date());
  const [calendarMonth, setCalendarMonth] = useState(() => startOfMonth(new Date()));
  const [selectedCategory, setSelectedCategory] = useState("All");
  const [showMap, setShowMap] = useState(false);
  const [selectedEventId, setSelectedEventId] = useState<string | null>(null);

  // Events of the selected day, filtered by the API
  const day = useEvents({ date: toISODateStr(selectedDate), category: selectedCategory });
  // Events of the month shown in the mini calendar, only to mark the days that have events
  const month = useEvents({ ...monthRange(calendarMonth), category: selectedCategory });

  const eventDates = useMemo(() => new Set(month.events.map((e) => e.date)), [month.events]);
  const dayEvents = day.events;

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Category filter */}
      <div className="mb-8">
        <CategoryFilter selected={selectedCategory} onChange={setSelectedCategory} />
      </div>

      <div className="flex gap-8">
        {/* Left column: calendar + map toggle */}
        <aside className="hidden lg:flex flex-col gap-4 w-72 shrink-0">
          <CalendarWidget
            selectedDate={selectedDate}
            onSelect={setSelectedDate}
            eventDates={eventDates}
            month={calendarMonth}
            onMonthChange={setCalendarMonth}
          />

          {/* Map / Calendar toggle */}
          <SwitchMapWidget setShowMap={setShowMap} showMap={showMap} />
          {showMap && !day.loading && !day.error && (
            <CompactEventList events={dayEvents} selectedEventId={selectedEventId} onSelect={setSelectedEventId} />
          )}
        </aside>

        {/* Right column: event list */}
        <section className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-5">
            <div>
              <h2 className="text-xl font-bold text-gray-900">{formatLongDate(selectedDate, i18n.language)}</h2>
              {!day.loading && !day.error && (
                <p className="text-sm text-gray-500 mt-0.5">{t("common.eventsFound", { count: dayEvents.length })}</p>
              )}
            </div>
          </div>

          {/* Desktop map view replaces the main event list */}
          {showMap && (
            <div className="hidden lg:block">
              <MapView
                events={dayEvents}
                selectedEventId={selectedEventId}
                onSelectEvent={setSelectedEventId}
                className="h-[32rem]"
              />
            </div>
          )}

          {day.error ? (
            <LoadError onRetry={day.retry} />
          ) : day.loading ? (
            <p className={`text-center py-20 text-gray-400 ${showMap ? "lg:hidden" : ""}`}>{t("common.loading")}</p>
          ) : dayEvents.length === 0 ? (
            <div className={`text-center py-20 text-gray-400 ${showMap ? "lg:hidden" : ""}`}>
              <CalendarDays className="w-12 h-12 mx-auto mb-3 opacity-30" />
              <p className="font-medium">{t("common.noEventsOnDay")}</p>
              <p className="text-sm mt-1">{t("common.tryDifferentFilter")}</p>
            </div>
          ) : (
            <div className={`flex flex-col gap-3 ${showMap ? "lg:hidden" : ""}`}>
              {dayEvents.map((event) => (
                <EventCard key={event.id} event={event} compact />
              ))}
            </div>
          )}

          {/* Mobile map */}
          <div className="mt-8 lg:hidden">
            <h3 className="font-semibold text-gray-900 mb-3">{t("common.eventsMap")}</h3>
            <MapView events={dayEvents} selectedEventId={selectedEventId} onSelectEvent={setSelectedEventId} />
          </div>
        </section>
      </div>
    </div>
  );
}
