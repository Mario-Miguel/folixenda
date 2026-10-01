import { UserSavedEvents, SavedEvent } from "../types";
import { apiClient } from "./api";

// Pass a token when calling from the server; in the browser the interceptor adds it
export async function getSavedEvents(token?: string | null): Promise<UserSavedEvents> {
  const { data } = await apiClient.get<UserSavedEvents>(`/savedEvents`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return data;
}

export async function createSavedEvent(eventId: string): Promise<SavedEvent> {
  const { data } = await apiClient.post<SavedEvent>(`/savedEvents`, { eventId: eventId });
  return data;
}

export async function deleteSavedEvent(eventId: string): Promise<void> {
  await apiClient.delete(`/savedEvents/${eventId}`);
}

export async function isEventSaved(eventId: string, token?: string | null): Promise<boolean> {
  const { savedEvents } = await getSavedEvents(token);
  return savedEvents.some((e) => e.id === eventId);
}
