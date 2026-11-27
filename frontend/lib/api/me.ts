// The current user's own data. The Go API takes the user from the JWT, never from the URL.
// Pass a token when calling from a Server Component; in the browser the axios interceptor adds it.
import {
  MeResponse,
  NotificationPreferences,
  PublisherEvent,
  PublisherProfile,
  PublisherProfileInput,
} from "../types";
import { apiClient } from "./api";

const auth = (token?: string | null) => (token ? { Authorization: `Bearer ${token}` } : undefined);

export async function getMe(token?: string | null): Promise<MeResponse> {
  const { data } = await apiClient.get<MeResponse>("/me", { headers: auth(token) });
  return data;
}

export async function updateNotifications(prefs: NotificationPreferences): Promise<NotificationPreferences> {
  const { data } = await apiClient.put<NotificationPreferences>("/me/notifications", prefs);
  return data;
}

// 404 when the publisher hasn't created it yet; 403 for non-publishers
export async function getPublisherProfile(token?: string | null): Promise<PublisherProfile> {
  const { data } = await apiClient.get<PublisherProfile>("/me/publisher-profile", { headers: auth(token) });
  return data;
}

// Creates or replaces it. 400 on validation errors; 403 for non-publishers
export async function savePublisherProfile(
  profile: PublisherProfileInput,
  token?: string | null
): Promise<PublisherProfile> {
  const { data } = await apiClient.put<PublisherProfile>("/me/publisher-profile", profile, { headers: auth(token) });
  return data;
}

// The publisher's own events (past and upcoming, by start time) with their save counts
export async function getMyEvents(token?: string | null): Promise<{ events: PublisherEvent[]; total: number }> {
  const { data } = await apiClient.get<{ events: PublisherEvent[]; total: number }>("/me/events", {
    headers: auth(token),
  });
  return data;
}
