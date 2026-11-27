// Date helpers shared by the pages. Event dates are plain "YYYY-MM-DD" strings in Spanish local time.

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

// Current wall-clock time in the events' time zone as "YYYY-MM-DDTHH:MM",
// independent of the time zone of the machine running the code
function nowInEventTimeZone(now: Date): string {
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", {
      timeZone: EVENT_TIME_ZONE,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    })
      .formatToParts(now)
      .map((p) => [p.type, p.value]),
  );
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

function addDays(date: string, days: number): string {
  const d = new Date(`${date}T12:00:00`);
  d.setDate(d.getDate() + days);
  return toISODateStr(d);
}

// True once the event's end time has gone by in Spain. Without an end time the event lasts
// the whole day; an end time earlier than the start (e.g. 22:00–02:00) ends the next day.
export function hasEventEnded(
  event: { date: string; startTime?: string; endTime?: string },
  now: Date = new Date(),
): boolean {
  const endTime = event.endTime || "23:59";
  const endDate = event.startTime && endTime < event.startTime ? addDays(event.date, 1) : event.date;
  return `${endDate}T${endTime}` < nowInEventTimeZone(now);
}
