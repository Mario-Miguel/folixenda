"use client";

import { useTranslation } from "react-i18next";
import type { Event } from "@/lib/types";
import {
  endsNextDay,
  formatEventDateRange,
  formatScheduleSummary,
  formatSessionTime,
  isMultiDay,
  sessionDayIndex,
  sessionOn,
} from "@/lib/dates";

// How an event's time is described in cards and lists.
// - With `day` (a day view: "/", a calendar day, a group in /my-events): that day's session times.
// - Without it: single-day events show their times; multi-day events their date range and a
//   schedule summary ("18–20 dic · Vie–Dom · 10:00–20:00").
export function useEventWhen(event: Event, day?: string) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const multiDay = isMultiDay(event);
  const session = day ? sessionOn(event, day) : undefined;

  if (session) {
    const index = multiDay ? sessionDayIndex(event, session.date) : undefined;
    return {
      time: formatSessionTime(session),
      nextDay: endsNextDay(session),
      range: null,
      dayBadge: index ? t("display.dayOfTotal", index) : null,
      dayBadgeShort: index ? t("display.dayOfTotalShort", index) : null,
      multiDay,
    };
  }

  if (multiDay) {
    return {
      time: formatScheduleSummary(event, lang, t("display.variableSchedule")),
      nextDay: false,
      range: formatEventDateRange(event, lang),
      dayBadge: null,
      dayBadgeShort: null,
      multiDay,
    };
  }

  const first = { date: event.date, startTime: event.startTime, endTime: event.endTime };
  return {
    time: formatSessionTime(event.sessions?.[0] ?? first),
    nextDay: endsNextDay(event.sessions?.[0] ?? first),
    range: null,
    dayBadge: null,
    dayBadgeShort: null,
    multiDay,
  };
}

// Small pill: "Día 2 de 3"
export function DayBadge({ label }: { label: string | null }) {
  if (!label) return null;
  return (
    <span
      className="text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-orange-50 shrink-0 whitespace-nowrap"
      style={{ color: "#ec5b13" }}
    >
      {label}
    </span>
  );
}

// "+1 día" after a time range that ends after midnight
export function NextDayNote({ show }: { show: boolean }) {
  const { t } = useTranslation();
  if (!show) return null;
  return <span className="text-gray-400">({t("display.endsNextDay")})</span>;
}
