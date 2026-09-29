import { UserSavedEvents, SavedEvent, UserSavedEvent } from "../types";
import { apiClient } from "./api";

export async function getSavedEvents(userId: string): Promise<UserSavedEvents> {
  const { data } = await apiClient.get<UserSavedEvents>(`/user/${userId}/savedEvents`);
  return data;
}

export async function createSavedEvent(userId: string, newSavedEvent: Omit<UserSavedEvent, "id">): Promise<SavedEvent> {
  const { data } = await apiClient.post<SavedEvent>(`/user/${userId}/savedEvents`, newSavedEvent);
  return data;
}

export async function deleteSavedEvent(userId: string, eventId: string): Promise<void> {
  await apiClient.delete(`/user/${userId}/savedEvents/${eventId}`);
}

export async function isEventSaved(userId: string, eventId: string): Promise<boolean> {
  const { savedEvents } = await getSavedEvents(userId);
  return savedEvents.some((e) => e.id === eventId);
}
