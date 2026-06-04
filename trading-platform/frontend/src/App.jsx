import { BrowserRouter as Router, Routes, Route, Link, useLocation } from 'react-router-dom';
import './App.css';
import SubmitPage from './pages/SubmitPage';
import Dashboard from './pages/Dashboard';
import Leaderboard from './pages/Leaderboard';
import ContestPage from './pages/ContestPage';

function Shell() {
  const location = useLocation();

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
          <Link className={`nav-link ${location.pathname === '/leaderboard' ? 'active' : ''}`} to="/leaderboard">
            Leaderboard
          </Link>
          <Link className={`nav-link ${location.pathname === '/submit' ? 'active' : ''}`} to="/submit">
            Submit engine
          </Link>
          <Link className={`nav-link ${location.pathname === '/contests' ? 'active' : ''}`} to="/contests">
            Contests
          </Link>
        </nav>
      </header>

      <main className="page-frame">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/leaderboard" element={<Leaderboard />} />
          <Route path="/submit" element={<SubmitPage />} />
          <Route path="/contests" element={<ContestPage />} />
        </Routes>
      </main>
    </div>
  );
}

function App() {
  return (
    <Router>
      <Shell />
    </Router>
  );
}

export default App;