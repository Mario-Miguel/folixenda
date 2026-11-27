"use client";

import { useCallback, useEffect, useState } from "react";
import { EventsQuery, getEvents } from "@/lib/api/events";
import { Event } from "@/lib/types";

interface Result {
  key: string;
  events: Event[];
  error: unknown;
}

// Loads events matching `query` from the API, refetching whenever the query changes.
// `loading` is derived from whether the stored result belongs to the current query, so a
// stale response for a previous query never shows up as the current one.
export function useEvents(query: EventsQuery) {
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<Result | null>(null);
  const key = `${JSON.stringify(query)}#${attempt}`;

  useEffect(() => {
    let cancelled = false;
    const params: EventsQuery = JSON.parse(key.slice(0, key.lastIndexOf("#")));
    getEvents(params)
      .then((r) => !cancelled && setResult({ key, events: r.events ?? [], error: null }))
      .catch((error) => !cancelled && setResult({ key, events: [], error }));
    return () => {
      cancelled = true;
    };
  }, [key]);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  const current = result?.key === key ? result : null;

  return {
    events: current?.events ?? [],
    loading: current === null,
    error: current?.error ?? null,
    retry,
  };
}
