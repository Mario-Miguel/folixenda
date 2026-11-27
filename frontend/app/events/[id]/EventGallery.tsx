"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import EventImage from "@/components/EventImage";
import type { EventImage as EventImageData } from "@/lib/types";

// Horizontal distance (px) a swipe must travel to change image
const SWIPE_THRESHOLD = 50;

// Promotional gallery: responsive grid of thumbnails; clicking one opens a lightbox (native modal
// <dialog>: focus trap, Esc and focus restore come from the browser) with prev/next, arrow keys
// and swipe.
export default function EventGallery({ images, title }: { images: EventImageData[]; title: string }) {
  const { t } = useTranslation();
  const [openIndex, setOpenIndex] = useState<number | null>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const swipeStartX = useRef<number | null>(null);
  const swiped = useRef(false); // the click that ends a swipe must not close the lightbox
  const total = images.length;

  const alt = (i: number) => t("event.galleryImageAlt", { title, index: i + 1, total });
  const show = useCallback((delta: number) => {
    setOpenIndex((i) => (i === null ? i : (i + delta + total) % total));
  }, [total]);

  // Keep the dialog in sync with the state, and stop the page behind it from scrolling
  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (openIndex !== null && !dialog.open) dialog.showModal();
    if (openIndex === null && dialog.open) dialog.close();
  }, [openIndex]);

  const isOpen = openIndex !== null;
  useEffect(() => {
    if (!isOpen) return;
    const root = document.documentElement;
    const previous = root.style.overflow;
    root.style.overflow = "hidden";
    return () => {
      root.style.overflow = previous;
    };
  }, [isOpen]);

  if (total === 0) return null;

  const current = openIndex !== null ? images[openIndex] : null;
  const navBtn =
    "absolute top-1/2 -translate-y-1/2 w-11 h-11 flex items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/30 transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-white";

  return (
    <section>
      <h3 className="font-semibold text-gray-900 mb-3">{t("event.gallery")}</h3>
      <ul className="grid grid-cols-2 sm:grid-cols-3 gap-2">
        {images.map((img, i) => (
          <li key={img.id}>
            <button
              type="button"
              onClick={() => setOpenIndex(i)}
              aria-label={t("event.openImage", { index: i + 1, total })}
              className="group relative block w-full aspect-[4/3] overflow-hidden rounded-xl bg-gray-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
              style={{ outlineColor: "#ec5b13" }}
            >
              <EventImage
                src={img.url}
                alt={alt(i)}
                width={img.width}
                height={img.height}
                className="absolute inset-0 h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
              />
            </button>
          </li>
        ))}
      </ul>

      <dialog
        ref={dialogRef}
        aria-label={t("event.gallery")}
        // Esc closes the native dialog: mirror it in the state
        onClose={() => setOpenIndex(null)}
        onKeyDown={(e) => {
          if (e.key === "ArrowLeft") {
            e.preventDefault();
            show(-1);
          } else if (e.key === "ArrowRight") {
            e.preventDefault();
            show(1);
          }
        }}
        className="fixed inset-0 m-0 h-dvh w-dvw max-h-none max-w-none bg-transparent p-0 backdrop:bg-black/90"
      >
        {current && openIndex !== null && (
          <div
            className="relative h-full w-full flex items-center justify-center select-none"
            // Clicking the dark area around the image closes the lightbox
            onClick={(e) => {
              if (e.target === e.currentTarget && !swiped.current) setOpenIndex(null);
              swiped.current = false;
            }}
            onPointerDown={(e) => {
              swipeStartX.current = e.clientX;
              swiped.current = false;
            }}
            onPointerUp={(e) => {
              const start = swipeStartX.current;
              swipeStartX.current = null;
              if (start === null) return;
              const dx = e.clientX - start;
              if (Math.abs(dx) >= SWIPE_THRESHOLD) {
                swiped.current = true;
                show(dx < 0 ? 1 : -1);
              }
            }}
            style={{ touchAction: "pan-y pinch-zoom" }}
          >
            <EventImage
              key={current.id}
              src={current.url}
              alt={alt(openIndex)}
              loading="eager"
              draggable={false}
              width={current.width}
              height={current.height}
              className="max-h-[85dvh] max-w-[92vw] w-auto h-auto object-contain rounded-lg shadow-2xl"
            />

            <p className="absolute top-4 left-4 text-sm font-medium text-white/80" aria-live="polite">
              {t("event.imageCounter", { index: openIndex + 1, total })}
            </p>
            <button
              type="button"
              onClick={() => setOpenIndex(null)}
              aria-label={t("event.closeGallery")}
              autoFocus
              className="absolute top-3 right-3 w-11 h-11 flex items-center justify-center rounded-full bg-white/15 text-white hover:bg-white/30 transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-white"
            >
              <X className="w-5 h-5" />
            </button>
            {total > 1 && (
              <>
                <button
                  type="button"
                  onClick={() => show(-1)}
                  aria-label={t("event.previousImage")}
                  className={`${navBtn} left-3`}
                >
                  <ChevronLeft className="w-6 h-6" />
                </button>
                <button
                  type="button"
                  onClick={() => show(1)}
                  aria-label={t("event.nextImage")}
                  className={`${navBtn} right-3`}
                >
                  <ChevronRight className="w-6 h-6" />
                </button>
              </>
            )}
          </div>
        )}
      </dialog>
    </section>
  );
}
