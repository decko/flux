import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  createRoute,
  redirect,
  useParams,
  useNavigate,
} from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

// ─── Types ──────────────────────────────────────────────────────────────

interface ProjectDefinition {
  language: string;
  framework: string;
  conventions: string[];
  architecture: string;
}

interface Project {
  id: string;
  name: string;
  repo_url: string;
  definition: ProjectDefinition;
  adapters: unknown[];
  pipelines: string[];
  installation_id: number;
  created_at: string;
  updated_at: string;
}

interface TriggerRule {
  id: string;
  project_id: string;
  label: string;
  pipeline: string;
  enabled: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

// ─── Route ──────────────────────────────────────────────────────────────

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects/$id',
  beforeLoad: ({ location }) => {
    const token = localStorage.getItem('flux_token');
    if (!token) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
  component: ProjectDetailPage,
});

// ─── API helpers ────────────────────────────────────────────────────────

function getToken(): string | null {
  try {
    return localStorage.getItem('flux_token');
  } catch {
    return null;
  }
}

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

async function fetchProject(id: string): Promise<Project> {
  const res = await fetch(`/api/v1/projects/${id}`, { headers: authHeaders() });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
  return res.json() as Promise<Project>;
}

async function fetchTriggerRules(projectId: string): Promise<TriggerRule[]> {
  const res = await fetch(`/api/v1/projects/${projectId}/trigger-rules`, {
    headers: authHeaders(),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
  return res.json() as Promise<TriggerRule[]>;
}

async function createTriggerRule(
  projectId: string,
  data: { label: string; pipeline: string },
): Promise<TriggerRule> {
  const res = await fetch(`/api/v1/projects/${projectId}/trigger-rules`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
  return res.json() as Promise<TriggerRule>;
}

async function updateTriggerRule(
  projectId: string,
  ruleId: string,
  data: Partial<{ label: string; pipeline: string; enabled: boolean }>,
): Promise<TriggerRule> {
  const res = await fetch(
    `/api/v1/projects/${projectId}/trigger-rules/${ruleId}`,
    {
      method: 'PUT',
      headers: authHeaders(),
      body: JSON.stringify(data),
    },
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
  return res.json() as Promise<TriggerRule>;
}

async function deleteTriggerRule(
  projectId: string,
  ruleId: string,
): Promise<void> {
  const res = await fetch(
    `/api/v1/projects/${projectId}/trigger-rules/${ruleId}`,
    { method: 'DELETE', headers: authHeaders() },
  );
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }
}

// ─── Helpers ────────────────────────────────────────────────────────────

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

// ─── Page component ─────────────────────────────────────────────────────

export function ProjectDetailPage() {
  const { id } = useParams({ from: Route.id });
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const projectQuery = useQuery<Project>({
    queryKey: ['project', id],
    queryFn: () => fetchProject(id),
  });

  const rulesQuery = useQuery<TriggerRule[]>({
    queryKey: ['trigger-rules', id],
    queryFn: () => fetchTriggerRules(id),
  });

  const addMutation = useMutation<TriggerRule, Error, { label: string; pipeline: string }>({
    mutationFn: (data) => createTriggerRule(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['trigger-rules', id] });
    },
  });

  const updateMutation = useMutation<
    TriggerRule,
    Error,
    { ruleId: string; data: Partial<{ label: string; pipeline: string; enabled: boolean }> }
  >({
    mutationFn: ({ ruleId, data }) => updateTriggerRule(id, ruleId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['trigger-rules', id] });
    },
  });

  const deleteMutation = useMutation<void, Error, string>({
    mutationFn: (ruleId) => deleteTriggerRule(id, ruleId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['trigger-rules', id] });
    },
  });

  // --- Loading ---
  if (projectQuery.isPending) {
    return (
      <div>
        <ProjectDetailSkeleton />
      </div>
    );
  }

  // --- Error ---
  if (projectQuery.isError) {
    return (
      <div>
        <ErrorBanner
          message={extractErrorMessage(projectQuery.error)}
          onRetry={() => projectQuery.refetch()}
        />
      </div>
    );
  }

  const project = projectQuery.data;
  const rules = rulesQuery.data ?? [];

  return (
    <div>
      {/* Back button */}
      <button
        type="button"
        onClick={() => navigate({ to: '/projects' })}
        className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700"
      >
        &larr; Back to Projects
      </button>

      {/* Project header */}
      <section className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h1 className="text-2xl font-bold text-gray-900">{project.name}</h1>
        <p className="mt-1 text-sm text-gray-500">{project.repo_url}</p>
        <p className="mt-1 text-xs text-gray-400">
          Created {formatDate(project.created_at)}
        </p>
        <div className="mt-2">
          <DefinitionBadge definition={project.definition} />
        </div>
      </section>

      {/* Pipelines */}
      <section className="mt-6">
        <h2 className="text-lg font-semibold text-gray-900">Pipelines</h2>
        {project.pipelines.length === 0 ? (
          <p className="mt-2 text-sm text-gray-500">No pipelines configured.</p>
        ) : (
          <div className="mt-3 space-y-2">
            {project.pipelines.map((pipeline, idx) => (
              <div
                key={idx}
                className="rounded-lg border border-gray-200 bg-white px-4 py-3 text-sm text-gray-700"
              >
                {pipeline}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Trigger Rules */}
      <section className="mt-6">
        <h2 className="text-lg font-semibold text-gray-900">Trigger Rules</h2>

        {rulesQuery.isError && (
          <ErrorBanner
            message={extractErrorMessage(rulesQuery.error)}
            onRetry={() => rulesQuery.refetch()}
          />
        )}

        {rules.length === 0 && !rulesQuery.isError ? (
          <p className="mt-2 text-sm text-gray-500">
            No trigger rules configured.
          </p>
        ) : (
          <div className="mt-3 space-y-2">
            {rules.map((rule) => (
              <TriggerRuleRow
                key={rule.id}
                rule={rule}
                pipelines={project.pipelines}
                onToggle={(enabled) =>
                  updateMutation.mutate({
                    ruleId: rule.id,
                    data: { enabled },
                  })
                }
                onUpdate={(data) =>
                  updateMutation.mutate({ ruleId: rule.id, data })
                }
                onDelete={() => deleteMutation.mutate(rule.id)}
                isUpdating={updateMutation.isPending}
                isDeleting={deleteMutation.isPending}
              />
            ))}
          </div>
        )}

        {/* Add form */}
        <div className="mt-4">
          <AddTriggerRuleForm
            pipelines={project.pipelines}
            onSave={(data) => addMutation.mutate(data)}
            isSaving={addMutation.isPending}
            error={
              addMutation.isError
                ? extractErrorMessage(addMutation.error)
                : null
            }
          />
        </div>
      </section>
    </div>
  );
}

// ─── Trigger rule row (inline edit) ─────────────────────────────────────

interface TriggerRuleRowProps {
  rule: TriggerRule;
  pipelines: string[];
  onToggle: (enabled: boolean) => void;
  onUpdate: (data: { label?: string; pipeline?: string; enabled?: boolean }) => void;
  onDelete: () => void;
  isUpdating: boolean;
  isDeleting: boolean;
}

function TriggerRuleRow({
  rule,
  pipelines,
  onToggle,
  onUpdate,
  onDelete,
  isUpdating,
  isDeleting,
}: TriggerRuleRowProps) {
  const [editing, setEditing] = useState(false);
  const [editLabel, setEditLabel] = useState(rule.label);
  const [editPipeline, setEditPipeline] = useState(rule.pipeline);

  function handleSave() {
    onUpdate({ label: editLabel, pipeline: editPipeline });
    setEditing(false);
  }

  function handleCancel() {
    setEditLabel(rule.label);
    setEditPipeline(rule.pipeline);
    setEditing(false);
  }

  if (editing) {
    return (
      <div className="rounded-lg border border-blue-200 bg-blue-50 p-4">
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col text-xs font-medium text-gray-700">
            Label
            <input
              type="text"
              value={editLabel}
              onChange={(e) => setEditLabel(e.target.value)}
              className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm"
            />
          </label>
          <label className="flex flex-col text-xs font-medium text-gray-700">
            Pipeline
            <select
              value={editPipeline}
              onChange={(e) => setEditPipeline(e.target.value)}
              className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm"
            >
              {pipelines.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            onClick={handleSave}
            disabled={isUpdating}
            className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            Save
          </button>
          <button
            type="button"
            onClick={handleCancel}
            className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            Cancel
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-4 py-3">
      <div className="flex items-center gap-3">
        <label className="inline-flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={rule.enabled}
            onChange={(e) => onToggle(e.target.checked)}
            disabled={isUpdating}
            className="h-4 w-4 rounded border-gray-300 text-blue-600"
          />
          <span className="font-medium">{rule.label}</span>
        </label>
        <span className="text-xs text-gray-400">{rule.pipeline}</span>
      </div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => {
            setEditLabel(rule.label);
            setEditPipeline(rule.pipeline);
            setEditing(true);
          }}
          className="text-xs text-blue-600 hover:text-blue-800"
        >
          Edit
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={isDeleting}
          className="text-xs text-red-600 hover:text-red-800 disabled:opacity-50"
        >
          Delete
        </button>
      </div>
    </div>
  );
}

// ─── Add trigger rule form ──────────────────────────────────────────────

interface AddTriggerRuleFormProps {
  pipelines: string[];
  onSave: (data: { label: string; pipeline: string }) => void;
  isSaving: boolean;
  error: string | null;
}

function AddTriggerRuleForm({
  pipelines,
  onSave,
  isSaving,
  error,
}: AddTriggerRuleFormProps) {
  const [label, setLabel] = useState('');
  const [pipeline, setPipeline] = useState(pipelines[0] ?? '');

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!label.trim() || !pipeline) return;
    onSave({ label: label.trim(), pipeline });
    setLabel('');
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="rounded-lg border border-dashed border-gray-300 p-4"
    >
      <h3 className="text-sm font-medium text-gray-700">Add Trigger Rule</h3>
      <div className="mt-2 flex flex-wrap items-end gap-3">
        <label className="flex flex-col text-xs font-medium text-gray-700">
          Label
          <input
            type="text"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            required
            placeholder="e.g. Auto-deploy main"
            className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm"
          />
        </label>
        <label className="flex flex-col text-xs font-medium text-gray-700">
          Pipeline
          <select
            value={pipeline}
            onChange={(e) => setPipeline(e.target.value)}
            required
            className="mt-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm"
          >
            {pipelines.length === 0 && (
              <option value="">No pipelines available</option>
            )}
            {pipelines.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </label>
        <button
          type="submit"
          disabled={isSaving || pipelines.length === 0}
          className="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {isSaving ? 'Saving...' : 'Save'}
        </button>
      </div>
      {error && (
        <p role="alert" className="mt-2 text-sm text-red-600">
          {error}
        </p>
      )}
    </form>
  );
}

// ─── Sub-components ─────────────────────────────────────────────────────

function ProjectDetailSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label="loading">
      <div className="animate-pulse rounded-lg border border-gray-200 bg-white p-6">
        <div className="h-6 w-1/3 rounded bg-gray-200" />
        <div className="mt-2 h-4 w-1/2 rounded bg-gray-200" />
        <div className="mt-2 h-3 w-1/4 rounded bg-gray-200" />
      </div>
      <div className="animate-pulse rounded-lg border border-gray-200 bg-white p-4">
        <div className="h-4 w-1/4 rounded bg-gray-200" />
        <div className="mt-2 h-4 w-3/4 rounded bg-gray-200" />
      </div>
      <div className="animate-pulse rounded-lg border border-gray-200 bg-white p-4">
        <div className="h-4 w-1/4 rounded bg-gray-200" />
        <div className="mt-2 h-4 w-3/4 rounded bg-gray-200" />
      </div>
    </div>
  );
}

interface ErrorBannerProps {
  message: string;
  onRetry?: () => void;
}

function ErrorBanner({ message, onRetry }: ErrorBannerProps) {
  return (
    <div role="alert" className="rounded-lg border border-red-200 bg-red-50 p-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-red-800">{message}</p>
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="rounded-md bg-red-100 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-200"
          >
            Retry
          </button>
        )}
      </div>
    </div>
  );
}

function DefinitionBadge({ definition }: { definition: ProjectDefinition }) {
  if (!definition.language) return null;

  const label = definition.framework
    ? `${definition.language} / ${definition.framework}`
    : definition.language;

  return (
    <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-800">
      {label}
    </span>
  );
}
