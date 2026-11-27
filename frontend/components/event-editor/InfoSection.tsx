"use client";

import { KeyboardEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import { Info, X } from "lucide-react";
import { CATEGORIES } from "@/lib/categories";
import { EventCategory } from "@/lib/types";
import { DESCRIPTION_MAX, EditorErrors, EditorForm, MAX_PERKS, PERK_MAX, TITLE_MAX } from "./model";
import { Counter, describedBy, Field, inputCls, Section } from "./ui";

interface Props {
  form: EditorForm;
  errors: EditorErrors;
  update: (patch: Partial<EditorForm>) => void;
}

export default function InfoSection({ form, errors, update }: Props) {
  const { t } = useTranslation();

  return (
    <Section id="info" title={t("editor.info.title")} icon={<Info className="w-5 h-5" />}>
      <Field
        id="title"
        label={t("editor.info.name")}
        error={errors.title}
        required
        aside={<Counter value={form.title.length} max={TITLE_MAX} />}
      >
        <input
          id="title"
          className={inputCls(errors.title)}
          value={form.title}
          onChange={(e) => update({ title: e.target.value })}
          placeholder={t("editor.info.namePlaceholder")}
          aria-invalid={!!errors.title}
          aria-describedby={describedBy("title", errors.title)}
          aria-required
        />
      </Field>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Field id="category" label={t("editor.info.category")} error={errors.category} required>
          <select
            id="category"
            className={inputCls(errors.category)}
            value={form.category}
            onChange={(e) => update({ category: e.target.value as EventCategory })}
            aria-invalid={!!errors.category}
            aria-describedby={describedBy("category", errors.category)}
            aria-required
          >
            <option value="" disabled>
              {t("editor.info.categoryPlaceholder")}
            </option>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {t(`categories.${c}`)}
              </option>
            ))}
          </select>
        </Field>
        <Field id="artistName" label={t("editor.info.artist")} hint={t("editor.info.artistHint")}>
          <input
            id="artistName"
            className={inputCls()}
            value={form.artistName}
            onChange={(e) => update({ artistName: e.target.value })}
            aria-describedby="artistName-hint"
          />
        </Field>
      </div>

      <Field
        id="description"
        label={t("editor.info.description")}
        error={errors.description}
        aside={<Counter value={form.description.length} max={DESCRIPTION_MAX} />}
      >
        <textarea
          id="description"
          className={`${inputCls(errors.description)} resize-y min-h-32`}
          rows={6}
          value={form.description}
          onChange={(e) => update({ description: e.target.value })}
          placeholder={t("editor.info.descriptionPlaceholder")}
          aria-invalid={!!errors.description}
          aria-describedby={describedBy("description", errors.description)}
        />
      </Field>

      <Field id="price" label={t("editor.info.price")} error={errors.price}>
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative w-40">
            <input
              id="price"
              type="number"
              min="0"
              step="0.01"
              inputMode="decimal"
              className={`${inputCls(errors.price)} pr-8`}
              value={form.free ? "" : form.price}
              placeholder={form.free ? t("common.free") : "0,00"}
              onChange={(e) => update({ price: e.target.value })}
              disabled={form.free}
              aria-invalid={!!errors.price}
              aria-describedby={describedBy("price", errors.price)}
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-400" aria-hidden="true">
              €
            </span>
          </div>
          <label className="flex items-center gap-2 text-sm text-gray-700 cursor-pointer select-none">
            <button
              type="button"
              role="switch"
              aria-checked={form.free}
              onClick={() => update({ free: !form.free })}
              className={`relative w-10 h-6 rounded-full transition ${form.free ? "" : "bg-gray-200"}`}
              style={form.free ? { backgroundColor: "#ec5b13" } : undefined}
            >
              <span
                className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform ${
                  form.free ? "translate-x-4" : ""
                }`}
              />
            </button>
            {t("editor.info.free")}
          </label>
        </div>
      </Field>

      <PerksInput perks={form.perks} onChange={(perks) => update({ perks })} />
    </Section>
  );
}

// Chips input: Enter or comma adds, Backspace on an empty input removes the last one
function PerksInput({ perks, onChange }: { perks: string[]; onChange: (perks: string[]) => void }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState("");
  const full = perks.length >= MAX_PERKS;

  function commit() {
    const value = draft.trim().slice(0, PERK_MAX);
    if (value && !full && !perks.includes(value)) onChange([...perks, value]);
    setDraft("");
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit();
    } else if (e.key === "Backspace" && draft === "" && perks.length > 0) {
      onChange(perks.slice(0, -1));
    }
  }

  return (
    <Field id="perks" label={t("editor.info.perks")} hint={t("editor.info.perksHint", { max: MAX_PERKS })}>
      <div className="flex flex-wrap items-center gap-2 border border-gray-200 rounded-lg px-2 py-1.5 bg-white focus-within:ring-2 focus-within:ring-orange-200 focus-within:border-orange-400 transition">
        {perks.map((perk, i) => (
          <span
            key={`${perk}-${i}`}
            className="inline-flex items-center gap-1 rounded-full bg-orange-50 text-orange-800 text-xs font-medium pl-2.5 pr-1 py-1"
          >
            {perk}
            <button
              type="button"
              onClick={() => onChange(perks.filter((_, j) => j !== i))}
              aria-label={t("editor.info.removePerk", { perk })}
              className="p-0.5 rounded-full hover:bg-orange-100"
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}
        <input
          id="perks"
          className="flex-1 min-w-40 text-sm outline-none py-1 bg-transparent disabled:cursor-not-allowed"
          value={draft}
          maxLength={PERK_MAX}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          onBlur={commit}
          disabled={full}
          placeholder={perks.length === 0 ? t("editor.info.perksPlaceholder") : ""}
          aria-describedby="perks-hint"
        />
      </div>
    </Field>
  );
}
