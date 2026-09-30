import axios, { AxiosError } from "axios";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export const apiClient = axios.create({
  baseURL: `${API_BASE}/api`,
});

// In the browser, send the Better Auth JWT so the Go API can identify the user
apiClient.interceptors.request.use(async (config) => {
  if (typeof window === "undefined") return config;
  const { getApiToken } = await import("@/lib/better-auth/client");
  const token = await getApiToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ error?: string }>) => {
    const status = error.response?.status ?? 0;
    const message = error.response?.data?.error ?? error.message;
    return Promise.reject(new ApiError(status, message));
  }
);
