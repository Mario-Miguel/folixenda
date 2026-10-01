"use client";

import { ApiError } from "@/lib/api/api";
import { createSavedEvent, deleteSavedEvent } from "@/lib/api/userSavedEvents";
import { Bookmark } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { useTranslation } from "react-i18next";

interface SaveEventButtonProps {
  eventId: string;
  saved: boolean;
  // "icon": round bookmark button over the hero image; "button": full-width sidebar button
  variant?: "button" | "icon";
}

export default function SaveEventButton({ eventId, saved, variant = "button" }: SaveEventButtonProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const pathname = usePathname();
  const [saving, setSaving] = useState(false);
  const [refreshing, startTransition] = useTransition();
  const busy = saving || refreshing;

  const toggleSaved = async () => {
    setSaving(true);
    try {
      if (saved) {
        await deleteSavedEvent(eventId);
      } else {
        await createSavedEvent(eventId);
      }
      // Re-render the server page so every save control picks up the new state
      startTransition(() => router.refresh());
    } catch (err) {
      // No session (or it expired): log in and come back to this page
      if (err instanceof ApiError && err.status === 401) {
        router.push(`/login?redirect=${encodeURIComponent(pathname)}`);
        return;
      }
      throw err;
    } finally {
      setSaving(false);
    }
  };

  const label = saved ? t("event.saved") : t("event.saveForLater");

  if (variant === "icon") {
    return (
      <button
        aria-label={label}
        aria-pressed={saved}
        onClick={toggleSaved}
        disabled={busy}
        className="w-9 h-9 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm text-gray-600 hover:text-gray-900 transition-colors disabled:opacity-60"
        style={{ color: saved ? "#ec5b13" : undefined }}
      >
        <Bookmark className="w-4 h-4" fill={saved ? "currentColor" : "none"} />
      </button>
    );
  }

  return (
    <button
      className="w-full py-3 rounded-xl border border-gray-200 text-gray-700 font-medium text-sm mt-3 hover:bg-gray-50 transition-colors disabled:opacity-60"
      style={saved ? { borderColor: "#ec5b13", color: "#ec5b13" } : undefined}
      aria-pressed={saved}
      onClick={toggleSaved}
      disabled={busy}
    >
      {label}
    </button>
  );
}
