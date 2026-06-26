"use client";

import { MapPin } from "lucide-react";

const LEGEND_ITEMS = [
  { label: "Music", color: "#8b5cf6" },
  { label: "Theater", color: "#3b82f6" },
  { label: "Parties", color: "#ec4899" },
  { label: "Wellness", color: "#14b8a6" },
  { label: "Art", color: "#f43f5e" },
  { label: "Food", color: "#eab308" },
];

export default function MapView() {
  // TODO: add leaflet map, center in user's location or in Oviedo. Show pins on events of the day
  return (
    <div className="bg-white rounded-2xl overflow-hidden border border-gray-100 shadow-sm">
      {/* Map placeholder */}
      <div className="relative bg-gradient-to-br from-green-50 via-blue-50 to-indigo-100 h-72 flex items-center justify-center"></div>

      {/* Legend */}
      <div className="px-5 py-3 border-t border-gray-100">
        <p className="text-xs font-medium text-gray-500 mb-2">Event types</p>
        <div className="flex flex-wrap gap-3">
          {LEGEND_ITEMS.map(({ label, color }) => (
            <div key={label} className="flex items-center gap-1.5">
              <div className="w-3 h-3 rounded-full" style={{ backgroundColor: color }} />
              <span className="text-xs text-gray-600">{label}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
