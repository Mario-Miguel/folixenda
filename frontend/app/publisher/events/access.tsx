import Link from "next/link";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { ChevronLeft, ShieldAlert } from "lucide-react";
import { ReactNode } from "react";
import { auth } from "@/lib/better-auth/server";
import type { EditorHome } from "@/components/event-editor/routes";
import { homeHref } from "@/components/event-editor/routes";

// Session + role checks shared by /publisher/events/new and /publisher/events/[id]/edit

export async function requireSession(returnTo: string) {
  let session: Awaited<ReturnType<typeof auth.api.getSession>> = null;
  try {
    session = await auth.api.getSession({ headers: await headers() });
  } catch {
    session = null;
  }
  if (!session) redirect(`/login?redirect=${encodeURIComponent(returnTo)}`);
  return session;
}

// Publishers and admins may use the editor. Mirrors auth.CanPublish in the Go API, which stays
// the single real gate.
// Future: publishing will require a paid subscription. When it does, this is where the page would
// check the account's subscription and show a "subscription required" card instead of the editor.
export function editorHomeFor(role: string | null | undefined): EditorHome | null {
  if (role === "admin") return "admin";
  if (role === "publisher") return "publisher";
  return null;
}

export function AccessCard({
  title,
  text,
  href,
  linkLabel,
}: {
  title: string;
  text: string;
  href: string;
  linkLabel: string;
}) {
  return (
    <div className="min-h-[70vh] flex items-center justify-center px-4">
      <div className="w-full max-w-md bg-white rounded-2xl shadow-sm border border-gray-100 p-8 text-center">
        <ShieldAlert className="w-10 h-10 mx-auto mb-4" style={{ color: "#ec5b13" }} />
        <h1 className="text-xl font-bold text-gray-900 mb-2">{title}</h1>
        <p className="text-sm text-gray-500 mb-6 leading-relaxed">{text}</p>
        <Link
          href={href}
          className="inline-block px-4 py-2 rounded-lg text-sm font-semibold text-white"
          style={{ backgroundColor: "#ec5b13" }}
        >
          {linkLabel}
        </Link>
      </div>
    </div>
  );
}

export function EditorShell({
  home,
  backLabel,
  title,
  subtitle,
  children,
}: {
  home: EditorHome;
  backLabel: string;
  title: string;
  subtitle?: string;
  children: ReactNode;
}) {
  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 pt-6 sm:pt-8 pb-4">
      <nav aria-label="breadcrumb" className="mb-4">
        <Link
          href={homeHref(home)}
          className="inline-flex items-center gap-1 text-sm font-medium text-gray-500 hover:text-gray-800"
        >
          <ChevronLeft className="w-4 h-4" />
          {backLabel}
        </Link>
      </nav>
      <h1 className="text-2xl font-bold text-gray-900">{title}</h1>
      {subtitle && <p className="text-sm text-gray-500 mt-0.5 truncate">{subtitle}</p>}
      <div className="mt-6">{children}</div>
    </div>
  );
}
