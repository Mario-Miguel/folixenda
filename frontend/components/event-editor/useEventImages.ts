"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { deleteEventImage, reorderEventImages, uploadEventImage } from "@/lib/api/events";
import { EventImage, EventImageKind } from "@/lib/types";
import { describeApiError } from "./errors";
import { newKey } from "./model";

// Cover + gallery state of the editor.
// With an eventId (editing) every change goes to the API right away; without one (new event) the
// files stay local until uploadPending() runs after the event has been created.

export const ACCEPTED_TYPES = ["image/jpeg", "image/png", "image/webp"];
export const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
export const MAX_GALLERY = 10;

export interface ImageItem {
  key: string;
  url: string; // what to show: the local preview while there is one, else the uploaded URL
  remote?: EventImage; // set once uploaded
  file?: File; // local file (new events, or while uploading)
  progress?: number; // 0..1 while uploading
}

export interface ImageProblem {
  key: string;
  name: string;
  message: string;
}

type Slot = EventImageKind;

const fromRemote = (img: EventImage): ImageItem => ({ key: img.id, url: img.url, remote: img });

export function useEventImages(eventId: string | undefined, initial: EventImage[]) {
  const { t } = useTranslation();
  const [cover, setCover] = useState<ImageItem | null>(() => {
    const c = initial.find((i) => i.kind === "cover");
    return c ? fromRemote(c) : null;
  });
  const [gallery, setGallery] = useState<ImageItem[]>(() =>
    initial
      .filter((i) => i.kind === "gallery")
      .sort((a, b) => a.position - b.position)
      .map(fromRemote),
  );
  const [problems, setProblems] = useState<Record<Slot, ImageProblem[]>>({ cover: [], gallery: [] });
  const [busy, setBusy] = useState(0); // API calls in flight
  const previews = useRef(new Set<string>());

  // Object URLs are only released when the editor goes away (they stay as previews after upload)
  useEffect(() => {
    const urls = previews.current;
    return () => {
      urls.forEach((u) => URL.revokeObjectURL(u));
      urls.clear();
    };
  }, []);

  function preview(file: File): string {
    const url = URL.createObjectURL(file);
    previews.current.add(url);
    return url;
  }

  function release(item: ImageItem | null | undefined) {
    if (item && previews.current.has(item.url)) {
      URL.revokeObjectURL(item.url);
      previews.current.delete(item.url);
    }
  }

  function fileProblem(file: File): string | null {
    if (!ACCEPTED_TYPES.includes(file.type)) return t("editor.images.errors.type");
    if (file.size > MAX_IMAGE_BYTES) return t("editor.images.errors.size");
    return null;
  }

  const addProblems = (slot: Slot, list: ImageProblem[]) =>
    list.length > 0 && setProblems((p) => ({ ...p, [slot]: [...p[slot], ...list] }));
  const dismissProblem = (slot: Slot, key: string) =>
    setProblems((p) => ({ ...p, [slot]: p[slot].filter((x) => x.key !== key) }));
  const clearProblems = (slot: Slot) => setProblems((p) => ({ ...p, [slot]: [] }));

  async function track<T>(work: () => Promise<T>): Promise<T> {
    setBusy((b) => b + 1);
    try {
      return await work();
    } finally {
      setBusy((b) => b - 1);
    }
  }

  const apiMessage = (err: unknown) => describeApiError(err, t, "image").message;

  async function setCoverFile(file: File) {
    clearProblems("cover");
    const problem = fileProblem(file);
    if (problem) return addProblems("cover", [{ key: newKey(), name: file.name, message: problem }]);

    const previous = cover;
    const item: ImageItem = { key: newKey(), url: preview(file), file, progress: eventId ? 0 : undefined };
    setCover(item);
    if (!eventId) {
      release(previous);
      return;
    }
    await track(async () => {
      try {
        const img = await uploadEventImage(eventId, file, "cover", (p) =>
          setCover((c) => (c?.key === item.key ? { ...c, progress: p } : c)),
        );
        setCover((c) => (c?.key === item.key ? { ...item, remote: img, progress: undefined } : c));
      } catch (err) {
        setCover((c) => (c?.key === item.key ? previous : c));
        release(item);
        addProblems("cover", [{ key: newKey(), name: file.name, message: apiMessage(err) }]);
      }
    });
  }

  async function removeCover() {
    const current = cover;
    if (!current) return;
    clearProblems("cover");
    if (eventId && current.remote) {
      await track(async () => {
        try {
          await deleteEventImage(eventId, current.remote!.id);
          setCover(null);
        } catch (err) {
          addProblems("cover", [{ key: newKey(), name: "", message: apiMessage(err) }]);
        }
      });
    } else {
      release(current);
      setCover(null);
    }
  }

  async function addGalleryFiles(files: File[]) {
    clearProblems("gallery");
    const accepted: ImageItem[] = [];
    const rejected: ImageProblem[] = [];
    let room = MAX_GALLERY - gallery.length;
    for (const file of files) {
      const problem = fileProblem(file) ?? (room <= 0 ? t("editor.images.errors.galleryFull") : null);
      if (problem) {
        rejected.push({ key: newKey(), name: file.name, message: problem });
        continue;
      }
      room -= 1;
      accepted.push({ key: newKey(), url: preview(file), file, progress: eventId ? 0 : undefined });
    }
    addProblems("gallery", rejected);
    if (accepted.length === 0) return;
    setGallery((g) => [...g, ...accepted]);
    if (!eventId) return;

    // One at a time, so the server keeps the order they were picked in
    await track(async () => {
      for (const item of accepted) {
        try {
          const img = await uploadEventImage(eventId, item.file!, "gallery", (p) =>
            setGallery((g) => g.map((x) => (x.key === item.key ? { ...x, progress: p } : x))),
          );
          setGallery((g) => g.map((x) => (x.key === item.key ? { ...x, remote: img, progress: undefined } : x)));
        } catch (err) {
          setGallery((g) => g.filter((x) => x.key !== item.key));
          release(item);
          addProblems("gallery", [{ key: newKey(), name: item.file!.name, message: apiMessage(err) }]);
        }
      }
    });
  }

  async function removeGalleryItem(key: string) {
    const item = gallery.find((x) => x.key === key);
    if (!item) return;
    if (eventId && item.remote) {
      await track(async () => {
        try {
          await deleteEventImage(eventId, item.remote!.id);
          setGallery((g) => g.filter((x) => x.key !== key));
        } catch (err) {
          addProblems("gallery", [{ key: newKey(), name: item.file?.name ?? "", message: apiMessage(err) }]);
        }
      });
    } else {
      release(item);
      setGallery((g) => g.filter((x) => x.key !== key));
    }
  }

  async function moveGalleryItem(from: number, to: number) {
    if (busy > 0 || from === to || to < 0 || to >= gallery.length) return;
    const previous = gallery;
    const next = [...gallery];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    setGallery(next);
    if (!eventId || !next.every((x) => x.remote)) return;
    await track(async () => {
      try {
        await reorderEventImages(
          eventId,
          next.map((x) => x.remote!.id),
        );
      } catch (err) {
        setGallery(previous);
        addProblems("gallery", [{ key: newKey(), name: "", message: apiMessage(err) }]);
      }
    });
  }

  const pendingCount = (cover && !cover.remote ? 1 : 0) + gallery.filter((x) => !x.remote).length;

  // New events: upload the local files to the just-created event (cover, then the gallery in
  // order). Returns the names of the files that failed; never throws.
  async function uploadPending(newEventId: string, onStep?: (done: number, total: number) => void): Promise<string[]> {
    const total = pendingCount;
    let done = 0;
    const failed: string[] = [];
    onStep?.(0, total);

    if (cover?.file && !cover.remote) {
      const item = cover;
      try {
        const img = await uploadEventImage(newEventId, item.file!, "cover", (p) =>
          setCover((c) => (c?.key === item.key ? { ...c, progress: p } : c)),
        );
        setCover((c) => (c?.key === item.key ? { ...c, remote: img, progress: undefined } : c));
      } catch {
        failed.push(item.file!.name);
        setCover((c) => (c?.key === item.key ? { ...c, progress: undefined } : c));
      }
      onStep?.(++done, total);
    }

    const uploaded: EventImage[] = [];
    for (const item of gallery) {
      if (item.remote || !item.file) continue;
      try {
        const img = await uploadEventImage(newEventId, item.file, "gallery", (p) =>
          setGallery((g) => g.map((x) => (x.key === item.key ? { ...x, progress: p } : x))),
        );
        uploaded.push(img);
        setGallery((g) => g.map((x) => (x.key === item.key ? { ...x, remote: img, progress: undefined } : x)));
      } catch {
        failed.push(item.file.name);
        setGallery((g) => g.map((x) => (x.key === item.key ? { ...x, progress: undefined } : x)));
      }
      onStep?.(++done, total);
    }

    // Sequential uploads should already be in order; fix it if the server numbered them otherwise
    const inOrder = uploaded.every((img, i) => i === 0 || img.position > uploaded[i - 1].position);
    if (!inOrder) {
      try {
        await reorderEventImages(
          newEventId,
          uploaded.map((img) => img.id),
        );
      } catch {
        // Not worth failing over: the images are there, just in another order
      }
    }
    return failed;
  }

  return {
    cover,
    gallery,
    problems,
    busy: busy > 0,
    pendingCount,
    setCoverFile,
    removeCover,
    addGalleryFiles,
    removeGalleryItem,
    moveGalleryItem,
    dismissProblem,
    uploadPending,
  };
}

export type EventImagesState = ReturnType<typeof useEventImages>;
