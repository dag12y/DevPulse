"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when workspace changes */

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import {
  addMember,
  createWorkspace,
  deleteWorkspace,
  listMembers,
  removeMember,
  renameWorkspace,
  updateMemberRole,
  type WorkspaceMember,
} from "@/lib/api";

export default function AccountPage() {
  const { user, workspaces, selectedWorkspaceID, selectWorkspace, selectedMembership, logout, refresh, loading } =
    useAuth();
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [membersError, setMembersError] = useState<string | null>(null);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("viewer");
  const [actionError, setActionError] = useState<string | null>(null);
  const [newWorkspace, setNewWorkspace] = useState("");
  const [editingWorkspaceID, setEditingWorkspaceID] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const [confirmDeleteID, setConfirmDeleteID] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedWorkspaceID) {
      setMembers([]);
      return;
    }
    let cancelled = false;
    setMembersError(null);
    listMembers(selectedWorkspaceID)
      .then((list) => {
        if (!cancelled) setMembers(list);
      })
      .catch((e) => {
        if (!cancelled) setMembersError(e.message);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedWorkspaceID]);

  if (loading) {
    return (
      <div className="p-6">
        <p className="text-zinc-500">Loading account…</p>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="p-6 space-y-3">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Account</h2>
        <p className="text-sm text-zinc-500">
          <Link className="underline" href="/login">Sign in</Link> to manage workspaces and members.
        </p>
      </div>
    );
  }

  const canManage = selectedMembership && selectedMembership.role !== "viewer";

  const reloadMembers = () => {
    if (!selectedWorkspaceID) return;
    listMembers(selectedWorkspaceID).then(setMembers).catch((e) => setMembersError(e.message));
  };

  const invite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedWorkspaceID) return;
    setActionError(null);
    try {
      await addMember(selectedWorkspaceID, inviteEmail, inviteRole);
      setInviteEmail("");
      reloadMembers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Invite failed");
    }
  };

  const changeRole = async (userID: string, role: string) => {
    if (!selectedWorkspaceID) return;
    setActionError(null);
    try {
      await updateMemberRole(selectedWorkspaceID, userID, role);
      reloadMembers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Role change failed");
    }
  };

  const remove = async (userID: string) => {
    if (!selectedWorkspaceID) return;
    setActionError(null);
    try {
      await removeMember(selectedWorkspaceID, userID);
      reloadMembers();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Removal failed");
    }
  };

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setActionError(null);
    try {
      const membership = await createWorkspace(newWorkspace);
      setNewWorkspace("");
      refresh();
      selectWorkspace(membership.workspace_id);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Creation failed");
    }
  };

  const canEdit = (role: string) => role === "owner" || role === "admin";
  const canDelete = (role: string) => role === "owner";

  const startRename = (workspaceID: string, currentName: string) => {
    setConfirmDeleteID(null);
    setEditingWorkspaceID(workspaceID);
    setEditingName(currentName);
  };

  const saveRename = async (e: React.FormEvent, workspaceID: string) => {
    e.preventDefault();
    setActionError(null);
    try {
      await renameWorkspace(workspaceID, editingName);
      setEditingWorkspaceID(null);
      setEditingName("");
      refresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Rename failed");
    }
  };

  const destroy = async (workspaceID: string) => {
    setActionError(null);
    try {
      await deleteWorkspace(workspaceID);
      setConfirmDeleteID(null);
      refresh();
      if (workspaceID === selectedWorkspaceID) {
        const next = workspaces.find((m) => m.workspace_id !== workspaceID);
        selectWorkspace(next ? next.workspace_id : "");
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Delete failed");
    }
  };

  return (
    <div className="max-w-3xl space-y-8 p-6">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Account</h2>
        <p className="mt-1 text-sm text-zinc-500">{user.email}</p>
      </div>

      <section className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="font-medium text-zinc-900 dark:text-zinc-100">Workspaces</h3>
        <ul className="mt-3 space-y-2 text-sm">
          {workspaces.map((membership) => (
            <li key={membership.workspace_id} className="flex items-center justify-between gap-3">
              {editingWorkspaceID === membership.workspace_id ? (
                <form onSubmit={(e) => saveRename(e, membership.workspace_id)} className="flex flex-1 gap-2">
                  <input
                    value={editingName}
                    onChange={(e) => setEditingName(e.target.value)}
                    aria-label="Workspace name"
                    className="flex-1 rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-2 py-1 text-sm"
                  />
                  <button type="submit" className="rounded-md border px-2 py-1 text-xs">
                    Save
                  </button>
                  <button
                    type="button"
                    onClick={() => setEditingWorkspaceID(null)}
                    className="rounded-md border px-2 py-1 text-xs"
                  >
                    Cancel
                  </button>
                </form>
              ) : (
                <span>
                  {membership.workspace_name}{" "}
                  <span className="text-zinc-500">· {membership.role}</span>
                </span>
              )}
              {editingWorkspaceID !== membership.workspace_id && (
                <span className="flex items-center gap-2">
                  {membership.workspace_id === selectedWorkspaceID ? (
                    <span className="text-xs text-zinc-500">Selected</span>
                  ) : (
                    <button
                      onClick={() => selectWorkspace(membership.workspace_id)}
                      className="rounded-md border px-2 py-1 text-xs"
                    >
                      Select
                    </button>
                  )}
                  {canEdit(membership.role) && (
                    <button
                      onClick={() => startRename(membership.workspace_id, membership.workspace_name)}
                      className="rounded-md border px-2 py-1 text-xs"
                    >
                      Rename
                    </button>
                  )}
                  {canDelete(membership.role) &&
                    (confirmDeleteID === membership.workspace_id ? (
                      <>
                        <button
                          onClick={() => destroy(membership.workspace_id)}
                          className="rounded-md border border-red-300 dark:border-red-700 px-2 py-1 text-xs text-red-600 dark:text-red-400"
                        >
                          Confirm delete
                        </button>
                        <button
                          onClick={() => setConfirmDeleteID(null)}
                          className="rounded-md border px-2 py-1 text-xs"
                        >
                          Cancel
                        </button>
                      </>
                    ) : (
                      <button
                        onClick={() => setConfirmDeleteID(membership.workspace_id)}
                        className="rounded-md border px-2 py-1 text-xs text-red-600 dark:text-red-400"
                      >
                        Delete
                      </button>
                    ))}
                </span>
              )}
            </li>
          ))}
        </ul>
        {confirmDeleteID && (
          <p className="mt-2 text-xs text-red-600 dark:text-red-400">
            Deleting a workspace permanently removes its projects, analytics data, API keys, and memberships.
          </p>
        )}
        <form onSubmit={create} className="mt-4 flex gap-2">
          <input
            value={newWorkspace}
            onChange={(e) => setNewWorkspace(e.target.value)}
            placeholder="New workspace name"
            aria-label="New workspace name"
            className="flex-1 rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-sm"
          />
          <button type="submit" className="rounded-md border px-3 py-1.5 text-sm">
            Create
          </button>
        </form>
        {actionError && <p className="mt-2 text-sm text-red-600">{actionError}</p>}
      </section>

      <section className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="font-medium text-zinc-900 dark:text-zinc-100">Members</h3>
        {!selectedWorkspaceID ? (
          <p className="mt-2 text-sm text-zinc-500">Select a workspace first.</p>
        ) : membersError ? (
          <p className="mt-2 text-sm text-red-600">{membersError}</p>
        ) : (
          <table className="mt-3 w-full text-sm">
            <thead>
              <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
                <th className="pb-2 font-medium">Email</th>
                <th className="pb-2 font-medium">Role</th>
                <th className="pb-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => (
                <tr key={member.user_id} className="border-b border-zinc-100 dark:border-zinc-800/50">
                  <td className="py-2">{member.email}</td>
                  <td className="py-2">
                    {canManage ? (
                      <select
                        aria-label={`Role for ${member.email}`}
                        value={member.role}
                        onChange={(e) => changeRole(member.user_id, e.target.value)}
                        className="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-2 py-1 text-sm"
                      >
                        <option value="owner">owner</option>
                        <option value="admin">admin</option>
                        <option value="viewer">viewer</option>
                      </select>
                    ) : (
                      member.role
                    )}
                  </td>
                  <td className="py-2 text-right">
                    {(canManage || member.user_id === user.id) && (
                      <button
                        onClick={() => remove(member.user_id)}
                        className="rounded-md border px-2 py-1 text-xs"
                      >
                        {member.user_id === user.id ? "Leave" : "Remove"}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {canManage && selectedWorkspaceID && (
          <form onSubmit={invite} className="mt-4 flex flex-wrap gap-2">
            <input
              type="email"
              required
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              placeholder="teammate@example.com"
              aria-label="Invite email"
              className="flex-1 rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-sm"
            />
            <select
              aria-label="Invite role"
              value={inviteRole}
              onChange={(e) => setInviteRole(e.target.value)}
              className="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-2 py-1.5 text-sm"
            >
              <option value="viewer">viewer</option>
              <option value="admin">admin</option>
              <option value="owner">owner</option>
            </select>
            <button type="submit" className="rounded-md border px-3 py-1.5 text-sm">
              Invite
            </button>
          </form>
        )}
        {actionError && <p className="mt-2 text-sm text-red-600">{actionError}</p>}
        <p className="mt-2 text-xs text-zinc-500">
          Invites only work for registered emails. The last owner cannot be demoted or removed.
        </p>
      </section>

      <button
        onClick={() => logout()}
        className="rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1.5 text-sm"
      >
        Sign out
      </button>
    </div>
  );
}
