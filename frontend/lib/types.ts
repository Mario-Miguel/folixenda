export type EventCategory =
  | "Music"
  | "Theater"
  | "Parties"
  | "Sports"
  | "Food"
  | "Art"
  | "Wellness";

export interface Event {
  id: string;
  title: string;
  description: string;
  category: EventCategory;
  date: string; // ISO date string
  startTime: string; // "HH:MM"
  endTime: string; // "HH:MM"
  venue: string;
  address: string;
  price: number;
  imageUrl?: string;
  artistName?: string;
  perks?: string[];
  isSaved?: boolean;
}

export interface ApiEventsResponse {
  events: Event[];
  total: number;
}
