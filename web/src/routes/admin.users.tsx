import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/users',
  component: AdminUsersPage,
});

/**
 * Minimal stub for the admin users management page.
 * The real implementation will render a user table with role management.
 */
function AdminUsersPage() {
  return <div>Admin Users</div>;
}
