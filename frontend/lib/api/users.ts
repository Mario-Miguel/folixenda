import { User } from "../types";
import { apiClient } from "./api";

export async function getUsers(): Promise<User[]> {
  const { data } = await apiClient.get<User[]>("/users");
  return data;
}

export async function getUser(id: string): Promise<User> {
  const { data } = await apiClient.get<User>(`/users/${id}`);
  return data;
}

export async function updateUser(id: string, user: Partial<Omit<User, "id" | "email">>): Promise<User> {
  const { data } = await apiClient.put<User>(`/users/${id}`, user);
  return data;
}

// Users are deleted through Better Auth (app/admin/actions.ts), not the Go API.
