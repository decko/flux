import {
  createRouter,
  type RouterHistory,
} from '@tanstack/react-router';
import { Route as rootRoute } from './routes/__root';
import { Route as indexRoute } from './routes/index';
import { Route as projectsRoute } from './routes/projects';
import { Route as ticketsRoute } from './routes/tickets';
import { Route as pullRequestsRoute } from './routes/pull-requests';
import { Route as pipelineRunsRoute } from './routes/pipeline-runs';

const routeTree = rootRoute.addChildren([
  indexRoute,
  projectsRoute,
  ticketsRoute,
  pullRequestsRoute,
  pipelineRunsRoute,
]);

export function createAppRouter(history?: RouterHistory) {
  return createRouter({
    routeTree,
    defaultPreload: 'intent',
    history,
  });
}

export const router = createAppRouter();

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
