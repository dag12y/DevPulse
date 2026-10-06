"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import AuthShell from "@/components/ui/AuthShell";
import Button from "@/components/ui/Button";
import TextField from "@/components/ui/TextField";
import PasswordField from "@/components/ui/PasswordField";
import { friendlyAuthError } from "@/components/ui/auth-errors";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function LoginForm() {
  const { user, loading: authLoading, login } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const next = searchParams.get("next") || "/";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [passwordError, setPasswordError] = useState<string | null>(null);
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
    setFormError(null);
    if (!EMAIL_RE.test(trimmed)) {
      setEmailError("Enter a valid email address.");
      valid = false;
    }
    if (!password) {
      setPasswordError("Enter your password.");
      valid = false;
    }
    if (!valid) return;
    setBusy(true);
    try {
      await login(trimmed, password);
      router.push(next);
    } catch (err) {
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Sign in failed. Please try again."));
    } finally {
      setBusy(false);
    }
  };

  const registerHref = next && next !== "/" ? `/register?next=${encodeURIComponent(next)}` : "/register";

  return (
    <AuthShell
      title="Welcome back"
      subtitle="Sign in to your DevPulse workspace to view analytics."
      footer={<>No account? <Link className="font-semibold text-indigo-600 hover:text-indigo-500 dark:text-indigo-400" href={registerHref}>Create one</Link></>}
    >
      <form onSubmit={submit} noValidate className="space-y-4">
        <TextField label="Email" type="email" autoComplete="email" autoFocus placeholder="you@company.com" value={email} onChange={(e) => setEmail(e.target.value)} error={emailError} required />
        <div>
          <div className="mb-1.5 flex items-center justify-between">
            <span className="text-sm font-medium text-zinc-700 dark:text-zinc-300">Password</span>
            <span className="text-xs text-zinc-400 dark:text-zinc-500" title="Password reset is coming soon">Forgot password? (soon)</span>
          </div>
          <PasswordField label="Password" hideLabel autoComplete="current-password" placeholder="Your password" value={password} onChange={(e) => setPassword(e.target.value)} error={passwordError} id="login-password" required />
        </div>
        {formError && (
          <p role="alert" className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{formError}</p>
        )}
        <Button loading={busy}>{busy ? "Signing in..." : "Sign in"}</Button>
      </form>
    </AuthShell>
  );
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}
