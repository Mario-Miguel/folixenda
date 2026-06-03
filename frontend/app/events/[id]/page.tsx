import { notFound } from "next/navigation";
import Link from "next/link";
import { Clock, MapPin, Calendar, ChevronLeft, Star, Share2, Bookmark } from "lucide-react";
import { getEvent, getEvents } from "@/lib/api/events";
import EventCard from "@/components/EventCard";

const CATEGORY_COLORS: Record<string, string> = {
  Music: "bg-purple-100 text-purple-700",
  Theater: "bg-blue-100 text-blue-700",
  Parties: "bg-pink-100 text-pink-700",
  Sports: "bg-green-100 text-green-700",
  Food: "bg-yellow-100 text-yellow-700",
  Art: "bg-rose-100 text-rose-700",
  Wellness: "bg-teal-100 text-teal-700",
};

function formatDate(dateStr: string) {
  return new Date(dateStr + "T12:00:00").toLocaleDateString("en-US", {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

export default async function EventDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  let event;
  try {
    event = await getEvent(id);
  } catch {
    notFound();
  }

  const { events: allEvents } = await getEvents({ category: event.category });
  const related = allEvents.filter((e) => e.id !== event.id).slice(0, 3);

  const categoryColor = CATEGORY_COLORS[event.category] ?? "bg-gray-100 text-gray-600";

  return (
    <div className="max-w-4xl mx-auto px-6 py-8">
      {/* Back */}
      <Link
        href="/"
        className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-800 mb-6 transition-colors"
      >
        <ChevronLeft className="w-4 h-4" />
        Back to events
      </Link>

      {/* Hero */}
      <div className="rounded-2xl overflow-hidden bg-gradient-to-br from-orange-100 to-orange-300 h-64 mb-6 relative flex items-center justify-center">
        <span className="text-8xl opacity-20 select-none">🎵</span>
        <div className="absolute top-4 right-4 flex gap-2">
          <button className="w-9 h-9 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm text-gray-600 hover:text-gray-900 transition-colors">
            <Share2 className="w-4 h-4" />
          </button>
          <button
            className="w-9 h-9 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm text-gray-600 transition-colors"
            style={{ color: event.isSaved ? "#ec5b13" : undefined }}
          >
            <Bookmark className="w-4 h-4" />
          </button>
        </div>
        <div className="absolute bottom-4 left-4">
          <span className={`text-xs font-semibold px-3 py-1 rounded-full bg-white/90 ${categoryColor}`}>
            {event.category}
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Main info */}
        <div className="lg:col-span-2 space-y-6">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 mb-2">{event.title}</h1>
            {event.artistName && (
              <p className="text-gray-500 text-sm">
                by <span className="font-medium text-gray-700">{event.artistName}</span>
              </p>
            )}
          </div>

          <p className="text-gray-600 leading-relaxed">{event.description}</p>

          {/* Details */}
          <div className="space-y-3">
            <div className="flex items-start gap-3">
              <div
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "#fff3ed" }}
              >
                <Calendar className="w-5 h-5" style={{ color: "#ec5b13" }} />
              </div>
              <div>
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wide">Date</p>
                <p className="font-medium text-gray-800">{formatDate(event.date)}</p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "#fff3ed" }}
              >
                <Clock className="w-5 h-5" style={{ color: "#ec5b13" }} />
              </div>
              <div>
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wide">Time</p>
                <p className="font-medium text-gray-800">
                  {event.startTime} – {event.endTime}
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ backgroundColor: "#fff3ed" }}
              >
                <MapPin className="w-5 h-5" style={{ color: "#ec5b13" }} />
              </div>
              <div>
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wide">Venue</p>
                <p className="font-medium text-gray-800">{event.venue}</p>
                <p className="text-sm text-gray-500">{event.address}</p>
              </div>
            </div>
          </div>

          {/* Perks */}
          {event.perks && event.perks.length > 0 && (
            <div>
              <h3 className="font-semibold text-gray-900 mb-3">What&apos;s included</h3>
              <ul className="space-y-2">
                {event.perks.map((perk) => (
                  <li key={perk} className="flex items-center gap-2 text-sm text-gray-600">
                    <Star className="w-4 h-4 shrink-0" style={{ color: "#ec5b13" }} />
                    {perk}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>

        {/* Ticket sidebar */}
        <div className="lg:col-span-1">
          <div className="sticky top-24 bg-white rounded-2xl border border-gray-100 shadow-sm p-6">
            <div className="mb-4">
              <p className="text-xs text-gray-400 uppercase tracking-wide font-medium mb-1">Price</p>
              <p className="text-3xl font-bold text-gray-900">{event.price === 0 ? "Free" : `$${event.price}`}</p>
              {event.price > 0 && <p className="text-xs text-gray-400 mt-0.5">per person</p>}
            </div>

            <div
              className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium mb-5"
              style={{ backgroundColor: "#fff3ed", color: "#ec5b13" }}
            >
              🔥 Selling Fast
            </div>

            <button
              className="w-full py-3 rounded-xl text-white font-semibold text-sm transition-opacity hover:opacity-90"
              style={{ backgroundColor: "#ec5b13" }}
            >
              {event.price === 0 ? "Register for free" : "Get tickets"}
            </button>

            <button className="w-full py-3 rounded-xl border border-gray-200 text-gray-700 font-medium text-sm mt-3 hover:bg-gray-50 transition-colors">
              Save for later
            </button>
          </div>
        </div>
      </div>

      {/* Related events */}
      {related.length > 0 && (
        <section className="mt-12">
          <h2 className="text-xl font-bold text-gray-900 mb-5">More {event.category} events</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {related.map((e) => (
              <EventCard key={e.id} event={e} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
