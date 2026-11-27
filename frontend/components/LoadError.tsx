"use client";

import { AlertCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

// Error state for event lists that failed to load, with a retry action
export default function LoadError({ onRetry, className = "" }: { onRetry?: () => void; className?: string }) {
  const { t } = useTranslation();

  return (
    <div role="alert" className={`text-center py-20 text-gray-500 ${className}`}>
      <AlertCircle className="w-12 h-12 mx-auto mb-3 text-red-400 opacity-70" />
      <p className="font-medium">{t("common.loadEventsError")}</p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-3 px-4 py-2 rounded-full text-sm font-medium text-white hover:opacity-90 transition-opacity"
          style={{ backgroundColor: "#ec5b13" }}
        >
          {t("common.retry")}
        </button>
      )}
    </div>
  );
}
