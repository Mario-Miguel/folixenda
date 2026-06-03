import { User } from "../types";
import { apiClient } from "./api";

export async function login(username: string, password: string): Promise<User> {
  const { data } = await apiClient.post<User>("/auth/login", { username, password });
  return data;
}
