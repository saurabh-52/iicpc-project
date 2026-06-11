import { useState, useEffect } from 'react';
import { useParams, Navigate, Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

const gradeColors = {
  S: { bg: 'linear-gradient(135deg, #fbbf24, #f59e0b)', color: '#78350f', glow: 'rgba(251,191,36,0.3)' },
  A: { bg: 'linear-gradient(135deg, #34d399, #10b981)', color: '#064e3b', glow: 'rgba(52,211,153,0.3)' },
  B: { bg: 'linear-gradient(135deg, #60a5fa, #3b82f6)', color: '#1e3a5f', glow: 'rgba(96,165,250,0.3)' },
  C: { bg: 'linear-gradient(135deg, #a78bfa, #8b5cf6)', color: '#2e1065', glow: 'rgba(167,139,250,0.3)' },
  F: { bg: 'linear-gradient(135deg, #f87171, #ef4444)', color: '#7f1d1d', glow: 'rgba(248,113,113,0.3)' },
};

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

function ScoreBar({ label, value, max, color }) {
  const pct = Math.min((value / max) * 100, 100);
  return (
    <div className="score-bar-container">
      <div className="score-bar-label">
        <span>{label}</span>
        <span>{value?.toFixed(1)}/{max}</span>
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
            <h3>{submission.submission_id?.slice(0, 18)}</h3>
            <span className="detail-strategy-tag">{submission.strategy}</span>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close">✕</button>
        </div>

        <div className="detail-scores">
          <div className="detail-total">
            <GradeBadge grade={submission.grade} />
            <div className="detail-total-num">
              <strong>{submission.total_score?.toFixed(1)}</strong>
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
            <strong>{submission.strategy}</strong>
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

export default function ProfilePage() {
  const { username } = useParams();
  const { user } = useAuth();
  
  const [profile, setProfile] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [selectedSubmission, setSelectedSubmission] = useState(null);

  useEffect(() => {
    setLoading(true);
    fetch(`/api/users/${username}/profile?page=${page}&pageSize=${pageSize}`)
      .then(res => {
        if (!res.ok) throw new Error('User not found or API error');
        return res.json();
      })
      .then(data => {
        setProfile(data);
        setLoading(false);
      })
      .catch(err => {
        setError(err.message);
        setLoading(false);
      });
  }, [username, page, pageSize]);

  if (loading) {
    return (
      <div className="leaderboard-container">
        <div className="leaderboard-loading">
          <div className="spinner" />
          <p>Loading profile...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="leaderboard-container">
        <div className="feedback error panel">
          Failed to load profile: {error}
        </div>
      </div>
    );
  }

  if (!profile) return null;

  const { best_score: bestScore, history, total } = profile;
  const isMe = user && user.username === username;
  const totalPages = Math.ceil(total / pageSize) || 1;

  return (
    <div className="dashboard-container">
      <div className="dashboard-header panel" style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
        <div className="nav-user-avatar" style={{ width: '64px', height: '64px', fontSize: '2rem' }}>
          {profile.user.username.charAt(0).toUpperCase()}
        </div>
        <div>
          <span className="section-tag">User Profile</span>
          <h2>{profile.user.username}</h2>
        </div>
      </div>

      <div className="dashboard-grid">
        <div className="dashboard-main">
          {bestScore ? (
            <article className="score-card panel highlight-panel">
              <div className="score-header">
                <h3>Personal Best Score</h3>
                <span className="timestamp">{new Date(bestScore.submitted_at).toLocaleString()}</span>
              </div>
              <div className="score-hero">
                <GradeBadge grade={bestScore.grade} />
                <div className="score-big">
                  {bestScore.total_score.toFixed(1)} <span className="score-max">/ 100</span>
                </div>
              </div>
              <div className="metrics-grid">
                <div className="metric">
                  <span className="metric-label">Strategy</span>
                  <span className="metric-val">{bestScore.strategy}</span>
                </div>
                <div className="metric">
                  <span className="metric-label">P99 Latency</span>
                  <span className="metric-val">{formatLatency(bestScore.p99_latency_ms)}</span>
                </div>
                <div className="metric">
                  <span className="metric-label">Throughput</span>
                  <span className="metric-val">{formatTPS(bestScore.tps)} TPS</span>
                </div>
                <div className="metric">
                  <span className="metric-label">Correctness</span>
                  <span className="metric-val">{(bestScore.correctness_score / 20 * 100).toFixed(0)}%</span>
                </div>
              </div>
            </article>
          ) : (
             <article className="score-card panel">
              <div className="score-header">
                <h3>Personal Best Score</h3>
              </div>
              <div className="score-hero" style={{ opacity: 0.5 }}>
                <p>No practice runs recorded yet.</p>
              </div>
            </article>
          )}

          <div className="leaderboard-table-wrap panel" style={{ marginTop: '1.5rem' }}>
            <h3 style={{ marginBottom: '1rem' }}>Submission History</h3>
            {history.length === 0 ? (
              <p style={{ opacity: 0.6 }}>No practice history found.</p>
            ) : (
              <>
                <table className="leaderboard-table">
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Strategy</th>
                      <th>Grade</th>
                      <th>Score</th>
                      <th>P99</th>
                      <th>TPS</th>
                    </tr>
                  </thead>
                  <tbody>
                    {history.map(entry => (
                      <tr 
                        key={entry.submission_id} 
                        className="lb-row" 
                        onClick={() => setSelectedSubmission(entry)}
                      >
                        <td>{new Date(entry.submitted_at).toLocaleString()}</td>
                        <td>{entry.strategy}</td>
                        <td><GradeBadge grade={entry.grade} /></td>
                        <td className="score-cell">{entry.total_score?.toFixed(1)}</td>
                        <td>{formatLatency(entry.p99_latency_ms)}</td>
                        <td>{formatTPS(entry.tps)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <div className="pagination" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '1rem', padding: '0.5rem' }}>
                  <button 
                    disabled={page === 1} 
                    onClick={() => setPage(p => p - 1)}
                    style={{ padding: '0.5rem 1rem', borderRadius: '4px', background: 'var(--border)', border: 'none', cursor: page === 1 ? 'not-allowed' : 'pointer' }}
                  >
                    Previous
                  </button>
                  <span>Page {page} of {totalPages}</span>
                  <button 
                    disabled={page === totalPages} 
                    onClick={() => setPage(p => p + 1)}
                    style={{ padding: '0.5rem 1rem', borderRadius: '4px', background: 'var(--border)', border: 'none', cursor: page === totalPages ? 'not-allowed' : 'pointer' }}
                  >
                    Next
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      <SubmissionDetail submission={selectedSubmission} onClose={() => setSelectedSubmission(null)} />
    </div>
  );
}
