"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Search, Bell, Ticket } from "lucide-react";

const NAV_LINKS = [
  { href: "/", label: "Discover" },
  { href: "/my-events", label: "My Events" },
  { href: "/community", label: "Community" },
];

export default function Navbar() {
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-50 bg-white border-b border-gray-100 shadow-sm">
      <div className="max-w-7xl mx-auto px-6 h-16 flex items-center gap-8">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 shrink-0">
          <Ticket className="text-primary w-5 h-5" style={{ color: "#ec5b13" }} />
          <span className="font-bold text-gray-900 text-lg tracking-tight">
            folixenda
          </span>
        </Link>

        {/* Nav links */}
        <nav className="hidden md:flex items-center gap-1">
          {NAV_LINKS.map((link) => {
            const isActive = pathname === link.href;
            return (
              <Link
                key={link.href}
                href={link.href}
                className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                  isActive
                    ? "text-primary bg-orange-50"
                    : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
                }`}
                style={isActive ? { color: "#ec5b13" } : undefined}
              >
                {link.label}
              </Link>
            );
          })}
        </nav>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Actions */}
        <div className="flex items-center gap-3">
          <button className="w-9 h-9 flex items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 transition-colors">
            <Search className="w-5 h-5" />
          </button>
          <button className="w-9 h-9 flex items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 transition-colors relative">
            <Bell className="w-5 h-5" />
            <span className="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-primary" style={{ backgroundColor: "#ec5b13" }} />
          </button>
          <div className="w-9 h-9 rounded-full overflow-hidden bg-orange-100 flex items-center justify-center shrink-0">
            <span className="text-sm font-semibold" style={{ color: "#ec5b13" }}>M</span>
          </div>
        </div>
      </div>
    </header>
  );
}
