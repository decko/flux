import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react';
import { setRefreshCallback } from '@/api/client';

const STORAGE_KEY = 'flux_token';

interface AuthContextValue {
  /** Current JWT token, or null if unauthenticated. */
  token: string | null;
  /** True when a non-null token is present. */
  isAuthenticated: boolean;
  /** Persist a token and mark the session as authenticated. */
  login: (token: string) => void;
  /** Remove the token and mark the session as unauthenticated. */
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/**
 * Reads a value from localStorage, returning null on error (e.g. SSR).
 */
function readToken(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

/**
 * Writes a value to localStorage, ignoring quota / permission errors.
 */
function writeToken(token: string | null): void {
  try {
    if (token === null) {
      localStorage.removeItem(STORAGE_KEY);
    } else {
      localStorage.setItem(STORAGE_KEY, token);
    }
  } catch {
    /* storage unavailable – state is still updated */
  }
}

/**
 * Provides authentication state to the component tree.
 *
 * On mount it reads the token from localStorage and registers a
 * callback with the API client so that token refreshes (triggered
 * by the 401 interceptor) stay in sync with React state.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(readToken);

  /* Keep the API-client refresh callback in sync with this state. */
  useEffect(() => {
    setRefreshCallback(setToken);
    return () => {
      setRefreshCallback(null);
    };
  }, []);

  const login = useCallback((newToken: string) => {
    writeToken(newToken);
    setToken(newToken);
  }, []);

  const logout = useCallback(() => {
    writeToken(null);
    setToken(null);
  }, []);

  const value: AuthContextValue = {
    token,
    isAuthenticated: token !== null,
    login,
    logout,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/**
 * Hook to access the current auth context.
 * Must be called inside an `<AuthProvider>`.
 */
export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
