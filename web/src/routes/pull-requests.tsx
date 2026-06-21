import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pull-requests',
  component: PullRequestsPage,
});

function PullRequestsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Pull Requests</h1>
      <p className="mt-2 text-gray-600">Review open pull requests across repositories.</p>
    </div>
  );
}
