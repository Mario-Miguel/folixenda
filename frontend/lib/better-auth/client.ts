import { createAuthClient } from "better-auth/react";
import { inferAdditionalFields, jwtClient } from "better-auth/client/plugins";
import type { auth } from "./server";

export const authClient = createAuthClient({
  plugins: [inferAdditionalFields<typeof auth>(), jwtClient()],
});

export const { signIn, signOut, signUp, useSession } = authClient;

// Seconds before expiry at which a cached token is considered stale
const EXPIRY_MARGIN = 30;

let cached: { token: string; exp: number } | null = null;

function readExp(token: string): number {
  const payload = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
  return (JSON.parse(atob(payload)) as { exp: number }).exp;
}

// Returns a JWT for the Go API, or null when there is no active session
export async function getApiToken(): Promise<string | null> {
  const now = Date.now() / 1000;
  if (cached && cached.exp - EXPIRY_MARGIN > now) return cached.token;

  const { data } = await authClient.token();
  if (!data?.token) {
    cached = null;
    return null;
  }
  cached = { token: data.token, exp: readExp(data.token) };
  return cached.token;
}

export function clearApiToken(): void {
  cached = null;
}
