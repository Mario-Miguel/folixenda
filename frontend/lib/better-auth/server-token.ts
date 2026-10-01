import { headers } from "next/headers";
import { auth } from "./server";

// Server-side counterpart of getApiToken: reads the session from the request cookies
export async function getServerApiToken(): Promise<string | null> {
  try {
    const { token } = await auth.api.getToken({ headers: await headers() });
    return token;
  } catch {
    return null; // no active session
  }
}
