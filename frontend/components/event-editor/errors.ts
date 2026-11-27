import type { TFunction } from "i18next";
import { ApiError } from "@/lib/api/api";

export interface EditorApiError {
  message: string;
  status: number;
  field?: string; // editor field the 400 message refers to, when recognisable
  sessionExpired?: boolean;
}

// First word of a Go validation message ("title is required", "venue is required"…) → field id
const FIELD_PREFIXES: Record<string, string> = {
  title: "title",
  category: "category",
  description: "description",
  venue: "venue",
  price: "price",
  imageurl: "imageUrl",
  sessions: "days",
};

// Maps an API failure to a translated message. `context` picks the texts for 404/409.
export function describeApiError(err: unknown, t: TFunction, context: "event" | "image"): EditorApiError {
  if (!(err instanceof ApiError)) return { status: -1, message: t("editor.errors.generic") };
  switch (err.status) {
    case 400: {
      const word = err.message.trim().split(/[\s.[:]/)[0]?.toLowerCase() ?? "";
      return { status: 400, message: t("editor.errors.badRequest", { message: err.message }), field: FIELD_PREFIXES[word] };
    }
    case 401:
      return { status: 401, message: t("editor.errors.unauthorized"), sessionExpired: true };
    case 403:
      return { status: 403, message: t("editor.errors.forbidden") };
    case 404:
      return { status: 404, message: context === "image" ? t("editor.errors.imageNotFound") : t("editor.errors.notFound") };
    case 409:
      return { status: 409, message: context === "image" ? t("editor.images.errors.galleryFull") : t("editor.errors.conflict") };
    case 413:
      return { status: 413, message: t("editor.images.errors.size") };
    case 0:
      return { status: 0, message: t("editor.errors.network") };
    default:
      return { status: err.status, message: t("editor.errors.generic") };
  }
}
