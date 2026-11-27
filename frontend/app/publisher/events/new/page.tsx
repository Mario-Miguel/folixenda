import { getTranslation } from "@/i18n/server";
import EventEditor from "@/components/event-editor/EventEditor";
import { AccessCard, EditorShell, editorHomeFor, requireSession } from "../access";

export default async function NewEventPage() {
  const session = await requireSession("/publisher/events/new");
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

  return (
    <EditorShell
      home={home}
      backLabel={home === "admin" ? t("editor.backToAdmin") : t("editor.backToPublisher")}
      title={t("editor.newTitle")}
    >
      <EventEditor home={home} />
    </EditorShell>
  );
}
