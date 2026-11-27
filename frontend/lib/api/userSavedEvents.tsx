import { UserSavedEvents, SavedEvent } from "../types";
import { ApiError, apiClient } from "./api";

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

// Whether one event is saved by the current user. Without a session (401) it counts as not saved.
// Pass a token when calling from the server; in the browser the interceptor adds it.
export async function isEventSaved(eventId: string, token?: string | null): Promise<boolean> {
  try {
    const { data } = await apiClient.get<{ saved: boolean }>(`/savedEvents/${eventId}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    });
    return data.saved;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return false;
    throw err;
  }
}
