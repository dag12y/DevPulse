"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";

const selectClass =
  "w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100";

export default function WorkspaceSelector() {
  const { user, workspaces, selectedWorkspaceID, selectWorkspace, loading, usingEnvKey } = useAuth();

  if (loading) {
    return (
      <div className="rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-500 shadow-xs dark:border-zinc-800 dark:bg-zinc-900">
        Loading…
      </div>
    );
  }

  if (!user) {
    return (
      <div className="rounded-lg border border-dashed border-zinc-300 px-3 py-2 text-sm text-zinc-500 dark:border-zinc-700">
        {usingEnvKey ? (
          <>API key mode</>
        ) : (
          <>
            Not signed in — <Link className="font-medium text-indigo-600 underline underline-offset-2 dark:text-indigo-400" href="/login">Sign in</Link>
          </>
        )}
      </div>
    );
  }

  if (workspaces.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-zinc-300 px-3 py-2 text-sm text-zinc-500 dark:border-zinc-700">
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
        className={selectClass}
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
