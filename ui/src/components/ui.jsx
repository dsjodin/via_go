"use client";

import { useEffect } from "react";

// Small shared pieces. Deliberately plain: this is an appliance console, and
// consistency matters more here than novelty.

export function Button({ variant = "primary", className = "", ...props }) {
  const variants = {
    primary:
      "bg-sky-600 text-white hover:bg-sky-500 disabled:bg-slate-600 disabled:text-slate-400",
    secondary:
      "bg-slate-700 text-slate-100 hover:bg-slate-600 disabled:text-slate-500",
    danger: "bg-red-700 text-white hover:bg-red-600 disabled:bg-slate-600",
  };

  return (
    <button
      className={`rounded px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed ${variants[variant]} ${className}`}
      {...props}
    />
  );
}

export function Field({ label, hint, children }) {
  return (
    // The label wraps its control, which associates them implicitly. Biome
    // cannot see that through an opaque {children}.
    // biome-ignore lint/a11y/noLabelWithoutControl: the control is passed in as children
    <label className="block">
      <span className="mb-1 block text-xs font-medium tracking-wide text-slate-400 uppercase">
        {label}
      </span>
      {children}
      {hint && (
        <span className="mt-1 block text-xs text-slate-500">{hint}</span>
      )}
    </label>
  );
}

export function Input(props) {
  return (
    <input
      className="w-full rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100 outline-none focus:border-sky-500"
      {...props}
    />
  );
}

export function Select({ children, ...props }) {
  return (
    <select
      className="w-full rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100 outline-none focus:border-sky-500"
      {...props}
    >
      {children}
    </select>
  );
}

export function Checkbox({ label, ...props }) {
  return (
    <label className="flex items-center gap-2 text-sm text-slate-300">
      <input type="checkbox" className="accent-sky-600" {...props} />
      {label}
    </label>
  );
}

export function Card({ title, actions, children }) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/60">
      {(title || actions) && (
        <header className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
          <h2 className="text-sm font-semibold text-slate-200">{title}</h2>
          <div className="flex gap-2">{actions}</div>
        </header>
      )}
      <div className="p-4">{children}</div>
    </section>
  );
}

export function Table({ columns, children, empty }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-slate-800 text-xs tracking-wide text-slate-500 uppercase">
            {columns.map((c) => (
              <th key={c} className="px-3 py-2 font-medium whitespace-nowrap">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800/70">{children}</tbody>
      </table>
      {empty && (
        <p className="px-3 py-6 text-center text-sm text-slate-500">{empty}</p>
      )}
    </div>
  );
}

export function Alert({ kind = "error", children }) {
  if (!children) return null;

  const kinds = {
    error: "border-red-800 bg-red-950/60 text-red-200",
    warn: "border-amber-800 bg-amber-950/60 text-amber-200",
    info: "border-sky-800 bg-sky-950/60 text-sky-200",
  };

  return (
    <div className={`rounded border px-3 py-2 text-sm ${kinds[kind]}`}>
      {children}
    </div>
  );
}

// Progress mirrors the percentages the backend records as a host installs:
// mboot 10, crypto64 12, boot.cfg 15, kickstart 50.
export function Progress({ value, text }) {
  const pct = Math.max(0, Math.min(100, value || 0));

  return (
    <div className="min-w-32">
      <div className="h-1.5 w-full overflow-hidden rounded bg-slate-800">
        <div
          className={`h-full transition-all ${pct >= 100 ? "bg-emerald-500" : "bg-sky-500"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="mt-1 block text-xs text-slate-500">
        {text ? `${pct}% ${text}` : `${pct}%`}
      </span>
    </div>
  );
}

export function Modal({ title, onClose, children }) {
  // Escape closes, which is what a dialog is expected to do and is the only
  // way out for anyone not using a mouse.
  useEffect(() => {
    const onKey = (e) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-6"
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      {/* A real button rather than a click handler on the backdrop div, so
          dismissing by clicking away is reachable from the keyboard too. */}
      <button
        type="button"
        aria-label="Close"
        onClick={onClose}
        className="absolute inset-0 cursor-default bg-black/60"
      />

      <div className="relative mt-10 w-full max-w-lg rounded-lg border border-slate-700 bg-slate-900 shadow-xl">
        <header className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
          <h2 className="text-sm font-semibold text-slate-200">{title}</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-slate-500 hover:text-slate-300"
            aria-label="Close"
          >
            ✕
          </button>
        </header>
        <div className="p-4">{children}</div>
      </div>
    </div>
  );
}
