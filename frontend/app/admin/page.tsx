"use client";

import { useState, useEffect, FormEvent } from "react";
import { User, Event, EventCategory } from "@/lib/types";
import { getUsers, createUser, updateUser, deleteUser } from "@/lib/api/users";
import { login } from "@/lib/api/auth";
import { getEvents, createEvent, updateEvent, deleteEvent } from "@/lib/api/events";
import { getStoredUser, setStoredUser, clearStoredUser } from "@/lib/auth";
import { LogOut, Plus, Pencil, Trash2, ShieldAlert } from "lucide-react";

const CATEGORIES: EventCategory[] = ["Music", "Theater", "Parties", "Sports", "Food", "Art", "Wellness"];

const ROLES = ["admin", "consumer", "publisher"] as const;

// ── Admin page root ──────────────────────────────────────────────────────────

export default function AdminPage() {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setUser(getStoredUser());
    setReady(true);
  }, []);

  function handleLogin(u: User) {
    setStoredUser(u);
    setUser(u);
  }

  function handleLogout() {
    clearStoredUser();
    setUser(null);
  }

  if (!ready) return null;
  if (!user || user.role !== "admin") {
    return <LoginSection onLogin={handleLogin} />;
  }
  return <Dashboard user={user} onLogout={handleLogout} />;
}

// ── Login ─────────────────────────────────────────────────────────────────────

function LoginSection({ onLogin }: { onLogin: (u: User) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const u = await login(username, password);
      if (u.role !== "admin") {
        setError("This account does not have admin access.");
        return;
      }
      onLogin(u);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      setError(
        msg === "invalid_credentials" ? "Wrong username or password." : "Something went wrong. Please try again.",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-[80vh] flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-2 mb-8 justify-center">
          <ShieldAlert className="w-6 h-6" style={{ color: "#ec5b13" }} />
          <h1 className="text-2xl font-bold text-gray-900">Admin access</h1>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-white rounded-2xl shadow-sm border border-gray-100 p-8 flex flex-col gap-5"
        >
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-gray-700">Username</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
              className="border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-gray-700">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="border border-gray-200 rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-400 transition"
            />
          </div>

          {error && <p className="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">{error}</p>}

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 rounded-lg text-sm font-semibold text-white transition disabled:opacity-60"
            style={{ backgroundColor: "#ec5b13" }}
          >
            {loading ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  );
}

// ── Dashboard shell ───────────────────────────────────────────────────────────

function Dashboard({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [tab, setTab] = useState<"users" | "events">("users");

  return (
    <div className="max-w-6xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Admin Dashboard</h1>
          <p className="text-sm text-gray-500 mt-0.5">Logged in as {user.name}</p>
        </div>
        <button
          onClick={onLogout}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100 transition"
        >
          <LogOut className="w-4 h-4" />
          Log out
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 border-b border-gray-200">
        {(["users", "events"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-5 py-2.5 text-sm font-medium capitalize transition -mb-px border-b-2 ${
              tab === t ? "border-orange-500 text-gray-900" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
            style={tab === t ? { borderColor: "#ec5b13", color: "#1a1a1a" } : undefined}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === "users" ? <UsersSection /> : <EventsSection />}
    </div>
  );
}

// ── Users section ─────────────────────────────────────────────────────────────

type UserModal = { mode: "create" } | { mode: "edit"; user: User };

function UsersSection() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [modal, setModal] = useState<UserModal | null>(null);
  const [feedback, setFeedback] = useState("");

  async function load() {
    try {
      setUsers(await getUsers());
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleDelete(id: string, name: string) {
    if (!confirm(`Delete user "${name}"?`)) return;
    try {
      await deleteUser(id);
      setFeedback("User deleted.");
      load();
    } catch {
      setFeedback("Failed to delete user.");
    }
  }

  async function handleSave(data: Partial<User> & { password?: string }, id?: string) {
    try {
      if (id) {
        await updateUser(id, data);
        setFeedback("User updated.");
      } else {
        await createUser(data as Omit<User, "id"> & { password: string });
        setFeedback("User created.");
      }
      setModal(null);
      load();
    } catch {
      setFeedback(id ? "Failed to update user." : "Failed to create user.");
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-gray-500">{users.length} users</p>
        <button
          onClick={() => setModal({ mode: "create" })}
          className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium text-white transition"
          style={{ backgroundColor: "#ec5b13" }}
        >
          <Plus className="w-4 h-4" />
          New user
        </button>
      </div>

      {feedback && <p className="text-sm text-gray-600 bg-gray-50 rounded-lg px-3 py-2 mb-4">{feedback}</p>}

      {loading ? (
        <p className="text-sm text-gray-400 py-10 text-center">Loading…</p>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {["Name", "Username", "Email", "Role", ""].map((h) => (
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
                  <td className="px-5 py-3 text-gray-600">@{u.username}</td>
                  <td className="px-5 py-3 text-gray-600">{u.email}</td>
                  <td className="px-5 py-3">
                    <RoleBadge role={u.role} />
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2 justify-end">
                      <button
                        onClick={() => setModal({ mode: "edit", user: u })}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(u.id, u.name)}
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

      {modal && <UserModalForm modal={modal} onSave={handleSave} onClose={() => setModal(null)} />}
    </div>
  );
}

function RoleBadge({ role }: { role: string }) {
  const colors: Record<string, string> = {
    admin: "bg-orange-100 text-orange-700",
    publisher: "bg-blue-100 text-blue-700",
    consumer: "bg-gray-100 text-gray-600",
  };
  return (
    <span
      className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-medium ${colors[role] ?? "bg-gray-100 text-gray-600"}`}
    >
      {role}
    </span>
  );
}

function UserModalForm({
  modal,
  onSave,
  onClose,
}: {
  modal: UserModal;
  onSave: (data: Partial<User> & { password?: string }, id?: string) => Promise<void>;
  onClose: () => void;
}) {
  const editing = modal.mode === "edit" ? modal.user : null;
  const [form, setForm] = useState({
    name: editing?.name ?? "",
    username: editing?.username ?? "",
    email: editing?.email ?? "",
    role: editing?.role ?? "consumer",
    password: "",
  });
  const [saving, setSaving] = useState(false);

  function set(field: string, value: string) {
    setForm((f) => ({ ...f, [field]: value }));
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    const payload: Partial<User> & { password?: string } = {
      name: form.name,
      username: form.username,
      email: form.email,
      role: form.role as User["role"],
    };
    if (form.password) payload.password = form.password;
    await onSave(payload, editing?.id);
    setSaving(false);
  }

  return (
    <Modal title={editing ? "Edit user" : "New user"} onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Name">
          <input className={inputCls} value={form.name} onChange={(e) => set("name", e.target.value)} required />
        </Field>
        <Field label="Username">
          <input
            className={inputCls}
            value={form.username}
            onChange={(e) => set("username", e.target.value)}
            required
          />
        </Field>
        <Field label="Email">
          <input
            type="email"
            className={inputCls}
            value={form.email}
            onChange={(e) => set("email", e.target.value)}
            required
          />
        </Field>
        <Field label="Role">
          <select className={inputCls} value={form.role} onChange={(e) => set("role", e.target.value)}>
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </Field>
        <Field label={editing ? "New password (leave blank to keep)" : "Password"}>
          <input
            type="password"
            className={inputCls}
            value={form.password}
            onChange={(e) => set("password", e.target.value)}
            required={!editing}
          />
        </Field>
        <ModalActions onClose={onClose} saving={saving} />
      </form>
    </Modal>
  );
}

// ── Events section ────────────────────────────────────────────────────────────

type EventModal = { mode: "create" } | { mode: "edit"; event: Event };

function EventsSection() {
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [modal, setModal] = useState<EventModal | null>(null);
  const [feedback, setFeedback] = useState("");

  async function load() {
    try {
      const res = await getEvents();
      setEvents(res.events);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleDelete(id: string, title: string) {
    if (!confirm(`Delete event "${title}"?`)) return;
    try {
      await deleteEvent(id);
      setFeedback("Event deleted.");
      load();
    } catch {
      setFeedback("Failed to delete event.");
    }
  }

  async function handleSave(data: Partial<Event>, id?: string) {
    try {
      if (id) {
        await updateEvent(id, data);
        setFeedback("Event updated.");
      } else {
        await createEvent(data as Omit<Event, "id">);
        setFeedback("Event created.");
      }
      setModal(null);
      load();
    } catch {
      setFeedback(id ? "Failed to update event." : "Failed to create event.");
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-gray-500">{events.length} events</p>
        <button
          onClick={() => setModal({ mode: "create" })}
          className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium text-white transition"
          style={{ backgroundColor: "#ec5b13" }}
        >
          <Plus className="w-4 h-4" />
          New event
        </button>
      </div>

      {feedback && <p className="text-sm text-gray-600 bg-gray-50 rounded-lg px-3 py-2 mb-4">{feedback}</p>}

      {loading ? (
        <p className="text-sm text-gray-400 py-10 text-center">Loading…</p>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                {["Title", "Category", "Date", "Price", ""].map((h) => (
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
                  <td className="px-5 py-3 text-gray-600">{ev.category}</td>
                  <td className="px-5 py-3 text-gray-600">{ev.date}</td>
                  <td className="px-5 py-3 text-gray-600">{ev.price === 0 ? "Free" : `$${ev.price}`}</td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2 justify-end">
                      <button
                        onClick={() => setModal({ mode: "edit", event: ev })}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(ev.id, ev.title)}
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

      {modal && <EventModalForm modal={modal} onSave={handleSave} onClose={() => setModal(null)} />}
    </div>
  );
}

function EventModalForm({
  modal,
  onSave,
  onClose,
}: {
  modal: EventModal;
  onSave: (data: Partial<Event>, id?: string) => Promise<void>;
  onClose: () => void;
}) {
  const editing = modal.mode === "edit" ? modal.event : null;
  const [form, setForm] = useState({
    title: editing?.title ?? "",
    description: editing?.description ?? "",
    category: editing?.category ?? "Music",
    date: editing?.date ?? "",
    startTime: editing?.startTime ?? "",
    endTime: editing?.endTime ?? "",
    venue: editing?.venue ?? "",
    address: editing?.address ?? "",
    price: String(editing?.price ?? 0),
    artistName: editing?.artistName ?? "",
  });
  const [saving, setSaving] = useState(false);

  function set(field: string, value: string) {
    setForm((f) => ({ ...f, [field]: value }));
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    const payload: Partial<Event> = {
      title: form.title,
      description: form.description,
      category: form.category as EventCategory,
      date: form.date,
      startTime: form.startTime,
      endTime: form.endTime,
      venue: form.venue,
      address: form.address,
      price: parseFloat(form.price) || 0,
      artistName: form.artistName || undefined,
    };
    await onSave(payload, editing?.id);
    setSaving(false);
  }

  return (
    <Modal title={editing ? "Edit event" : "New event"} onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Title">
          <input className={inputCls} value={form.title} onChange={(e) => set("title", e.target.value)} required />
        </Field>
        <Field label="Description">
          <textarea
            className={`${inputCls} resize-none`}
            rows={3}
            value={form.description}
            onChange={(e) => set("description", e.target.value)}
          />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Category">
            <select className={inputCls} value={form.category} onChange={(e) => set("category", e.target.value)}>
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Date">
            <input
              type="date"
              className={inputCls}
              value={form.date}
              onChange={(e) => set("date", e.target.value)}
              required
            />
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Start time">
            <input
              type="time"
              className={inputCls}
              value={form.startTime}
              onChange={(e) => set("startTime", e.target.value)}
              required
            />
          </Field>
          <Field label="End time">
            <input
              type="time"
              className={inputCls}
              value={form.endTime}
              onChange={(e) => set("endTime", e.target.value)}
              required
            />
          </Field>
        </div>
        <Field label="Venue">
          <input className={inputCls} value={form.venue} onChange={(e) => set("venue", e.target.value)} required />
        </Field>
        <Field label="Address">
          <input className={inputCls} value={form.address} onChange={(e) => set("address", e.target.value)} />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Price ($)">
            <input
              type="number"
              min="0"
              step="0.01"
              className={inputCls}
              value={form.price}
              onChange={(e) => set("price", e.target.value)}
            />
          </Field>
          <Field label="Artist / Host (optional)">
            <input className={inputCls} value={form.artistName} onChange={(e) => set("artistName", e.target.value)} />
          </Field>
        </div>
        <ModalActions onClose={onClose} saving={saving} />
      </form>
    </Modal>
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
  return (
    <div className="flex justify-end gap-2 pt-2">
      <button
        type="button"
        onClick={onClose}
        className="px-4 py-2 rounded-lg text-sm font-medium text-gray-600 hover:bg-gray-100 transition"
      >
        Cancel
      </button>
      <button
        type="submit"
        disabled={saving}
        className="px-4 py-2 rounded-lg text-sm font-semibold text-white transition disabled:opacity-60"
        style={{ backgroundColor: "#ec5b13" }}
      >
        {saving ? "Saving…" : "Save"}
      </button>
    </div>
  );
}

function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
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
          <button onClick={onClose} className="text-gray-400 hover:text-gray-700 transition text-xl leading-none">
            &times;
          </button>
        </div>
        <div className="px-6 py-5">{children}</div>
      </div>
    </div>
  );
}
