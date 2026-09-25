"use client";

import dynamic from "next/dynamic";
import { MapPin } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Event, EventCategory } from "@/lib/types";
import type { MapMarker } from "@/components/EventsMap";

const EventsMap = dynamic(() => import("@/components/EventsMap"), {
  ssr: false,
  loading: () => <div className="h-full w-full animate-pulse bg-gray-100" />,
});

const LEGEND_ITEMS: Partial<Record<EventCategory, string>> = {
  Music: "#8b5cf6",
  Theater: "#3b82f6",
  Parties: "#ec4899",
  Wellness: "#14b8a6",
  Art: "#f43f5e",
  Food: "#eab308",
};

interface MapViewProps {
  events?: Event[];
  selectedEventId?: string | null;
  onSelectEvent?: (eventId: string | null) => void;
  className?: string;
}

export default function MapView({
  events = [],
  selectedEventId = null,
  onSelectEvent,
  className = "h-72",
}: MapViewProps) {
  const { t } = useTranslation();
  const selectedEvent = events.find((e) => e.id === selectedEventId);
  // TODO: build markers from events once the API returns event coordinates (lat/lon),
  // using LEGEND_ITEMS for the category color
  const markers: MapMarker[] = events.map((e) => ({
    id: e.id,
    lat: e.lat || 43.3614,
    lon: e.lon || -5.8494,
    precise: e.lat != null && e.lon != null,
    event: e,
    color: "#14b8a6",
  }));

  return (
    <div className="bg-white rounded-2xl overflow-hidden border border-gray-100 shadow-sm">
      {/* isolate keeps Leaflet's high z-index panes from covering the navbar */}
      <div className={`relative isolate ${className}`}>
        <EventsMap markers={markers} selectedId={selectedEventId} onSelect={onSelectEvent} />
        {selectedEvent && (
          <div className="absolute z-[1000] top-3 left-14 right-3 sm:right-auto sm:max-w-xs bg-white rounded-xl shadow-md px-3 py-2 flex items-center gap-2">
            <MapPin className="w-4 h-4 shrink-0" style={{ color: "#ec5b13" }} />
            <div className="min-w-0">
              <p className="text-sm font-semibold text-gray-900 truncate">{selectedEvent.title}</p>
              <p className="text-xs text-gray-500 truncate">{selectedEvent.venue}</p>
            </div>
          </div>
        )}
      </div>

      {/* Legend */}
      <div className="px-5 py-3 border-t border-gray-100">
        <p className="text-xs font-medium text-gray-500 mb-2">{t("common.eventTypes")}</p>
        <div className="flex flex-wrap gap-3">
          {(Object.entries(LEGEND_ITEMS) as [EventCategory, string][]).map(([label, color]) => (
            <div key={label} className="flex items-center gap-1.5">
              <div className="w-3 h-3 rounded-full" style={{ backgroundColor: color }} />
              <span className="text-xs text-gray-600">{t(`categories.${label}`)}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
