"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";

export default function WorkspaceSelector() {
  const { user, workspaces, selectedWorkspaceID, selectWorkspace, loading, usingEnvKey } = useAuth();

  if (loading) {
    return (
      <div className="rounded-md border border-zinc-200 dark:border-zinc-800 px-3 py-2 text-sm text-zinc-500">
        Loading…
      </div>
    );
  }

  if (!user) {
    return (
      <div className="rounded-md border border-dashed border-zinc-300 dark:border-zinc-700 px-3 py-2 text-sm text-zinc-500">
        {usingEnvKey ? (
          <>API key mode</>
        ) : (
          <>
            Not signed in — <Link className="underline" href="/login">Sign in</Link>
          </>
        )}
      </div>
    );
  }

  if (workspaces.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-zinc-300 dark:border-zinc-700 px-3 py-2 text-sm text-zinc-500">
        No workspaces
      </div>
    );
  }

  return (
    <label className="block">
      <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500 dark:text-zinc-400">
        Workspace
      </span>
      <select
        aria-label="Select workspace"
        value={selectedWorkspaceID ?? ""}
        onChange={(e) => selectWorkspace(e.target.value)}
        className="w-full rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm text-zinc-900 dark:text-zinc-100"
      >
        {workspaces.map((membership) => (
          <option key={membership.workspace_id} value={membership.workspace_id}>
            {membership.workspace_name} · {membership.role}
          </option>
        ))}
      </select>
    </label>
  );
}
