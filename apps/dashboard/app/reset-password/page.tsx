"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import AuthShell from "@/components/ui/AuthShell";
import Button from "@/components/ui/Button";
import PasswordField from "@/components/ui/PasswordField";
import { friendlyAuthError } from "@/components/ui/auth-errors";
import { resetPassword } from "@/lib/api";

function ResetPasswordContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return;
    let valid = true;
    setPasswordError(null);
    setConfirmError(null);
    setFormError(null);
    if (password.length < 12) {
      setPasswordError("Use at least 12 characters.");
      valid = false;
    }
    if (confirm !== password) {
      setConfirmError("Passwords do not match.");
      valid = false;
    }
    if (!valid) return;
    setBusy(true);
    try {
      await resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't update your password. Try again."));
    } finally {
      setBusy(false);
    }
  };

  const footer = (
    <>
      Need a new link?{" "}
      <Link className="font-semibold text-indigo-600 hover:text-indigo-500 dark:text-indigo-400" href="/forgot-password">
        Reset password
      </Link>
    </>
  );

  if (!token) {
    return (
      <AuthShell title="Link didn't work" subtitle="This password reset link is missing or incomplete." footer={footer}>
        <p className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          Open the link from your reset email, or request a new one.
        </p>
        <Link
          href="/forgot-password"
          className="mt-4 inline-block w-full rounded-lg bg-indigo-600 px-4 py-2.5 text-center text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 dark:bg-indigo-500 dark:hover:bg-indigo-400"
        >
          Request a new link
        </Link>
      </AuthShell>
    );
  }

  if (done) {
    return (
      <AuthShell title="Password updated" subtitle="Your password has been changed and other sessions were signed out." footer={footer}>
        <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm leading-6 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-200">
          You can now sign in with your new password.
        </div>
        <Button className="mt-4" onClick={() => router.push("/login")}>
          Sign in
        </Button>
      </AuthShell>
    );
  }

  return (
    <AuthShell title="Choose a new password" subtitle="Pick something you haven't used on DevPulse before." footer={footer}>
      <form onSubmit={submit} noValidate className="space-y-4">
        <PasswordField
          label="New password"
          autoComplete="new-password"
          autoFocus
          placeholder="12+ characters"
          minLength={12}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          error={passwordError}
          hint="Use at least 12 characters."
          showStrength
          required
        />
        <PasswordField
          label="Confirm password"
          autoComplete="new-password"
          placeholder="Repeat your password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          error={confirmError}
          id="confirm-reset-password"
          required
        />
        {formError && (
          <p role="alert" className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
            {formError}
          </p>
        )}
        <Button loading={busy}>{busy ? "Updating..." : "Update password"}</Button>
      </form>
    </AuthShell>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense>
      <ResetPasswordContent />
    </Suspense>
  );
}
