import { ApiEventsResponse, Event, EventImage, EventImageKind } from "../types";
import { apiClient } from "./api";

export interface EventsQuery {
  date?: string; // YYYY-MM-DD: events on this day
  from?: string; // YYYY-MM-DD, inclusive range start (use with `to`)
  to?: string; // YYYY-MM-DD, inclusive range end
  category?: string; // omit (or "All") for every category
}

// What clients send to create or replace an event. The API sets the owner (publisherId) from the
// caller's token and fills organizer itself, so neither is sent.
// `sessions` is authoritative (1–60 sessions, no duplicate date+startTime); `date`, `startTime`,
// `endTime` and `endDate` are derived by the API from it and not sent. Images are managed with the
// image endpoints below, not in this body; `imageUrl` is only kept for events without an uploaded cover.
export type EventInput = Omit<
  Event,
  "id" | "publisherId" | "organizer" | "date" | "startTime" | "endTime" | "endDate" | "images"
>;

export async function getEvents(params?: EventsQuery): Promise<ApiEventsResponse> {
  // "All" is a UI-only filter value: don't send it to the API
  const query = params?.category === "All" ? { ...params, category: undefined } : params;
  const { data } = await apiClient.get<ApiEventsResponse>("/events", { params: query });
  return data;
}

export async function getEvent(id: string): Promise<Event> {
  const { data } = await apiClient.get<Event>(`/events/${id}`);
  return data;
}

// Admins and publishers. A publisher becomes the event's owner.
export async function createEvent(event: EventInput): Promise<Event> {
  const { data } = await apiClient.post<Event>("/events", event);
  return data;
}

// PUT replaces the whole event: send every field (missing ones are cleared).
// Publishers may only update their own events (403 otherwise).
export async function updateEvent(id: string, event: EventInput): Promise<Event> {
  const { data } = await apiClient.put<Event>(`/events/${id}`, event);
  return data;
}

// Publishers may only delete their own events (403 otherwise).
export async function deleteEvent(id: string): Promise<void> {
  await apiClient.delete(`/events/${id}`);
}

// ── Images (owner publisher or admin) ───────────────────────────────────────────────────────────
// JPEG, PNG or WebP, max 5 MB. Uploading a cover replaces the previous one. Max 10 gallery images.
// Errors: 400 bad file/kind, 403 not the owner, 404 event not found, 409 gallery full, 413 too big.

export async function uploadEventImage(
  eventId: string,
  file: File,
  kind: EventImageKind,
  onProgress?: (fraction: number) => void
): Promise<EventImage> {
  const body = new FormData();
  body.append("file", file);
  body.append("kind", kind);
  const { data } = await apiClient.post<EventImage>(`/events/${eventId}/images`, body, {
    onUploadProgress: (e) => e.total && onProgress?.(e.loaded / e.total),
  });
  return data;
}

export async function deleteEventImage(eventId: string, imageId: string): Promise<void> {
  await apiClient.delete(`/events/${eventId}/images/${imageId}`);
}

// Reorders the gallery: must list every gallery image id exactly once. Returns all images.
export async function reorderEventImages(eventId: string, imageIds: string[]): Promise<EventImage[]> {
  const { data } = await apiClient.put<EventImage[]>(`/events/${eventId}/images/order`, { imageIds });
  return data;
}
