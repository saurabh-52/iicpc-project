import { BrowserRouter as Router, Routes, Route, Link, useLocation, Navigate } from 'react-router-dom';
import './App.css';
import { AuthProvider, useAuth } from './context/AuthContext';
import ProtectedRoute from './components/ProtectedRoute';
import SubmitPage from './pages/SubmitPage';
import Dashboard from './pages/Dashboard';
import Leaderboard from './pages/Leaderboard';
import ContestPage from './pages/ContestPage';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';

function Shell() {
  const location = useLocation();
  const { user, isAuthenticated, logout, loading } = useAuth();

  // Don't show the shell nav on auth pages
  const isAuthPage = location.pathname === '/login' || location.pathname === '/register';

  if (isAuthPage) {
    return (
      <Routes>
        <Route path="/login" element={
          isAuthenticated ? <Navigate to="/" replace /> : <LoginPage />
        } />
        <Route path="/register" element={
          isAuthenticated ? <Navigate to="/" replace /> : <RegisterPage />
        } />
      </Routes>
    );
  }

  if (loading) {
    return (
      <div className="auth-loading" style={{ minHeight: '100vh' }}>
        <div className="auth-loading-spinner" />
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand-lockup">
          <div className="brand-mark">TP</div>
          <div>
            <p className="eyebrow">Trading Platform</p>
            <h1 className="brand-title">Operational control for engines and stress tests</h1>
          </div>
        </div>

        <nav className="nav-actions" aria-label="Primary navigation">
          <Link className={`nav-link ${location.pathname === '/' ? 'active' : ''}`} to="/">
            Dashboard
          </Link>
          <Link className={`nav-link ${location.pathname === '/submit' ? 'active' : ''}`} to="/submit">
            ⚡ Practice
          </Link>
          <Link className={`nav-link ${location.pathname === '/contests' ? 'active' : ''}`} to="/contests">
            🏆 Contests
          </Link>
          <Link className={`nav-link ${location.pathname === '/leaderboard' ? 'active' : ''}`} to="/leaderboard">
            Leaderboard
          </Link>

          {isAuthenticated ? (
            <div className="nav-user-section">
              <div className="nav-user-avatar">
                {user?.username?.charAt(0)?.toUpperCase() || '?'}
              </div>
              <span className="nav-user-name">{user?.username}</span>
              <button className="nav-logout-btn" onClick={logout} title="Sign out">
                ↪
              </button>
            </div>
          ) : (
            <Link className="nav-link nav-login-btn" to="/login">
              Sign in
            </Link>
          )}
        </nav>
      </header>

      <main className="page-frame">
        <Routes>
          <Route path="/" element={
            <ProtectedRoute><Dashboard /></ProtectedRoute>
          } />
          <Route path="/leaderboard" element={<Leaderboard />} />
          <Route path="/submit" element={
            <ProtectedRoute><SubmitPage /></ProtectedRoute>
          } />
          <Route path="/contests" element={
            <ProtectedRoute><ContestPage /></ProtectedRoute>
          } />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}

function App() {
  return (
    <Router>
      <AuthProvider>
        <Shell />
      </AuthProvider>
    </Router>
  );
}

export default App;