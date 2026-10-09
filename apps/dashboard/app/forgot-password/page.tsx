"use client";

import { useState } from "react";
import Link from "next/link";
import AuthShell from "@/components/ui/AuthShell";
import Button from "@/components/ui/Button";
import TextField from "@/components/ui/TextField";
import { friendlyAuthError } from "@/components/ui/auth-errors";
import { forgotPassword } from "@/lib/api";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [sentTo, setSentTo] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = email.trim().toLowerCase();
    if (!EMAIL_RE.test(trimmed)) {
      setEmailError("Enter a valid email address.");
      return;
    }
    setEmailError(null);
    setFormError(null);
    setBusy(true);
    try {
      await forgotPassword(trimmed);
      setSentTo(trimmed);
    } catch (err) {
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't send the reset email. Try again."));
    } finally {
      setBusy(false);
    }
  };

  const footer = (
    <>
      Remembered it?{" "}
      <Link className="font-semibold text-indigo-600 hover:text-indigo-500 dark:text-indigo-400" href="/login">
        Sign in
      </Link>
    </>
  );

  if (sentTo) {
    return (
      <AuthShell title="Check your email" subtitle="If an account exists for that address, a reset link is on its way." footer={footer}>
        <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm leading-6 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-200">
          We sent a password reset link to <span className="font-semibold">{sentTo}</span>. Open it within the hour to choose a new password.
        </div>
        <p className="mt-4 text-xs leading-5 text-zinc-400 dark:text-zinc-500">
          Didn&apos;t get it? Check spam, verify the address, or request another link. For security, we don&apos;t reveal whether an email is registered.
        </p>
        <Link
          href="/forgot-password"
          className="mt-4 inline-block text-sm font-semibold text-indigo-600 hover:text-indigo-500 dark:text-indigo-400"
        >
          Send another link
        </Link>
      </AuthShell>
    );
  }

  return (
    <AuthShell title="Reset your password" subtitle="Enter your account email and we'll send you a reset link." footer={footer}>
      <form onSubmit={submit} noValidate className="space-y-4">
        <TextField
          label="Email"
          type="email"
          autoComplete="email"
          autoFocus
          placeholder="you@company.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          error={emailError}
          required
        />
        {formError && (
          <p role="alert" className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {formError}
          </p>
        )}
        <Button loading={busy}>{busy ? "Sending..." : "Send reset link"}</Button>
      </form>
      <p className="mt-4 text-xs leading-5 text-zinc-400 dark:text-zinc-500">The link expires in 1 hour and can only be used once.</p>
    </AuthShell>
  );
}
