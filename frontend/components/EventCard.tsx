"use client";

import Link from "next/link";
import { MapPin, Clock, Bookmark, BookmarkCheck, Plus } from "lucide-react";
import { Event } from "@/lib/types";

const CATEGORY_COLORS: Record<string, string> = {
  Music: "bg-purple-100 text-purple-700",
  Theater: "bg-blue-100 text-blue-700",
  Parties: "bg-pink-100 text-pink-700",
  Sports: "bg-green-100 text-green-700",
  Food: "bg-yellow-100 text-yellow-700",
  Art: "bg-rose-100 text-rose-700",
  Wellness: "bg-teal-100 text-teal-700",
};

interface EventCardProps {
  event: Event;
  compact?: boolean;
}

export default function EventCard({ event, compact = false }: EventCardProps) {
  const categoryColor = CATEGORY_COLORS[event.category] ?? "bg-gray-100 text-gray-600";

  if (compact) {
    return (
      <Link href={`/events/${event.id}`}>
        <div className="bg-white rounded-xl p-4 border border-gray-100 hover:border-orange-200 hover:shadow-md transition-all cursor-pointer group">
          <div className="flex items-start justify-between gap-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${categoryColor}`}>
                  {event.category}
                </span>
              </div>
              <h3 className="font-semibold text-gray-900 text-sm truncate group-hover:text-primary transition-colors"
                  style={{ ['--tw-text-opacity' as string]: '1' }}>
                {event.title}
              </h3>
              <div className="flex items-center gap-1 mt-1 text-xs text-gray-500">
                <Clock className="w-3 h-3" />
                <span>{event.startTime} – {event.endTime}</span>
              </div>
              <div className="flex items-center gap-1 mt-0.5 text-xs text-gray-500 truncate">
                <MapPin className="w-3 h-3 shrink-0" />
                <span className="truncate">{event.venue}</span>
              </div>
            </div>
            <div className="flex flex-col items-end gap-2 shrink-0">
              <span className="text-sm font-bold text-gray-900">
                {event.price === 0 ? "Free" : `$${event.price}`}
              </span>
              <button className="w-7 h-7 flex items-center justify-center rounded-full bg-orange-50 text-primary hover:bg-orange-100 transition-colors"
                      style={{ color: "#ec5b13" }}>
                <Plus className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </Link>
    );
  }

  return (
    <Link href={`/events/${event.id}`}>
      <div className="bg-white rounded-2xl overflow-hidden border border-gray-100 hover:shadow-lg transition-all cursor-pointer group">
        {/* Image placeholder */}
        <div className="h-40 bg-gradient-to-br from-orange-100 to-orange-200 relative">
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-4xl opacity-30">🎵</span>
          </div>
          <div className="absolute top-3 right-3 flex gap-2">
            <button
              onClick={(e) => e.preventDefault()}
              className="w-8 h-8 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm text-gray-600 hover:text-primary transition-colors"
            >
              {event.isSaved ? (
                <BookmarkCheck className="w-4 h-4" style={{ color: "#ec5b13" }} />
              ) : (
                <Bookmark className="w-4 h-4" />
              )}
            </button>
          </div>
          <div className="absolute bottom-3 left-3">
            <span className={`text-xs font-medium px-2.5 py-1 rounded-full bg-white/90 ${categoryColor}`}>
              {event.category}
            </span>
          </div>
        </div>

        <div className="p-4">
          <h3 className="font-bold text-gray-900 group-hover:text-primary transition-colors mb-1"
              style={{ ['--tw-text-opacity' as string]: '1' }}>
            {event.title}
          </h3>
          <p className="text-sm text-gray-500 line-clamp-2 mb-3">{event.description}</p>

          <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
            <Clock className="w-3.5 h-3.5" />
            <span>{event.startTime} – {event.endTime}</span>
          </div>
          <div className="flex items-center gap-1 text-xs text-gray-500">
            <MapPin className="w-3.5 h-3.5" />
            <span>{event.venue}</span>
          </div>

          <div className="flex items-center justify-between mt-4">
            <span className="font-bold text-gray-900">
              {event.price === 0 ? "Free" : `From $${event.price}`}
            </span>
            <span className="text-xs px-2.5 py-1 rounded-full text-white font-medium"
                  style={{ backgroundColor: "#ec5b13" }}>
              Get tickets
            </span>
          </div>
        </div>
      </div>
    </Link>
  );
}
