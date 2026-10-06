"use client";

import type { InputHTMLAttributes } from "react";

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  hint?: string;
  error?: string | null;
}

export default function TextField({ label, hint, error, id, className = "", ...rest }: TextFieldProps) {
  const inputId = id ?? `field-${label.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
  const describedBy = error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined;
  return (
    <div className="block">
      <label htmlFor={inputId} className="mb-1.5 block text-sm font-medium text-zinc-700 dark:text-zinc-300">
        {label}
      </label>
      <input
        id={inputId}
        aria-invalid={Boolean(error)}
        aria-describedby={describedBy}
        className={`w-full rounded-lg border bg-white px-3.5 py-2.5 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:ring-4 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500 ${
          error
            ? "border-red-400 focus:border-red-500 focus:ring-red-500/15 dark:border-red-500"
            : "border-zinc-300 focus:border-indigo-500 focus:ring-indigo-500/15 dark:border-zinc-700 dark:focus:border-indigo-400"
        } ${className}`}
        {...rest}
      />
      {error ? (
        <p id={`${inputId}-error`} role="alert" className="mt-1.5 text-xs text-red-600 dark:text-red-400">
          {error}
        </p>
      ) : hint ? (
        <p id={`${inputId}-hint`} className="mt-1.5 text-xs text-zinc-500 dark:text-zinc-400">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
