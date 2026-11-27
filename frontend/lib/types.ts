export type EventCategory = "Music" | "Theater" | "Parties" | "Sports" | "Food" | "Art" | "Wellness";

export interface Event {
  id: string;
  title: string;
  description: string;
  category: EventCategory;
  // First session (kept so single-day UIs keep working). For multi-day events use `sessions`.
  date: string; // YYYY-MM-DD of the first session
  startTime: string; // "HH:MM" of the first session
  endTime: string; // "HH:MM" of the first session
  endDate: string; // YYYY-MM-DD of the last session (== date for single-day events)
  sessions: EventSession[]; // always ≥ 1, sorted by date then startTime
  venue: string;
  address: string;
  price: number;
  imageUrl?: string; // cover image URL (uploaded cover, or a scraped image URL)
  images: EventImage[]; // uploaded images, cover first then gallery by position ([] for scraped events)
  artistName?: string;
  perks?: string[];
  lat?: number;
  lon?: number;
  publisherId?: string; // owner, when a publisher created it (scraped events have none)
  organizer?: Organizer; // only on GET /api/events/{id}, when publisherId is set
}

// One day of an event. endTime < startTime means it ends after midnight (next day).
export interface EventSession {
  date: string; // YYYY-MM-DD
  startTime: string; // "HH:MM"
  endTime: string; // "HH:MM"
}

export type EventImageKind = "cover" | "gallery";

// An image uploaded by the publisher, stored in object storage (MinIO / S3)
export interface EventImage {
  id: string;
  url: string; // public URL, ready for <img src>
  kind: EventImageKind; // at most 1 cover; up to 10 gallery images
  position: number; // gallery order, 0-based (cover is always 0)
  width?: number;
  height?: number;
}

// Address search result (GET /api/geocode/search, /api/geocode/reverse)
export interface GeocodeResult {
  label: string; // full display text, e.g. "Calle Uría 10, Oviedo, Asturias"
  address: string; // street + number, suitable for Event.address
  municipality?: string;
  province?: string;
  postalCode?: string;
  lat: number;
  lon: number;
}

// Public data of the publisher that owns an event
export interface Organizer {
  id: string;
  orgName: string;
  website?: string;
  phone?: string;
  description?: string;
}

// A publisher's own event, with how many users saved it (GET /api/me/events)
export interface PublisherEvent extends Event {
  savesCount: number;
}

// Organization data of a publisher account (GET/PUT /api/me/publisher-profile)
export interface PublisherProfile {
  userId: string;
  orgName: string; // required, 2–120 chars
  website?: string; // http(s) URL
  phone?: string; // max 30 chars
  description?: string; // max 1000 chars
  createdAt: string; // RFC 3339
  updatedAt: string;
}

export type PublisherProfileInput = Pick<PublisherProfile, "orgName" | "website" | "phone" | "description">;

// Stored preferences only: nothing is sent yet (GET via /api/me, PUT /api/me/notifications)
export interface NotificationPreferences {
  savedEventReminders: boolean; // remind me before events I saved (default true)
  categoryNews: boolean; // new events in my favourite categories (default false)
  categories: EventCategory[]; // favourite categories for categoryNews
  newsletter: boolean; // occasional folixenda news (default false)
  publisherActivity: boolean; // publishers: activity on my events, e.g. saves (default true)
}

// GET /api/me
export interface MeResponse {
  user: User;
  publisherProfile: PublisherProfile | null; // null for consumers or publishers who haven't filled it
  notifications: NotificationPreferences; // defaults when never saved
}

export interface ApiEventsResponse {
  events: Event[];
  total: number;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: "admin" | "consumer" | "publisher";
  subscriptionType?: string;
  paymentMethod?: string;
  location?: number[];
  eventPreferences?: string[];
}

export interface SavedEvent extends Event {
  savedAt: string;
}

export interface UserSavedEvents {
  userId: string;
  savedEvents: SavedEvent[];
}
