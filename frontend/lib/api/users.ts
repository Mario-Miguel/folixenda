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

export async function createUser(user: Omit<User, "id"> & { password: string }): Promise<User> {
  const { data } = await apiClient.post<User>("/users", user);
  return data;
}

export async function updateUser(id: string, user: Partial<User> & { password?: string }): Promise<User> {
  const { data } = await apiClient.put<User>(`/users/${id}`, user);
  return data;
}

export async function deleteUser(id: string): Promise<void> {
  await apiClient.delete(`/users/${id}`);
}
