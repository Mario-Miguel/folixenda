"use client";

import { useState, useMemo, useEffect } from "react";
import CalendarWidget from "@/components/CalendarWidget";
import CategoryFilter from "@/components/CategoryFilter";
import EventCard from "@/components/EventCard";
import MapView from "@/components/MapView";
import { getEvents } from "@/lib/api/events";
import { Event } from "@/lib/types";
import { CalendarDays, Map } from "lucide-react";

function formatDate(date: Date) {
  return date.toLocaleDateString("en-US", {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
}

function toISODateStr(d: Date) {
  return d.toISOString().split("T")[0];
}

export default function HomePage() {
  const [selectedDate, setSelectedDate] = useState(new Date());
  const [selectedCategory, setSelectedCategory] = useState("All");
  const [showMap, setShowMap] = useState(false);
  const [allEvents, setAllEvents] = useState<Event[]>([]);

  useEffect(() => {
    getEvents()
      .then((r) => setAllEvents(r.events))
      .catch(console.error);
  }, []);

  const eventDates = useMemo(() => new Set(allEvents.map((e) => e.date)), [allEvents]);

  const dayEvents = useMemo(() => {
    const dateStr = toISODateStr(selectedDate);
    const events = allEvents.filter((e) => e.date === dateStr);
    if (selectedCategory === "All") return events;
    return events.filter((e) => e.category === selectedCategory);
  }, [selectedDate, selectedCategory, allEvents]);

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Category filter */}
      <div className="mb-8">
        <CategoryFilter selected={selectedCategory} onChange={setSelectedCategory} />
      </div>

      <div className="flex gap-8">
        {/* Left column: calendar + map toggle */}
        <aside className="hidden lg:flex flex-col gap-4 w-72 shrink-0">
          <CalendarWidget selectedDate={selectedDate} onSelect={setSelectedDate} eventDates={eventDates} />

          {/* Map / Calendar toggle */}
          <div className="flex rounded-xl border border-gray-200 overflow-hidden bg-white">
            <button
              onClick={() => setShowMap(false)}
              className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-sm font-medium transition-colors ${
                !showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
              }`}
              style={!showMap ? { backgroundColor: "#ec5b13" } : undefined}
            >
              <CalendarDays className="w-4 h-4" />
              Calendar
            </button>
            <button
              onClick={() => setShowMap(true)}
              className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-sm font-medium transition-colors ${
                showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
              }`}
              style={showMap ? { backgroundColor: "#ec5b13" } : undefined}
            >
              <Map className="w-4 h-4" />
              Map
            </button>
          </div>

          {showMap && <MapView />}
        </aside>

        {/* Right column: event list */}
        <section className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-5">
            <div>
              <h2 className="text-xl font-bold text-gray-900">{formatDate(selectedDate)}</h2>
              <p className="text-sm text-gray-500 mt-0.5">
                {dayEvents.length} {dayEvents.length === 1 ? "event" : "events"} found
              </p>
            </div>
          </div>

          {dayEvents.length === 0 ? (
            <div className="text-center py-20 text-gray-400">
              <CalendarDays className="w-12 h-12 mx-auto mb-3 opacity-30" />
              <p className="font-medium">No events on this day</p>
              <p className="text-sm mt-1">Try selecting a different date or category</p>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {dayEvents.map((event) => (
                <EventCard key={event.id} event={event} compact />
              ))}
            </div>
          )}

          {/* Mobile map */}
          <div className="mt-8 lg:hidden">
            <h3 className="font-semibold text-gray-900 mb-3">Events Map</h3>
            <MapView />
          </div>
        </section>
      </div>
    </div>
  );
}
