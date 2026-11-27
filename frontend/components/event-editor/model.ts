import type { TFunction } from "i18next";
import { EventInput } from "@/lib/api/events";
import { CATEGORIES } from "@/lib/categories";
import { Event, EventCategory, EventSession } from "@/lib/types";
import { addDays, dayDiff, isValidDate, weekdayOf } from "./dates";

// Form state of the event editor, its validation (mirrors the Go API) and the EventInput it produces.

export const TITLE_MAX = 200;
export const DESCRIPTION_MAX = 5000;
export const MAX_SESSIONS = 60;
export const MAX_RANGE_DAYS = 60; // inclusive span between the first and the last day
export const MAX_PERKS = 20;
export const PERK_MAX = 80;
export const ALL_WEEKDAYS = [0, 1, 2, 3, 4, 5, 6]; // Date.getDay(): 0 = Sunday

export const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

export type ScheduleMode = "single" | "multi";

// One editable day of a multi-day event
export interface DayRow {
  key: string;
  date: string;
  startTime: string;
  endTime: string;
  extra?: boolean; // added by hand: kept even if it falls outside the range or the weekday filter
}

export interface EditorForm {
  title: string;
  category: EventCategory | "";
  description: string;
  artistName: string;
  free: boolean;
  price: string;
  perks: string[];

  mode: ScheduleMode;
  date: string; // single day
  startTime: string; // single day, and the common schedule of multi-day events
  endTime: string;
  rangeStart: string;
  rangeEnd: string;
  weekdays: number[];
  days: DayRow[];

  venue: string;
  address: string;
  lat?: number;
  lon?: number;

  imageUrl: string; // external image link, only for events without an uploaded cover
}

export type EditorErrors = Partial<Record<string, string>>;

// Row keys: deterministic for the initial state (same on server and client), a counter afterwards
let keyCounter = 0;
export function newKey(): string {
  keyCounter += 1;
  return `n${keyCounter}`;
}

export const isOvernight = (start: string, end: string) => TIME_RE.test(start) && TIME_RE.test(end) && end < start;

function mostCommonSchedule(sessions: EventSession[]): { startTime: string; endTime: string } {
  const counts = new Map<string, number>();
  for (const s of sessions) counts.set(`${s.startTime}|${s.endTime}`, (counts.get(`${s.startTime}|${s.endTime}`) ?? 0) + 1);
  let best = "";
  let bestCount = 0;
  for (const [k, n] of counts) {
    if (n > bestCount) {
      best = k;
      bestCount = n;
    }
  }
  const [startTime = "", endTime = ""] = best.split("|");
  return { startTime, endTime };
}

const bySession = (a: EventSession, b: EventSession) =>
  a.date.localeCompare(b.date) || a.startTime.localeCompare(b.startTime);

export function initialForm(event?: Event | null): EditorForm {
  const sessions: EventSession[] = event
    ? [...(event.sessions?.length ? event.sessions : [{ date: event.date, startTime: event.startTime, endTime: event.endTime }])].sort(
        bySession,
      )
    : [];
  const multi = sessions.length > 1;
  const first = sessions[0];
  const common = multi ? mostCommonSchedule(sessions) : { startTime: first?.startTime ?? "", endTime: first?.endTime ?? "" };

  return {
    title: event?.title ?? "",
    category: event?.category ?? "",
    description: event?.description ?? "",
    artistName: event?.artistName ?? "",
    free: event ? event.price === 0 : true,
    price: event && event.price > 0 ? String(event.price) : "",
    perks: event?.perks ?? [],

    mode: multi ? "multi" : "single",
    date: first?.date ?? "",
    startTime: common.startTime,
    endTime: common.endTime,
    rangeStart: first?.date ?? "",
    rangeEnd: sessions[sessions.length - 1]?.date ?? "",
    weekdays: multi ? [...new Set(sessions.map((s) => weekdayOf(s.date)))].sort() : ALL_WEEKDAYS,
    days: multi ? sessions.map((s, i) => ({ key: `s${i}`, ...s })) : [],

    venue: event?.venue ?? "",
    address: event?.address ?? "",
    lat: event?.lat,
    lon: event?.lon,

    // With an uploaded cover, imageUrl is that cover: the link field only holds external images
    imageUrl: event?.images?.some((i) => i.kind === "cover") ? "" : (event?.imageUrl ?? ""),
  };
}

export function sortDays(days: DayRow[]): DayRow[] {
  return [...days].sort((a, b) => a.date.localeCompare(b.date) || a.startTime.localeCompare(b.startTime));
}

export function rangeError(start: string, end: string): "order" | "span" | null {
  if (!isValidDate(start) || !isValidDate(end)) return null;
  if (end < start) return "order";
  if (dayDiff(start, end) + 1 > MAX_RANGE_DAYS) return "span";
  return null;
}

// Days of the range that match the weekday filter. Rows already there for a date keep their
// times (per-day adjustments survive); hand-added rows outside the range are kept too.
export function generateDays(
  start: string,
  end: string,
  weekdays: number[],
  rows: DayRow[],
  common: { startTime: string; endTime: string },
): DayRow[] {
  if (!isValidDate(start) || !isValidDate(end) || rangeError(start, end)) return rows;
  const wanted = new Set<string>();
  for (let d = start; d <= end; d = addDays(d, 1)) if (weekdays.includes(weekdayOf(d))) wanted.add(d);

  const result: DayRow[] = [];
  for (const date of wanted) {
    const existing = rows.filter((r) => r.date === date);
    if (existing.length > 0) result.push(...existing);
    else result.push({ key: newKey(), date, ...common });
  }
  for (const r of rows) if (r.extra && !wanted.has(r.date)) result.push(r);
  return sortDays(result);
}

export function toSessions(form: EditorForm): EventSession[] {
  if (form.mode === "single") return [{ date: form.date, startTime: form.startTime, endTime: form.endTime }];
  return sortDays(form.days).map(({ date, startTime, endTime }) => ({ date, startTime, endTime }));
}

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}

// Field ids double as error keys, so a field's error can be scrolled to and focused
export function validate(form: EditorForm, t: TFunction, opts: { hasUploadedCover: boolean }): EditorErrors {
  const errs: EditorErrors = {};
  const title = form.title.trim();
  if (!title) errs.title = t("editor.errors.titleRequired");
  else if (title.length > TITLE_MAX) errs.title = t("editor.errors.tooLong", { max: TITLE_MAX });
  if (!CATEGORIES.includes(form.category as EventCategory)) errs.category = t("editor.errors.categoryRequired");
  if (form.description.length > DESCRIPTION_MAX) errs.description = t("editor.errors.tooLong", { max: DESCRIPTION_MAX });
  if (!form.free) {
    const price = Number(form.price.replace(",", "."));
    if (form.price.trim() === "" || !Number.isFinite(price) || price < 0) errs.price = t("editor.errors.invalidPrice");
  }

  if (form.mode === "single") {
    if (!isValidDate(form.date)) errs.date = t("editor.errors.invalidDate");
    if (!TIME_RE.test(form.startTime)) errs.startTime = t("editor.errors.invalidTime");
    if (!TIME_RE.test(form.endTime)) errs.endTime = t("editor.errors.invalidTime");
  } else {
    if (!isValidDate(form.rangeStart)) errs.rangeStart = t("editor.errors.invalidDate");
    if (!isValidDate(form.rangeEnd)) errs.rangeEnd = t("editor.errors.invalidDate");
    const range = rangeError(form.rangeStart, form.rangeEnd);
    if (range === "order") errs.rangeEnd = t("editor.errors.rangeOrder");
    if (range === "span") errs.rangeEnd = t("editor.errors.rangeSpan", { max: MAX_RANGE_DAYS });

    if (form.days.length === 0) errs.days = t("editor.errors.noDays");
    else if (form.days.length > MAX_SESSIONS) errs.days = t("editor.errors.tooManySessions", { max: MAX_SESSIONS });
    const seen = new Set<string>();
    for (const row of sortDays(form.days)) {
      const id = `day-${row.key}`;
      if (!isValidDate(row.date)) errs[id] = t("editor.errors.invalidDate");
      else if (!TIME_RE.test(row.startTime) || !TIME_RE.test(row.endTime)) errs[id] = t("editor.errors.invalidTime");
      else if (seen.has(`${row.date}|${row.startTime}`)) errs[id] = t("editor.errors.duplicateSession");
      seen.add(`${row.date}|${row.startTime}`);
    }
  }

  if (!form.venue.trim()) errs.venue = t("editor.errors.venueRequired");
  const imageUrl = form.imageUrl.trim();
  if (!opts.hasUploadedCover && imageUrl && !isHttpUrl(imageUrl)) errs.imageUrl = t("editor.errors.invalidUrl");
  return errs;
}

// PUT replaces the whole event, so every field is always sent.
// cover: the cover in the editor, if any. An uploaded one is sent back as imageUrl so a backend
// that stores the body as-is keeps it; a cover still waiting to upload sends nothing; the
// external link field is only used without a cover.
export function toPayload(form: EditorForm, cover: { remote?: { url: string } } | null): EventInput {
  const imageUrl = cover ? cover.remote?.url : form.imageUrl.trim() || undefined;
  const perks = form.perks.map((p) => p.trim()).filter(Boolean);
  return {
    title: form.title.trim(),
    description: form.description.trim(),
    category: form.category as EventCategory,
    sessions: toSessions(form),
    venue: form.venue.trim(),
    address: form.address.trim(),
    price: form.free ? 0 : Math.round(Number(form.price.replace(",", ".")) * 100) / 100,
    imageUrl,
    artistName: form.artistName.trim() || undefined,
    perks: perks.length > 0 ? perks : undefined,
    lat: form.lat,
    lon: form.lon,
  };
}
