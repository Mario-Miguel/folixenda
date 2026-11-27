// Address search for the event editor. The Go API proxies CartoCiudad (Spain) with Nominatim as
// fallback, so the browser never calls those services directly. Requires a session that can
// publish (publisher or admin); 401/403 otherwise.
import { GeocodeResult } from "../types";
import { apiClient } from "./api";

// Autocomplete: up to `limit` (default 8, max 10) results for a free-text query (min 3 chars)
export async function searchAddress(q: string, limit = 8, signal?: AbortSignal): Promise<GeocodeResult[]> {
  const { data } = await apiClient.get<GeocodeResult[]>("/geocode/search", { params: { q, limit }, signal });
  return data;
}

// The address at a map point; 404 when nothing is found there
export async function reverseGeocode(lat: number, lon: number, signal?: AbortSignal): Promise<GeocodeResult> {
  const { data } = await apiClient.get<GeocodeResult>("/geocode/reverse", { params: { lat, lon }, signal });
  return data;
}
