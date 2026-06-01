import { Event, ApiEventsResponse } from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function getEvents(params?: {
  date?: string;
  category?: string;
}): Promise<ApiEventsResponse> {
  const url = new URL(`${API_BASE}/api/events`);
  if (params?.date) url.searchParams.set("date", params.date);
  if (params?.category) url.searchParams.set("category", params.category);

  const res = await fetch(url.toString(), { next: { revalidate: 60 } });
  if (!res.ok) throw new Error("Failed to fetch events");
  return res.json();
}

export async function getEvent(id: string): Promise<Event> {
  const res = await fetch(`${API_BASE}/api/events/${id}`, {
    next: { revalidate: 60 },
  });
  if (!res.ok) throw new Error("Failed to fetch event");
  return res.json();
}
