import { ReactNode } from "react";

// Building blocks shared by the editor sections

export function inputCls(error?: string) {
  return `w-full border rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 transition bg-white disabled:bg-gray-50 disabled:text-gray-400 ${
    error
      ? "border-red-300 focus:ring-red-100 focus:border-red-400"
      : "border-gray-200 focus:ring-orange-200 focus:border-orange-400"
  }`;
}

// aria-describedby for a control rendered inside <Field id=…>
export function describedBy(id: string, error?: string, hint?: ReactNode): string | undefined {
  if (error) return `${id}-error`;
  return hint ? `${id}-hint` : undefined;
}

export function Section({
  id,
  title,
  icon,
  description,
  children,
}: {
  id: string;
  title: string;
  icon: ReactNode;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section aria-labelledby={`${id}-title`} className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5 sm:p-6">
      <div className="flex items-start gap-3 mb-5">
        <span
          className="flex items-center justify-center w-9 h-9 rounded-xl bg-orange-50 shrink-0"
          style={{ color: "#ec5b13" }}
        >
          {icon}
        </span>
        <div className="min-w-0">
          <h2 id={`${id}-title`} className="font-semibold text-gray-900">
            {title}
          </h2>
          {description && <p className="text-sm text-gray-500 mt-0.5">{description}</p>}
        </div>
      </div>
      <div className="flex flex-col gap-4">{children}</div>
    </section>
  );
}

// Label + control + error/hint. `group` renders the label as text for controls that have their own labels.
export function Field({
  id,
  label,
  error,
  hint,
  required,
  group,
  aside,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  hint?: ReactNode;
  required?: boolean;
  group?: boolean;
  aside?: ReactNode; // e.g. a character counter, right of the label
  children: ReactNode;
}) {
  const labelContent = (
    <>
      {label}
      {required && (
        <span style={{ color: "#ec5b13" }} aria-hidden="true">
          {" "}
          *
        </span>
      )}
    </>
  );
  return (
    <div className="flex flex-col gap-1.5 min-w-0" data-editor-error={error ? "true" : undefined}>
      <div className="flex items-baseline justify-between gap-2">
        {group ? (
          <span id={`${id}-label`} className="text-xs font-medium text-gray-600">
            {labelContent}
          </span>
        ) : (
          <label htmlFor={id} className="text-xs font-medium text-gray-600">
            {labelContent}
          </label>
        )}
        {aside}
      </div>
      {children}
      {error ? (
        <p id={`${id}-error`} className="text-xs text-red-600">
          {error}
        </p>
      ) : (
        hint && (
          <p id={`${id}-hint`} className="text-xs text-gray-400">
            {hint}
          </p>
        )
      )}
    </div>
  );
}

export function Counter({ value, max }: { value: number; max: number }) {
  return (
    <span className={`text-xs tabular-nums ${value > max ? "text-red-600" : "text-gray-400"}`} aria-live="polite">
      {value}/{max}
    </span>
  );
}

export function ProgressBar({ value, label }: { value: number; label: string }) {
  const pct = Math.round(Math.min(1, Math.max(0, value)) * 100);
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={pct}
      className="h-1.5 w-full rounded-full bg-white/70 overflow-hidden"
    >
      <div className="h-full rounded-full transition-[width]" style={{ width: `${pct}%`, backgroundColor: "#ec5b13" }} />
    </div>
  );
}
