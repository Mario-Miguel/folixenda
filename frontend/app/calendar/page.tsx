"use client";

import { useState, useMemo, useEffect } from "react";
import Link from "next/link";
import { ChevronLeft, ChevronRight, Map as MapIcon, CalendarDays } from "lucide-react";
import CategoryFilter from "@/components/CategoryFilter";
import MapView from "@/components/MapView";
import { getEvents } from "@/lib/api/events";
import { Event } from "@/lib/types";

const DAYS_SHORT = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const CATEGORY_DOT_COLORS: Record<string, string> = {
  Music: "#8b5cf6",
  Theater: "#3b82f6",
  Parties: "#ec4899",
  Sports: "#22c55e",
  Food: "#eab308",
  Art: "#f43f5e",
  Wellness: "#14b8a6",
};

function toISODateStr(year: number, month: number, day: number) {
  return `${year}-${String(month + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

export default function CalendarPage() {
  const [viewDate, setViewDate] = useState(new Date(2024, 9, 1)); // October 2024
  const [selectedCategory, setSelectedCategory] = useState("All");
  const [showMap, setShowMap] = useState(false);
  const [allEvents, setAllEvents] = useState<Event[]>([]);

  useEffect(() => {
    getEvents()
      .then((r) => setAllEvents(r.events))
      .catch(console.error);
  }, []);

  const year = viewDate.getFullYear();
  const month = viewDate.getMonth();
  const firstDay = new Date(year, month, 1).getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();

  const prevMonth = () => setViewDate(new Date(year, month - 1, 1));
  const nextMonth = () => setViewDate(new Date(year, month + 1, 1));

  const cells = useMemo(() => {
    const result: { day: number | null; dateStr: string | null; events: Event[] }[] = [];

    for (let i = 0; i < firstDay; i++) {
      result.push({ day: null, dateStr: null, events: [] });
    }

    for (let d = 1; d <= daysInMonth; d++) {
      const dateStr = toISODateStr(year, month, d);
      let events = allEvents.filter((e) => e.date === dateStr);
      if (selectedCategory !== "All") {
        events = events.filter((e) => e.category === selectedCategory);
      }
      result.push({ day: d, dateStr, events });
    }

    return result;
  }, [year, month, firstDay, daysInMonth, selectedCategory, allEvents]);

  return (
    <div className="max-w-7xl mx-auto px-6 py-8">
      {/* Tab bar */}
      <div className="flex items-center gap-1 mb-6 border-b border-gray-200">
        {[
          { href: "/calendar", label: "Calendar", active: true },
          { href: "/", label: "Discover", active: false },
          { href: "/my-events", label: "My Tickets", active: false },
        ].map(({ href, label, active }) => (
          <Link
            key={label}
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
            className="w-8 h-8 flex items-center justify-center rounded-full border border-gray-200 hover:bg-gray-50 transition-colors text-gray-600"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <h2 className="text-lg font-bold text-gray-900 min-w-[160px] text-center">
            {MONTHS[month]} {year}
          </h2>
          <button
            onClick={nextMonth}
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
            Calendar
          </button>
          <button
            onClick={() => setShowMap(true)}
            className={`flex items-center gap-1.5 px-4 py-2 text-sm font-medium transition-colors ${
              showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
            }`}
            style={showMap ? { backgroundColor: "#ec5b13" } : undefined}
          >
            <MapIcon className="w-4 h-4" />
            Map
          </button>
        </div>
      </div>

      {showMap ? (
        <MapView />
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          {/* Day headers */}
          <div className="grid grid-cols-7 border-b border-gray-100">
            {DAYS_SHORT.map((day) => (
              <div key={day} className="py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wide">
                {day}
              </div>
            ))}
          </div>

          {/* Calendar grid */}
          <div className="grid grid-cols-7 divide-x divide-gray-50">
            {cells.map(({ day, dateStr, events }, i) => (
              <div
                key={i}
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
                            style={{ backgroundColor: CATEGORY_DOT_COLORS[event.category] ?? "#6b7280" }}
                          >
                            {event.title}
                          </div>
                        </Link>
                      ))}
                      {events.length > 2 && (
                        <span className="text-xs text-gray-400 font-medium">+{events.length - 2} more</span>
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
      {!showMap && (
        <div className="mt-4 flex flex-wrap gap-3">
          {Object.entries(CATEGORY_DOT_COLORS).map(([label, color]) => (
            <div key={label} className="flex items-center gap-1.5">
              <div className="w-3 h-3 rounded-sm" style={{ backgroundColor: color }} />
              <span className="text-xs text-gray-500">{label}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
