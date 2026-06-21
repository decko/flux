import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/projects',
  component: ProjectsPage,
});

function ProjectsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Projects</h1>
      <p className="mt-2 text-gray-600">Manage your projects.</p>
    </div>
  );
}
