"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { safeNext } from "@/lib/paths";
import AuthShell from "@/components/ui/AuthShell";
import Button from "@/components/ui/Button";
import TextField from "@/components/ui/TextField";
import PasswordField from "@/components/ui/PasswordField";
import OAuthButtons from "@/components/ui/OAuthButtons";
import { friendlyAuthError } from "@/components/ui/auth-errors";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function RegisterForm() {
  const { user, loading: authLoading, register } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const next = safeNext(searchParams.get("next"));
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [workspaceName, setWorkspaceName] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!authLoading && user) router.replace(next);
  }, [authLoading, user, router, next]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = email.trim().toLowerCase();
    let valid = true;
    setEmailError(null);
    setPasswordError(null);
    setConfirmError(null);
    setFormError(null);
    if (!EMAIL_RE.test(trimmed)) {
      setEmailError("Enter a valid email address.");
      valid = false;
    }
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
      await register(trimmed, password, workspaceName.trim() || undefined);
      // No session yet: send the visitor to the check-your-inbox screen,
      // which can re-send the link if it never arrives.
      const params = new URLSearchParams({ email: trimmed });
      if (next && next !== "/") params.set("next", next);
      router.push(`/verify-email?${params.toString()}`);
    } catch (err) {
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Registration failed. Please try again."));
    } finally {
      setBusy(false);
    }
  };

  const loginHref = next && next !== "/" ? `/login?next=${encodeURIComponent(next)}` : "/login";

  return (
    <AuthShell
      title="Create your account"
      subtitle="Get a personal workspace and start tracking in minutes."
      footer={<>Already have an account? <Link className="font-semibold text-indigo-600 hover:text-indigo-500 dark:text-indigo-400" href={loginHref}>Sign in</Link></>}
    >
      <form onSubmit={submit} noValidate className="space-y-4">
        <TextField label="Email" type="email" autoComplete="email" autoFocus placeholder="you@company.com" value={email} onChange={(e) => setEmail(e.target.value)} error={emailError} required />
        <PasswordField label="Password" autoComplete="new-password" placeholder="12+ characters" minLength={12} value={password} onChange={(e) => setPassword(e.target.value)} error={passwordError} hint="Use at least 12 characters." showStrength required />
        <PasswordField label="Confirm password" autoComplete="new-password" placeholder="Repeat your password" value={confirm} onChange={(e) => setConfirm(e.target.value)} error={confirmError} id="confirm-password" required />
        <TextField label="Workspace name" type="text" autoComplete="organization" placeholder="My workspace (optional)" value={workspaceName} onChange={(e) => setWorkspaceName(e.target.value)} hint="You can rename it later." />
        {formError && (
          <p role="alert" className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{formError}</p>
        )}
        <Button loading={busy}>{busy ? "Creating..." : "Create account"}</Button>
        <p className="text-center text-xs leading-5 text-zinc-400 dark:text-zinc-500">By creating an account you agree to the Terms and Privacy Policy.</p>
      </form>
      <OAuthButtons next={next} />
    </AuthShell>
  );
}

export default function RegisterPage() {
  return (
    <Suspense>
      <RegisterForm />
    </Suspense>
  );
}
