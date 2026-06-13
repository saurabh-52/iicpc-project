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
        padding: '0.2rem 0.6rem',
        borderRadius: '0.5rem',
        fontWeight: '800',
        fontSize: '0.8rem',
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        minWidth: '1.6rem',
        height: '1.6rem',
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
            <ScoreBar label="Latency" value={submission.latency_score} max={25} color="#3b82f6" />
            <ScoreBar label="Throughput" value={submission.throughput_score} max={25} color="#10b981" />
            <ScoreBar label="Correctness" value={submission.correctness_score} max={50} color="#f59e0b" />
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
    <div className="dashboard-grid">
      {/* Left Column: Submission History */}
      <section className="panel" style={{ borderRadius: '1.5rem', padding: '1.75rem', gridColumn: 'span 1' }}>
        <div style={{ marginBottom: '1.5rem' }}>
          <span className="section-tag">Performance History</span>
          <h3 style={{ fontSize: '1.25rem', fontWeight: '700', color: 'var(--text-h)', margin: '0.25rem 0 0 0' }}>Submission Archive</h3>
        </div>

        {history.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '3rem 1rem', background: 'rgba(15, 23, 42, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(148, 163, 184, 0.25)' }}>
            <p style={{ color: '#64748b', margin: 0, fontSize: '0.95rem' }}>No practice history found for this user.</p>
          </div>
        ) : (
          <>
            <div className="leaderboard-table-wrap" style={{ border: '1px solid rgba(148, 163, 184, 0.15)', background: '#fff', borderRadius: '12px', overflow: 'hidden' }}>
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
                      style={{ cursor: 'pointer' }}
                      title="Click to view details"
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
            </div>

            <div className="pagination" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '1.25rem', padding: '0 0.5rem' }}>
              <button 
                disabled={page === 1} 
                onClick={() => setPage(p => p - 1)}
                style={{
                  padding: '0.5rem 1rem',
                  borderRadius: '999px',
                  background: page === 1 ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.8)',
                  border: '1px solid rgba(148,163,184,0.25)',
                  color: page === 1 ? '#94a3b8' : 'var(--text-h)',
                  cursor: page === 1 ? 'not-allowed' : 'pointer',
                  fontWeight: '600',
                  fontSize: '0.82rem',
                  transition: 'all 0.15s ease'
                }}
              >
                Previous
              </button>
              <span style={{ fontSize: '0.88rem', color: '#64748b', fontWeight: '500' }}>Page {page} of {totalPages}</span>
              <button 
                disabled={page === totalPages} 
                onClick={() => setPage(p => p + 1)}
                style={{
                  padding: '0.5rem 1rem',
                  borderRadius: '999px',
                  background: page === totalPages ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.8)',
                  border: '1px solid rgba(148,163,184,0.25)',
                  color: page === totalPages ? '#94a3b8' : 'var(--text-h)',
                  cursor: page === totalPages ? 'not-allowed' : 'pointer',
                  fontWeight: '600',
                  fontSize: '0.82rem',
                  transition: 'all 0.15s ease'
                }}
              >
                Next
              </button>
            </div>
          </>
        )}
      </section>

      {/* Right Column: User Profile Card & Best Score */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
        <article className="panel" style={{ borderRadius: '1.5rem', padding: '2rem', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '1rem', textAlign: 'center', background: 'rgba(255, 255, 255, 0.85)' }}>
          <div className="nav-user-avatar" style={{ width: '80px', height: '80px', fontSize: '2.5rem', borderRadius: '50%', boxShadow: '0 8px 20px rgba(99, 102, 241, 0.25)' }}>
            {profile.user.username.charAt(0).toUpperCase()}
          </div>
          <div>
            <h2 style={{ fontSize: '1.75rem', fontWeight: '800', margin: '0 0 0.25rem 0', color: 'var(--text-h)' }}>{profile.user.username}</h2>
            <p style={{ fontSize: '0.88rem', color: '#64748b', margin: 0 }}>{profile.user.email}</p>
            {isMe && (
              <span style={{ display: 'inline-block', marginTop: '0.5rem', padding: '0.25rem 0.75rem', borderRadius: '999px', background: 'rgba(37, 99, 235, 0.1)', color: '#2563eb', fontSize: '0.75rem', fontWeight: '700' }}>
                Your Profile
              </span>
            )}
          </div>
        </article>

        {bestScore ? (
          <article className="panel" style={{ borderRadius: '1.5rem', padding: '1.75rem', display: 'flex', flexDirection: 'column', gap: '1rem', background: 'linear-gradient(135deg, rgba(255,255,255,0.95), rgba(248,250,252,0.95))' }}>
            <span className="section-tag" style={{ color: '#2563eb', fontWeight: '800' }}>🏆 Personal Best</span>
            
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: '0.25rem' }}>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.25rem' }}>
                <strong style={{ fontSize: '3rem', fontWeight: '900', color: 'var(--text-h)', letterSpacing: '-0.04em', lineHeight: 1 }}>{bestScore.total_score.toFixed(1)}</strong>
                <span style={{ fontSize: '1rem', color: '#64748b', fontWeight: '600' }}>/ 100</span>
              </div>
              <GradeBadge grade={bestScore.grade} />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginTop: '0.5rem', paddingTop: '1rem', borderTop: '1px solid rgba(148, 163, 184, 0.15)' }}>
              <div>
                <span style={{ display: 'block', fontSize: '0.75rem', color: '#64748b', fontWeight: '600', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Strategy</span>
                <strong style={{ display: 'block', fontSize: '0.92rem', color: 'var(--text-h)', marginTop: '0.15rem' }}>{bestScore.strategy}</strong>
              </div>
              <div>
                <span style={{ display: 'block', fontSize: '0.75rem', color: '#64748b', fontWeight: '600', textTransform: 'uppercase', letterSpacing: '0.05em' }}>P99 Latency</span>
                <strong style={{ display: 'block', fontSize: '0.92rem', color: 'var(--text-h)', marginTop: '0.15rem' }}>{formatLatency(bestScore.p99_latency_ms)}</strong>
              </div>
              <div>
                <span style={{ display: 'block', fontSize: '0.75rem', color: '#64748b', fontWeight: '600', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Throughput</span>
                <strong style={{ display: 'block', fontSize: '0.92rem', color: 'var(--text-h)', marginTop: '0.15rem' }}>{formatTPS(bestScore.tps)} TPS</strong>
              </div>
              <div>
                <span style={{ display: 'block', fontSize: '0.75rem', color: '#64748b', fontWeight: '600', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Correctness</span>
                <strong style={{ display: 'block', fontSize: '0.92rem', color: 'var(--text-h)', marginTop: '0.15rem' }}>{(bestScore.correctness_score / 50 * 100).toFixed(0)}%</strong>
              </div>
            </div>

            <div style={{ fontSize: '0.78rem', color: '#94a3b8', textAlign: 'right', marginTop: '0.25rem' }}>
              Tested {new Date(bestScore.submitted_at).toLocaleDateString()}
            </div>
          </article>
        ) : (
          <article className="panel" style={{ borderRadius: '1.5rem', padding: '1.75rem', textAlign: 'center', background: 'rgba(255,255,255,0.7)' }}>
            <span className="section-tag">🏆 Personal Best</span>
            <p style={{ color: '#64748b', fontSize: '0.9rem', margin: '1.5rem 0' }}>No practice runs recorded yet.</p>
          </article>
        )}
      </div>

      <SubmissionDetail submission={selectedSubmission} onClose={() => setSelectedSubmission(null)} />
    </div>
  );
}
