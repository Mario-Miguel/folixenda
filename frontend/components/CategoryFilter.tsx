"use client";

import { Music, Drama, PartyPopper, Dumbbell, UtensilsCrossed, Palette, HeartPulse } from "lucide-react";
import { EventCategory } from "@/lib/types";

const CATEGORIES: { label: EventCategory | "All"; icon: React.ReactNode }[] = [
  { label: "All", icon: null },
  { label: "Music", icon: <Music className="w-4 h-4" /> },
  { label: "Theater", icon: <Drama className="w-4 h-4" /> },
  { label: "Parties", icon: <PartyPopper className="w-4 h-4" /> },
  { label: "Sports", icon: <Dumbbell className="w-4 h-4" /> },
  { label: "Food", icon: <UtensilsCrossed className="w-4 h-4" /> },
  { label: "Art", icon: <Palette className="w-4 h-4" /> },
  { label: "Wellness", icon: <HeartPulse className="w-4 h-4" /> },
];

interface CategoryFilterProps {
  selected: string;
  onChange: (category: string) => void;
}

export default function CategoryFilter({ selected, onChange }: CategoryFilterProps) {
  return (
    <div className="flex items-center gap-2 overflow-x-auto pb-1 scrollbar-hide">
      {CATEGORIES.map(({ label, icon }) => {
        const isActive = selected === label;
        return (
          <button
            key={label}
            onClick={() => onChange(label)}
            className={`flex items-center gap-1.5 px-4 py-2 rounded-full text-sm font-medium whitespace-nowrap transition-all border ${
              isActive
                ? "text-white border-primary shadow-sm"
                : "text-gray-600 bg-white border-gray-200 hover:border-gray-300 hover:text-gray-800"
            }`}
            style={isActive ? { backgroundColor: "#ec5b13", borderColor: "#ec5b13" } : undefined}
          >
            {icon}
            {label}
          </button>
        );
      })}
    </div>
  );
}
