"use client";

import { useState, useEffect, useCallback, FormEvent } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslation } from "react-i18next";
import { User, Event } from "@/lib/types";
import { getUsers, updateUser } from "@/lib/api/users";
import { deleteUserAction, DeleteUserResult } from "./actions";
import { getEvents, deleteEvent } from "@/lib/api/events";
import { ApiError } from "@/lib/api/api";
import { clearApiToken, signOut } from "@/lib/better-auth/client";
import { formatEventDateRange } from "@/lib/dates";
import { LogOut, Plus, Pencil, Trash2 } from "lucide-react";
import { editHref, NEW_EVENT_HREF, SAVED_PARAM } from "@/components/event-editor/routes";

const ROLES = ["admin", "consumer", "publisher"] as const;

// The account type is fixed at signup: a person (consumer) can't become a company
// (publisher) or vice versa; they need a new account. Granting/removing admin is fine.
// Mirrors auth.AccountTypeChangeAllowed in the Go API.
function roleChangeAllowed(from: User["role"], to: User["role"]) {
  return !((from === "consumer" && to === "publisher") || (from === "publisher" && to === "consumer"));
}

// Picks a message for a failed API call: permission errors get their own text
function useErrorMessage() {
  const { t } = useTranslation();
  return useCallback(
    (err: unknown, fallback: string) => {
      if (err instanceof ApiError && err.status === 403) return t("admin.errors.forbidden");
      if (err instanceof ApiError && err.status === 401) return t("admin.errors.unauthorized");
      return fallback;
    },
    [t],
  );
}

// ── Dashboard shell ───────────────────────────────────────────────────────────

// Rendered by the server page only for sessions with the admin role
export default function AdminDashboard({ name, currentUserId }: { name: string; currentUserId: string }) {
  const { t } = useTranslation();
  const router = useRouter();
  // The event editor sends admins back with ?tab=events (and ?saved=created|updated)
  const searchParams = useSearchParams();
  const [tab, setTab] = useState<"users" | "events">(() =>
    searchParams.get("tab") === "events" || searchParams.has(SAVED_PARAM) ? "events" : "users",
  );

  async function handleLogout() {
    await signOut();
    clearApiToken();
    router.push("/");
    router.refresh();
  }

  return (
    <div className="max-w-6xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{t("admin.dashboard")}</h1>
          <p className="text-sm text-gray-500 mt-0.5">{t("admin.loggedInAs", { name })}</p>
        </div>
        <button
          onClick={handleLogout}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100 transition"
        >
          <LogOut className="w-4 h-4" />
          {t("admin.logOut")}
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 border-b border-gray-200">
        {(["users", "events"] as const).map((key) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-5 py-2.5 text-sm font-medium capitalize transition -mb-px border-b-2 ${
              tab === key ? "border-orange-500 text-gray-900" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
            style={tab === key ? { borderColor: "#ec5b13", color: "#1a1a1a" } : undefined}
          >
            {t(`admin.tabs.${key}`)}
          </button>
        ))}
      </div>

      {tab === "users" ? <UsersSection currentUserId={currentUserId} /> : <EventsSection />}
    </div>
  );
}

// ── Users section ─────────────────────────────────────────────────────────────
// Accounts are created through sign up (Better Auth), so admins can only edit or delete them here

type UserUpdate = Pick<User, "name" | "role">;

function UsersSection({ currentUserId }: { currentUserId: string }) {
  const { t } = useTranslation();
  const errorMessage = useErrorMessage();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<User | null>(null);
  const [feedback, setFeedback] = useState("");

  const [version, setVersion] = useState(0);
  const load = () => setVersion((v) => v + 1);

  useEffect(() => {
    getUsers()
      .then(setUsers)
      .catch((err) => setFeedback(errorMessage(err, t("admin.users.loadFailed"))))
      .finally(() => setLoading(false));
  }, [version, errorMessage, t]);

  async function handleDelete(u: User) {
    if (!confirm(t("admin.users.confirmDelete", { name: u.name || u.email }))) return;
    // Deleted in Better Auth; the Go profile and saved events cascade
    const result = await deleteUserAction(u.id).catch((): DeleteUserResult => ({ ok: false, error: "failed" }));
    if (result.ok) {
      setFeedback(t("admin.users.deleted"));
    } else {
      const messages: Record<typeof result.error, string> = {
        unauthorized: t("admin.errors.unauthorized"),
        forbidden: t("admin.errors.forbidden"),
        self: t("admin.users.deleteSelf"),
        notFound: t("admin.users.notFound"),
        failed: t("admin.users.deleteFailed"),
      };
      setFeedback(messages[result.error]);
    }
    load();
  }

  async function handleSave(id: string, data: UserUpdate) {
    try {
      await updateUser(id, data);
      setFeedback(t("admin.users.updated"));
      setEditing(null);
      load();
    } catch (err) {
      setFeedback(
        err instanceof ApiError && err.status === 409
          ? t("admin.users.accountTypeFixed")
          : errorMessage(err, t("admin.users.updateFailed"))
      );
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-gray-500">{t("admin.users.count", { count: users.length })}</p>
      </div>

      {feedback && <p className="text-sm text-gray-600 bg-gray-50 rounded-lg px-3 py-2 mb-4">{feedback}</p>}

      {loading ? (
        <p className="text-sm text-gray-400 py-10 text-center">{t("common.loading")}</p>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {[t("admin.fields.name"), t("admin.fields.email"), t("admin.fields.role"), ""].map((h) => (
                  <th
                    key={h}
                    className="text-left px-5 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {users.map((u) => (
                <tr key={u.id} className="hover:bg-gray-50 transition">
                  <td className="px-5 py-3 font-medium text-gray-900">{u.name}</td>
                  <td className="px-5 py-3 text-gray-600">{u.email}</td>
                  <td className="px-5 py-3">
                    <RoleBadge role={u.role} />
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2 justify-end">
                      <button
                        onClick={() => setEditing(u)}
                        aria-label={t("common.edit")}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(u)}
                        disabled={u.id === currentUserId}
                        title={u.id === currentUserId ? t("admin.users.deleteSelf") : undefined}
                        aria-label={t("common.delete")}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 transition disabled:opacity-30 disabled:pointer-events-none"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editing && <UserModalForm user={editing} onSave={handleSave} onClose={() => setEditing(null)} />}
    </div>
  );
}

function RoleBadge({ role }: { role: User["role"] }) {
  const { t } = useTranslation();
  const colors: Record<string, string> = {
    admin: "bg-orange-100 text-orange-700",
    publisher: "bg-blue-100 text-blue-700",
    consumer: "bg-gray-100 text-gray-600",
  };
  return (
    <span
      className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium ${colors[role] ?? "bg-gray-100 text-gray-600"}`}
    >
      {t(`admin.roles.${role}`)}
    </span>
  );
}

function UserModalForm({
  user,
  onSave,
  onClose,
}: {
  user: User;
  onSave: (id: string, data: UserUpdate) => Promise<void>;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [form, setForm] = useState<UserUpdate>({ name: user.name, role: user.role });
  const [saving, setSaving] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    await onSave(user.id, form);
    setSaving(false);
  }

  return (
    <Modal title={t("admin.users.edit")} onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {/* The email is the Better Auth login, so it is shown but not editable here */}
        <Field label={t("admin.fields.email")}>
          <input type="email" className={`${inputCls} bg-gray-50 text-gray-500`} value={user.email} readOnly disabled />
        </Field>
        <Field label={t("admin.fields.name")}>
          <input
            className={inputCls}
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
          />
        </Field>
        <Field label={t("admin.fields.role")}>
          <select
            className={inputCls}
            value={form.role}
            onChange={(e) => setForm((f) => ({ ...f, role: e.target.value as User["role"] }))}
          >
            {ROLES.filter((r) => roleChangeAllowed(user.role, r)).map((r) => (
              <option key={r} value={r}>
                {t(`admin.roles.${r}`)}
              </option>
            ))}
          </select>
          <p className="mt-1 text-xs text-gray-500">{t("admin.users.accountTypeFixed")}</p>
        </Field>
        <ModalActions onClose={onClose} saving={saving} />
      </form>
    </Modal>
  );
}

// ── Events section ────────────────────────────────────────────────────────────

// Creating and editing happen on the full-page editor (/publisher/events/new, /publisher/events/{id}/edit)
function EventsSection() {
  const { t, i18n } = useTranslation();
  const errorMessage = useErrorMessage();
  const router = useRouter();
  const pathname = usePathname();
  const saved = useSearchParams().get(SAVED_PARAM);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [feedback, setFeedback] = useState(() =>
    saved === "created" ? t("admin.events.created") : saved === "updated" ? t("admin.events.updated") : "",
  );

  // Drop ?saved= so a reload doesn't show the notice again (stay on the events tab)
  useEffect(() => {
    if (saved) router.replace(`${pathname}?tab=events`, { scroll: false });
  }, [saved, pathname, router]);

  const [version, setVersion] = useState(0);
  const load = () => setVersion((v) => v + 1);

  useEffect(() => {
    getEvents()
      .then((res) => setEvents(res.events))
      .catch((err) => setFeedback(errorMessage(err, t("admin.events.loadFailed"))))
      .finally(() => setLoading(false));
  }, [version, errorMessage, t]);

  async function handleDelete(id: string, title: string) {
    if (!confirm(t("admin.events.confirmDelete", { title }))) return;
    try {
      await deleteEvent(id);
      setFeedback(t("admin.events.deleted"));
      load();
    } catch (err) {
      setFeedback(errorMessage(err, t("admin.events.deleteFailed")));
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-gray-500">{t("common.eventCount", { count: events.length })}</p>
        <Link
          href={NEW_EVENT_HREF}
          className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium text-white transition"
          style={{ backgroundColor: "#ec5b13" }}
        >
          <Plus className="w-4 h-4" />
          {t("admin.events.new")}
        </Link>
      </div>

      {feedback && <p className="text-sm text-gray-600 bg-gray-50 rounded-lg px-3 py-2 mb-4">{feedback}</p>}

      {loading ? (
        <p className="text-sm text-gray-400 py-10 text-center">{t("common.loading")}</p>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {[
                  t("admin.fields.title"),
                  t("admin.fields.category"),
                  t("admin.fields.date"),
                  t("admin.fields.price"),
                  t("admin.fields.owner"),
                  "",
                ].map((h) => (
                  <th
                    key={h}
                    className="text-left px-5 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {events.map((ev) => (
                <tr key={ev.id} className="hover:bg-gray-50 transition">
                  <td className="px-5 py-3 font-medium text-gray-900 max-w-xs truncate">{ev.title}</td>
                  <td className="px-5 py-3 text-gray-600">{t(`categories.${ev.category}`)}</td>
                  <td className="px-5 py-3 text-gray-600 whitespace-nowrap" title={ev.date}>
                    {formatEventDateRange(ev, i18n.language, true)}
                  </td>
                  <td className="px-5 py-3 text-gray-600">
                    {ev.price === 0 ? t("common.free") : t("common.price", { price: ev.price })}
                  </td>
                  <td className="px-5 py-3">
                    {/* No name lookup endpoint for publishers: show the kind and a short id */}
                    {ev.publisherId ? (
                      <span
                        title={ev.publisherId}
                        className="inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700"
                      >
                        {t("admin.events.ownerPublisher", { id: ev.publisherId.slice(0, 8) })}
                      </span>
                    ) : (
                      <span className="inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600">
                        {t("admin.events.ownerScraped")}
                      </span>
                    )}
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2 justify-end">
                      <Link
                        href={editHref(ev.id)}
                        aria-label={t("common.edit")}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition"
                      >
                        <Pencil className="w-4 h-4" />
                      </Link>
                      <button
                        onClick={() => handleDelete(ev.id, ev.title)}
                        aria-label={t("common.delete")}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 transition"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ── Shared UI primitives ──────────────────────────────────────────────────────

const inputCls =
  "w-full border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition bg-white";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs font-medium text-gray-600">{label}</label>
      {children}
    </div>
  );
}

function ModalActions({ onClose, saving }: { onClose: () => void; saving: boolean }) {
  const { t } = useTranslation();
  return (
    <div className="flex justify-end gap-2 pt-2">
      <button
        type="button"
        onClick={onClose}
        className="px-4 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100 transition"
      >
        {t("common.cancel")}
      </button>
      <button
        type="submit"
        disabled={saving}
        className="px-4 py-2 rounded-lg text-sm font-semibold text-white transition disabled:opacity-60"
        style={{ backgroundColor: "#ec5b13" }}
      >
        {saving ? t("common.saving") : t("common.save")}
      </button>
    </div>
  );
}

function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  const { t } = useTranslation();
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-lg bg-white rounded-2xl shadow-xl overflow-y-auto max-h-[90vh]">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h2 className="font-semibold text-gray-900">{title}</h2>
          <button
            onClick={onClose}
            aria-label={t("common.close")}
            className="text-gray-400 hover:text-gray-700 transition text-xl leading-none"
          >
            &times;
          </button>
        </div>
        <div className="px-6 py-5">{children}</div>
      </div>
    </div>
  );
}
