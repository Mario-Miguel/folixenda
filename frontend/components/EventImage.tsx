"use client";

import { ReactNode, useEffect, useState } from "react";

// Remote event image (hosts come from scrapers, so next/image can't know them ahead of time).
//
// Holding a live Image per URL keeps it in the document's list of available images, so later
// <img> mounts of the same URL (map card on every hover, detail page after client-side
// navigation) render it without a new request.
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
}

export default function EventImage({ src, alt, className, fallback = null }: EventImageProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);

  useEffect(() => {
    if (src) retainImage(src);
  }, [src]);

  if (!src || failedSrc === src) return fallback;

  return (
    // eslint-disable-next-line @next/next/no-img-element -- remote hosts are not known ahead of time
    <img
      src={src}
      alt={alt}
      // Some source sites (e.g. feteas.org) drop hotlinked requests that carry a foreign Referer
      referrerPolicy="no-referrer"
      onError={() => setFailedSrc(src)}
      className={className}
    />
  );
}
