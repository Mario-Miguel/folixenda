"use client";

import { ConsumerUser } from "@/data/users";
import { createSavedEvent, deleteSavedEvent } from "@/lib/api/userSavedEvents";
import { UserSavedEvent } from "@/lib/types";
import { Bookmark } from "lucide-react";
import { useRouter } from "next/navigation";
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
  const [saving, setSaving] = useState(false);
  const [refreshing, startTransition] = useTransition();
  const busy = saving || refreshing;

  const toggleSaved = async () => {
    setSaving(true);
    try {
      if (saved) {
        await deleteSavedEvent(ConsumerUser.id, eventId);
      } else {
        const newSavedEvent: Omit<UserSavedEvent, "id"> = { userId: ConsumerUser.id, eventId };
        await createSavedEvent(ConsumerUser.id, newSavedEvent);
      }
      // Re-render the server page so every save control picks up the new state
      startTransition(() => router.refresh());
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
