import { ApiEventsResponse, Event } from "../types";
import { apiClient } from "./api";

export async function getEvents(params?: { date?: string; category?: string }): Promise<ApiEventsResponse> {
  const { data } = await apiClient.get<ApiEventsResponse>("/events", { params });
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
