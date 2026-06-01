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
  return (
    <div className="bg-white rounded-2xl overflow-hidden border border-gray-100 shadow-sm">
      {/* Map placeholder */}
      <div className="relative bg-gradient-to-br from-green-50 via-blue-50 to-indigo-100 h-72 flex items-center justify-center">
        {/* Fake map pins */}
        <div className="absolute inset-0 p-6">
          {[
            { top: "20%", left: "30%", color: "#8b5cf6" },
            { top: "45%", left: "55%", color: "#ec4899" },
            { top: "60%", left: "25%", color: "#3b82f6" },
            { top: "30%", left: "70%", color: "#f43f5e" },
            { top: "70%", left: "65%", color: "#eab308" },
            { top: "55%", left: "40%", color: "#ec5b13" },
          ].map((pin, i) => (
            <button
              key={i}
              className="absolute -translate-x-1/2 -translate-y-1/2 flex flex-col items-center group"
              style={{ top: pin.top, left: pin.left }}
            >
              <div
                className="w-8 h-8 rounded-full flex items-center justify-center shadow-md transition-transform group-hover:scale-110"
                style={{ backgroundColor: pin.color }}
              >
                <MapPin className="w-4 h-4 text-white" />
              </div>
              <div className="w-2 h-2 rounded-full mt-0.5" style={{ backgroundColor: pin.color }} />
            </button>
          ))}
        </div>

        {/* Overlay text */}
        <div className="text-center text-gray-400 select-none">
          <MapPin className="w-8 h-8 mx-auto mb-2 opacity-30" />
          <p className="text-sm opacity-50">Click pins to see event details</p>
        </div>
      </div>

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
