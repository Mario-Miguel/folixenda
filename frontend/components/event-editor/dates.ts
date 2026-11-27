// Plain "YYYY-MM-DD" date arithmetic for the editor (noon avoids DST edge cases)

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const pad = (n: number) => String(n).padStart(2, "0");

const toDate = (value: string) => new Date(`${value}T12:00:00`);
const toStr = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

export function isValidDate(value: string): boolean {
  if (!DATE_RE.test(value)) return false;
  const d = toDate(value);
  return !Number.isNaN(d.getTime()) && toStr(d) === value;
}

export function addDays(value: string, days: number): string {
  const d = toDate(value);
  d.setDate(d.getDate() + days);
  return toStr(d);
}

export function dayDiff(from: string, to: string): number {
  return Math.round((toDate(to).getTime() - toDate(from).getTime()) / 86_400_000);
}

export function weekdayOf(value: string): number {
  return toDate(value).getDay();
}

// "vie, 5 dic 2026"
export function formatDayLabel(value: string, locale: string): string {
  if (!isValidDate(value)) return value;
  const s = toDate(value).toLocaleDateString(locale, { weekday: "short", day: "numeric", month: "short", year: "numeric" });
  return s.charAt(0).toLocaleUpperCase() + s.slice(1);
}

// Monday-first weekday labels for the filter, as [dayIndex, label]
export function mondayFirstWeekdays(locale: string): [number, string][] {
  return [1, 2, 3, 4, 5, 6, 0].map((day) => {
    const s = new Date(2023, 0, 1 + day).toLocaleDateString(locale, { weekday: "short" }); // 2023-01-01 = Sunday
    return [day, s.charAt(0).toLocaleUpperCase() + s.slice(1)];
  });
}
