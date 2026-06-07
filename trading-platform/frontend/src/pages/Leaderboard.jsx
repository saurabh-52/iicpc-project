import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import useWebSocket from '../hooks/useWebSocket';
import { useAuth } from '../context/AuthContext';

const gradeColors = {
  S: { bg: 'linear-gradient(135deg, #fbbf24, #f59e0b)', color: '#78350f', glow: 'rgba(251,191,36,0.3)' },
  A: { bg: 'linear-gradient(135deg, #34d399, #10b981)', color: '#064e3b', glow: 'rgba(52,211,153,0.3)' },
  B: { bg: 'linear-gradient(135deg, #60a5fa, #3b82f6)', color: '#1e3a5f', glow: 'rgba(96,165,250,0.3)' },
  C: { bg: 'linear-gradient(135deg, #a78bfa, #8b5cf6)', color: '#2e1065', glow: 'rgba(167,139,250,0.3)' },
  F: { bg: 'linear-gradient(135deg, #f87171, #ef4444)', color: '#7f1d1d', glow: 'rgba(248,113,113,0.3)' },
};

const strategyTabs = [
  { value: 'bbo_heavy', label: 'Common' },
  { value: 'flash_crash', label: 'Flash Crash' },
  { value: 'high_cancel', label: 'High Cancel' },
  { value: 'wide_spread', label: 'Wide Spread' },
  { value: 'market_maker', label: 'Market Maker' },
  { value: 'iceberg', label: 'Iceberg' },
  { value: 'momentum_burst', label: 'Momentum Burst' },
];

function displayName(entry) {
  if (entry.system_name && entry.system_name.trim()) return entry.system_name;
  return entry.submission_id?.slice(0, 18) || '—';
}

function GradeBadge({ grade }) {
  const style = gradeColors[grade] || gradeColors.F;
  return (
    <span
      className="grade-badge"
      style={{
        background: style.bg,
        color: style.color,
        boxShadow: `0 4px 16px ${style.glow}`,
      }}
    >
      {grade}
    </span>
  );
}

function formatLatency(ms) {
  if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`;
  if (ms < 100) return `${ms.toFixed(2)}ms`;
  return `${ms.toFixed(0)}ms`;
}

function formatTPS(tps) {
  if (tps >= 1000) return `${(tps / 1000).toFixed(1)}K`;
  return tps.toFixed(0);
}

function strategyLabel(strategy) {
  const found = strategyTabs.find(t => t.value === strategy);
  return found ? found.label : strategy || '—';
}

function ScoreBar({ label, value, max, color }) {
  const pct = Math.min((value / max) * 100, 100);
  return (
    <div className="score-bar-container">
      <div className="score-bar-label">
        <span>{label}</span>
        <span>{value.toFixed(1)}/{max}</span>
      </div>
      <div className="score-bar-track">
        <div
          className="score-bar-fill"
          style={{ width: `${pct}%`, background: color }}
        />
      </div>
    </div>
  );
}

function SubmissionDetail({ submission, onClose }) {
  if (!submission) return null;
  return (
    <div className="detail-overlay" onClick={onClose}>
      <article className="detail-card panel" onClick={e => e.stopPropagation()}>
        <div className="detail-header">
          <div>
            <span className="section-tag">Submission Detail</span>
            <h3>{displayName(submission)}</h3>
            <span className="detail-strategy-tag">{strategyLabel(submission.strategy)}</span>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close">✕</button>
        </div>

        <div className="detail-scores">
          <div className="detail-total">
            <GradeBadge grade={submission.grade} />
            <div className="detail-total-num">
              <strong>{submission.total_score.toFixed(1)}</strong>
              <span>/100</span>
            </div>
          </div>

          <div className="detail-breakdown">
            <ScoreBar label="Latency" value={submission.latency_score} max={50} color="#3b82f6" />
            <ScoreBar label="Throughput" value={submission.throughput_score} max={30} color="#10b981" />
            <ScoreBar label="Correctness" value={submission.correctness_score} max={20} color="#f59e0b" />
          </div>
        </div>

        <div className="detail-metrics">
          <div className="metric-cell">
            <span>P99 Latency</span>
            <strong>{formatLatency(submission.p99_latency_ms)}</strong>
          </div>
          <div className="metric-cell">
            <span>TPS</span>
            <strong>{formatTPS(submission.tps)}</strong>
          </div>
          <div className="metric-cell">
            <span>Orders</span>
            <strong>{(submission.orders_processed || 0).toLocaleString()}</strong>
          </div>
          <div className="metric-cell">
            <span>Crosses</span>
            <strong className={submission.cross_events > 0 ? 'error-text' : ''}>
              {submission.cross_events || 0}
            </strong>
          </div>
          <div className="metric-cell">
            <span>Strategy</span>
            <strong>{strategyLabel(submission.strategy)}</strong>
          </div>
          <div className="metric-cell">
            <span>Submitted</span>
            <strong>{new Date(submission.submitted_at).toLocaleString()}</strong>
          </div>
        </div>
      </article>
    </div>
  );
}

export default function Leaderboard() {
  const { user } = useAuth();
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selected, setSelected] = useState(null);
  const [activeStrategy, setActiveStrategy] = useState('bbo_heavy');
  const [modeFilter, setModeFilter] = useState('all'); // 'all' | 'practice' | 'contest_live' | 'contest_final'
  const { connected, updateTrigger } = useWebSocket();

  const location = useLocation();
  const queryParams = new URLSearchParams(location.search);
  const contestId = queryParams.get('contest_id');
  const isContestMode = !!contestId;

  // Fetch leaderboard with optional strategy filter
  const fetchLeaderboard = (strategy) => {
    const params = new URLSearchParams({ limit: '50' });
    if (strategy && !isContestMode) params.set('strategy', strategy);

    const endpoint = isContestMode 
      ? `/api/contests/${contestId}/leaderboard?${params.toString()}`
      : `/api/leaderboard?${params.toString()}`;

    fetch(endpoint)
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then(data => {
        setEntries(data.leaderboard || []);
        if (isContestMode && data.type) {
          setModeFilter(data.type === 'final' ? 'contest_final' : 'contest_live');
        }
        setLoading(false);
      })
      .catch(err => {
        setError(err.message);
        setLoading(false);
      });
  };

  // Fetch on load, on strategy change, or on live websocket update
  useEffect(() => {
    fetchLeaderboard(activeStrategy);
  }, [activeStrategy, updateTrigger, contestId]);

  const displayEntries = modeFilter === 'all'
    ? entries
    : entries.filter(e => (e.judging_mode || 'practice') === modeFilter);

  const handleStrategyChange = (strategy) => {
    setActiveStrategy(strategy);
    setLoading(true);
    setError(null);
  };

  const getRankEmoji = (index) => {
    if (index === 0) return '🥇';
    if (index === 1) return '🥈';
    if (index === 2) return '🥉';
    return `#${index + 1}`;
  };

  if (loading) {
    return (
      <section className="leaderboard-container">
        <div className="leaderboard-loading">
          <div className="spinner" />
          <p>Loading leaderboard...</p>
        </div>
      </section>
    );
  }

  return (
    <section className="leaderboard-container">
      {/* Header */}
      <div className="leaderboard-header panel">
        <div className="leaderboard-header-copy">
          <span className="section-tag">Leaderboard</span>
          <h2>{isContestMode ? `Contest Leaderboard: ${contestId}` : 'Trading Engine Rankings'}</h2>
          <p>
            {isContestMode 
              ? 'Real-time scores for this specific contest. Ranks are determined by the contest rules.'
              : 'Real-time scores from benchmarked submissions. Engines are ranked by composite score across latency, throughput, and correctness.'}
          </p>
        </div>
        <div className="leaderboard-status">
          <span className={`status-pill ${connected ? 'success' : 'offline'}`}>
            {connected ? '● Live' : '○ Offline'}
          </span>
          <span className="entry-count">{displayEntries.length} submissions</span>
        </div>
      </div>

      {!isContestMode && (
        <>
          <div className="strategy-tabs panel" style={{ marginBottom: '0.5rem' }}>
            {[
              { value: 'all', label: '📊 All' },
              { value: 'practice', label: '⚡ Practice' },
              { value: 'contest_live', label: '🔴 Contest Live' },
              { value: 'contest_final', label: '🏁 Contest Final' },
            ].map(tab => (
              <button
                key={tab.value}
                className={`strategy-tab ${modeFilter === tab.value ? 'active' : ''}`}
                onClick={() => setModeFilter(tab.value)}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="strategy-tabs panel">
            {strategyTabs.map(tab => (
              <button
                key={tab.value}
                className={`strategy-tab ${activeStrategy === tab.value ? 'active' : ''}`}
                onClick={() => handleStrategyChange(tab.value)}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </>
      )}

      <details className="scoring-info panel">
        <summary>How is the Total Score calculated?</summary>
        <div className="scoring-details">
          <div className="math-formula">
            <span>S<sub>Total</sub></span> = S<sub>L</sub> + S<sub>T</sub> + S<sub>C</sub>
          </div>
          <div className="math-formula">
            <span>S<sub>L</sub> (Latency)</span> = 50 × max(0, min(1, (100 - P99) / 95))
          </div>
          <div className="math-formula">
            <span>S<sub>T</sub> (Throughput)</span> = 30 × max(0, min(1, (TPS - 100) / 4900))
          </div>
          <div className="math-formula">
            <span>S<sub>C</sub> (Correctness)</span> = 20 × max(0, 1 - (Crosses / Orders))
          </div>
        </div>
      </details>

      {error && (
        <div className="feedback error">
          Backend unavailable: {error}. Showing cached data if available.
        </div>
      )}

      {displayEntries.length === 0 && !error ? (
        <div className="leaderboard-empty panel">
          <div className="empty-icon">🏁</div>
          <h3>No submissions yet</h3>
          <p>
            {activeStrategy
              ? `No submissions found for ${strategyLabel(activeStrategy)}. Try another strategy or submit an engine.`
              : 'Submit a trading engine to see it ranked here.'}
          </p>
        </div>
      ) : (
        <>
          {/* Top 3 podium */}
          {displayEntries.length >= 3 && (
            <div className="podium-grid">
              {displayEntries.slice(0, 3).map((entry, i) => (
                <article
                  key={entry.submission_id}
                  className={`podium-card panel podium-${i + 1}`}
                  onClick={() => setSelected(entry)}
                >
                  <div className="podium-rank">{getRankEmoji(i)}</div>
                  <GradeBadge grade={entry.grade} />
                  <strong className="podium-score">{entry.total_score.toFixed(1)}</strong>
                  <div className="podium-meta">
                    <span>P99 {formatLatency(entry.p99_latency_ms)}</span>
                    <span>{formatTPS(entry.tps)} TPS</span>
                  </div>
                  <span className="podium-id">{displayName(entry)}</span>
                  {entry.strategy && (
                    <span className="podium-strategy">{strategyLabel(entry.strategy)}</span>
                  )}
                </article>
              ))}
            </div>
          )}

          {/* Full table */}
          <div className="leaderboard-table-wrap panel">
            <table className="leaderboard-table" id="leaderboard-table">
              <thead>
                <tr>
                  <th>Rank</th>
                  <th>Engine</th>
                  <th>Mode</th>
                  <th>Strategy</th>
                  <th>Grade</th>
                  <th>Score</th>
                  <th>Latency</th>
                  <th>Throughput</th>
                  <th>Correctness</th>
                  <th>P99</th>
                  <th>TPS</th>
                </tr>
              </thead>
              <tbody>
                {displayEntries.map((entry, i) => {
                  const isMe = user && entry.system_name === user.username;
                  return (
                  <tr
                    key={entry.submission_id}
                    className={`lb-row ${i < 3 ? 'lb-row-top' : ''} ${isMe ? 'lb-row-me' : ''}`}
                    onClick={() => setSelected(entry)}
                  >
                    <td className="rank-cell">{getRankEmoji(i)}</td>
                    <td className="id-cell" title={entry.submission_id}>
                      {displayName(entry)}
                    </td>
                    <td>
                      <span style={{
                        fontSize: '0.7rem',
                        padding: '2px 6px',
                        borderRadius: '4px',
                        fontWeight: 600,
                        background: (entry.judging_mode || 'practice') === 'practice'
                          ? 'rgba(99, 102, 241, 0.1)'
                          : (entry.judging_mode || 'practice') === 'contest_final'
                          ? 'rgba(34, 197, 94, 0.1)'
                          : 'rgba(239, 68, 68, 0.1)',
                        color: (entry.judging_mode || 'practice') === 'practice'
                          ? '#6366f1'
                          : (entry.judging_mode || 'practice') === 'contest_final'
                          ? '#22c55e'
                          : '#ef4444',
                      }}>
                        {(entry.judging_mode || 'practice') === 'practice' ? '⚡' : (entry.judging_mode || 'practice') === 'contest_final' ? '🏁' : '🔴'}
                      </span>
                    </td>
                    <td className="strategy-cell">{strategyLabel(entry.strategy)}</td>
                    <td><GradeBadge grade={entry.grade} /></td>
                    <td className="score-cell">{entry.total_score.toFixed(1)}</td>
                    <td>{entry.latency_score.toFixed(1)}<span className="dim">/50</span></td>
                    <td>{entry.throughput_score.toFixed(1)}<span className="dim">/30</span></td>
                    <td>{entry.correctness_score.toFixed(1)}<span className="dim">/20</span></td>
                    <td>{formatLatency(entry.p99_latency_ms)}</td>
                    <td>{formatTPS(entry.tps)}</td>
                  </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}

      <SubmissionDetail submission={selected} onClose={() => setSelected(null)} />
    </section>
  );
}
