import { EventCategory } from "./types";

// Single source of truth for category colors: badge classes (light background + text)
// and a solid hex color (calendar chips, map markers, legends)
export const CATEGORY_STYLES: Record<EventCategory, { badge: string; color: string }> = {
  Music: { badge: "bg-purple-100 text-purple-700", color: "#8b5cf6" },
  Theater: { badge: "bg-blue-100 text-blue-700", color: "#3b82f6" },
  Parties: { badge: "bg-pink-100 text-pink-700", color: "#ec4899" },
  Sports: { badge: "bg-green-100 text-green-700", color: "#22c55e" },
  Food: { badge: "bg-yellow-100 text-yellow-700", color: "#eab308" },
  Art: { badge: "bg-rose-100 text-rose-700", color: "#f43f5e" },
  Wellness: { badge: "bg-teal-100 text-teal-700", color: "#14b8a6" },
};

export const CATEGORIES = Object.keys(CATEGORY_STYLES) as EventCategory[];

const FALLBACK = { badge: "bg-gray-100 text-gray-600", color: "#6b7280" };

// Lookups tolerate unknown values coming from the API
export function categoryBadgeClass(category: string): string {
  return (CATEGORY_STYLES[category as EventCategory] ?? FALLBACK).badge;
}

export function categoryColor(category: string): string {
  return (CATEGORY_STYLES[category as EventCategory] ?? FALLBACK).color;
}
