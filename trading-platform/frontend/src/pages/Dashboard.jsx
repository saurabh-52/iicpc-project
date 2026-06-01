import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';

export default function Dashboard() {
  const [stats, setStats] = useState({
    activeEngines: '—',
    totalSubmissions: '—',
    topScore: '—',
    topGrade: '—',
  });
  const [topEntries, setTopEntries] = useState([]);

  useEffect(() => {
    fetch('/api/leaderboard?limit=5')
      .then(res => res.ok ? res.json() : Promise.reject())
      .then(data => {
        const lb = data.leaderboard || [];
        setTopEntries(lb);
        if (lb.length > 0) {
          setStats({
            activeEngines: String(lb.length),
            totalSubmissions: String(lb.length),
            topScore: lb[0].total_score.toFixed(1),
            topGrade: lb[0].grade,
          });
        }
      })
      .catch(() => {
        // Backend unavailable — keep defaults
      });
  }, []);

  const statCards = [
    { label: 'Submissions', value: stats.totalSubmissions },
    { label: 'Top score', value: stats.topScore },
    { label: 'Best grade', value: stats.topGrade },
    { label: 'Sandbox status', value: 'Ready' },
  ];

  return (
    <section className="dashboard-grid">
      <article className="hero-card panel">
        <div className="hero-copy">
          <span className="section-tag">Dashboard</span>
          <h2>Track the state of your trading systems in one place.</h2>
          <p>
            Review submission capacity, scan the latest run status, and jump directly to the
            upload flow when you are ready to test a new engine.
          </p>

          <div className="hero-actions">
            <Link className="button button-primary" to="/submit">
              Submit an engine
            </Link>
            <Link className="button button-secondary" to="/leaderboard">
              View leaderboard
            </Link>
          </div>
        </div>

        <div className="status-card">
          <span className="status-pill success">System healthy</span>
          <div className="status-metric">
            <strong>Sandbox</strong>
            <span>Ready for compile and run</span>
          </div>
        </div>
      </article>

      <section className="stats-grid" id="overview" aria-label="System overview metrics">
        {statCards.map((stat) => (
          <article key={stat.label} className="stat-card panel">
            <span>{stat.label}</span>
            <strong>{stat.value}</strong>
          </article>
        ))}
      </section>

      <section className="panel activity-panel">
        <div className="section-header">
          <div>
            <span className="section-tag">Recent activity</span>
            <h3>Latest submissions</h3>
          </div>
          <Link to="/leaderboard" className="subtle-chip" style={{ textDecoration: 'none' }}>
            View all →
          </Link>
        </div>

        <div className="timeline-list">
          {topEntries.length === 0 ? (
            <div className="timeline-item">
              <span className="timeline-dot" />
              <p>No submissions yet. Submit a trading engine to get started.</p>
            </div>
          ) : (
            topEntries.map((entry) => (
              <div key={entry.submission_id} className="timeline-item">
                <span className="timeline-dot" />
                <p>
                  <strong>{entry.grade}</strong> — Score {entry.total_score.toFixed(1)} —{' '}
                  <span style={{ fontFamily: 'var(--mono)', fontSize: '0.85em', color: '#64748b' }}>
                    {entry.submission_id.slice(0, 24)}
                  </span>
                </p>
              </div>
            ))
          )}
        </div>
      </section>
    </section>
  );
}