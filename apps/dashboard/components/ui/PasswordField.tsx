"use client";

import { useState, type InputHTMLAttributes } from "react";

interface PasswordFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type"> {
  label: string;
  hint?: string;
  error?: string | null;
  showStrength?: boolean;
  hideLabel?: boolean;
}

export function passwordScore(password: string): { score: number; label: string } {
  if (!password) return { score: 0, label: "Enter a password" };
  let score = 0;
  if (password.length >= 12) score += 1;
  if (password.length >= 16) score += 1;
  if (/[A-Z]/.test(password) && /[a-z]/.test(password)) score += 1;
  if (/\d/.test(password)) score += 1;
  if (/[^A-Za-z0-9]/.test(password)) score += 1;
  if (score <= 2) return { score, label: score <= 1 ? "Weak" : "Fair" };
  if (score === 3) return { score, label: "Good" };
  return { score, label: "Strong" };
}

export default function PasswordField({ label, hint, error, id, showStrength = false, hideLabel = false, value, ...rest }: PasswordFieldProps) {
  const [visible, setVisible] = useState(false);
  const inputId = id ?? `field-${label.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
  const strength = showStrength && typeof value === "string" ? passwordScore(value) : null;

  return (
    <div className="block">
      {hideLabel ? (
        <label htmlFor={inputId} className="sr-only">{label}</label>
      ) : (
      <label htmlFor={inputId} className="mb-1.5 block text-sm font-medium text-zinc-700 dark:text-zinc-300">
        {label}
      </label>
      )}
      <div className="relative">
        <input
          id={inputId}
          type={visible ? "text" : "password"}
          aria-invalid={Boolean(error)}
          value={value}
          className={`w-full rounded-lg border bg-white px-3.5 py-2.5 pr-16 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:ring-4 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500 ${
            error
              ? "border-red-400 focus:border-red-500 focus:ring-red-500/15 dark:border-red-500"
              : "border-zinc-300 focus:border-indigo-500 focus:ring-indigo-500/15 dark:border-zinc-700 dark:focus:border-indigo-400"
          }`}
          {...rest}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          aria-pressed={visible}
          aria-label={visible ? "Hide password" : "Show password"}
          className="absolute inset-y-0 right-0 rounded-r-lg px-3.5 text-xs font-semibold text-zinc-500 transition hover:text-zinc-800 focus-visible:outline-2 focus-visible:outline-indigo-600 dark:text-zinc-400 dark:hover:text-zinc-100"
        >
          {visible ? "Hide" : "Show"}
        </button>
      </div>
      {strength && typeof value === "string" && value.length > 0 && (
        <div className="mt-2" aria-live="polite">
          <div className="flex gap-1" aria-hidden="true">
            {[1, 2, 3, 4].map((bar) => {
              const filled = strength.score >= bar + 1 || (strength.score >= 3 && bar <= 2);
              const color =
                strength.score <= 2 ? "bg-red-400" : strength.score === 3 ? "bg-amber-400" : "bg-emerald-500";
              return (
                <div key={bar} className={`h-1 flex-1 rounded-full ${filled ? color : "bg-zinc-200 dark:bg-zinc-700"}`} />
              );
            })}
          </div>
          <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
            Strength: <span className="font-medium">{strength.label}</span>
            {typeof value === "string" && value.length < 12 ? " — use at least 12 characters" : ""}
          </p>
        </div>
      )}
      {error ? (
        <p role="alert" className="mt-1.5 text-xs text-red-600 dark:text-red-400">
          {error}
        </p>
      ) : hint && !(strength && typeof value === "string" && value.length > 0) ? (
        <p className="mt-1.5 text-xs text-zinc-500 dark:text-zinc-400">{hint}</p>
      ) : null}
    </div>
  );
}
