import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pipeline-runs',
  component: PipelineRunsPage,
});

function PipelineRunsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Pipeline Runs</h1>
      <p className="mt-2 text-gray-600">Monitor pipeline execution status.</p>
    </div>
  );
}
