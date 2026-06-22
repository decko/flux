import { Link, Outlet, createRootRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useAuth } from '../auth/AuthContext';

export const Route = createRootRoute({
  component: RootLayout,
});

interface NavItem {
  to: string;
  label: string;
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Dashboard' },
  { to: '/projects', label: 'Projects' },
  { to: '/tickets', label: 'Tickets' },
  { to: '/pull-requests', label: 'Pull Requests' },
  { to: '/pipeline-runs', label: 'Pipeline Runs' },
];

function RootLayout() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { isAuthenticated, logout } = useAuth();
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate({ to: '/login' });
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="border-b border-gray-200 bg-white">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3">
          <span className="text-lg font-bold text-gray-900">Flux</span>

          {/* Desktop nav links + auth */}
          <div className="hidden md:flex md:items-center md:gap-6">
            {NAV_ITEMS.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                activeProps={{ className: 'text-blue-600 font-semibold' }}
                className="text-sm text-gray-600 transition-colors duration-150 hover:text-gray-900"
              >
                {item.label}
              </Link>
            ))}
            <span className="border-l border-gray-200 pl-6">
              {isAuthenticated ? (
                <button
                  type="button"
                  onClick={handleLogout}
                  className="text-sm text-gray-600 transition-colors duration-150 hover:text-gray-900"
                >
                  Logout
                </button>
              ) : (
                <Link
                  to="/login"
                  className="text-sm text-gray-600 transition-colors duration-150 hover:text-gray-900"
                >
                  Login
                </Link>
              )}
            </span>
          </div>

          {/* Mobile hamburger button */}
          <button
            type="button"
            className="rounded-md p-2 text-gray-600 hover:bg-gray-100 hover:text-gray-900 md:hidden"
            onClick={() => setMobileMenuOpen((prev) => !prev)}
            aria-label={mobileMenuOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={mobileMenuOpen}
          >
            <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              {mobileMenuOpen ? (
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
              )}
            </svg>
          </button>
        </div>

        {/* Mobile nav panel */}
        {mobileMenuOpen && (
          <div className="border-t border-gray-200 md:hidden">
            <div className="space-y-1 px-4 py-3">
              {NAV_ITEMS.map((item) => (
                <Link
                  key={item.to}
                  to={item.to}
                  activeProps={{ className: 'text-blue-600 font-semibold' }}
                  className="block text-sm text-gray-600 transition-colors duration-150 hover:text-gray-900"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  {item.label}
                </Link>
              ))}
              {isAuthenticated ? (
                <button
                  type="button"
                  onClick={() => {
                    setMobileMenuOpen(false);
                    handleLogout();
                  }}
                  className="block text-sm text-gray-600 transition-colors duration-150 hover:text-gray-900"
                >
                  Logout
                </button>
              ) : (
                <Link
                  to="/login"
                  className="block text-sm text-gray-600 transition-colors duration-150 hover:text-gray-900"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  Login
                </Link>
              )}
            </div>
          </div>
        )}
      </nav>

      <main className="mx-auto max-w-7xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
