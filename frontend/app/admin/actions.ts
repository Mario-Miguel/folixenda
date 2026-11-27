"use server";

import { headers } from "next/headers";
import { auth } from "@/lib/better-auth/server";

export type DeleteUserResult =
  | { ok: true }
  | { ok: false; error: "unauthorized" | "forbidden" | "self" | "notFound" | "failed" };

// Deletes a user in Better Auth (user, sessions and accounts). The Go profile
// and the user's saved events go with it through ON DELETE CASCADE on
// public.users.id → auth."user"(id) (backend/migrations/00002_user_cascade.sql).
// Only admins may call it, and never on themselves.
export async function deleteUserAction(userId: string): Promise<DeleteUserResult> {
  const session = await auth.api.getSession({ headers: await headers() });
  if (!session) return { ok: false, error: "unauthorized" };
  if (session.user.role !== "admin") return { ok: false, error: "forbidden" };
  if (session.user.id === userId) return { ok: false, error: "self" };

  try {
    const ctx = await auth.$context;
    const user = await ctx.internalAdapter.findUserById(userId);
    if (!user) return { ok: false, error: "notFound" };
    await ctx.internalAdapter.deleteUser(userId);
    return { ok: true };
  } catch (err) {
    console.error("deleteUserAction failed", err);
    return { ok: false, error: "failed" };
  }
}
