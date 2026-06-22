import { render, screen, fireEvent } from '@testing-library/react';
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act } from 'react';
import { describe, it, expect, beforeEach } from 'vitest';
import { createAppRouter } from '../router';
import { AuthProvider } from '../auth/AuthContext';

async function renderWithRouter(initialPath = '/') {
  const queryClient = new QueryClient();
  const memoryHistory = createMemoryHistory({ initialEntries: [initialPath] });
  const router = createAppRouter(memoryHistory);

  await act(async () => {
    await router.load();
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

describe('RootLayout navigation', () => {
  beforeEach(() => {
    localStorage.clear();
  });
  it('renders all 5 nav links with correct labels', async () => {
    await renderWithRouter();

    expect(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Projects' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Tickets' })).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Pull Requests' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Pipeline Runs' }),
    ).toBeInTheDocument();
  });

  it('applies active styling to the current route link', async () => {
    localStorage.setItem('flux_token', 'test-token');
    await renderWithRouter('/projects');

    const projectsLink = screen.getByRole('link', { name: 'Projects' });
    const dashboardLink = screen.getByRole('link', { name: 'Dashboard' });

    expect(projectsLink.className).toContain('text-blue-600');
    expect(projectsLink.className).toContain('font-semibold');
    expect(dashboardLink.className).not.toContain('text-blue-600');
  });

  it('toggles mobile hamburger menu open and closed', async () => {
    const { container } = await renderWithRouter();

    // Initial state: menu closed
    expect(
      screen.getByRole('button', { name: 'Open menu' }),
    ).toBeInTheDocument();
    expect(
      container.querySelector('.border-t.border-gray-200'),
    ).not.toBeInTheDocument();

    // Open menu
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }));
    expect(
      screen.getByRole('button', { name: 'Close menu' }),
    ).toBeInTheDocument();
    expect(
      container.querySelector('.border-t.border-gray-200'),
    ).toBeInTheDocument();

    // Close menu
    fireEvent.click(screen.getByRole('button', { name: 'Close menu' }));
    expect(
      screen.getByRole('button', { name: 'Open menu' }),
    ).toBeInTheDocument();
    expect(
      container.querySelector('.border-t.border-gray-200'),
    ).not.toBeInTheDocument();
  });

  it('closes mobile menu when a nav link is clicked', async () => {
    const { container } = await renderWithRouter();

    // Open menu
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }));
    expect(
      container.querySelector('.border-t.border-gray-200'),
    ).toBeInTheDocument();

    // Click a mobile nav link
    const projectsLinks = screen.getAllByRole('link', { name: 'Projects' });
    // Index 0 = desktop link, index 1 = mobile link (in mobile panel)
    expect(projectsLinks.length).toBeGreaterThan(1);
    const mobileLink = projectsLinks[1];
    if (mobileLink) {
      fireEvent.click(mobileLink);
    }

    // Menu should close
    expect(
      container.querySelector('.border-t.border-gray-200'),
    ).not.toBeInTheDocument();
  });
});

describe('RootLayout auth links', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('shows Login link when no token is present', async () => {
    await renderWithRouter();

    expect(screen.getByRole('link', { name: 'Login' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Logout' })).not.toBeInTheDocument();
  });

  it('shows Logout button when token is present', async () => {
    localStorage.setItem('flux_token', 'test-token');

    await renderWithRouter();

    expect(screen.getByRole('button', { name: 'Logout' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Login' })).not.toBeInTheDocument();
  });

  it('Logout click clears token from localStorage', async () => {
    localStorage.setItem('flux_token', 'test-token');

    await renderWithRouter();

    fireEvent.click(screen.getByRole('button', { name: 'Logout' }));

    expect(localStorage.getItem('flux_token')).toBeNull();
  });

  it('Logout click reveals Login link', async () => {
    localStorage.setItem('flux_token', 'test-token');

    await renderWithRouter();

    expect(screen.getByRole('button', { name: 'Logout' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Login' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Logout' }));

    expect(screen.getByRole('link', { name: 'Login' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Logout' })).not.toBeInTheDocument();
  });
});
