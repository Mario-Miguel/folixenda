import { ApiEventsResponse, Event } from "../types";
import { apiClient } from "./api";

export interface EventsQuery {
  date?: string; // YYYY-MM-DD: events on this day
  from?: string; // YYYY-MM-DD, inclusive range start (use with `to`)
  to?: string; // YYYY-MM-DD, inclusive range end
  category?: string; // omit (or "All") for every category
}

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

export async function createEvent(event: Omit<Event, "id">): Promise<Event> {
  const { data } = await apiClient.post<Event>("/events", event);
  return data;
}

export async function updateEvent(id: string, event: Partial<Event>): Promise<Event> {
  const { data } = await apiClient.put<Event>(`/events/${id}`, event);
  return data;
}

export async function deleteEvent(id: string): Promise<void> {
  await apiClient.delete(`/events/${id}`);
}
