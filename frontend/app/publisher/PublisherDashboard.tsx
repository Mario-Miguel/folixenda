"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslation } from "react-i18next";
import {
  Bookmark,
  CalendarCheck,
  CalendarClock,
  CalendarPlus,
  Clock,
  ExternalLink,
  MapPin,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import { editHref, NEW_EVENT_HREF, SAVED_PARAM } from "@/components/event-editor/routes";
import EventImage from "@/components/EventImage";
import { formatLongDate } from "@/i18n/format";
import { formatEventDateRange, formatScheduleSummary, formatSessionTime, isMultiDay } from "@/lib/dates";
import { ApiError } from "@/lib/api/api";
import { deleteEvent } from "@/lib/api/events";
import { categoryBadgeClass } from "@/lib/categories";
import { PublisherEvent } from "@/lib/types";

type Action = "delete";

interface Props {
  orgName: string;
  upcoming: PublisherEvent[]; // soonest first
  past: PublisherEvent[]; // most recent first
}

export default function PublisherDashboard({ orgName, upcoming, past }: Props) {
  const { t } = useTranslation();
  const router = useRouter();
  const [tab, setTab] = useState<"upcoming" | "past">("upcoming");
  const pathname = usePathname();
  const searchParams = useSearchParams();
  // The event editor (/publisher/events/…) comes back with ?saved=created|updated
  const saved = searchParams.get(SAVED_PARAM);
  const [feedback, setFeedback] = useState<{ kind: "ok" | "error"; text: string } | null>(() =>
    saved === "created" || saved === "updated"
      ? { kind: "ok", text: saved === "created" ? t("publisher.created") : t("publisher.updated") }
      : null,
  );

  // Drop ?saved= so a reload doesn't show the notice again
  useEffect(() => {
    if (saved) router.replace(pathname, { scroll: false });
  }, [saved, pathname, router]);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const totalSaves = [...upcoming, ...past].reduce((sum, e) => sum + (e.savesCount ?? 0), 0);
  const hasEvents = upcoming.length + past.length > 0;
  const shown = tab === "upcoming" ? upcoming : past;

  function errorMessage(err: unknown, action: Action): string {
    if (err instanceof ApiError) {
      if (err.status === 401) return t("publisher.errors.unauthorized");
      if (err.status === 403) return t("publisher.errors.forbidden");
      if (err.status === 400) return t("publisher.errors.badRequest", { message: err.message });
      if (err.status === 404) return t("publisher.errors.notFound");
    }
    return t(`publisher.errors.${action}Failed`);
  }

  async function handleDelete(event: PublisherEvent) {
    if (!confirm(t("publisher.confirmDelete", { title: event.title }))) return;
    setDeletingId(event.id);
    try {
      await deleteEvent(event.id);
      setFeedback({ kind: "ok", text: t("publisher.deleted") });
      router.refresh();
    } catch (err) {
      setFeedback({ kind: "error", text: errorMessage(err, "delete") });
      // The event may be gone already (404): refresh so the list matches the server
      if (err instanceof ApiError && err.status === 404) router.refresh();
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div>
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold text-gray-900">{t("publisher.title")}</h1>
          <p className="text-sm text-gray-500 mt-0.5 truncate">{t("publisher.subtitle", { org: orgName })}</p>
        </div>
        {/* Future: when publishing becomes paid, gate this link behind the subscription */}
        <Link
          href={NEW_EVENT_HREF}
          className="flex items-center justify-center gap-1.5 px-4 py-2 rounded-lg text-sm font-semibold text-white transition hover:opacity-90 shrink-0"
          style={{ backgroundColor: "#ec5b13" }}
        >
          <Plus className="w-4 h-4" />
          {t("publisher.newEvent")}
        </Link>
      </div>

      {feedback && (
        <p
          role="status"
          className={`text-sm rounded-lg px-3 py-2 mb-6 ${
            feedback.kind === "ok" ? "text-gray-700 bg-gray-50" : "text-red-700 bg-red-50"
          }`}
        >
          {feedback.text}
        </p>
      )}

      {!hasEvents ? (
        <div className="bg-white rounded-2xl border border-dashed border-gray-200 py-16 px-6 text-center">
          <CalendarPlus className="w-12 h-12 mx-auto mb-4" style={{ color: "#ec5b13" }} />
          <h2 className="text-lg font-bold text-gray-900 mb-1">{t("publisher.empty.firstTitle")}</h2>
          <p className="text-sm text-gray-500 mb-6 max-w-sm mx-auto">{t("publisher.empty.firstText")}</p>
          <Link
            href={NEW_EVENT_HREF}
            className="inline-flex items-center gap-1.5 px-5 py-2.5 rounded-lg text-sm font-semibold text-white transition hover:opacity-90"
            style={{ backgroundColor: "#ec5b13" }}
          >
            <Plus className="w-4 h-4" />
            {t("publisher.empty.firstCta")}
          </Link>
        </div>
      ) : (
        <>
          {/* Stats */}
          <div className="grid grid-cols-3 gap-3 mb-6">
            <Stat
              icon={<CalendarClock className="w-5 h-5" />}
              label={t("publisher.stats.upcoming")}
              value={upcoming.length}
            />
            <Stat icon={<Bookmark className="w-5 h-5" />} label={t("publisher.stats.saves")} value={totalSaves} />
            <Stat icon={<CalendarCheck className="w-5 h-5" />} label={t("publisher.stats.past")} value={past.length} />
          </div>

          {/* Tabs */}
          <div role="tablist" className="flex gap-1 mb-4 border-b border-gray-200">
            {(["upcoming", "past"] as const).map((key) => {
              const count = key === "upcoming" ? upcoming.length : past.length;
              const active = tab === key;
              return (
                <button
                  key={key}
                  role="tab"
                  aria-selected={active}
                  onClick={() => setTab(key)}
                  className={`flex items-center gap-2 px-5 py-2.5 text-sm font-medium transition -mb-px border-b-2 ${
                    active ? "text-gray-900" : "border-transparent text-gray-500 hover:text-gray-700"
                  }`}
                  style={active ? { borderColor: "#ec5b13" } : undefined}
                >
                  {t(`publisher.tabs.${key}`)}
                  <span className="text-xs rounded-full bg-gray-100 text-gray-600 px-2 py-0.5">{count}</span>
                </button>
              );
            })}
          </div>

          {shown.length === 0 ? (
            <div className="text-center py-14 text-gray-400">
              <p className="font-medium">
                {tab === "upcoming" ? t("publisher.empty.noUpcoming") : t("publisher.empty.noPast")}
              </p>
              {tab === "upcoming" && (
                <Link
                  href={NEW_EVENT_HREF}
                  className="inline-block text-sm mt-1 font-medium hover:underline"
                  style={{ color: "#ec5b13" }}
                >
                  {t("publisher.newEvent")}
                </Link>
              )}
            </div>
          ) : (
            <ul className="flex flex-col gap-3">
              {shown.map((event) => (
                <PublisherEventRow
                  key={event.id}
                  event={event}
                  ended={tab === "past"}
                  deleting={deletingId === event.id}
                  editHref={editHref(event.id)}
                  onDelete={() => handleDelete(event)}
                />
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  );
}

function Stat({ icon, label, value }: { icon: React.ReactNode; label: string; value: number }) {
  return (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-4">
      <div className="mb-1" style={{ color: "#ec5b13" }}>
        {icon}
      </div>
      <p className="text-2xl font-bold text-gray-900 tabular-nums">{value}</p>
      <p className="text-xs text-gray-500">{label}</p>
    </div>
  );
}

function PublisherEventRow({
  event,
  ended,
  deleting,
  editHref,
  onDelete,
}: {
  event: PublisherEvent;
  ended: boolean;
  deleting: boolean;
  editHref: string;
  onDelete: () => void;
}) {
  const { t, i18n } = useTranslation();
  const iconBtn = "p-2 rounded-lg text-gray-400 transition";

  return (
    <li
      className={`flex flex-col sm:flex-row gap-4 bg-white rounded-2xl border border-gray-100 shadow-sm p-3 ${
        ended ? "opacity-75" : ""
      }`}
    >
      <div className="relative sm:w-40 h-32 sm:h-auto sm:min-h-24 shrink-0 rounded-xl overflow-hidden bg-gradient-to-br from-orange-100 to-orange-300">
        <EventImage src={event.imageUrl} alt={event.title} className="absolute inset-0 h-full w-full object-cover" />
      </div>

      <div className="flex-1 min-w-0 flex flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className={`text-xs font-semibold px-2.5 py-0.5 rounded-full ${categoryBadgeClass(event.category)}`}>
            {t(`categories.${event.category}`)}
          </span>
          {ended && (
            <span className="text-xs font-medium px-2.5 py-0.5 rounded-full bg-gray-100 text-gray-500">
              {t("publisher.ended")}
            </span>
          )}
          <span className="text-xs font-semibold text-gray-700 ml-auto">
            {event.price === 0 ? t("common.free") : t("common.price", { price: event.price })}
          </span>
        </div>

        <h3 className="font-semibold text-gray-900 truncate">{event.title}</h3>

        <p className="flex items-center gap-1.5 text-sm text-gray-600">
          <Clock className="w-4 h-4 shrink-0 text-gray-400" />
          <span className="truncate">
            {isMultiDay(event)
              ? `${formatEventDateRange(event, i18n.language, true)} · ${formatScheduleSummary(event, i18n.language, t("display.variableSchedule"))}`
              : `${formatLongDate(new Date(event.date + "T12:00:00"), i18n.language, true)} · ${formatSessionTime(event)}`}
          </span>
        </p>
        {event.venue && (
          <p className="flex items-center gap-1.5 text-sm text-gray-600">
            <MapPin className="w-4 h-4 shrink-0 text-gray-400" />
            <span className="truncate">{event.venue}</span>
          </p>
        )}

        <div className="flex flex-wrap items-center gap-2 mt-auto pt-1">
          <p className="flex items-center gap-1.5 text-sm font-medium" style={{ color: "#ec5b13" }}>
            <Bookmark className="w-4 h-4" />
            {t("publisher.savedBy", { count: event.savesCount ?? 0 })}
          </p>
          <div className="flex items-center gap-1 ml-auto">
            <Link
              href={`/events/${event.id}?from=publisher`}
              aria-label={t("publisher.viewPublic")}
              title={t("publisher.viewPublic")}
              className={`${iconBtn} hover:text-gray-700 hover:bg-gray-100`}
            >
              <ExternalLink className="w-4 h-4" />
            </Link>
            <Link
              href={editHref}
              aria-label={t("common.edit")}
              title={t("common.edit")}
              className={`${iconBtn} hover:text-gray-700 hover:bg-gray-100`}
            >
              <Pencil className="w-4 h-4" />
            </Link>
            <button
              onClick={onDelete}
              disabled={deleting}
              aria-label={t("common.delete")}
              title={t("common.delete")}
              className={`${iconBtn} hover:text-red-600 hover:bg-red-50 disabled:opacity-40`}
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </li>
  );
}
