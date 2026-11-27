"use client";

import dynamic from "next/dynamic";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Crosshair, Loader2, MapPinned, X } from "lucide-react";
import { ApiError } from "@/lib/api/api";
import { reverseGeocode } from "@/lib/api/geocode";
import { GeocodeResult } from "@/lib/types";
import AddressSearch from "./AddressSearch";
import { EditorErrors, EditorForm } from "./model";
import { describedBy, Field, inputCls, Section } from "./ui";

const LocationPicker = dynamic(() => import("./LocationPicker"), {
  ssr: false,
  loading: () => <div className="h-full w-full animate-pulse bg-gray-100" />,
});

// "Calle Uría 10" + ", Oviedo" unless the street text already names the municipality
export function addressFromResult(r: GeocodeResult): string {
  const street = r.address?.trim();
  if (!street) return r.label;
  if (r.municipality && !street.toLowerCase().includes(r.municipality.toLowerCase())) return `${street}, ${r.municipality}`;
  return street;
}

interface Props {
  form: EditorForm;
  errors: EditorErrors;
  update: (patch: Partial<EditorForm>) => void;
}

type ReverseState = "idle" | "loading" | "notFound" | "failed";

export default function PlaceSection({ form, errors, update }: Props) {
  const { t } = useTranslation();
  const [focusKey, setFocusKey] = useState(0);
  const [reverse, setReverse] = useState<ReverseState>("idle");
  const controller = useRef<AbortController | null>(null);
  const hasPoint = form.lat != null && form.lon != null;

  useEffect(() => () => controller.current?.abort(), []);

  function selectResult(r: GeocodeResult) {
    controller.current?.abort();
    setReverse("idle");
    update({ address: addressFromResult(r), lat: r.lat, lon: r.lon });
    setFocusKey((k) => k + 1);
  }

  // Map click or marker drag: move the point, then look up its address (the venue name is never touched)
  async function pick(lat: number, lon: number) {
    update({ lat, lon });
    controller.current?.abort();
    const ctrl = new AbortController();
    controller.current = ctrl;
    setReverse("loading");
    try {
      const r = await reverseGeocode(lat, lon, ctrl.signal);
      if (ctrl.signal.aborted) return;
      update({ address: addressFromResult(r) });
      setReverse("idle");
    } catch (err) {
      if (ctrl.signal.aborted) return;
      setReverse(err instanceof ApiError && err.status === 404 ? "notFound" : "failed");
    }
  }

  function clearPoint() {
    controller.current?.abort();
    setReverse("idle");
    update({ lat: undefined, lon: undefined });
  }

  return (
    <Section
      id="place"
      title={t("editor.place.title")}
      icon={<MapPinned className="w-5 h-5" />}
      description={t("editor.place.description")}
    >
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Map first on mobile, right column on desktop */}
        <div className="lg:order-last flex flex-col gap-2">
          <div className="relative isolate h-64 sm:h-80 lg:h-full lg:min-h-80 rounded-xl overflow-hidden border border-gray-100">
            <LocationPicker
              lat={form.lat}
              lon={form.lon}
              focusKey={focusKey}
              onPick={pick}
              markerLabel={t("editor.place.marker")}
            />
            {!hasPoint && (
              <div className="pointer-events-none absolute z-[1000] bottom-3 left-3 right-3 rounded-lg bg-white/95 shadow px-3 py-2 text-xs text-gray-600 flex items-center gap-2">
                <Crosshair className="w-4 h-4 shrink-0" style={{ color: "#ec5b13" }} />
                {t("editor.place.mapHint")}
              </div>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-4">
          <Field id="venue" label={t("editor.place.venue")} error={errors.venue} required>
            <input
              id="venue"
              className={inputCls(errors.venue)}
              value={form.venue}
              onChange={(e) => update({ venue: e.target.value })}
              placeholder={t("editor.place.venuePlaceholder")}
              aria-invalid={!!errors.venue}
              aria-describedby={describedBy("venue", errors.venue)}
              aria-required
            />
          </Field>

          <Field id="address" label={t("editor.place.address")} hint={t("editor.place.addressHint")}>
            <AddressSearch
              id="address"
              value={form.address}
              onChange={(address) => update({ address })}
              onSelect={selectResult}
              describedBy="address-hint"
            />
          </Field>

          <div className="rounded-xl bg-gray-50 px-3 py-2.5 text-xs text-gray-600" aria-live="polite">
            {hasPoint ? (
              <div className="flex items-center gap-2">
                <span className="font-medium text-gray-700">{t("editor.place.coordinates")}</span>
                <span className="tabular-nums">
                  {form.lat!.toFixed(5)}, {form.lon!.toFixed(5)}
                </span>
                <button
                  type="button"
                  onClick={clearPoint}
                  className="ml-auto inline-flex items-center gap-1 text-gray-500 hover:text-red-600"
                >
                  <X className="w-3.5 h-3.5" />
                  {t("editor.place.clearPoint")}
                </button>
              </div>
            ) : (
              <p>{t("editor.place.noPoint")}</p>
            )}
            {reverse === "loading" && (
              <p className="flex items-center gap-1.5 mt-1.5 text-gray-500">
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                {t("editor.place.reverseLoading")}
              </p>
            )}
            {reverse === "notFound" && <p className="mt-1.5 text-amber-700">{t("editor.place.reverseNotFound")}</p>}
            {reverse === "failed" && <p className="mt-1.5 text-amber-700">{t("editor.place.reverseFailed")}</p>}
          </div>
        </div>
      </div>
    </Section>
  );
}
