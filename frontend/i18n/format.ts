// Locale-aware date helpers: month/day names come from Intl instead of the dictionaries

function capitalize(s: string) {
  return s.charAt(0).toLocaleUpperCase() + s.slice(1);
}

export function formatLongDate(date: Date, locale: string, withYear = false) {
  return capitalize(
    date.toLocaleDateString(locale, {
      weekday: "long",
      month: "long",
      day: "numeric",
      ...(withYear && { year: "numeric" }),
    }),
  );
}

export function formatMonthYear(year: number, month: number, locale: string) {
  return capitalize(new Date(year, month, 1).toLocaleDateString(locale, { month: "long", year: "numeric" }));
}

// Weekday names starting on Sunday (2023-01-01 was a Sunday)
export function weekdayNames(locale: string, format: "short" | "narrow") {
  return Array.from({ length: 7 }, (_, i) =>
    capitalize(new Date(2023, 0, 1 + i).toLocaleDateString(locale, { weekday: format })),
  );
}
