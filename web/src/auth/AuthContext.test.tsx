import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AuthProvider, useAuth } from './AuthContext';
import type { ReactNode } from 'react';

function TestConsumer() {
  const { token, isAuthenticated, login, logout } = useAuth();
  return (
    <div>
      <div data-testid="token-value">{token ?? 'null'}</div>
      <div data-testid="is-authenticated">{String(isAuthenticated)}</div>
      <button
        data-testid="login-btn"
        onClick={() => login('test-token')}
        type="button"
      >
        Login
      </button>
      <button
        data-testid="logout-btn"
        onClick={() => logout()}
        type="button"
      >
        Logout
      </button>
    </div>
  );
}

function renderWithAuth(children?: ReactNode) {
  return render(
    <AuthProvider>{children ?? <TestConsumer />}</AuthProvider>,
  );
}

describe('AuthContext', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('reads token from localStorage on mount', () => {
    localStorage.setItem('flux_token', 'stored-token');
    renderWithAuth();
    expect(screen.getByTestId('token-value').textContent).toBe('stored-token');
    expect(screen.getByTestId('is-authenticated').textContent).toBe('true');
  });

  it('defaults to unauthenticated when no token stored', () => {
    renderWithAuth();
    expect(screen.getByTestId('token-value').textContent).toBe('null');
    expect(screen.getByTestId('is-authenticated').textContent).toBe('false');
  });

  it('login stores token in localStorage and updates state', () => {
    renderWithAuth();
    fireEvent.click(screen.getByTestId('login-btn'));
    expect(screen.getByTestId('token-value').textContent).toBe('test-token');
    expect(screen.getByTestId('is-authenticated').textContent).toBe('true');
    expect(localStorage.getItem('flux_token')).toBe('test-token');
  });

  it('logout clears token from localStorage and state', () => {
    localStorage.setItem('flux_token', 'stored-token');
    renderWithAuth();
    fireEvent.click(screen.getByTestId('logout-btn'));
    expect(screen.getByTestId('token-value').textContent).toBe('null');
    expect(screen.getByTestId('is-authenticated').textContent).toBe('false');
    expect(localStorage.getItem('flux_token')).toBeNull();
  });

  it('throws error when useAuth is used outside AuthProvider', () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => render(<TestConsumer />)).toThrow(
      'useAuth must be used within an AuthProvider',
    );
    consoleSpy.mockRestore();
  });
});
