import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: Dashboard,
});

function Dashboard() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
      <p className="mt-2 text-gray-600">
        Welcome to Flux — your control plane for agentic software development.
      </p>
      <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Projects" count={0} href="/projects" />
        <StatCard title="Tickets" count={0} href="/tickets" />
        <StatCard title="Pull Requests" count={0} href="/pull-requests" />
        <StatCard title="Pipeline Runs" count={0} href="/pipeline-runs" />
      </div>
    </div>
  );
}

interface StatCardProps {
  title: string;
  count: number;
  href: string;
}

function StatCard({ title, count, href }: StatCardProps) {
  return (
    <a
      href={href}
      className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition hover:shadow-md"
    >
      <p className="text-sm text-gray-500">{title}</p>
      <p className="mt-1 text-3xl font-semibold text-gray-900">{count}</p>
    </a>
  );
}
