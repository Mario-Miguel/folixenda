"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Share2 } from "lucide-react";
import { useTranslation } from "react-i18next";

const CONFIRMATION_MS = 2000;

// Shares the current page with the Web Share API; where it isn't available (most desktop
// browsers) copies the URL to the clipboard and briefly confirms it
export default function ShareButton({ title }: { title: string }) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<"idle" | "copied" | "error">("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const flash = (next: "copied" | "error") => {
    setStatus(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setStatus("idle"), CONFIRMATION_MS);
  };

  const share = async () => {
    const url = window.location.href;
    if (navigator.share) {
      try {
        await navigator.share({ title, url });
        return;
      } catch (err) {
        // The user closed the share sheet: nothing else to do
        if (err instanceof DOMException && err.name === "AbortError") return;
        // Any other failure (e.g. not allowed): fall back to copying
      }
    }
    try {
      await navigator.clipboard.writeText(url);
      flash("copied");
    } catch {
      flash("error");
    }
  };

  return (
    <div className="relative">
      <button
        type="button"
        aria-label={t("event.share")}
        onClick={share}
        className="w-9 h-9 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm text-gray-600 hover:text-gray-900 transition-colors"
      >
        {status === "copied" ? <Check className="w-4 h-4" style={{ color: "#ec5b13" }} /> : <Share2 className="w-4 h-4" />}
      </button>
      <span role="status" aria-live="polite">
        {status !== "idle" && (
          <span className="absolute right-0 top-11 whitespace-nowrap rounded-lg bg-gray-900/90 px-2.5 py-1 text-xs font-medium text-white shadow">
            {status === "copied" ? t("event.linkCopied") : t("event.shareError")}
          </span>
        )}
      </span>
    </div>
  );
}
