import { Link, Outlet, createRootRoute } from '@tanstack/react-router';

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-7xl items-center gap-6 px-4 py-3">
          <Link to="/" className="text-lg font-bold text-gray-900">
            Flux
          </Link>
          <Link to="/projects" className="text-sm text-gray-600 hover:text-gray-900">
            Projects
          </Link>
          <Link to="/tickets" className="text-sm text-gray-600 hover:text-gray-900">
            Tickets
          </Link>
          <Link to="/pull-requests" className="text-sm text-gray-600 hover:text-gray-900">
            Pull Requests
          </Link>
          <Link to="/pipeline-runs" className="text-sm text-gray-600 hover:text-gray-900">
            Pipeline Runs
          </Link>
        </div>
      </nav>
      <main className="mx-auto max-w-7xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
