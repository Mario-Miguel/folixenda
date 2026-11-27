"use client";

import { FormEvent, KeyboardEvent, useEffect, useState } from "react";
import { flushSync } from "react-dom";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTranslation } from "react-i18next";
import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react";
import { createEvent, updateEvent } from "@/lib/api/events";
import { Event } from "@/lib/types";
import { describeApiError, EditorApiError } from "./errors";
import ImagesSection from "./ImagesSection";
import InfoSection from "./InfoSection";
import { EditorErrors, EditorForm, initialForm, toPayload, validate } from "./model";
import PlaceSection from "./PlaceSection";
import {
  CREATED_PARAM,
  editHref,
  EditorHome,
  FAILED_PARAM,
  homeHref,
  MAX_FAILED_NAMES,
  publicHref,
} from "./routes";
import ScheduleSection from "./ScheduleSection";
import { useEventImages } from "./useEventImages";

interface Props {
  event?: Event; // omit to create a new event
  home: EditorHome;
  // Shown on top after creating an event whose image uploads partly failed
  createdNotice?: { failed: string[] };
}

type Phase = "idle" | "saving" | "uploading" | "done";

function scrollToFirstError() {
  const el = document.querySelector<HTMLElement>("[data-editor-error]");
  if (!el) return;
  el.scrollIntoView({ behavior: "smooth", block: "center" });
  const control = el.querySelector<HTMLElement>("input:not([type=file]):not([disabled]), select, textarea, [tabindex]");
  control?.focus({ preventScroll: true });
}

export default function EventEditor({ event, home, createdNotice }: Props) {
  const { t } = useTranslation();
  const router = useRouter();
  const pathname = usePathname();
  const isEdit = !!event;

  const [form, setForm] = useState<EditorForm>(() => initialForm(event));
  const [snapshot] = useState(() => JSON.stringify(initialForm(event)));
  const [errors, setErrors] = useState<EditorErrors>({});
  const [submitError, setSubmitError] = useState<EditorApiError | null>(null);
  const [phase, setPhase] = useState<Phase>("idle");
  const [uploadStep, setUploadStep] = useState({ done: 0, total: 0 });
  const images = useEventImages(event?.id, event?.images ?? []);

  // Images of an existing event are saved as they change; a new event's local files are unsaved work
  const dirty = phase !== "done" && (JSON.stringify(form) !== snapshot || (!isEdit && images.pendingCount > 0));
  const working = phase === "saving" || phase === "uploading";

  useEffect(() => {
    if (!dirty && !working) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = ""; // older browsers need it to show the prompt
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty, working]);

  function update(patch: Partial<EditorForm>) {
    setForm((f) => ({ ...f, ...patch }));
    setErrors((e) => {
      const keys = Object.keys(patch).filter((k) => e[k]);
      // Editing the days list clears the per-day errors too
      const dayKeys = "days" in patch || "rangeStart" in patch || "rangeEnd" in patch || "weekdays" in patch
        ? Object.keys(e).filter((k) => k.startsWith("day-") || k === "days")
        : [];
      if (keys.length === 0 && dayKeys.length === 0) return e;
      const next = { ...e };
      for (const k of [...keys, ...dayKeys]) delete next[k];
      return next;
    });
  }

  const confirmLeave = () => !dirty || confirm(t("editor.leaveConfirm"));

  function finish(href: string, replace = false) {
    setPhase("done");
    if (replace) router.replace(href);
    else router.push(href);
    router.refresh();
  }

  async function save(view: boolean) {
    if (working) return;
    const errs = validate(form, t, { hasUploadedCover: !!images.cover });
    flushSync(() => setErrors(errs));
    if (Object.keys(errs).length > 0) {
      setSubmitError(null);
      scrollToFirstError();
      return;
    }
    if (images.busy) {
      setSubmitError({ status: -1, message: t("editor.errors.imagesBusy") });
      return;
    }

    setSubmitError(null);
    setPhase("saving");
    // Future: when publishing becomes paid, a 402 / "subscription required" answer would be handled
    // here by pointing to the subscription page (auth.CanPublish in Go is the single gate).
    const payload = toPayload(form, images.cover);
    try {
      if (isEdit) {
        await updateEvent(event.id, payload);
        finish(view ? publicHref(event.id, home) : homeHref(home, "updated"));
        return;
      }

      const created = await createEvent(payload);
      let failed: string[] = [];
      if (images.pendingCount > 0) {
        setPhase("uploading");
        failed = await images.uploadPending(created.id, (done, total) => setUploadStep({ done, total }));
      }
      if (failed.length > 0) {
        // Keep everything: continue on the edit page of the new event, listing what didn't upload
        const params = new URLSearchParams({ [CREATED_PARAM]: "1" });
        for (const name of failed.slice(0, MAX_FAILED_NAMES)) params.append(FAILED_PARAM, name.slice(0, 80));
        finish(`${editHref(created.id)}?${params}`, true);
      } else {
        finish(view ? publicHref(created.id, home) : homeHref(home, "created"));
      }
    } catch (err) {
      setPhase("idle");
      const described = describeApiError(err, t, "event");
      setSubmitError(described);
      if (described.field) {
        const field = described.field;
        flushSync(() => setErrors((e) => ({ ...e, [field]: described.message })));
        scrollToFirstError();
      }
    }
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    save(false);
  }

  // Enter in a single-line input must not submit the whole editor by accident
  function onKeyDown(e: KeyboardEvent<HTMLFormElement>) {
    const target = e.target as HTMLElement;
    if (e.key === "Enter" && target instanceof HTMLInputElement && target.type !== "submit") e.preventDefault();
  }

  const status =
    phase === "saving"
      ? t("editor.bar.saving")
      : phase === "uploading"
        ? t("editor.bar.uploading", { done: uploadStep.done, total: uploadStep.total })
        : dirty
          ? t("editor.bar.unsaved")
          : isEdit
            ? t("editor.bar.noChanges")
            : "";

  return (
    <form onSubmit={onSubmit} onKeyDown={onKeyDown} noValidate className="flex flex-col gap-5">
      {createdNotice && (
        <div role="status" className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          <p className="flex items-center gap-2 font-semibold">
            <CheckCircle2 className="w-4 h-4" />
            {t("editor.notice.created")}
          </p>
          {createdNotice.failed.length > 0 && (
            <>
              <p className="mt-1">{t("editor.notice.uploadsFailed", { count: createdNotice.failed.length })}</p>
              <ul className="mt-1 list-disc pl-5 text-amber-800">
                {createdNotice.failed.map((name, i) => (
                  <li key={`${name}-${i}`} className="break-all">
                    {name}
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
      )}

      <InfoSection form={form} errors={errors} update={update} />
      <ScheduleSection form={form} errors={errors} update={update} />
      <PlaceSection form={form} errors={errors} update={update} />
      <ImagesSection
        images={images}
        immediate={isEdit}
        imageUrl={form.imageUrl}
        onImageUrlChange={(imageUrl) => update({ imageUrl })}
        errors={errors}
      />

      {submitError && (
        <div role="alert" className="flex items-start gap-2 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">
          <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
          <p className="flex-1">
            {submitError.message}
            {submitError.sessionExpired && (
              <>
                {" "}
                <a
                  href={`/login?redirect=${encodeURIComponent(pathname)}`}
                  target="_blank"
                  rel="noopener"
                  className="font-semibold underline"
                >
                  {t("editor.errors.loginAgain")}
                </a>
              </>
            )}
          </p>
        </div>
      )}

      {/* Sticky action bar */}
      <div className="sticky bottom-0 z-[1100] -mx-4 sm:mx-0 mt-2 border-t sm:border border-gray-100 bg-white/95 backdrop-blur px-4 py-3 sm:rounded-2xl shadow-[0_-4px_16px_rgba(0,0,0,0.04)]">
        <div className="flex flex-wrap items-center gap-2">
          <p className="flex items-center gap-1.5 text-xs text-gray-500 mr-auto min-w-0" aria-live="polite">
            {working && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            <span className="truncate">{status}</span>
          </p>
          <Link
            href={homeHref(home)}
            onClick={(e) => {
              if (!confirmLeave()) e.preventDefault();
              else setPhase("done");
            }}
            className="px-4 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100 transition"
            aria-disabled={working}
          >
            {t("common.cancel")}
          </Link>
          <button
            type="button"
            onClick={() => save(true)}
            disabled={working}
            className="px-4 py-2 rounded-lg text-sm font-semibold border border-gray-200 text-gray-700 hover:bg-gray-50 transition disabled:opacity-60"
          >
            {t("editor.bar.saveAndView")}
          </button>
          <button
            type="submit"
            disabled={working}
            className="px-5 py-2 rounded-lg text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-60"
            style={{ backgroundColor: "#ec5b13" }}
          >
            {working ? t("common.saving") : isEdit ? t("editor.bar.save") : t("editor.bar.publish")}
          </button>
        </div>
      </div>
    </form>
  );
}
