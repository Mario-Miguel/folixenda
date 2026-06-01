"use client";

import { useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

const DAYS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

interface CalendarWidgetProps {
  selectedDate: Date;
  onSelect: (date: Date) => void;
  eventDates?: Set<string>; // ISO date strings with events
}

function toISODateStr(d: Date) {
  return d.toISOString().split("T")[0];
}

export default function CalendarWidget({
  selectedDate,
  onSelect,
  eventDates = new Set(),
}: CalendarWidgetProps) {
  const [viewDate, setViewDate] = useState(
    new Date(selectedDate.getFullYear(), selectedDate.getMonth(), 1)
  );

  const year = viewDate.getFullYear();
  const month = viewDate.getMonth();
  const firstDay = new Date(year, month, 1).getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const today = toISODateStr(new Date());

  const prevMonth = () => setViewDate(new Date(year, month - 1, 1));
  const nextMonth = () => setViewDate(new Date(year, month + 1, 1));

  const cells: (number | null)[] = [
    ...Array(firstDay).fill(null),
    ...Array.from({ length: daysInMonth }, (_, i) => i + 1),
  ];

  return (
    <div className="bg-white rounded-2xl p-5 shadow-sm border border-gray-100 w-full">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <span className="font-semibold text-gray-900 text-sm">
          {MONTHS[month]} {year}
        </span>
        <div className="flex items-center gap-1">
          <button
            onClick={prevMonth}
            className="w-7 h-7 flex items-center justify-center rounded-full hover:bg-gray-100 transition-colors text-gray-500"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <button
            onClick={nextMonth}
            className="w-7 h-7 flex items-center justify-center rounded-full hover:bg-gray-100 transition-colors text-gray-500"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Day headers */}
      <div className="grid grid-cols-7 mb-2">
        {DAYS.map((d) => (
          <div key={d} className="text-center text-xs text-gray-400 font-medium py-1">
            {d}
          </div>
        ))}
      </div>

      {/* Day cells */}
      <div className="grid grid-cols-7">
        {cells.map((day, i) => {
          if (!day) return <div key={`empty-${i}`} />;

          const dateStr = `${year}-${String(month + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
          const isSelected = toISODateStr(selectedDate) === dateStr;
          const isToday = today === dateStr;
          const hasEvent = eventDates.has(dateStr);

          return (
            <div key={dateStr} className="flex flex-col items-center py-0.5">
              <button
                onClick={() => onSelect(new Date(dateStr + "T12:00:00"))}
                className={`w-8 h-8 flex items-center justify-center rounded-full text-sm font-medium transition-all ${
                  isSelected
                    ? "text-white shadow-sm"
                    : isToday
                    ? "text-primary font-bold"
                    : "text-gray-700 hover:bg-gray-100"
                }`}
                style={
                  isSelected
                    ? { backgroundColor: "#ec5b13" }
                    : isToday
                    ? { color: "#ec5b13" }
                    : undefined
                }
              >
                {day}
              </button>
              {hasEvent && (
                <div
                  className="w-1 h-1 rounded-full mt-0.5"
                  style={{ backgroundColor: isSelected ? "#ec5b13" : "#9ca3af" }}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
