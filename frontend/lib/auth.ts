import { User } from "./types";

const KEY = "folixenda_user";

export function getStoredUser(): User | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function setStoredUser(user: User): void {
  localStorage.setItem(KEY, JSON.stringify(user));
}

export function clearStoredUser(): void {
  localStorage.removeItem(KEY);
}
