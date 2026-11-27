import { notFound } from "next/navigation";
import { getTranslation } from "@/i18n/server";
import EventEditor from "@/components/event-editor/EventEditor";
import { CREATED_PARAM, FAILED_PARAM, homeHref, MAX_FAILED_NAMES } from "@/components/event-editor/routes";
import { ApiError } from "@/lib/api/api";
import { getEvent } from "@/lib/api/events";
import { Event } from "@/lib/types";
import { AccessCard, EditorShell, editorHomeFor, requireSession } from "../../access";

export default async function EditEventPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const { id } = await params;
  const query = await searchParams;
  const session = await requireSession(`/publisher/events/${encodeURIComponent(id)}/edit`);
  const { t } = await getTranslation();

  const home = editorHomeFor(session.user.role);
  if (!home) {
    return (
      <AccessCard
        title={t("editor.access.onlyPublishersTitle")}
        text={t("editor.access.onlyPublishersText")}
        href="/"
        linkLabel={t("editor.access.backHome")}
      />
    );
  }

  let event: Event;
  try {
    event = await getEvent(id);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) notFound();
    console.error(err);
    return (
      <AccessCard
        title={t("editor.access.loadErrorTitle")}
        text={t("editor.access.loadErrorText")}
        href={homeHref(home)}
        linkLabel={home === "admin" ? t("editor.backToAdmin") : t("editor.backToPublisher")}
      />
    );
  }

  // Publishers edit only their own events; admins edit any (the API enforces the same: 403)
  if (home === "publisher" && event.publisherId !== session.user.id) {
    return (
      <AccessCard
        title={t("editor.access.notOwnerTitle")}
        text={t("editor.access.notOwnerText")}
        href={homeHref(home)}
        linkLabel={t("editor.backToPublisher")}
      />
    );
  }

  const failedRaw = query[FAILED_PARAM];
  const failed = (Array.isArray(failedRaw) ? failedRaw : failedRaw ? [failedRaw] : []).slice(0, MAX_FAILED_NAMES);
  const createdNotice = query[CREATED_PARAM] === "1" ? { failed } : undefined;

  return (
    <EditorShell
      home={home}
      backLabel={home === "admin" ? t("editor.backToAdmin") : t("editor.backToPublisher")}
      title={t("editor.editTitle")}
      subtitle={event.title}
    >
      {/* key: a different event (or a fresh load after saving) starts a fresh editor */}
      <EventEditor key={event.id} event={event} home={home} createdNotice={createdNotice} />
    </EditorShell>
  );
}
