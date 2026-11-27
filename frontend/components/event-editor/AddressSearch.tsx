"use client";

import { KeyboardEvent, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Loader2, MapPin, Search } from "lucide-react";
import { ApiError } from "@/lib/api/api";
import { searchAddress } from "@/lib/api/geocode";
import { GeocodeResult } from "@/lib/types";
import { inputCls } from "./ui";

const DEBOUNCE_MS = 300;
const MIN_CHARS = 3;

interface Props {
  id: string;
  value: string;
  onChange: (value: string) => void; // free typing: the address is always editable by hand
  onSelect: (result: GeocodeResult) => void;
  describedBy?: string;
}

// Address input with autocomplete (ARIA 1.2 combobox with a listbox popup)
export default function AddressSearch({ id, value, onChange, onSelect, describedBy }: Props) {
  const { t } = useTranslation();
  const [results, setResults] = useState<GeocodeResult[]>([]);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const [loading, setLoading] = useState(false);
  const [problem, setProblem] = useState<"unavailable" | "failed" | "empty" | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const controller = useRef<AbortController | null>(null);
  const listId = `${id}-listbox`;

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
      controller.current?.abort();
    },
    [],
  );

  async function search(q: string) {
    controller.current?.abort();
    const ctrl = new AbortController();
    controller.current = ctrl;
    setLoading(true);
    try {
      const found = await searchAddress(q, 8, ctrl.signal);
      if (ctrl.signal.aborted) return;
      setResults(found);
      setActive(-1);
      setOpen(true);
      setProblem(found.length === 0 ? "empty" : null);
    } catch (err) {
      if (ctrl.signal.aborted) return;
      setResults([]);
      setOpen(false);
      setProblem(err instanceof ApiError && (err.status === 401 || err.status === 403) ? "unavailable" : "failed");
    } finally {
      if (!ctrl.signal.aborted) setLoading(false);
    }
  }

  function handleInput(text: string) {
    onChange(text);
    if (timer.current) clearTimeout(timer.current);
    if (text.trim().length < MIN_CHARS) {
      controller.current?.abort();
      setLoading(false);
      setResults([]);
      setOpen(false);
      setProblem(null);
      return;
    }
    timer.current = setTimeout(() => search(text.trim()), DEBOUNCE_MS);
  }

  function choose(result: GeocodeResult) {
    onSelect(result);
    setOpen(false);
    setResults([]);
    setActive(-1);
    setProblem(null);
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      if (results.length === 0) return;
      e.preventDefault();
      setOpen(true);
      setActive((i) => (i + 1) % results.length);
    } else if (e.key === "ArrowUp") {
      if (results.length === 0) return;
      e.preventDefault();
      setOpen(true);
      setActive((i) => (i <= 0 ? results.length - 1 : i - 1));
    } else if (e.key === "Enter") {
      e.preventDefault(); // never submit the editor from here
      if (open && active >= 0 && results[active]) choose(results[active]);
    } else if (e.key === "Escape") {
      if (open) {
        e.preventDefault();
        setOpen(false);
        setActive(-1);
      }
    }
  }

  const expanded = open && results.length > 0;

  return (
    <div className="relative">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 pointer-events-none" />
      <input
        id={id}
        role="combobox"
        aria-expanded={expanded}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={expanded && active >= 0 ? `${listId}-${active}` : undefined}
        aria-describedby={[describedBy, problem ? `${id}-search-status` : undefined].filter(Boolean).join(" ") || undefined}
        autoComplete="off"
        className={`${inputCls()} pl-9 pr-9`}
        value={value}
        placeholder={t("editor.place.searchPlaceholder")}
        onChange={(e) => handleInput(e.target.value)}
        onKeyDown={onKeyDown}
        onFocus={() => results.length > 0 && setOpen(true)}
        onBlur={() => setOpen(false)}
      />
      {loading && (
        <Loader2
          className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 animate-spin"
          aria-label={t("editor.place.searching")}
        />
      )}

      <ul
        id={listId}
        role="listbox"
        aria-label={t("editor.place.suggestions")}
        hidden={!expanded}
        className="absolute z-[1200] mt-1 w-full max-h-72 overflow-auto rounded-xl border border-gray-100 bg-white shadow-lg py-1"
      >
        {results.map((r, i) => (
          <li
            key={`${r.lat},${r.lon},${i}`}
            id={`${listId}-${i}`}
            role="option"
            aria-selected={i === active}
            // mousedown keeps the focus in the input (blur would close the list first)
            onMouseDown={(e) => {
              e.preventDefault();
              choose(r);
            }}
            onMouseEnter={() => setActive(i)}
            className={`flex items-start gap-2 px-3 py-2 text-sm cursor-pointer ${i === active ? "bg-orange-50" : ""}`}
          >
            <MapPin className="w-4 h-4 mt-0.5 shrink-0" style={{ color: "#ec5b13" }} />
            <span className="min-w-0">
              <span className="block text-gray-900 truncate">{r.address || r.label}</span>
              <span className="block text-xs text-gray-500 truncate">{r.label}</span>
            </span>
          </li>
        ))}
      </ul>

      <p id={`${id}-search-status`} className="text-xs text-gray-500 mt-1 min-h-0" aria-live="polite">
        {problem === "empty" && t("editor.place.noResults")}
        {problem === "failed" && t("editor.place.searchFailed")}
        {problem === "unavailable" && t("editor.place.searchUnavailable")}
      </p>
    </div>
  );
}
