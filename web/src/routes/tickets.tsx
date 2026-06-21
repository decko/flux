import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/tickets',
  component: TicketsPage,
});

function TicketsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Tickets</h1>
      <p className="mt-2 text-gray-600">View and manage tickets from connected sources.</p>
    </div>
  );
}
