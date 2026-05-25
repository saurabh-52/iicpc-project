import { Link } from 'react-router-dom';

const stats = [
  { label: 'Active engines', value: '12' },
  { label: 'Runs today', value: '84' },
  { label: 'Avg. latency', value: '1.8ms' },
  { label: 'Sandbox status', value: 'Ready' },
];

const timeline = [
  'Latest binary upload passed validation',
  'Stress test queue is clear for new submissions',
  'Sandbox workers are healthy and available',
];

export default function Dashboard() {
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
            <a className="button button-secondary" href="#overview">
              View overview
            </a>
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
        {stats.map((stat) => (
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
            <h3>Operational notes</h3>
          </div>
          <span className="subtle-chip">Live</span>
        </div>

        <div className="timeline-list">
          {timeline.map((item) => (
            <div key={item} className="timeline-item">
              <span className="timeline-dot" />
              <p>{item}</p>
            </div>
          ))}
        </div>
      </section>
    </section>
  );
}