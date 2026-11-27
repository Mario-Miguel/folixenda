// Where the editor's back link, Cancel and Save lead, per role

export type EditorHome = "publisher" | "admin";

export type SavedNotice = "created" | "updated";

// Query param read by PublisherDashboard / AdminDashboard to show the success notice
export const SAVED_PARAM = "saved";

export function homeHref(home: EditorHome, saved?: SavedNotice): string {
  const params = new URLSearchParams();
  if (home === "admin") params.set("tab", "events");
  if (saved) params.set(SAVED_PARAM, saved);
  const q = params.toString();
  return `/${home}${q ? `?${q}` : ""}`;
}

export const editHref = (id: string) => `/publisher/events/${encodeURIComponent(id)}/edit`;
export const NEW_EVENT_HREF = "/publisher/events/new";

export function publicHref(id: string, home: EditorHome): string {
  return `/events/${encodeURIComponent(id)}${home === "publisher" ? "?from=publisher" : ""}`;
}

// After creating an event whose images partly failed, the edit page shows which ones
export const FAILED_PARAM = "failedUploads";
export const CREATED_PARAM = "created";
export const MAX_FAILED_NAMES = 10;
