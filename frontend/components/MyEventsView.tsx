"use client";

import { useState } from "react";
import { useTranslation } from "react-i18next";
import EventCard from "@/components/EventCard";
import MapView from "@/components/MapView";
import { formatLongDate } from "@/i18n/format";
import { Event } from "@/lib/types";

export interface EventDay {
  date: string; // YYYY-MM-DD
  events: Event[];
}

// Saved events grouped by day next to a map; selecting a card highlights its pin (and vice versa)
export default function MyEventsView({ days }: { days: EventDay[] }) {
  const { t, i18n } = useTranslation();
  const [selectedEventId, setSelectedEventId] = useState<string | null>(null);

  const toggle = (id: string) => setSelectedEventId((current) => (current === id ? null : id));

  return (
    <div className="flex flex-col lg:flex-row gap-8">
      {/* Event list grouped by day */}
      <div className="flex-1 min-w-0 space-y-8">
        {days.map(({ date, events }) => (
          <section key={date}>
            <div className="mb-4">
              <h2 className="text-xl font-bold text-gray-900">
                {formatLongDate(new Date(date + "T12:00:00"), i18n.language)}
              </h2>
              <p className="text-sm text-gray-500 mt-0.5">{t("common.eventCount", { count: events.length })}</p>
            </div>
            <div className="flex flex-col gap-3">
              {events.map((event) => (
                <EventCard
                  key={event.id}
                  event={event}
                  compact
                  selected={event.id === selectedEventId}
                  onSelect={() => toggle(event.id)}
                  href={`/events/${event.id}?from=my-events`}
                />
              ))}
            </div>
          </section>
        ))}
      </div>

      {/* Map of all upcoming saved events: above the list on mobile, sticky column on desktop */}
      <aside className="order-first lg:order-none lg:w-[28rem] shrink-0">
        <div className="lg:sticky lg:top-24">
          <MapView
            events={days.flatMap((d) => d.events)}
            selectedEventId={selectedEventId}
            onSelectEvent={setSelectedEventId}
            className="h-72 lg:h-[32rem]"
          />
        </div>
      </aside>
    </div>
  );
}
