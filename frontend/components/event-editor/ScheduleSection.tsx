"use client";

import { useTranslation } from "react-i18next";
import { CalendarDays, CalendarPlus, Moon, RefreshCw, Trash2 } from "lucide-react";
import { addDays, formatDayLabel, isValidDate, mondayFirstWeekdays } from "./dates";
import {
  ALL_WEEKDAYS,
  DayRow,
  EditorErrors,
  EditorForm,
  generateDays,
  isOvernight,
  MAX_RANGE_DAYS,
  MAX_SESSIONS,
  newKey,
  ScheduleMode,
  sortDays,
} from "./model";
import { describedBy, Field, inputCls, Section } from "./ui";

interface Props {
  form: EditorForm;
  errors: EditorErrors;
  update: (patch: Partial<EditorForm>) => void;
}

export default function ScheduleSection({ form, errors, update }: Props) {
  const { t, i18n } = useTranslation();
  const common = { startTime: form.startTime, endTime: form.endTime };

  function setMode(mode: ScheduleMode) {
    if (mode === form.mode) return;
    if (mode === "multi") {
      const start = form.rangeStart || form.date;
      const end = form.rangeEnd && form.rangeEnd >= start ? form.rangeEnd : start;
      update({
        mode,
        rangeStart: start,
        rangeEnd: end,
        days: form.days.length > 0 ? form.days : generateDays(start, end, form.weekdays, [], common),
      });
    } else {
      const first = sortDays(form.days)[0];
      update({
        mode,
        date: form.date || first?.date || "",
        startTime: form.startTime || first?.startTime || "",
        endTime: form.endTime || first?.endTime || "",
      });
    }
  }

  // Range or weekday filter changed: regenerate the list, keeping per-day adjustments
  function setRange(patch: Partial<Pick<EditorForm, "rangeStart" | "rangeEnd" | "weekdays">>) {
    const next = { rangeStart: form.rangeStart, rangeEnd: form.rangeEnd, weekdays: form.weekdays, ...patch };
    // Moving the start past the end drags the end along, so the range stays valid
    if (patch.rangeStart && isValidDate(patch.rangeStart) && (!next.rangeEnd || next.rangeEnd < patch.rangeStart))
      next.rangeEnd = patch.rangeStart;
    update({ ...next, days: generateDays(next.rangeStart, next.rangeEnd, next.weekdays, form.days, common) });
  }

  // Changing the common schedule also moves the days that still follow it
  function setCommon(patch: Partial<Pick<EditorForm, "startTime" | "endTime">>) {
    const nextCommon = { ...common, ...patch };
    update({
      ...patch,
      days: form.days.map((d) =>
        d.startTime === common.startTime && d.endTime === common.endTime ? { ...d, ...nextCommon } : d,
      ),
    });
  }

  const setDay = (key: string, patch: Partial<DayRow>) =>
    update({ days: form.days.map((d) => (d.key === key ? { ...d, ...patch } : d)) });

  const removeDay = (key: string) => update({ days: form.days.filter((d) => d.key !== key) });

  function addDay() {
    const last = sortDays(form.days).at(-1)?.date;
    const date = last && isValidDate(last) ? addDays(last, 1) : form.rangeStart;
    update({ days: [...form.days, { key: newKey(), date, ...common, extra: true }] });
  }

  const applyToAll = () => update({ days: form.days.map((d) => ({ ...d, ...common })) });

  function toggleWeekday(day: number) {
    const weekdays = form.weekdays.includes(day) ? form.weekdays.filter((d) => d !== day) : [...form.weekdays, day].sort();
    setRange({ weekdays });
  }

  const days = sortDays(form.days);
  const maxEnd = isValidDate(form.rangeStart) ? addDays(form.rangeStart, MAX_RANGE_DAYS - 1) : undefined;

  return (
    <Section
      id="schedule"
      title={t("editor.schedule.title")}
      icon={<CalendarDays className="w-5 h-5" />}
      description={t("editor.schedule.description")}
    >
      <div role="radiogroup" aria-label={t("editor.schedule.mode")} className="inline-flex self-start rounded-xl bg-gray-100 p-1">
        {(["single", "multi"] as const).map((mode) => {
          const active = form.mode === mode;
          return (
            <button
              key={mode}
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => setMode(mode)}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium transition ${
                active ? "bg-white shadow-sm text-gray-900" : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {t(`editor.schedule.${mode}`)}
            </button>
          );
        })}
      </div>

      {form.mode === "single" ? (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <Field id="date" label={t("editor.schedule.date")} error={errors.date} required>
              <input
                id="date"
                type="date"
                className={inputCls(errors.date)}
                value={form.date}
                onChange={(e) => update({ date: e.target.value })}
                aria-invalid={!!errors.date}
                aria-describedby={describedBy("date", errors.date)}
              />
            </Field>
            <TimeField id="startTime" label={t("editor.schedule.start")} value={form.startTime} error={errors.startTime} onChange={(v) => update({ startTime: v })} />
            <TimeField id="endTime" label={t("editor.schedule.end")} value={form.endTime} error={errors.endTime} onChange={(v) => update({ endTime: v })} />
          </div>
          {isOvernight(form.startTime, form.endTime) && <OvernightNote />}
        </>
      ) : (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Field id="rangeStart" label={t("editor.schedule.rangeStart")} error={errors.rangeStart} required>
              <input
                id="rangeStart"
                type="date"
                className={inputCls(errors.rangeStart)}
                value={form.rangeStart}
                onChange={(e) => setRange({ rangeStart: e.target.value })}
                aria-invalid={!!errors.rangeStart}
                aria-describedby={describedBy("rangeStart", errors.rangeStart)}
              />
            </Field>
            <Field
              id="rangeEnd"
              label={t("editor.schedule.rangeEnd")}
              error={errors.rangeEnd}
              hint={t("editor.schedule.rangeHint", { max: MAX_RANGE_DAYS })}
              required
            >
              <input
                id="rangeEnd"
                type="date"
                className={inputCls(errors.rangeEnd)}
                value={form.rangeEnd}
                min={form.rangeStart || undefined}
                max={maxEnd}
                onChange={(e) => setRange({ rangeEnd: e.target.value })}
                aria-invalid={!!errors.rangeEnd}
                aria-describedby={describedBy("rangeEnd", errors.rangeEnd, true)}
              />
            </Field>
          </div>

          <div className="rounded-xl border border-gray-100 bg-gray-50/60 p-4 flex flex-col gap-4">
            <div className="grid grid-cols-2 sm:grid-cols-[1fr_1fr_auto] gap-4 items-end">
              <TimeField id="startTime" label={t("editor.schedule.commonStart")} value={form.startTime} error={errors.startTime} onChange={(v) => setCommon({ startTime: v })} />
              <TimeField id="endTime" label={t("editor.schedule.commonEnd")} value={form.endTime} error={errors.endTime} onChange={(v) => setCommon({ endTime: v })} />
              <button
                type="button"
                onClick={applyToAll}
                disabled={days.length === 0}
                className="col-span-2 sm:col-span-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium border border-gray-200 bg-white text-gray-700 hover:bg-gray-50 transition disabled:opacity-50"
              >
                <RefreshCw className="w-4 h-4" />
                {t("editor.schedule.applyToAll")}
              </button>
            </div>
            {isOvernight(form.startTime, form.endTime) && <OvernightNote />}

            <div>
              <p id="weekdays-label" className="text-xs font-medium text-gray-600 mb-2">
                {t("editor.schedule.weekdays")}
              </p>
              <div role="group" aria-labelledby="weekdays-label" className="flex flex-wrap gap-1.5">
                {mondayFirstWeekdays(i18n.language).map(([day, label]) => {
                  const on = form.weekdays.includes(day);
                  return (
                    <button
                      key={day}
                      type="button"
                      aria-pressed={on}
                      onClick={() => toggleWeekday(day)}
                      className={`min-w-12 px-2.5 py-1.5 rounded-lg text-xs font-semibold border transition ${
                        on ? "text-white border-transparent" : "bg-white text-gray-500 border-gray-200 hover:border-gray-300"
                      }`}
                      style={on ? { backgroundColor: "#ec5b13" } : undefined}
                    >
                      {label}
                    </button>
                  );
                })}
                {form.weekdays.length < 7 && (
                  <button
                    type="button"
                    onClick={() => setRange({ weekdays: ALL_WEEKDAYS })}
                    className="px-2.5 py-1.5 text-xs font-medium hover:underline"
                    style={{ color: "#ec5b13" }}
                  >
                    {t("editor.schedule.allWeekdays")}
                  </button>
                )}
              </div>
            </div>
          </div>

          <div data-editor-error={errors.days ? "true" : undefined}>
            <div className="flex items-center justify-between gap-2 mb-2">
              <h3 className="text-sm font-semibold text-gray-900">
                {t("editor.schedule.daysCount", { count: days.length })}
                <span className="font-normal text-gray-400"> · {t("editor.schedule.daysMax", { max: MAX_SESSIONS })}</span>
              </h3>
            </div>
            {errors.days && (
              <p id="days-error" className="text-xs text-red-600 mb-2" tabIndex={-1}>
                {errors.days}
              </p>
            )}
            {days.length === 0 ? (
              <p className="text-sm text-gray-400 py-6 text-center rounded-xl border border-dashed border-gray-200">
                {t("editor.schedule.noDays")}
              </p>
            ) : (
              <ul className="flex flex-col divide-y divide-gray-100 rounded-xl border border-gray-100">
                {days.map((row) => (
                  <DayRowItem
                    key={row.key}
                    row={row}
                    error={errors[`day-${row.key}`]}
                    locale={i18n.language}
                    onChange={(patch) => setDay(row.key, patch)}
                    onRemove={() => removeDay(row.key)}
                  />
                ))}
              </ul>
            )}
            <button
              type="button"
              onClick={addDay}
              disabled={days.length >= MAX_SESSIONS}
              className="mt-3 flex items-center gap-1.5 text-sm font-medium hover:underline disabled:opacity-50 disabled:no-underline"
              style={{ color: "#ec5b13" }}
            >
              <CalendarPlus className="w-4 h-4" />
              {t("editor.schedule.addDay")}
            </button>
          </div>
        </>
      )}
    </Section>
  );
}

function TimeField({
  id,
  label,
  value,
  error,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  error?: string;
  onChange: (value: string) => void;
}) {
  return (
    <Field id={id} label={label} error={error} required>
      <input
        id={id}
        type="time"
        className={inputCls(error)}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={!!error}
        aria-describedby={describedBy(id, error)}
      />
    </Field>
  );
}

function OvernightNote() {
  const { t } = useTranslation();
  return (
    <p className="-mt-1 flex items-center gap-1.5 text-xs text-gray-500">
      <Moon className="w-3.5 h-3.5" />
      {t("editor.schedule.endsNextDay")}
    </p>
  );
}

function DayRowItem({
  row,
  error,
  locale,
  onChange,
  onRemove,
}: {
  row: DayRow;
  error?: string;
  locale: string;
  onChange: (patch: Partial<DayRow>) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const id = `day-${row.key}`;
  const label = formatDayLabel(row.date, locale) || t("editor.schedule.newDay");
  const small = "border rounded-lg px-2 py-1.5 text-sm outline-none focus:ring-2 bg-white transition";
  const border = error ? "border-red-300 focus:ring-red-100" : "border-gray-200 focus:ring-orange-200 focus:border-orange-400";

  return (
    <li className="px-3 py-2.5" data-editor-error={error ? "true" : undefined}>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <div className="min-w-0 flex-1 basis-40">
          {row.extra ? (
            <input
              id={id}
              type="date"
              aria-label={t("editor.schedule.dayDate")}
              className={`${small} ${border} w-full sm:w-auto`}
              value={row.date}
              onChange={(e) => onChange({ date: e.target.value })}
              aria-invalid={!!error}
              aria-describedby={error ? `${id}-error` : undefined}
            />
          ) : (
            <span id={id} tabIndex={-1} className="text-sm font-medium text-gray-800">
              {label}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          <input
            type="time"
            aria-label={t("editor.schedule.dayStart", { day: label })}
            className={`${small} ${border}`}
            value={row.startTime}
            onChange={(e) => onChange({ startTime: e.target.value })}
            aria-invalid={!!error}
          />
          <span className="text-gray-400" aria-hidden="true">
            –
          </span>
          <input
            type="time"
            aria-label={t("editor.schedule.dayEnd", { day: label })}
            className={`${small} ${border}`}
            value={row.endTime}
            onChange={(e) => onChange({ endTime: e.target.value })}
            aria-invalid={!!error}
          />
          {isOvernight(row.startTime, row.endTime) && (
            <span title={t("editor.schedule.endsNextDay")} className="text-gray-400">
              <Moon className="w-4 h-4" aria-label={t("editor.schedule.endsNextDay")} />
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={onRemove}
          aria-label={t("editor.schedule.removeDay", { day: label })}
          title={t("editor.schedule.removeDay", { day: label })}
          className="p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 transition"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>
      {error && (
        <p id={`${id}-error`} className="text-xs text-red-600 mt-1">
          {error}
        </p>
      )}
    </li>
  );
}
