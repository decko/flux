import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

// --- Types matching the Go model (internal/model/project.go) ---

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
  pipelines: unknown[];
  created_at: string;
  updated_at: string;
}

interface CreateProjectInput {
  name: string;
  repo_url: string;
}

// --- Route ---

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects',
  component: ProjectsPage,
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
 * Fetches all projects.
 * GET /api/v1/projects → Project[]
 */
async function fetchProjects(): Promise<Project[]> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/projects', { headers });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<Project[]>;
}

/**
 * Creates a new project.
 * POST /api/v1/projects → Project
 */
async function createProject(input: CreateProjectInput): Promise<Project> {
  const token = getToken();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/projects', {
    method: 'POST',
    headers,
    body: JSON.stringify(input),
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as Record<string, unknown>).error as string || res.statusText);
  }

  return res.json() as Promise<Project>;
}

// --- Page component ---

/**
 * ProjectsPage displays the project list and a create form.
 * Supports loading (skeleton), empty, error, and success states.
 */
export function ProjectsPage() {
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [repoUrl, setRepoUrl] = useState('');

  const query = useQuery<Project[]>({
    queryKey: ['projects'],
    queryFn: fetchProjects,
  });

  const mutation = useMutation<Project, Error, CreateProjectInput>({
    mutationFn: createProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setName('');
      setRepoUrl('');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (name.trim() && repoUrl.trim()) {
      mutation.mutate({ name: name.trim(), repo_url: repoUrl.trim() });
    }
  };

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Projects</h1>

      {/* Create form */}
      <section className="mt-6 rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
        <h2 className="text-lg font-semibold text-gray-900">Create Project</h2>
        <form onSubmit={handleSubmit} className="mt-4 flex flex-wrap items-end gap-4">
          <div>
            <label htmlFor="project-name" className="block text-sm font-medium text-gray-700">
              Project Name
            </label>
            <input
              id="project-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-1 block w-60 rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              placeholder="my-project"
              required
            />
          </div>
          <div>
            <label htmlFor="repo-url" className="block text-sm font-medium text-gray-700">
              Repo URL
            </label>
            <input
              id="repo-url"
              type="url"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              className="mt-1 block w-80 rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              placeholder="https://github.com/org/repo"
              required
            />
          </div>
          <button
            type="submit"
            disabled={mutation.isPending}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {mutation.isPending ? 'Creating...' : 'Create'}
          </button>
        </form>
        {mutation.isError && (
          <p role="alert" className="mt-2 text-sm text-red-600">
            {mutation.error instanceof Error ? mutation.error.message : 'Failed to create project'}
          </p>
        )}
      </section>

      {/* Project list */}
      <section className="mt-6 space-y-4">
        {query.isPending && <ProjectSkeleton />}
        {query.isError && (
          <ErrorBanner
            message={query.error instanceof Error ? query.error.message : String(query.error)}
          />
        )}
        {query.isSuccess && query.data.length === 0 && <EmptyState />}
        {query.isSuccess && query.data.length > 0 && <ProjectList projects={query.data} />}
      </section>
    </div>
  );
}

// --- Sub-components ---

/** Skeleton loader shown while projects are fetching. */
function ProjectSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label="loading">
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
      className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800"
    >
      {message}
    </div>
  );
}

/** Empty state shown when no projects exist. */
function EmptyState() {
  return (
    <div
      role="status"
      className="rounded-lg border border-dashed border-gray-300 p-8 text-center text-gray-500"
    >
      No projects
    </div>
  );
}

// --- Project list ---

interface ProjectListProps {
  projects: Project[];
}

/** Renders the list of project cards. */
function ProjectList({ projects }: ProjectListProps) {
  return (
    <div className="space-y-4">
      {projects.map((project) => (
        <div
          key={project.id}
          data-testid="project-card"
          className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
        >
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-medium text-gray-900">{project.name}</h3>
            <DefinitionBadge definition={project.definition} />
          </div>
          <p className="mt-1 text-xs text-gray-500">{project.repo_url}</p>
          {project.definition.language && (
            <p className="mt-1 text-xs text-gray-400">
              {project.definition.language}
              {project.definition.framework ? ` / ${project.definition.framework}` : ''}
            </p>
          )}
          <div className="mt-2">
            <a
              href="#"
              className="text-xs text-blue-600 hover:text-blue-800"
              onClick={(e) => {
                e.preventDefault();
                // TODO: navigate to adapter config page
              }}
            >
              Adapter Config
            </a>
          </div>
        </div>
      ))}
    </div>
  );
}

/** Badge showing language and framework from the project definition. */
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
