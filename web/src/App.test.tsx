import { render, screen } from '@testing-library/react';
import { RouterProvider } from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act } from 'react';
import { describe, it, expect, beforeEach } from 'vitest';
import { createAppRouter } from './router';
import { createMemoryHistory } from '@tanstack/react-router';
import { AuthProvider } from './auth/AuthContext';

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

describe('App', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('renders the navigation bar with all links', async () => {
    await renderWithRouter();

    expect(screen.getByText('Flux')).toBeInTheDocument();
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

  it('renders the dashboard heading on the home page', async () => {
    await renderWithRouter();
    expect(
      screen.getByRole('heading', { name: 'Dashboard' }),
    ).toBeInTheDocument();
  });

  it('renders the projects page content', async () => {
    localStorage.setItem('flux_token', 'test-token');
    await renderWithRouter('/projects');
    expect(
      screen.getByRole('heading', { name: 'Projects' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Create Project')).toBeInTheDocument();
  });
});
