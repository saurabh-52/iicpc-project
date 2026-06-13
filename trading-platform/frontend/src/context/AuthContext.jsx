import { createContext, useContext, useState, useEffect, useCallback } from 'react';

const AuthContext = createContext(null);

const AUTH_KEY = 'iicpc_auth';

function getStoredAuth() {
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && parsed.token && parsed.user) {
        return parsed;
      }
    }
  } catch {
    // ignore
  }
  return null;
}

function storeAuth(data) {
  localStorage.setItem(AUTH_KEY, JSON.stringify(data));
}

function clearAuth() {
  localStorage.removeItem(AUTH_KEY);
}

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [token, setToken] = useState(null);
  const [loading, setLoading] = useState(true);

  // Restore session on mount
  useEffect(() => {
    const stored = getStoredAuth();
    if (stored) {
      setUser(stored.user);
      setToken(stored.token);
      // Validate token is still valid
      fetch('/api/me', {
        headers: { 'Authorization': `Bearer ${stored.token}` },
      })
        .then(res => {
          if (!res.ok) throw new Error('Token expired');
          return res.json();
        })
        .then(data => {
          setUser(data.user);
        })
        .catch(() => {
          // Token expired or invalid
          setUser(null);
          setToken(null);
          clearAuth();
        })
        .finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, []);

  const login = useCallback(async (username, password) => {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.message || 'Login failed');
    }
    setUser(data.user);
    setToken(data.token);
    storeAuth({ user: data.user, token: data.token });
    return data;
  }, []);

  const register = useCallback(async (username, email, password) => {
    const res = await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, email, password }),
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.message || 'Registration failed');
    }
    setUser(data.user);
    setToken(data.token);
    storeAuth({ user: data.user, token: data.token });
    return data;
  }, []);

  const logout = useCallback(() => {
    setUser(null);
    setToken(null);
    clearAuth();
    window.location.href = '/login';
  }, []);

  const authHeaders = useCallback(() => {
    if (!token) return {};
    return { 'Authorization': `Bearer ${token}` };
  }, [token]);

  const value = {
    user,
    token,
    loading,
    isAuthenticated: !!user && !!token,
    login,
    register,
    logout,
    authHeaders,
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
}

export default AuthContext;
