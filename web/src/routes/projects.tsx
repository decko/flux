import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { createRoute, redirect, Link } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';
import { InstallationPicker } from '@/components/InstallationPicker';
import { RepositoryPicker } from '@/components/RepositoryPicker';
import type { GitHubInstallation, GitHubInstallationRepo } from '@/api/github';

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
  installation_id: number;
  created_at: string;
  updated_at: string;
}

interface CreateProjectInput {
  name: string;
  repo_url: string;
  installation_id: number;
}

// --- Route ---

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects',
  beforeLoad: ({ location }) => {
    const token = localStorage.getItem('flux_token');
    if (!token) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
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

  // --- Wizard state ---
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [selectedInstallation, setSelectedInstallation] =
    useState<GitHubInstallation | null>(null);
  const [selectedRepo, setSelectedRepo] =
    useState<GitHubInstallationRepo | null>(null);

  const query = useQuery<Project[]>({
    queryKey: ['projects'],
    queryFn: fetchProjects,
  });

  const mutation = useMutation<Project, Error, CreateProjectInput>({
    mutationFn: createProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setStep(1);
      setSelectedInstallation(null);
      setSelectedRepo(null);
    },
  });

  const handleSelectInstallation = (inst: GitHubInstallation) => {
    setSelectedInstallation(inst);
    setStep(2);
  };

  const handleSelectRepo = (repo: GitHubInstallationRepo) => {
    setSelectedRepo(repo);
    setStep(3);
  };

  const handleBack = () => {
    setStep((s) => (s - 1) as 1 | 2 | 3);
  };

  const handleCreate = () => {
    if (selectedInstallation && selectedRepo) {
      mutation.mutate({
        name: selectedRepo.name,
        repo_url: selectedRepo.html_url,
        installation_id: selectedInstallation.id,
      });
    }
  };

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Projects</h1>

      {/* Create Project Wizard */}
      <section className="mt-6 rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
        <h2 className="text-lg font-semibold text-gray-900">Create Project</h2>
        <p className="mt-1 text-sm text-gray-500">
          {step === 1 && 'Step 1: Select a GitHub App installation'}
          {step === 2 && 'Step 2: Choose a repository'}
          {step === 3 && 'Step 3: Confirm project details'}
        </p>

        <div className="mt-4">
          {step === 1 && (
            <InstallationPicker onSelect={handleSelectInstallation} />
          )}

          {step === 2 && selectedInstallation && (
            <>
              <div className="mb-4 flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleBack}
                  className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
                >
                  ← Back
                </button>
                <span className="text-sm text-gray-500">
                  Selected: {selectedInstallation.account.login} (
                  {selectedInstallation.target_type})
                </span>
              </div>
              <RepositoryPicker
                installationId={selectedInstallation.id}
                onSelect={handleSelectRepo}
              />
            </>
          )}

          {step === 3 && selectedRepo && selectedInstallation && (
            <div>
              <button
                type="button"
                onClick={handleBack}
                className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                ← Back
              </button>

              <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-4">
                <dl className="space-y-2">
                  <div>
                    <dt className="text-xs font-medium text-gray-500">
                      Project Name
                    </dt>
                    <dd className="text-sm font-semibold text-gray-900">
                      {selectedRepo.name}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs font-medium text-gray-500">
                      Repository URL
                    </dt>
                    <dd className="text-sm text-gray-700">
                      {selectedRepo.html_url}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs font-medium text-gray-500">
                      Installation
                    </dt>
                    <dd className="text-sm text-gray-700">
                      {selectedInstallation.account.login} (ID:{' '}
                      {selectedInstallation.id})
                    </dd>
                  </div>
                </dl>
              </div>

              <button
                type="button"
                onClick={handleCreate}
                disabled={mutation.isPending}
                className="mt-4 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {mutation.isPending ? 'Creating...' : 'Create Project'}
              </button>

              {mutation.isError && (
                <p role="alert" className="mt-2 text-sm text-red-600">
                  {mutation.error instanceof Error
                    ? mutation.error.message
                    : 'Failed to create project'}
                </p>
              )}
            </div>
          )}
        </div>
      </section>

      {/* Project list — hidden during wizard steps 2 and 3 to avoid text conflicts */}
      {step === 1 && (
        <section className="mt-6 space-y-4">
          {query.isPending && <ProjectSkeleton />}
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
            <ProjectList projects={query.data} />
          )}
        </section>
      )}
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
        <Link
          key={project.id}
          to="/projects/$id"
          params={{ id: project.id }}
          data-testid="project-card"
          className="block rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-shadow hover:shadow-md"
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
        </Link>
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
