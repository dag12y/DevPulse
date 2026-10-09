"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { ApiError, resendVerification } from "@/lib/api";
import { safeNext } from "@/lib/paths";
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
  const next = safeNext(searchParams.get("next"));
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // True after a 403 "email address not verified": offer a resend for
  // the address already typed into the form.
  const [needsVerification, setNeedsVerification] = useState(false);
  const [resendState, setResendState] = useState<"idle" | "sending" | "sent">("idle");

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
    setNeedsVerification(false);
    setResendState("idle");
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
      if (err instanceof ApiError && err.status === 403) setNeedsVerification(true);
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Sign in failed. Please try again."));
    } finally {
      setBusy(false);
    }
  };

  const resend = async () => {
    const trimmed = email.trim().toLowerCase();
    if (!EMAIL_RE.test(trimmed)) {
      setEmailError("Enter a valid email address.");
      return;
    }
    setResendState("sending");
    setFormError(null);
    try {
      await resendVerification(trimmed);
      setResendState("sent");
    } catch (err) {
      setResendState("idle");
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't resend the email. Try again."));
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
            <Link href="/forgot-password" className="text-xs text-zinc-400 hover:text-indigo-600 hover:underline dark:text-zinc-500 dark:hover:text-indigo-400">
              Forgot password?
            </Link>
          </div>
          <PasswordField label="Password" hideLabel autoComplete="current-password" placeholder="Your password" value={password} onChange={(e) => setPassword(e.target.value)} error={passwordError} id="login-password" required />
        </div>
        {formError && (
          <p role="alert" className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">{formError}</p>
        )}
        {needsVerification && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm leading-6 text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-200">
            {resendState === "sent" ? (
              <p>Verification email sent. Open the link in that email, then sign in.</p>
            ) : (
              <>
                <p>Your email address isn&apos;t verified yet.</p>
                <Button type="button" variant="secondary" loading={resendState === "sending"} onClick={resend} className="mt-2">
                  {resendState === "sending" ? "Sending..." : "Resend verification email"}
                </Button>
              </>
            )}
          </div>
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
