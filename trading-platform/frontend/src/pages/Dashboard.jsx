import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';

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

function K8sHealthWidget() {
  const [metrics, setMetrics] = useState({
    cpu: 18,
    memory: 42,
    disk: 23,
    latency: 14
  });

  useEffect(() => {
    const interval = setInterval(() => {
      setMetrics({
        cpu: Math.floor(15 + Math.random() * 12),
        memory: Math.floor(40 + Math.random() * 3),
        disk: 23,
        latency: Math.floor(10 + Math.random() * 8)
      });
    }, 3000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="db-node-widget">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
        <h4 style={{ margin: 0, fontWeight: '600', color: '#fff', fontSize: '0.92rem', letterSpacing: '-0.01em' }}>💻 Sandbox Node Health</h4>
        <span className="status-pill success" style={{ padding: '0.2rem 0.5rem', fontSize: '0.72rem' }}>Active</span>
      </div>

      <div className="db-node-gauge">
        <span>CPU Allocation</span>
        <div className="db-node-bar-bg">
          <div className="db-node-bar-fill" style={{ width: `${metrics.cpu}%`, background: '#3b82f6' }} />
        </div>
        <strong>{metrics.cpu}%</strong>
      </div>

      <div className="db-node-gauge">
        <span>Memory Pressure</span>
        <div className="db-node-bar-bg">
          <div className="db-node-bar-fill" style={{ width: `${metrics.memory}%`, background: '#10b981' }} />
        </div>
        <strong>{metrics.memory}%</strong>
      </div>

      <div className="db-node-gauge">
        <span>Disk Storage</span>
        <div className="db-node-bar-bg">
          <div className="db-node-bar-fill" style={{ width: `${metrics.disk}%`, background: '#f59e0b' }} />
        </div>
        <strong>{metrics.disk}%</strong>
      </div>

      <div className="db-node-gauge">
        <span>Cluster Latency</span>
        <div className="db-node-bar-bg">
          <div className="db-node-bar-fill" style={{ width: `${metrics.latency * 5}%`, background: '#a855f7' }} />
        </div>
        <strong>{metrics.latency}ms</strong>
      </div>
    </div>
  );
}

export default function Dashboard() {
  const [stats, setStats] = useState({
    totalSubmissions: '0',
    topScore: '0.0',
    topGrade: '—',
    registeredContests: '0',
    upcomingContestsCount: '0',
  });
  const [topEntries, setTopEntries] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Fetch submissions
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
  }, []);

  return (
    <div className="dashboard-grid">
      {/* Welcome Banner */}
      <article className="hero-card panel" style={{ gridColumn: 'span 2' }}>
        <div className="hero-copy">
          <span className="section-tag">System Terminal</span>
          <h2>Operational control for matching engines</h2>
          <p>
            Review compilation health, test throughput parameters, and challenge your trading models on our standalone sandboxes or schedule stress runs in the upcoming qualifier contests.
          </p>

          <div className="hero-actions">
            <Link className="button button-primary" to="/submit">
              🚀 Submit an engine
            </Link>
            <Link className="button button-secondary" to="/contests">
              📅 Open contests
            </Link>
            <Link className="button button-secondary" to="/leaderboard">
              📊 View leaderboard
            </Link>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', justifyContent: 'center' }}>
          <div className="status-card">
            <span className="status-pill success">System healthy</span>
            <div className="status-metric">
              <strong>Kubernetes Cluster</strong>
              <span>Ready for sandbox deployments</span>
            </div>
          </div>
        </div>
      </article>

      {/* Metrics Row */}
      <div className="db-stats-panel">
        <div className="db-metric-grid">
          <div className="db-metric-item">
            <span className="db-metric-lbl">Total Runs</span>
            <strong className="db-metric-val">{stats.totalSubmissions}</strong>
          </div>
          <div className="db-metric-item">
            <span className="db-metric-lbl">Top Score</span>
            <strong className="db-metric-val">{stats.topScore}</strong>
          </div>
          <div className="db-metric-item">
            <span className="db-metric-lbl">Best Grade</span>
            <strong className="db-metric-val" style={{ color: '#10b981' }}>{stats.topGrade}</strong>
          </div>
          <div className="db-metric-item">
            <span className="db-metric-lbl">Schedule</span>
            <strong className="db-metric-val" style={{ color: '#2563eb' }}>
              {stats.registeredContests} <span style={{ fontSize: '0.9rem', color: '#64748b', fontWeight: '500' }}>/ {stats.upcomingContestsCount} active</span>
            </strong>
          </div>
        </div>
      </div>

      {/* Left Column: Latency Chart */}
      <section className="panel db-chart-card">
        <span className="section-tag">Historical Performance</span>
        <h3 style={{ margin: '4px 0 16px 0', fontSize: '1.2rem', fontWeight: '600' }}>Matching Engine Latency Trends</h3>
        <SubmissionChart entries={topEntries} />
      </section>

      {/* Right Column: Node Gauges */}
      <section className="panel" style={{ borderRadius: '1.5rem', padding: '1.5rem' }}>
        <span className="section-tag">Cluster Metrics</span>
        <h3 style={{ margin: '4px 0 16px 0', fontSize: '1.2rem', fontWeight: '600' }}>Kubernetes Agent Status</h3>
        <K8sHealthWidget />
      </section>

      {/* Activity Log Table */}
      <section className="panel activity-panel" style={{ gridColumn: 'span 2' }}>
        <div className="section-header" style={{ marginBottom: '16px' }}>
          <div>
            <span className="section-tag">Activity Log</span>
            <h3>Latest Run Submissions</h3>
          </div>
          <Link to="/leaderboard" className="subtle-chip" style={{ textDecoration: 'none' }}>
            Open Leaderboard →
          </Link>
        </div>

        {loading ? (
          <div style={{ textAlign: 'center', padding: '32px', color: '#64748b' }}>
            Loading submissions feed...
          </div>
        ) : topEntries.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '48px', color: '#64748b' }}>
            <span style={{ fontSize: '2rem', display: 'block', marginBottom: '8px' }}>🤖</span>
            <p style={{ margin: 0, fontWeight: '500' }}>No submissions recorded yet.</p>
            <p style={{ margin: '4px 0 16px 0', fontSize: '0.88rem', color: '#94a3b8' }}>Deploy a trading engine to see execution logs here.</p>
            <Link className="button button-primary" to="/submit" style={{ padding: '0.6rem 1.2rem', fontSize: '0.9rem' }}>Deploy now</Link>
          </div>
        ) : (
          <div style={{ borderRadius: '12px', border: '1px solid rgba(148, 163, 184, 0.15)', overflow: 'hidden' }}>
            <div className="db-table-header">
              <span>Grade</span>
              <span>System / Team</span>
              <span>Submission ID</span>
              <span>Submitted</span>
            </div>
            {topEntries.map((entry) => (
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
                <div style={{ fontWeight: '600', color: '#0f172a', textAlign: 'left', display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ width: '8px', height: '8px', borderRadius: '50%', background: getLanguageColor(entry.language), display: 'inline-block' }} />
                  {entry.system_name}
                </div>
                <div style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem', color: '#64748b', textAlign: 'left' }}>
                  {entry.submission_id.slice(0, 24)}...
                </div>
                <div style={{ fontSize: '0.88rem', color: '#64748b', textAlign: 'left' }}>
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