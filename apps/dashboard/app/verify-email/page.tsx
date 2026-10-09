"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import AuthShell from "@/components/ui/AuthShell";
import Button, { Spinner } from "@/components/ui/Button";
import TextField from "@/components/ui/TextField";
import { friendlyAuthError } from "@/components/ui/auth-errors";
import { resendVerification, verifyEmail } from "@/lib/api";
import { safeNext } from "@/lib/paths";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type Phase = "checking" | "verified" | "invalid" | "check-inbox";

function VerifyEmailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token");
  const next = safeNext(searchParams.get("next"));

  const [phase, setPhase] = useState<Phase>(token ? "checking" : "check-inbox");
  const [formError, setFormError] = useState<string | null>(null);
  const [email, setEmail] = useState(searchParams.get("email") ?? "");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [resendState, setResendState] = useState<"idle" | "sending" | "sent">("idle");
  const attempted = useRef(false);

  useEffect(() => {
    if (!token || attempted.current) return;
    attempted.current = true;
    verifyEmail(token)
      .then(() => setPhase("verified"))
      .catch((err) => {
        setPhase("invalid");
        setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "That verification link didn't work."));
      });
  }, [token]);

  const resend = async () => {
    const trimmed = email.trim().toLowerCase();
    if (!EMAIL_RE.test(trimmed)) {
      setEmailError("Enter a valid email address.");
      return;
    }
    setEmailError(null);
    setFormError(null);
    setResendState("sending");
    try {
      await resendVerification(trimmed);
      setResendState("sent");
    } catch (err) {
      setResendState("idle");
      setFormError(friendlyAuthError(err instanceof Error ? err.message : "", "Couldn't resend the email. Try again."));
    }
  };

  const loginHref = next && next !== "/" ? `/login?next=${encodeURIComponent(next)}` : "/login";
  const signInFooter = (
    <>
      Already verified?{" "}
      <Link className="font-semibold text-indigo-600 hover:text-indigo-500 dark:text-indigo-400" href={loginHref}>
        Sign in
      </Link>
    </>
  );

  if (phase === "checking") {
    return (
      <AuthShell title="Verifying your email" subtitle="Hang tight while we check your link." footer={signInFooter}>
        <div className="flex items-center gap-3 py-6 text-sm text-zinc-500 dark:text-zinc-400" role="status">
          <Spinner className="h-5 w-5 text-indigo-600 dark:text-indigo-400" />
          Checking your verification link…
        </div>
      </AuthShell>
    );
  }

  if (phase === "verified") {
    return (
      <AuthShell title="Email verified" subtitle="Your account is active. Sign in to reach your dashboard." footer={signInFooter}>
        <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm leading-6 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-200">
          Verification complete — welcome to DevPulse.
        </div>
        <Button className="mt-4" onClick={() => router.push(loginHref)}>
          Continue to sign in
        </Button>
      </AuthShell>
    );
  }

  const sent = resendState === "sent";
  return (
    <AuthShell
      title={phase === "invalid" ? "Link didn't work" : "Check your email"}
      subtitle={
        phase === "invalid"
          ? "Verification links can expire or be used only once. Request a fresh one below."
          : email
            ? `We sent a verification link to ${email}. Open it to activate your account.`
            : "Open the verification link we emailed you to activate your account."
      }
      footer={signInFooter}
    >
      {formError && (
        <p role="alert" className="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          {formError}
        </p>
      )}
      {sent ? (
        <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm leading-6 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-200">
          If that address has a pending account, a new verification email is on its way. The previous link no longer works.
        </div>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void resend();
          }}
          noValidate
          className="space-y-4"
        >
          <TextField
            label="Email"
            type="email"
            autoComplete="email"
            placeholder="you@company.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            error={emailError}
            required
          />
          <Button loading={resendState === "sending"}>{resendState === "sending" ? "Sending..." : "Resend verification email"}</Button>
        </form>
      )}
      <p className="mt-4 text-xs leading-5 text-zinc-400 dark:text-zinc-500">
        The link expires in 24 hours. Don&apos;t see it? Check spam, or make sure the address above is correct.
      </p>
    </AuthShell>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense>
      <VerifyEmailContent />
    </Suspense>
  );
}
