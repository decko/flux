import { useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { createRoute, redirect } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

// --- Types matching the Go model ---

interface User {
  id: string;
  email: string;
  role: string;
  created_at: string;
}

// --- Route ---

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/users',
  beforeLoad: ({ location }) => {
    const token = localStorage.getItem('flux_token');
    if (!token) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
  component: AdminUsersPage,
});

// --- API helpers ---

/** Read JWT token from localStorage (set by login flow). */
function getToken(): string | null {
  try {
    return localStorage.getItem('flux_token');
  } catch {
    return null;
  }
}

/**
 * Decode the payload of a JWT without verifying the signature.
 * Used for client-side role checks.
 */
function decodeToken(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = parts[1];
    if (!payload) return null;
    return JSON.parse(atob(payload));
  } catch {
    return null;
  }
}

/**
 * Fetches all users (admin only).
 * GET /api/v1/admin/users → User[]
 */
async function fetchUsers(): Promise<User[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/admin/users', { headers });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<User[]>;
}

/**
 * Updates a user's role.
 * PUT /api/v1/admin/users/{id}/role body: {"role": "..."}
 */
async function updateUserRole(id: string, role: string): Promise<void> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`/api/v1/admin/users/${id}/role`, {
    method: 'PUT',
    headers,
    body: JSON.stringify({ role }),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
}

/**
 * Deletes a user.
 * DELETE /api/v1/admin/users/{id}
 */
async function deleteUser(id: string): Promise<void> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`/api/v1/admin/users/${id}`, {
    method: 'DELETE',
    headers,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
}

/**
 * Generates a random alphanumeric password of the given length
 * using cryptographically secure random values.
 */
function generatePassword(length = 16): string {
  const charset = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  const maxValid = Math.floor(256 / charset.length) * charset.length;
  const array = new Uint8Array(length * 2); // oversample to avoid loops
  crypto.getRandomValues(array);
  const result: number[] = [];
  for (let i = 0; i < array.length && result.length < length; i++) {
    const byte = array[i]!;
    if (byte >= maxValid) continue; // reject biased values
    result.push(byte % charset.length);
  }
  return result.map((i) => charset[i]).join('');
}

/**
 * Creates a new user (admin only).
 * POST /api/v1/admin/users body: {email, password, role} → User
 */
async function createUser(email: string, password: string, role: string): Promise<User> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/admin/users', {
    method: 'POST',
    headers,
    body: JSON.stringify({ email, password, role }),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<User>;
}

/**
 * Resets a user's password (admin only).
 * PUT /api/v1/admin/users/{id}/password body: {password} → User
 */
async function resetPassword(id: string, password: string): Promise<User> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`/api/v1/admin/users/${id}/password`, {
    method: 'PUT',
    headers,
    body: JSON.stringify({ password }),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<User>;
}

// --- Page component ---

/**
 * AdminUsersPage displays a user management table with role change, delete,
 * create user, and reset password capabilities. Accessible only to users
 * with role "admin".
 *
 * States: loading skeleton, error banner, empty state, user table.
 */
function AdminUsersPage() {
  const queryClient = useQueryClient();
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);
  const [roleError, setRoleError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [resetTarget, setResetTarget] = useState<User | null>(null);
  const [resetError, setResetError] = useState<string | null>(null);
  const [resetPw, setResetPw] = useState('');
  const [resetConfirm, setResetConfirm] = useState('');
  const deleteCancelRef = useRef<HTMLButtonElement>(null);
  const resetPwRef = useRef<HTMLInputElement>(null);

  // Role guard — decode JWT to check admin status
  const token = getToken();
  const payload = token ? decodeToken(token) : null;
  const isAdmin = payload?.role === 'admin';

  const query = useQuery<User[]>({
    queryKey: ['admin-users'],
    queryFn: fetchUsers,
    enabled: isAdmin,
  });

  const roleMutation = useMutation<void, Error, { id: string; role: string }, { previousUsers: User[] | undefined }>({
    mutationFn: ({ id, role }) => updateUserRole(id, role),
    onMutate: async ({ id, role }) => {
      setRoleError(null);
      await queryClient.cancelQueries({ queryKey: ['admin-users'] });
      const previousUsers = queryClient.getQueryData<User[]>(['admin-users']);
      queryClient.setQueryData<User[]>(['admin-users'], (old) =>
        old?.map((u) => (u.id === id ? { ...u, role } : u)) ?? [],
      );
      return { previousUsers };
    },
    onError: (err, _vars, context) => {
      if (context?.previousUsers) {
        queryClient.setQueryData(['admin-users'], context.previousUsers);
      }
      setRoleError(err.message);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
    },
  });

  const deleteMutation = useMutation<void, Error, string, { previousUsers: User[] | undefined }>({
    mutationFn: deleteUser,
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['admin-users'] });
      const previousUsers = queryClient.getQueryData<User[]>(['admin-users']);
      queryClient.setQueryData<User[]>(['admin-users'], (old) =>
        old?.filter((u) => u.id !== id) ?? [],
      );
      return { previousUsers };
    },
    onError: (_err, _vars, context) => {
      if (context?.previousUsers) {
        queryClient.setQueryData(['admin-users'], context.previousUsers);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
      setDeleteTarget(null);
    },
  });

  const createMutation = useMutation<User, Error, { email: string; password: string; role: string }, { previousUsers: User[] | undefined }>({
    mutationFn: ({ email, password, role }) => createUser(email, password, role),
    onMutate: async ({ email, role }) => {
      setCreateError(null);
      await queryClient.cancelQueries({ queryKey: ['admin-users'] });
      const previousUsers = queryClient.getQueryData<User[]>(['admin-users']);
      const tempUser: User = {
        id: `temp-${Date.now()}`,
        email,
        role,
        created_at: new Date().toISOString(),
      };
      queryClient.setQueryData<User[]>(['admin-users'], (old) =>
        [...(old ?? []), tempUser],
      );
      return { previousUsers };
    },
    onSuccess: () => {
      setShowCreateForm(false);
    },
    onError: (err, _vars, context) => {
      if (context?.previousUsers) {
        queryClient.setQueryData(['admin-users'], context.previousUsers);
      }
      setCreateError(err.message);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
    },
  });

  const resetMutation = useMutation<User, Error, { id: string; password: string }, unknown>({
    mutationFn: ({ id, password }) => resetPassword(id, password),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
      setResetTarget(null);
      setResetPw('');
      setResetConfirm('');
      setResetError(null);
    },
    onError: (err) => {
      setResetError(err.message);
    },
  });

  const handleRoleChange = (id: string, role: string) => {
    roleMutation.mutate({ id, role });
  };

  const handleDeleteConfirm = () => {
    if (deleteTarget) {
      deleteMutation.mutate(deleteTarget.id);
    }
  };

  const handleCreateSubmit = (data: { email: string; password: string; role: string }) => {
    createMutation.mutate(data);
  };

  const handleResetConfirm = () => {
    setResetError(null);
    if (resetPw !== resetConfirm) {
      setResetError('Passwords do not match');
      return;
    }
    if (resetTarget) {
      resetMutation.mutate({ id: resetTarget.id, password: resetPw });
    }
  };

  const handleResetCancel = () => {
    setResetTarget(null);
    setResetPw('');
    setResetConfirm('');
    setResetError(null);
  };

  useEffect(() => {
    if (deleteTarget) {
      deleteCancelRef.current?.focus();
    }
  }, [deleteTarget]);

  useEffect(() => {
    if (resetTarget) {
      resetPwRef.current?.focus();
    }
  }, [resetTarget]);

  // Access denied for non-admin users
  if (!isAdmin) {
    return (
      <div className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500">
        Access denied
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">User Management</h1>
        <button
          type="button"
          onClick={() => setShowCreateForm(true)}
          className="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          Add User
        </button>
      </div>

      {query.isPending && <UsersSkeleton />}
      {query.isError && (
        <ErrorBanner
          message={
            query.error instanceof Error
              ? query.error.message
              : String(query.error)
          }
        />
      )}
      {query.isSuccess && query.data.length === 0 && <EmptyState />}
      {query.isSuccess && query.data.length > 0 && (
        <UsersTable
          users={query.data}
          onRoleChange={handleRoleChange}
          onDelete={(user) => setDeleteTarget(user)}
          onResetPassword={(user) => setResetTarget(user)}
          disableReset={resetTarget !== null}
        />
      )}

      {/* Create user modal */}
      {showCreateForm && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
          role="dialog"
          aria-modal="true"
          aria-labelledby="create-user-title"
          tabIndex={-1}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setShowCreateForm(false);
          }}
        >
          <div className="rounded-lg bg-white p-6 shadow-xl max-w-md w-full">
            <h2 id="create-user-title" className="text-lg font-semibold text-gray-900">Create User</h2>
            {createError && (
              <div
                role="alert"
                className="mt-3 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800"
              >
                {createError}
              </div>
            )}
            <CreateUserForm
              onSubmit={handleCreateSubmit}
              onCancel={() => setShowCreateForm(false)}
              isPending={createMutation.isPending}
            />
          </div>
        </div>
      )}

      {roleError && (
        <div
          role="alert"
          className="mt-4 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
        >
          {roleError}
        </div>
      )}

      {/* Delete confirmation dialog */}
      {deleteTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
          role="dialog"
          aria-modal="true"
          aria-labelledby="delete-title"
          tabIndex={-1}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setDeleteTarget(null);
          }}
        >
          <div className="rounded-lg bg-white p-6 shadow-xl">
            <h2 id="delete-title" className="text-lg font-semibold text-gray-900">Delete User</h2>
            <p className="mt-2 text-sm text-gray-700">
              Are you sure you want to delete{' '}
              <strong>{deleteTarget.email}</strong>?
            </p>
            <div className="mt-4 flex justify-end gap-3">
              <button
                type="button"
                ref={deleteCancelRef}
                onClick={() => setDeleteTarget(null)}
                className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleDeleteConfirm}
                disabled={deleteMutation.isPending}
                className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
              >
                {deleteMutation.isPending ? 'Deleting...' : 'Confirm Delete'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Reset password modal */}
      {resetTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
          role="dialog"
          aria-modal="true"
          aria-labelledby="reset-password-title"
          tabIndex={-1}
          onKeyDown={(e) => {
            if (e.key === 'Escape') handleResetCancel();
          }}
        >
          <div className="rounded-lg bg-white p-6 shadow-xl">
            <h2 id="reset-password-title" className="mb-4 text-lg font-semibold text-gray-900">
              Reset Password for {resetTarget.email}
            </h2>
            {resetError && (
              <div role="alert" className="mb-3 text-sm text-red-600">
                {resetError}
              </div>
            )}
            <div className="space-y-3">
              <input
                ref={resetPwRef}
                aria-label="New password"
                type="password"
                value={resetPw}
                onChange={(e) => setResetPw(e.target.value)}
                className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
              />
              <input
                aria-label="Confirm password"
                type="password"
                value={resetConfirm}
                onChange={(e) => setResetConfirm(e.target.value)}
                className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
              />
            </div>
            <div className="mt-4 flex justify-end gap-3">
              <button
                type="button"
                onClick={handleResetCancel}
                className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleResetConfirm}
                disabled={resetMutation.isPending}
                className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {resetMutation.isPending ? 'Resetting...' : 'Reset'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// --- Sub-components ---

/** Skeleton loader shown while users are being fetched. */
function UsersSkeleton() {
  return (
    <div className="mt-6 space-y-4" role="status" aria-label="loading users">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="animate-pulse rounded-lg border border-gray-200 bg-white p-4"
        >
          <div className="h-4 w-1/3 rounded bg-gray-200" />
          <div className="mt-2 h-4 w-1/2 rounded bg-gray-200" />
          <div className="mt-2 h-4 w-1/4 rounded bg-gray-200" />
        </div>
      ))}
    </div>
  );
}

/** Error banner displayed when the fetch fails. */
function ErrorBanner({ message }: { message: string }) {
  return (
    <div
      role="alert"
      className="mt-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
    >
      {message}
    </div>
  );
}

/** Empty state shown when no users exist. */
function EmptyState() {
  return (
    <div className="mt-6 rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500">
      No users found
    </div>
  );
}

// --- Create user form ---

interface CreateUserFormProps {
  onSubmit: (data: { email: string; password: string; role: string }) => void;
  onCancel: () => void;
  isPending: boolean;
}

/**
 * Form for creating a new user with email, password,
 * confirm password, and role fields. Includes a generate button
 * for random password creation. Used inside a modal overlay.
 */
function CreateUserForm({ onSubmit, onCancel, isPending }: CreateUserFormProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [role, setRole] = useState('user');
  const [localError, setLocalError] = useState<string | null>(null);
  const emailRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    emailRef.current?.focus();
  }, []);

  const handleGenerate = () => {
    const pw = generatePassword();
    setPassword(pw);
    setConfirmPassword(pw);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setLocalError(null);

    if (!email.trim()) {
      setLocalError('Email is required');
      return;
    }

    if (password !== confirmPassword) {
      setLocalError('Passwords do not match');
      return;
    }

    onSubmit({ email: email.trim(), password, role });
  };

  return (
    <form onSubmit={handleSubmit} className="mt-4 space-y-3">
      <input
        ref={emailRef}
        aria-label="Email"
        type="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="Email"
        className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
      />
        <div className="flex gap-2">
          <input
            aria-label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Password"
            className="flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm"
          />
          <button
            type="button"
            onClick={handleGenerate}
            className="rounded-md border border-gray-300 bg-white px-4 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            Generate
          </button>
        </div>
        <input
          aria-label="Confirm password"
          type="password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          placeholder="Confirm password"
          className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
        />
        <select
          aria-label="Role"
          value={role}
          onChange={(e) => setRole(e.target.value)}
          className="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm"
        >
          <option value="user">user</option>
          <option value="admin">admin</option>
        </select>
        {localError && (
          <div role="alert" className="text-sm text-red-600">
            {localError}
          </div>
        )}
        <div className="flex justify-end gap-3">
          <button
            type="button"
            onClick={onCancel}
            className="rounded-md border border-gray-300 bg-white px-4 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isPending}
            className="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {isPending ? 'Creating...' : 'Create'}
          </button>
        </div>
      </form>
  );
}

// --- User table ---

interface UsersTableProps {
  users: User[];
  onRoleChange: (id: string, role: string) => void;
  onDelete: (user: User) => void;
  onResetPassword: (user: User) => void;
  disableReset?: boolean;
}

/** Renders the admin user table with role management and delete actions. */
function UsersTable({ users, onRoleChange, onDelete, onResetPassword, disableReset }: UsersTableProps) {
  return (
    <div className="mt-6 overflow-x-auto">
      <table className="w-full border border-gray-200">
        <thead>
          <tr className="border-b border-gray-200 bg-gray-50">
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
              Email
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
              Role
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
              Created At
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
              Actions
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 bg-white">
          {users.map((user) => (
            <tr key={user.id}>
              <td className="px-4 py-3 text-sm text-gray-900">{user.email}</td>
              <td className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <RoleBadge role={user.role} />
                  <select
                    aria-label={`Role for ${user.email}`}
                    value={user.role}
                    onChange={(e) => onRoleChange(user.id, e.target.value)}
                    className="rounded-md border border-gray-300 px-2 py-1 text-xs focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  >
                    <option value="admin">admin</option>
                    <option value="user">user</option>
                  </select>
                </div>
              </td>
              <td className="px-4 py-3 text-sm text-gray-500">
                {formatDate(user.created_at)}
              </td>
              <td className="px-4 py-3">
                <button
                  type="button"
                  disabled={disableReset}
                  aria-disabled={disableReset}
                  aria-label={`Reset password for ${user.email}`}
                  onClick={() => onResetPassword(user)}
                  className={`text-sm ${disableReset ? 'text-gray-400 cursor-not-allowed' : 'text-blue-600 hover:text-blue-800'}`}
                >
                  Reset Password
                </button>
                <button
                  type="button"
                  aria-label={`Delete ${user.email}`}
                  onClick={() => onDelete(user)}
                  className="text-red-600 hover:text-red-800"
                >
                  Delete
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Badge showing the user role with colour coding. */
function RoleBadge({ role }: { role: string }) {
  const colorClass =
    role === 'admin'
      ? 'bg-blue-100 text-blue-800'
      : 'bg-gray-100 text-gray-800';

  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${colorClass}`}
    >
      {role}
    </span>
  );
}

/** Format an ISO date string to a human-readable locale date. */
function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    return date.toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return dateStr;
  }
}
