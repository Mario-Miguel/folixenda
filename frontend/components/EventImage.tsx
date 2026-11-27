"use client";

import { ReactNode, useState } from "react";

// Remote event image: uploaded images come from object storage (MinIO / S3) and scraped ones from
// hosts that aren't known ahead of time, so a plain <img> is used instead of next/image.
//
// Holding a live Image per URL keeps it in the document's list of available images, so later
// <img> mounts of the same URL (map card on every hover, detail page after client-side
// navigation) render it without a new request. Images are retained only once they have loaded,
// so lazy-loaded images (lists, gallery thumbnails) aren't fetched before they scroll into view.
const retainedImages = new Map<string, HTMLImageElement>();

function retainImage(url: string) {
  if (retainedImages.has(url)) return;
  const img = new Image();
  img.referrerPolicy = "no-referrer"; // must match the <img> below, and be set before src
  img.src = url;
  retainedImages.set(url, img);
}

interface EventImageProps {
  src?: string;
  alt: string;
  className?: string;
  // Rendered when there is no src or the image fails to load
  fallback?: ReactNode;
  // "eager" for above-the-fold images (detail hero, lightbox); lazy by default
  loading?: "lazy" | "eager";
  // Intrinsic size when known (uploaded images): reserves the right aspect ratio while loading
  width?: number;
  height?: number;
  draggable?: boolean;
}

export default function EventImage({
  src,
  alt,
  className,
  fallback = null,
  loading = "lazy",
  width,
  height,
  draggable,
}: EventImageProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);

  if (!src || failedSrc === src) return fallback;

  return (
    // eslint-disable-next-line @next/next/no-img-element -- remote hosts are not known ahead of time
    <img
      src={src}
      alt={alt}
      // Some source sites (e.g. feteas.org) drop hotlinked requests that carry a foreign Referer
      referrerPolicy="no-referrer"
      loading={loading}
      decoding="async"
      fetchPriority={loading === "eager" ? "high" : undefined}
      width={width}
      height={height}
      draggable={draggable}
      onLoad={() => retainImage(src)}
      onError={() => setFailedSrc(src)}
      className={className}
    />
  );
}
