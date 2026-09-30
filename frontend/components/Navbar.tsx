"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Search, Bell, Ticket, ShieldAlert, User as UserIcon, LogOut } from "lucide-react";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import { clearApiToken, signOut, useSession } from "@/lib/better-auth/client";

const NAV_LINKS = [
  { href: "/", label: "nav.discover" },
  { href: "/my-events", label: "nav.myEvents" },
  { href: "/community", label: "nav.community" },
] as const;

export default function Navbar() {
  const { t } = useTranslation();
  const pathname = usePathname();
  const router = useRouter();
  const { data: session } = useSession();
  const currentUser = session?.user;
  const [menuOpen, setMenuOpen] = useState(false);

  async function handleSignOut() {
    setMenuOpen(false);
    await signOut();
    clearApiToken();
    router.push("/");
    router.refresh();
  }

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
                {t(link.label)}
              </Link>
            );
          })}
        </nav>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Actions */}
        <div className="flex items-center gap-3">
          <LanguageSwitcher />
          {currentUser?.role === "admin" && (
            <Link
              href="/admin"
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                pathname === "/admin"
                  ? "bg-orange-50"
                  : "text-gray-500 hover:text-gray-900 hover:bg-gray-50"
              }`}
              style={pathname === "/admin" ? { color: "#ec5b13" } : undefined}
            >
              <ShieldAlert className="w-4 h-4" />
              {t("nav.admin")}
            </Link>
          )}
          <button
            aria-label={t("nav.search")}
            className="w-9 h-9 flex items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 transition-colors"
          >
            <Search className="w-5 h-5" />
          </button>
          <button
            aria-label={t("nav.notifications")}
            className="w-9 h-9 flex items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 transition-colors relative"
          >
            <Bell className="w-5 h-5" />
            <span className="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-primary" style={{ backgroundColor: "#ec5b13" }} />
          </button>
          {currentUser ? (
            <div className="relative">
              <button
                onClick={() => setMenuOpen((open) => !open)}
                title={currentUser.email}
                aria-expanded={menuOpen}
                className="w-9 h-9 rounded-full overflow-hidden bg-orange-100 flex items-center justify-center shrink-0"
              >
                <span className="text-sm font-semibold" style={{ color: "#ec5b13" }}>
                  {currentUser.email[0]?.toUpperCase()}
                </span>
              </button>
              {menuOpen && (
                <div className="absolute right-0 mt-2 w-56 bg-white rounded-xl border border-gray-100 shadow-lg py-2">
                  <p className="px-4 py-2 text-sm text-gray-500 truncate">{currentUser.email}</p>
                  <button
                    onClick={handleSignOut}
                    className="w-full flex items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
                  >
                    <LogOut className="w-4 h-4" />
                    {t("nav.logout")}
                  </button>
                </div>
              )}
            </div>
          ) : (
            <Link
              href="/login"
              aria-label={t("nav.login")}
              title={t("nav.login")}
              className="w-9 h-9 rounded-full bg-gray-100 flex items-center justify-center shrink-0 text-gray-500 hover:bg-gray-200 transition-colors"
            >
              <UserIcon className="w-5 h-5" />
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
