import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

function getLanguageColor(lang) {
  switch (lang?.toLowerCase()) {
    case 'cpp':
    case 'c++':
      return '#3b82f6';
    case 'go':
      return '#06b6d4';
    case 'rust':
      return '#f97316';
    case 'python':
    case 'py':
      return '#eab308';
    default:
      return '#64748b';
  }
}

function formatRelativeTime(iso) {
  if (!iso) return '—';
  const diff = Date.now() - new Date(iso);
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'Just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function SubmissionChart({ entries }) {
  const chartData = [...entries].reverse().slice(-7);
  
  if (chartData.length < 2) {
    return (
      <div style={{ height: '180px', display: 'grid', placeItems: 'center', color: '#64748b', fontSize: '0.88rem', background: 'rgba(0,0,0,0.01)', border: '1px dashed rgba(0,0,0,0.06)', borderRadius: '12px' }}>
        <span>📈 Run multiple tests to plot performance history</span>
      </div>
    );
  }

  const width = 500;
  const height = 180;
  const padding = 30;

  const points = chartData.map((d, i) => {
    const x = padding + (i * (width - 2 * padding)) / (chartData.length - 1);
    const y = height - padding - (d.total_score * (height - 2 * padding)) / 100;
    return { x, y, score: d.total_score, label: d.system_name };
  });

  const pathD = `M ${points.map(p => `${p.x} ${p.y}`).join(' L ')}`;
  const areaD = `${pathD} L ${points[points.length - 1].x} ${height - padding} L ${points[0].x} ${height - padding} Z`;

  return (
    <div style={{ width: '100%', overflow: 'hidden' }}>
      <svg viewBox={`0 0 ${width} ${height}`} width="100%" height={height} style={{ overflow: 'visible' }}>
        {[0, 25, 50, 75, 100].map((level) => {
          const y = height - padding - (level * (height - 2 * padding)) / 100;
          return (
            <g key={level}>
              <line x1={padding} y1={y} x2={width - padding} y2={y} stroke="rgba(0,0,0,0.06)" strokeDasharray="3,3" />
              <text x={padding - 6} y={y + 3} textAnchor="end" fontSize="9" fill="#94a3b8" fontWeight="600">{level}</text>
            </g>
          );
        })}

        {points.map((p, idx) => (
          <text key={idx} x={p.x} y={height - 8} textAnchor="middle" fontSize="9" fill="#94a3b8" fontWeight="600">
            Run #{chartData.length - idx}
          </text>
        ))}

        <path d={areaD} fill="url(#chartGrad)" opacity="0.15" />
        <path d={pathD} fill="none" stroke="#2563eb" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />

        {points.map((p, idx) => (
          <g key={idx}>
            <circle cx={p.x} cy={p.y} r="5" fill="#fff" stroke="#2563eb" strokeWidth="3" />
            <circle cx={p.x} cy={p.y} r="2" fill="#2563eb" />
            <text x={p.x} y={p.y - 10} textAnchor="middle" fontSize="9" fontWeight="700" fill="#0f172a">
              {p.score.toFixed(0)}
            </text>
          </g>
        ))}

        <defs>
          <linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#2563eb" />
            <stop offset="100%" stopColor="#2563eb" stopOpacity="0" />
          </linearGradient>
        </defs>
      </svg>
    </div>
  );
}

export default function Dashboard() {
  const { user, authHeaders } = useAuth();
  const [stats, setStats] = useState({
    totalSubmissions: '0',
    topScore: '0.0',
    topGrade: '—',
    registeredContests: '0',
    upcomingContestsCount: '0',
  });
  const [topEntries, setTopEntries] = useState([]);
  const [myHistory, setMyHistory] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Fetch global leaderboard for stats
    fetch('/api/leaderboard?limit=100')
      .then(res => res.ok ? res.json() : Promise.reject())
      .then(data => {
        const lb = data.leaderboard || [];
        setTopEntries(lb.slice(0, 5));
        
        let bestScore = 0.0;
        let bestGrade = '—';
        if (lb.length > 0) {
          bestScore = Math.max(...lb.map(x => x.total_score));
          const topEntry = lb.find(x => x.total_score === bestScore);
          if (topEntry) {
            bestGrade = topEntry.grade;
          }
        }

        setStats(prev => ({
          ...prev,
          totalSubmissions: String(lb.length),
          topScore: bestScore.toFixed(1),
          topGrade: bestGrade,
        }));
      })
      .catch(() => {})
      .finally(() => setLoading(false));

    // Fetch user's own history
    fetch('/api/history/me?limit=10', { headers: authHeaders() })
      .then(res => res.ok ? res.json() : Promise.reject())
      .then(data => setMyHistory(data.history || []))
      .catch(() => {});

    // Fetch contests
    fetch('/api/contests')
      .then(res => res.ok ? res.json() : Promise.reject())
      .then(data => {
        const contests = data.contests || [];
        const upcoming = contests.filter(c => new Date(c.startTime) > new Date());
        
        let regCount = 0;
        try {
          const reg = localStorage.getItem('registered_contests');
          if (reg) {
            const parsed = JSON.parse(reg);
            regCount = contests.filter(c => parsed.includes(c.id) && new Date(c.startTime) > new Date()).length;
          }
        } catch(e) {}

        setStats(prev => ({
          ...prev,
          upcomingContestsCount: String(upcoming.length),
          registeredContests: String(regCount),
        }));
      })
      .catch(() => {});
  }, [authHeaders]);

  // Compute personal best from history
  const myBestScore = myHistory.length > 0
    ? Math.max(...myHistory.map(h => h.total_score)).toFixed(1)
    : '—';
  const myBestGrade = myHistory.length > 0
    ? myHistory.reduce((best, h) => h.total_score > best.total_score ? h : best, myHistory[0]).grade
    : '—';

  return (
    <div className="dashboard-grid">
      {/* Welcome Banner */}
      <article className="hero-card panel" style={{ gridColumn: 'span 2' }}>
        <div className="hero-copy">
          <span className="section-tag">Welcome back</span>
          <h2>Hello, {user?.username || 'Competitor'} 👋</h2>
          <p>
            Choose your path — practice your trading engine in isolation with deterministic seeds, 
            or jump into a live contest and compete on the leaderboard.
          </p>

          <div className="hero-actions">
            <Link className="button button-primary" to="/submit">
              ⚡ Practice Mode
            </Link>
            <Link className="button button-secondary" to="/contests">
              🏆 Contests
            </Link>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', justifyContent: 'center' }}>
          <div className="status-card">
            <span className="status-pill success">Logged in</span>
            <div className="status-metric">
              <strong>{user?.username}</strong>
              <span>{user?.email}</span>
            </div>
          </div>
        </div>
      </article>

      {/* Two Pathway Cards */}
      <div className="db-pathway-grid" style={{ gridColumn: 'span 2' }}>
        <Link to="/submit" className="db-pathway-card db-pathway-practice">
          <div className="db-pathway-icon">⚡</div>
          <div className="db-pathway-content">
            <h3>Practice Mode</h3>
            <p>Submit your trading engine, run stress tests with deterministic seeds, and view your score history. Same code, same input, same output every time.</p>
            <ul className="db-pathway-features">
              <li>🔧 Upload &amp; run code in sandbox</li>
              <li>📊 View scores, latency, TPS history</li>
              <li>🎯 Deterministic &amp; reproducible results</li>
            </ul>
          </div>
          <span className="db-pathway-arrow">→</span>
        </Link>

        <Link to="/contests" className="db-pathway-card db-pathway-contest">
          <div className="db-pathway-icon">🏆</div>
          <div className="db-pathway-content">
            <h3>Contest Mode</h3>
            <p>Join live competitions, submit engines under contest conditions, and compete for rankings on the real-time leaderboard.</p>
            <ul className="db-pathway-features">
              <li>📅 Browse &amp; register for contests</li>
              <li>🔴 Submit during live contests</li>
              <li>🥇 Contest-specific leaderboards</li>
            </ul>
          </div>
          <span className="db-pathway-arrow">→</span>
        </Link>
      </div>

      {/* Your Stats */}
      <div className="db-stats-panel">
        <div className="db-metric-grid">
          <div className="db-metric-item">
            <span className="db-metric-lbl">Your Submissions</span>
            <strong className="db-metric-val">{myHistory.length}</strong>
          </div>
          <div className="db-metric-item">
            <span className="db-metric-lbl">Your Best Score</span>
            <strong className="db-metric-val">{myBestScore}</strong>
          </div>
          <div className="db-metric-item">
            <span className="db-metric-lbl">Your Best Grade</span>
            <strong className="db-metric-val" style={{ color: '#10b981' }}>{myBestGrade}</strong>
          </div>
          <div className="db-metric-item">
            <span className="db-metric-lbl">Schedule</span>
            <strong className="db-metric-val" style={{ color: '#2563eb' }}>
              {stats.registeredContests} <span style={{ fontSize: '0.9rem', color: '#64748b', fontWeight: '500' }}>/ {stats.upcomingContestsCount} active</span>
            </strong>
          </div>
        </div>
      </div>

      {/* Performance Chart */}
      <section className="panel db-chart-card">
        <span className="section-tag">Your Performance</span>
        <h3 style={{ margin: '4px 0 16px 0', fontSize: '1.2rem', fontWeight: '600' }}>Score Trend</h3>
        <SubmissionChart entries={myHistory} />
      </section>

      {/* Global Leaderboard Preview */}
      <section className="panel" style={{ borderRadius: '1.5rem', padding: '1.5rem' }}>
        <span className="section-tag">Global Rankings</span>
        <h3 style={{ margin: '4px 0 16px 0', fontSize: '1.2rem', fontWeight: '600' }}>Top Performers</h3>
        {topEntries.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '24px', color: '#64748b', fontSize: '0.9rem' }}>
            No submissions yet — be the first!
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {topEntries.slice(0, 5).map((entry, idx) => (
              <div key={entry.submission_id} style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                padding: '10px 12px',
                borderRadius: '10px',
                background: idx === 0 ? 'rgba(34, 197, 94, 0.06)' : 'rgba(0,0,0,0.02)',
                border: '1px solid rgba(0,0,0,0.04)',
              }}>
                <span style={{
                  width: '28px', height: '28px', borderRadius: '50%',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: '0.8rem', fontWeight: '700',
                  background: idx === 0 ? '#f59e0b' : idx === 1 ? '#94a3b8' : idx === 2 ? '#cd7f32' : 'rgba(0,0,0,0.06)',
                  color: idx < 3 ? '#fff' : '#64748b',
                }}>
                  {idx + 1}
                </span>
                <div style={{ flex: 1 }}>
                  <span style={{ fontWeight: '600', fontSize: '0.9rem', color: '#0f172a' }}>
                    {entry.system_name || 'Unknown'}
                  </span>
                </div>
                <span style={{
                  fontWeight: '700', fontSize: '0.85rem',
                  color: entry.grade === 'S' || entry.grade === 'A' ? '#10b981' : entry.grade === 'F' ? '#ef4444' : '#f59e0b',
                }}>
                  {entry.grade}
                </span>
                <span style={{ fontSize: '0.85rem', color: '#64748b', fontFamily: 'var(--mono)' }}>
                  {entry.total_score?.toFixed(1)}
                </span>
              </div>
            ))}
          </div>
        )}
        <Link to="/leaderboard" style={{ display: 'block', textAlign: 'center', marginTop: '16px', fontSize: '0.88rem', color: '#2563eb', textDecoration: 'none', fontWeight: '600' }}>
          View full leaderboard →
        </Link>
      </section>

      {/* Recent Activity */}
      <section className="panel activity-panel" style={{ gridColumn: 'span 2' }}>
        <div className="section-header" style={{ marginBottom: '16px' }}>
          <div>
            <span className="section-tag">Your History</span>
            <h3>Recent Submissions</h3>
          </div>
          <Link to="/submit" className="subtle-chip" style={{ textDecoration: 'none' }}>
            New submission →
          </Link>
        </div>

        {loading ? (
          <div style={{ textAlign: 'center', padding: '32px', color: '#64748b' }}>
            Loading your submissions...
          </div>
        ) : myHistory.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '48px', color: '#64748b' }}>
            <span style={{ fontSize: '2rem', display: 'block', marginBottom: '8px' }}>🤖</span>
            <p style={{ margin: 0, fontWeight: '500' }}>No submissions recorded yet.</p>
            <p style={{ margin: '4px 0 16px 0', fontSize: '0.88rem', color: '#94a3b8' }}>Deploy a trading engine to see your execution logs here.</p>
            <Link className="button button-primary" to="/submit" style={{ padding: '0.6rem 1.2rem', fontSize: '0.9rem' }}>Deploy now</Link>
          </div>
        ) : (
          <div style={{ borderRadius: '12px', border: '1px solid rgba(148, 163, 184, 0.15)', overflow: 'hidden' }}>
            <div className="db-table-header">
              <span>Grade</span>
              <span>Strategy</span>
              <span>Score</span>
              <span>TPS</span>
              <span>p99 Latency</span>
              <span>Submitted</span>
            </div>
            {myHistory.slice(0, 10).map((entry) => (
              <div key={entry.submission_id} className="db-table-row">
                <div>
                  <span style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: '32px',
                    height: '32px',
                    borderRadius: '8px',
                    fontWeight: '700',
                    fontSize: '1rem',
                    background: entry.grade === 'S' || entry.grade === 'A' ? 'rgba(34, 197, 94, 0.12)' : entry.grade === 'F' ? 'rgba(239, 68, 68, 0.12)' : 'rgba(245, 158, 11, 0.12)',
                    color: entry.grade === 'S' || entry.grade === 'A' ? '#10b981' : entry.grade === 'F' ? '#ef4444' : '#f59e0b'
                  }}>
                    {entry.grade}
                  </span>
                </div>
                <div style={{ fontSize: '0.88rem', color: '#0f172a', fontWeight: '500' }}>
                  {entry.strategy}
                </div>
                <div style={{ fontFamily: 'var(--mono)', fontSize: '0.88rem', fontWeight: '600' }}>
                  {entry.total_score?.toFixed(1)}
                </div>
                <div style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem', color: '#64748b' }}>
                  {entry.tps?.toFixed(0)}
                </div>
                <div style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem', color: '#64748b' }}>
                  {entry.p99_latency_ms?.toFixed(1)}ms
                </div>
                <div style={{ fontSize: '0.88rem', color: '#64748b' }}>
                  {formatRelativeTime(entry.submitted_at)}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}