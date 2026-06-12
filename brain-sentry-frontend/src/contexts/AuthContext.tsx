import * as React from "react";
import { jwtDecode } from "jwt-decode";

import { API_BASE_URL, getStoredTenantId } from "@/lib/api/client";

interface User {
  id: string;
  email: string;
  name?: string;
  tenantId?: string;
  roles?: string[];
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshToken: () => Promise<void>;
  updateUser: (user: User) => void;
}

const AuthContext = React.createContext<AuthContextValue | undefined>(undefined);

const TOKEN_KEY = "brain_sentry_token";
const USER_KEY = "brain_sentry_user";
const API_URL = API_BASE_URL;

interface AuthProviderProps {
  children: React.ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const [state, setState] = React.useState<AuthState>({
    user: null,
    token: null,
    isAuthenticated: false,
    isLoading: true,
  });

  // Initialize auth state from localStorage
  React.useEffect(() => {
    const token = localStorage.getItem(TOKEN_KEY);
    const userStr = localStorage.getItem(USER_KEY);

    if (token && userStr) {
      try {
        const user = JSON.parse(userStr);
        // Check if token is expired
        const decoded: any = jwtDecode(token);
        const now = Date.now() / 1000;

        if (decoded.exp && decoded.exp < now) {
          // Token expired
          logoutInternal();
        } else {
          setState({
            user,
            token,
            isAuthenticated: true,
            isLoading: false,
          });
        }
      } catch (error) {
        console.error("Error parsing stored user:", error);
        logoutInternal();
      }
    } else {
      setState((prev) => ({ ...prev, isLoading: false }));
    }
  }, []);

  const login = async (email: string, password: string) => {
    // Pré-auth não há JWT; envia o tenant conhecido (se houver) e deixa o
    // backend cair no default quando ausente. Nunca um ID hardcoded.
    const tenantId = getStoredTenantId();
    const response = await fetch(`${API_URL}/v1/auth/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(tenantId ? { "X-Tenant-ID": tenantId } : {}),
      },
      body: JSON.stringify({ email, password }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || "Login failed");
    }

    const data = await response.json();
    const { token, user, tenantId: responseTenantId } = data;

    localStorage.setItem(TOKEN_KEY, token);
    localStorage.setItem(USER_KEY, JSON.stringify(user));
    const resolvedTenantId = responseTenantId || user?.tenantId || tenantId;
    if (resolvedTenantId) {
      localStorage.setItem("tenant_id", resolvedTenantId);
    }

    setState({
      user,
      token,
      isAuthenticated: true,
      isLoading: false,
    });
  };

  const logout = () => {
    logoutInternal();
    // Optional: Call logout endpoint to invalidate token server-side
    fetch(`${API_URL}/v1/auth/logout`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(state.token && { Authorization: `Bearer ${state.token}` }),
      },
    }).catch(console.error);
  };

  const logoutInternal = () => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    setState({
      user: null,
      token: null,
      isAuthenticated: false,
      isLoading: false,
    });
  };

  const refreshToken = async () => {
    if (!state.token) return;

    try {
      const response = await fetch(`${API_URL}/v1/auth/refresh`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${state.token}`,
        },
      });

      if (!response.ok) {
        logoutInternal();
        throw new Error("Token refresh failed");
      }

      const data = await response.json();
      const { token } = data;

      localStorage.setItem(TOKEN_KEY, token);

      setState((prev) => ({ ...prev, token }));
    } catch (error) {
      logoutInternal();
      throw error;
    }
  };

  const updateUser = (user: User) => {
    localStorage.setItem(USER_KEY, JSON.stringify(user));
    setState((prev) => ({ ...prev, user }));
  };

  const value: AuthContextValue = {
    ...state,
    login,
    logout,
    refreshToken,
    updateUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = React.useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

// Hook to check if user has required roles
export function useHasRole(...roles: string[]) {
  const { user } = useAuth();
  if (!user?.roles) return false;
  return roles.some((role) => user.roles?.includes(role));
}

// Hook to get authorization header
export function useAuthHeader() {
  const { token } = useAuth();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// Hook for protected fetch
export function useAuthenticatedFetch() {
  const { token } = useAuth();

  const fetchWithAuth = async (url: string, options: RequestInit = {}) => {
    const headers = {
      ...options.headers,
      ...(token && { Authorization: `Bearer ${token}` }),
    };

    return fetch(url, { ...options, headers });
  };

  return fetchWithAuth;
}
