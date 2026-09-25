"use client";

import { Clock, MapPin } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Event } from "@/lib/types";
import { CATEGORY_COLORS } from "@/components/EventCard";

interface CompactEventListProps {
  events: Event[];
  selectedEventId: string | null;
  onSelect: (eventId: string | null) => void;
}

export default function CompactEventList({ events, selectedEventId, onSelect }: CompactEventListProps) {
  const { t } = useTranslation();

  if (events.length === 0) {
    return (
      <div className="bg-white rounded-2xl border border-gray-100 p-4 text-center text-sm text-gray-400">
        {t("common.noEventsOnDay")}
      </div>
    );
  }

  return (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
      <p className="px-4 pt-3 pb-2 text-xs font-medium text-gray-500">
        {t("common.eventCount", { count: events.length })}
      </p>
      <ul className="max-h-[28rem] overflow-y-auto divide-y divide-gray-100">
        {events.map((event) => {
          const selected = event.id === selectedEventId;
          const categoryColor = CATEGORY_COLORS[event.category] ?? "bg-gray-100 text-gray-600";

          return (
            <li key={event.id}>
              <button
                type="button"
                onClick={() => onSelect(selected ? null : event.id)}
                aria-pressed={selected}
                className={`w-full text-left px-4 py-2.5 border-l-4 transition-colors ${
                  selected ? "bg-orange-50" : "border-transparent hover:bg-gray-50"
                }`}
                style={selected ? { borderLeftColor: "#ec5b13" } : undefined}
              >
                <div className="flex items-center justify-between gap-2">
                  <h3
                    className="text-sm font-semibold text-gray-900 truncate"
                    style={selected ? { color: "#ec5b13" } : undefined}
                  >
                    {event.title}
                  </h3>
                  <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full shrink-0 ${categoryColor}`}>
                    {t(`categories.${event.category}`)}
                  </span>
                </div>
                <div className="flex items-center gap-3 mt-1 text-xs text-gray-500 min-w-0">
                  <span className="flex items-center gap-1 shrink-0">
                    <Clock className="w-3 h-3" />
                    {event.startTime}
                  </span>
                  <span className="flex items-center gap-1 min-w-0">
                    <MapPin className="w-3 h-3 shrink-0" />
                    <span className="truncate">{event.venue}</span>
                  </span>
                </div>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
