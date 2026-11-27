"use client";

import { DragEvent, ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, ArrowLeft, ArrowRight, ImagePlus, Images, RefreshCw, Trash2, Upload, X } from "lucide-react";
import EventImage from "@/components/EventImage";
import { EditorErrors } from "./model";
import { describedBy, Field, inputCls, ProgressBar, Section } from "./ui";
import { ACCEPTED_TYPES, EventImagesState, ImageItem, ImageProblem, MAX_GALLERY } from "./useEventImages";

const ACCEPT = ACCEPTED_TYPES.join(",");
const REORDER_TYPE = "application/x-folixenda-image";

interface Props {
  images: EventImagesState;
  immediate: boolean; // editing: changes are saved right away
  imageUrl: string;
  onImageUrlChange: (value: string) => void;
  errors: EditorErrors;
}

export default function ImagesSection({ images, immediate, imageUrl, onImageUrlChange, errors }: Props) {
  const { t } = useTranslation();
  const { cover, gallery, problems, busy } = images;
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const external = imageUrl.trim();

  return (
    <Section
      id="images"
      title={t("editor.images.title")}
      icon={<Images className="w-5 h-5" />}
      description={immediate ? t("editor.images.immediateNote") : t("editor.images.deferredNote")}
    >
      {/* Cover */}
      <div className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between gap-2">
          <h3 className="text-sm font-semibold text-gray-900">{t("editor.images.cover")}</h3>
          <span className="text-xs text-gray-400">{t("editor.images.rules")}</span>
        </div>
        <input
          id="cover-file"
          type="file"
          accept={ACCEPT}
          className="sr-only"
          onChange={(e) => {
            const file = e.target.files?.[0];
            e.target.value = "";
            if (file) images.setCoverFile(file);
          }}
          disabled={busy}
          aria-label={t("editor.images.chooseCover")}
        />
        {cover ? (
          <div className="relative aspect-video w-full rounded-xl overflow-hidden bg-gray-100 border border-gray-100">
            {/* eslint-disable-next-line @next/next/no-img-element -- local previews and object storage URLs */}
            <img src={cover.url} alt={t("editor.images.coverAlt")} className="absolute inset-0 h-full w-full object-cover" />
            {cover.progress != null && <UploadOverlay progress={cover.progress} label={t("editor.images.uploading")} />}
            <div className="absolute top-2 right-2 flex gap-2">
              <label
                htmlFor="cover-file"
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white/95 shadow text-xs font-medium text-gray-700 hover:bg-white cursor-pointer"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                {t("editor.images.replace")}
              </label>
              <button
                type="button"
                onClick={() => images.removeCover()}
                disabled={busy}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white/95 shadow text-xs font-medium text-red-600 hover:bg-white disabled:opacity-50"
              >
                <Trash2 className="w-3.5 h-3.5" />
                {t("editor.images.remove")}
              </button>
            </div>
          </div>
        ) : (
          <DropZone
            inputId="cover-file"
            onFiles={(files) => files[0] && images.setCoverFile(files[0])}
            className="aspect-video w-full"
          >
            {external ? (
              <div className="absolute inset-0">
                <EventImage src={external} alt={t("editor.images.coverAlt")} className="h-full w-full object-cover opacity-60" />
                <div className="absolute inset-0 flex flex-col items-center justify-center gap-1 bg-white/40 text-center px-4">
                  <Upload className="w-6 h-6" style={{ color: "#ec5b13" }} />
                  <span className="text-sm font-medium text-gray-800">{t("editor.images.replaceExternal")}</span>
                </div>
              </div>
            ) : (
              <>
                <ImagePlus className="w-8 h-8" style={{ color: "#ec5b13" }} />
                <span className="text-sm font-medium text-gray-800">{t("editor.images.dropCover")}</span>
                <span className="text-xs text-gray-500">{t("editor.images.coverHint")}</span>
              </>
            )}
          </DropZone>
        )}
        <Problems list={problems.cover} onDismiss={(key) => images.dismissProblem("cover", key)} />

        {!cover && (
          <Field
            id="imageUrl"
            label={t("editor.images.externalUrl")}
            error={errors.imageUrl}
            hint={t("editor.images.externalUrlHint")}
          >
            <div className="flex gap-2">
              <input
                id="imageUrl"
                type="url"
                inputMode="url"
                placeholder="https://"
                className={inputCls(errors.imageUrl)}
                value={imageUrl}
                onChange={(e) => onImageUrlChange(e.target.value)}
                aria-invalid={!!errors.imageUrl}
                aria-describedby={describedBy("imageUrl", errors.imageUrl, true)}
              />
              {external && (
                <button
                  type="button"
                  onClick={() => onImageUrlChange("")}
                  aria-label={t("editor.images.clearExternal")}
                  className="p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50"
                >
                  <X className="w-4 h-4" />
                </button>
              )}
            </div>
          </Field>
        )}
      </div>

      {/* Gallery */}
      <div className="flex flex-col gap-2 pt-2 border-t border-gray-100">
        <div className="flex items-baseline justify-between gap-2 pt-3">
          <h3 className="text-sm font-semibold text-gray-900">{t("editor.images.gallery")}</h3>
          <span className="text-xs text-gray-400 tabular-nums">
            {gallery.length}/{MAX_GALLERY}
          </span>
        </div>
        <p className="text-xs text-gray-500">{t("editor.images.galleryHint")}</p>
        <input
          id="gallery-files"
          type="file"
          accept={ACCEPT}
          multiple
          className="sr-only"
          onChange={(e) => {
            const files = Array.from(e.target.files ?? []);
            e.target.value = "";
            if (files.length) images.addGalleryFiles(files);
          }}
          disabled={gallery.length >= MAX_GALLERY}
          aria-label={t("editor.images.chooseGallery")}
        />
        <ul className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
          {gallery.map((item, index) => (
            <GalleryTile
              key={item.key}
              item={item}
              index={index}
              count={gallery.length}
              busy={busy}
              dragging={dragIndex === index}
              onDragStart={() => setDragIndex(index)}
              onDragEnd={() => setDragIndex(null)}
              onDropHere={() => {
                if (dragIndex != null) images.moveGalleryItem(dragIndex, index);
                setDragIndex(null);
              }}
              onMove={(to) => images.moveGalleryItem(index, to)}
              onRemove={() => images.removeGalleryItem(item.key)}
            />
          ))}
          {gallery.length < MAX_GALLERY && (
            <li>
              <DropZone inputId="gallery-files" onFiles={images.addGalleryFiles} className="aspect-square w-full">
                <ImagePlus className="w-6 h-6" style={{ color: "#ec5b13" }} />
                <span className="text-xs font-medium text-gray-700 px-2 text-center">{t("editor.images.addGallery")}</span>
              </DropZone>
            </li>
          )}
        </ul>
        <Problems list={problems.gallery} onDismiss={(key) => images.dismissProblem("gallery", key)} />
      </div>
    </Section>
  );
}

function DropZone({
  inputId,
  onFiles,
  className = "",
  children,
}: {
  inputId: string;
  onFiles: (files: File[]) => void;
  className?: string;
  children: ReactNode;
}) {
  const [over, setOver] = useState(false);
  const isFileDrag = (e: DragEvent) => e.dataTransfer.types.includes("Files");
  return (
    <label
      htmlFor={inputId}
      onDragOver={(e) => {
        if (!isFileDrag(e)) return;
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        if (!isFileDrag(e)) return;
        e.preventDefault();
        setOver(false);
        onFiles(Array.from(e.dataTransfer.files));
      }}
      className={`relative flex flex-col items-center justify-center gap-1.5 rounded-xl border-2 border-dashed cursor-pointer overflow-hidden transition ${
        over ? "border-orange-400 bg-orange-50" : "border-gray-200 bg-gray-50 hover:border-orange-300 hover:bg-orange-50/50"
      } ${className}`}
    >
      {children}
    </label>
  );
}

function UploadOverlay({ progress, label }: { progress: number; label: string }) {
  return (
    <div className="absolute inset-x-0 bottom-0 p-2 bg-gradient-to-t from-black/50 to-transparent">
      <span className="block text-[11px] font-medium text-white mb-1">
        {label} {Math.round(progress * 100)}%
      </span>
      <ProgressBar value={progress} label={label} />
    </div>
  );
}

function GalleryTile({
  item,
  index,
  count,
  busy,
  dragging,
  onDragStart,
  onDragEnd,
  onDropHere,
  onMove,
  onRemove,
}: {
  item: ImageItem;
  index: number;
  count: number;
  busy: boolean;
  dragging: boolean;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDropHere: () => void;
  onMove: (to: number) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const n = index + 1;
  const btn =
    "p-1.5 rounded-lg bg-white/95 shadow text-gray-700 hover:bg-white disabled:opacity-40 disabled:cursor-not-allowed";
  return (
    <li
      draggable={!busy}
      onDragStart={(e) => {
        e.dataTransfer.setData(REORDER_TYPE, item.key);
        e.dataTransfer.effectAllowed = "move";
        onDragStart();
      }}
      onDragEnd={onDragEnd}
      onDragOver={(e) => {
        if (e.dataTransfer.types.includes(REORDER_TYPE)) e.preventDefault();
      }}
      onDrop={(e) => {
        if (!e.dataTransfer.types.includes(REORDER_TYPE)) return;
        e.preventDefault();
        onDropHere();
      }}
      className={`group relative aspect-square rounded-xl overflow-hidden bg-gray-100 border border-gray-100 ${
        busy ? "" : "cursor-grab active:cursor-grabbing"
      } ${dragging ? "opacity-40" : ""}`}
    >
      {/* eslint-disable-next-line @next/next/no-img-element -- local previews and object storage URLs */}
      <img src={item.url} alt={t("editor.images.galleryAlt", { n })} className="absolute inset-0 h-full w-full object-cover" draggable={false} />
      <span className="absolute top-2 left-2 text-[11px] font-semibold rounded-full bg-black/50 text-white px-2 py-0.5">{n}</span>
      {item.progress != null && <UploadOverlay progress={item.progress} label={t("editor.images.uploading")} />}
      <div className="absolute top-2 right-2">
        <button type="button" onClick={onRemove} disabled={busy} aria-label={t("editor.images.removeN", { n })} className={`${btn} text-red-600`}>
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      </div>
      {item.progress == null && (
        <div className="absolute bottom-2 inset-x-2 flex justify-between">
          <button type="button" onClick={() => onMove(index - 1)} disabled={busy || index === 0} aria-label={t("editor.images.moveLeft", { n })} className={btn}>
            <ArrowLeft className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            onClick={() => onMove(index + 1)}
            disabled={busy || index === count - 1}
            aria-label={t("editor.images.moveRight", { n })}
            className={btn}
          >
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        </div>
      )}
    </li>
  );
}

function Problems({ list, onDismiss }: { list: ImageProblem[]; onDismiss: (key: string) => void }) {
  const { t } = useTranslation();
  if (list.length === 0) return null;
  return (
    <ul role="alert" className="flex flex-col gap-1.5">
      {list.map((p) => (
        <li key={p.key} className="flex items-start gap-2 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span className="flex-1 min-w-0 break-words">
            {p.name && <span className="font-semibold">{p.name}: </span>}
            {p.message}
          </span>
          <button type="button" onClick={() => onDismiss(p.key)} aria-label={t("common.close")} className="text-red-400 hover:text-red-700">
            <X className="w-3.5 h-3.5" />
          </button>
        </li>
      ))}
    </ul>
  );
}
