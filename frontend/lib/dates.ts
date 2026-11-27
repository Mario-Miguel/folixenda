// Date helpers shared by the pages. Event dates are plain "YYYY-MM-DD" strings and times "HH:MM"
// strings, both wall-clock values in Spain (Europe/Madrid).
//
// An event has one or more sessions (one per day, sorted). A session whose end time is earlier
// than its start time (e.g. 22:00–02:00) ends the next day. `event.date/startTime/endTime` mirror
// the FIRST session only: anything about "when is/was the event" must look at `sessions`.

import type { EventSession } from "@/lib/types";

const pad = (n: number) => String(n).padStart(2, "0");

// "YYYY-MM-DD" from the date's local parts (toISOString() would give the UTC day)
export function toISODateStr(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// Inclusive first/last day of the month that contains `d`, as "YYYY-MM-DD"
export function monthRange(d: Date): { from: string; to: string } {
  const year = d.getFullYear();
  const month = d.getMonth();
  return {
    from: toISODateStr(new Date(year, month, 1)),
    to: toISODateStr(new Date(year, month + 1, 0)),
  };
}

export function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1);
}

// Events take place in Asturias: their date/start/end are wall-clock times in this zone
export const EVENT_TIME_ZONE = "Europe/Madrid";

const madridParts = new Intl.DateTimeFormat("en-CA", {
  timeZone: EVENT_TIME_ZONE,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

function wallClockParts(now: Date): Record<string, string> {
  return Object.fromEntries(madridParts.formatToParts(now).map((p) => [p.type, p.value]));
}

// Current wall-clock time in the events' time zone as "YYYY-MM-DDTHH:MM",
// independent of the time zone of the machine running the code
function nowInEventTimeZone(now: Date): string {
  const p = wallClockParts(now);
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`;
}

// Today's date in Spain as "YYYY-MM-DD"
export function todayInEventTimeZone(now: Date = new Date()): string {
  return nowInEventTimeZone(now).slice(0, 10);
}

// Calendar arithmetic on "YYYY-MM-DD" strings (UTC math, so no DST or machine time zone involved)
export function addDays(date: string, days: number): string {
  const [y, m, d] = date.split("-").map(Number);
  const t = new Date(Date.UTC(y, m - 1, d + days));
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}`;
}

// A "YYYY-MM-DD" as a Date at 00:00 UTC: format it with `timeZone: "UTC"` to get that same day
function utcDay(date: string): Date {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d));
}

// ---------------------------------------------------------------------------------------------
// Sessions

// Minimal event shape the helpers need. `sessions` is always present in API data; the fallback
// to date/startTime/endTime keeps older payloads (and partial objects) working.
export interface EventTiming {
  date: string;
  startTime?: string;
  endTime?: string;
  endDate?: string;
  sessions?: EventSession[];
}

// The event's sessions, sorted; never empty
export function eventSessions(event: EventTiming): EventSession[] {
  if (event.sessions && event.sessions.length > 0) return event.sessions;
  return [{ date: event.date, startTime: event.startTime ?? "", endTime: event.endTime ?? "" }];
}

// True when the session ends after midnight (end time earlier than start time)
export function endsNextDay(session: EventSession): boolean {
  return !!session.startTime && !!session.endTime && session.endTime < session.startTime;
}

// Wall-clock start/end as "YYYY-MM-DDTHH:MM" (sortable). Without a start time the session starts at
// 00:00; without an end time it lasts the whole day (23:59).
function startKey(s: EventSession): string {
  return `${s.date}T${s.startTime || "00:00"}`;
}
function endKey(s: EventSession): string {
  return `${endsNextDay(s) ? addDays(s.date, 1) : s.date}T${s.endTime || "23:59"}`;
}

// Offset (ms) of Madrid from UTC at the instant `ts`
function madridOffset(ts: number): number {
  const p = wallClockParts(new Date(ts));
  const asUTC = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute);
  return asUTC - Math.floor(ts / 60000) * 60000;
}

// Instant of a Madrid wall-clock "YYYY-MM-DDTHH:MM"
function madridInstant(key: string): Date {
  const [date, time] = key.split("T");
  const [y, m, d] = date.split("-").map(Number);
  const [hh, mm] = time.split(":").map(Number);
  const asUTC = Date.UTC(y, m - 1, d, hh, mm);
  // Two passes settle the offset around DST changes
  let ts = asUTC - madridOffset(asUTC);
  ts = asUTC - madridOffset(ts);
  return new Date(ts);
}

// Absolute start/end instants of a session (Madrid wall clock; overnight sessions end next day)
export function sessionStart(session: EventSession): Date {
  return madridInstant(startKey(session));
}
export function sessionEnd(session: EventSession): Date {
  return madridInstant(endKey(session));
}

export function hasSessionEnded(session: EventSession, now: Date = new Date()): boolean {
  return endKey(session) < nowInEventTimeZone(now);
}

// The session that takes place on `isoDate` (the day it starts), if any
export function sessionOn(event: EventTiming, isoDate: string): EventSession | undefined {
  return eventSessions(event).find((s) => s.date === isoDate);
}

// The session in progress or the next one to start; undefined once the whole event has ended
export function nextSession(event: EventTiming, now: Date = new Date()): EventSession | undefined {
  const current = nowInEventTimeZone(now);
  return eventSessions(event).find((s) => endKey(s) >= current);
}

export function lastSession(event: EventTiming): EventSession {
  return eventSessions(event).at(-1)!;
}

// True once the LAST session's end time has gone by in Spain, whatever the machine's time zone
export function hasEventEnded(event: EventTiming, now: Date = new Date()): boolean {
  return hasSessionEnded(lastSession(event), now);
}

// Distinct session days, sorted
export function sessionDates(event: EventTiming): string[] {
  return [...new Set(eventSessions(event).map((s) => s.date))];
}

// True when the event takes place on more than one day (an overnight single session is not)
export function isMultiDay(event: EventTiming): boolean {
  return sessionDates(event).length > 1;
}

// Position of `isoDate` among the event's days ("Day 2 of 3"); undefined if it isn't one of them
export function sessionDayIndex(event: EventTiming, isoDate: string): { day: number; total: number } | undefined {
  const dates = sessionDates(event);
  const i = dates.indexOf(isoDate);
  return i < 0 ? undefined : { day: i + 1, total: dates.length };
}

// Comparators for "YYYY-MM-DDTHH:MM" ordering
export const compareSessions = (a: EventSession, b: EventSession) => startKey(a).localeCompare(startKey(b));
export const compareSessionEnds = (a: EventSession, b: EventSession) => endKey(a).localeCompare(endKey(b));

// ---------------------------------------------------------------------------------------------
// Formatting (Intl, active language)

function capitalize(s: string) {
  return s.charAt(0).toLocaleUpperCase() + s.slice(1);
}

// "20:00–23:00", "22:00–02:00", "20:00" (no end time)
export function formatSessionTime(session: Pick<EventSession, "startTime" | "endTime">): string {
  return [session.startTime, session.endTime].filter(Boolean).join("–");
}

// "Viernes, 18 de diciembre" (with `withYear`, "... de 2026")
export function formatSessionDate(isoDate: string, lang: string, withYear = false): string {
  return capitalize(
    new Intl.DateTimeFormat(lang, {
      timeZone: "UTC",
      weekday: "long",
      day: "numeric",
      month: "long",
      ...(withYear && { year: "numeric" }),
    }).format(utcDay(isoDate)),
  );
}

// "Vie", "Fri"
export function formatWeekdayShort(isoDate: string, lang: string): string {
  return capitalize(new Intl.DateTimeFormat(lang, { timeZone: "UTC", weekday: "short" }).format(utcDay(isoDate)));
}

// "20 dic" / "20–22 dic" / "30 nov – 2 dic" / "30 dic 2026 – 2 ene 2027" (en: "Dec 20 – 22").
// The year is shown when the range crosses years, or always with `withYear`.
export function formatEventDateRange(event: EventTiming, lang: string, withYear = false): string {
  const dates = sessionDates(event);
  const first = dates[0];
  const last = dates.at(-1)!;
  const year = withYear || first.slice(0, 4) !== last.slice(0, 4);
  const fmt = new Intl.DateTimeFormat(lang, {
    timeZone: "UTC",
    day: "numeric",
    month: "short",
    ...(year && { year: "numeric" }),
  });
  if (first === last) return fmt.format(utcDay(first));
  // formatRange collapses shared parts ("20–22 dic"); fall back to "a – b" where unsupported
  return typeof fmt.formatRange === "function"
    ? fmt.formatRange(utcDay(first), utcDay(last))
    : `${fmt.format(utcDay(first))} – ${fmt.format(utcDay(last))}`;
}

// "Vie–Dom · 10:00–20:00" for consecutive days with the same hours; "Vie, Dom, Mar · ..." when the
// days aren't consecutive (more than 4 non-consecutive days: just the range of weekdays is not
// meaningful, so only the hours are shown); `variableLabel` (e.g. "Horario variable") replaces
// the hours when they differ between sessions.
export function formatScheduleSummary(event: EventTiming, lang: string, variableLabel: string): string {
  const sessions = eventSessions(event);
  const dates = sessionDates(event);
  const consecutive = dates.every((d, i) => i === 0 || addDays(dates[i - 1], 1) === d);

  let days: string | null;
  if (dates.length === 1) days = formatWeekdayShort(dates[0], lang);
  else if (consecutive && dates.length <= 7)
    days = `${formatWeekdayShort(dates[0], lang)}–${formatWeekdayShort(dates.at(-1)!, lang)}`;
  else if (dates.length <= 4) days = dates.map((d) => formatWeekdayShort(d, lang)).join(", ");
  else days = null;

  const time = formatSessionTime(sessions[0]);
  const sameHours = sessions.every((s) => s.startTime === sessions[0].startTime && s.endTime === sessions[0].endTime);
  const hours = sameHours ? time : variableLabel;

  return [days, hours].filter(Boolean).join(" · ");
}
