import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import useWebSocket from '../hooks/useWebSocket';
import { useAuth } from '../context/AuthContext';

/* ── Constants ── */
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

const PROBLEM_ACCENTS = [
  { color: '#3b82f6', bg: 'rgba(59,130,246,0.06)', border: 'rgba(59,130,246,0.18)', gradient: 'linear-gradient(135deg, rgba(59,130,246,0.08), rgba(59,130,246,0.02))' },
  { color: '#8b5cf6', bg: 'rgba(139,92,246,0.06)', border: 'rgba(139,92,246,0.18)', gradient: 'linear-gradient(135deg, rgba(139,92,246,0.08), rgba(139,92,246,0.02))' },
  { color: '#10b981', bg: 'rgba(16,185,129,0.06)', border: 'rgba(16,185,129,0.18)', gradient: 'linear-gradient(135deg, rgba(16,185,129,0.08), rgba(16,185,129,0.02))' },
  { color: '#f59e0b', bg: 'rgba(245,158,11,0.06)', border: 'rgba(245,158,11,0.18)', gradient: 'linear-gradient(135deg, rgba(245,158,11,0.08), rgba(245,158,11,0.02))' },
  { color: '#ec4899', bg: 'rgba(236,72,153,0.06)', border: 'rgba(236,72,153,0.18)', gradient: 'linear-gradient(135deg, rgba(236,72,153,0.08), rgba(236,72,153,0.02))' },
];

/* ── Helpers ── */
function displayName(entry) {
  if (entry.username && entry.username.trim()) return entry.username;
  if (entry.system_name && entry.system_name.trim()) return entry.system_name;
  return entry.submission_id?.slice(0, 18) || '—';
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
  if (!strategy) return 'Common';
  const found = strategyTabs.find(t => t.value === strategy);
  return found ? found.label : strategy || '—';
}

function getModeInfo(mode) {
  switch (mode) {
    case 'contest_final': return { emoji: '🏁', label: 'Final', bg: 'rgba(16,185,129,0.08)', color: '#059669' };
    case 'contest_live': return { emoji: '🔴', label: 'Live', bg: 'rgba(239,68,68,0.08)', color: '#dc2626' };
    default: return { emoji: '⚡', label: 'Practice', bg: 'rgba(99,102,241,0.08)', color: '#6366f1' };
  }
}

/* ── Small Components ── */
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

/* ── Submission Detail Modal ── */
function SubmissionDetail({ submission, onClose }) {
  if (!submission) return null;
  return (
    <div className="detail-overlay" onClick={onClose} style={{ zIndex: 110 }}>
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
            {submission.judging_mode === 'contest_final' ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', flex: 1 }}>
                <strong style={{ fontSize: '0.85rem', color: 'var(--text-dim)' }}>Round Scores</strong>
                {(submission.round_scores || []).map((rs, i) => (
                  <ScoreBar key={i} label={rs.label} value={rs.score} max={100} color="#8b5cf6" />
                ))}
              </div>
            ) : (
              <>
                <ScoreBar label="Latency" value={submission.latency_score} max={25} color="#3b82f6" />
                <ScoreBar label="Throughput" value={submission.throughput_score} max={25} color="#10b981" />
                <ScoreBar label="Correctness" value={submission.correctness_score} max={50} color="#f59e0b" />
              </>
            )}
          </div>
        </div>

        {submission.judging_mode === 'contest_final' ? (
          <div className="detail-metrics" style={{ gridTemplateColumns: '1fr 1fr' }}>
            <div className="metric-cell">
              <span>Strategy</span>
              <strong>{strategyLabel(submission.strategy)}</strong>
            </div>
            <div className="metric-cell">
              <span>Finalized At</span>
              <strong>{new Date(submission.submitted_at).toLocaleString()}</strong>
            </div>
          </div>
        ) : (
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
        )}
      </article>
    </div>
  );
}

function AssignGradeJS(total) {
  if (total >= 90) return "S";
  if (total >= 75) return "A";
  if (total >= 60) return "B";
  if (total >= 45) return "C";
  return "F";
}

/* ── Final Problem Detail Modal ── */
function FinalProblemDetailModal({ problem, onClose }) {
  if (!problem) return null;

  const avgGrade = AssignGradeJS(problem.averageScore);

  return (
    <div className="detail-overlay" onClick={onClose} style={{ zIndex: 110 }}>
      <article className="detail-card panel" onClick={e => e.stopPropagation()} style={{ maxWidth: '520px' }}>
        {/* Header */}
        <div className="detail-header" style={{ marginBottom: '1.25rem' }}>
          <div>
            <span className="section-tag">Problem Detail</span>
            <h3 style={{ fontSize: '1.15rem', margin: '0.3rem 0 0' }}>
              {problem.problem_code ? `${problem.problem_code} - ${problem.problem_title}` : problem.problem_title}
            </h3>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close">✕</button>
        </div>

        {/* Problem Summary / Average Score Banner */}
        <div style={{
          marginBottom: '1.5rem',
          padding: '1.25rem 1.5rem',
          borderRadius: '1rem',
          background: 'linear-gradient(135deg, rgba(99,102,241,0.06), rgba(139,92,246,0.04))',
          border: '1px solid rgba(99,102,241,0.12)',
          display: 'flex',
          alignItems: 'center',
          gap: '1rem',
        }}>
          <GradeBadge grade={avgGrade} />
          <div>
            <div style={{ fontSize: '1.75rem', fontWeight: 800, color: 'var(--text-h)', letterSpacing: '-0.03em', lineHeight: 1 }}>
              {problem.averageScore.toFixed(1)}
            </div>
            <div style={{ fontSize: '0.75rem', fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.08em', marginTop: '0.2rem' }}>
              Average Score · {problem.rounds.length} Evaluated Strateg{problem.rounds.length !== 1 ? 'ies' : 'y'}
            </div>
          </div>
        </div>

        {/* List of strategy scores */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          <strong style={{ fontSize: '0.85rem', color: 'var(--text-dim)', marginBottom: '0.25rem' }}>
            Evaluated Strategies
          </strong>
          {problem.rounds.map((r, i) => {
            const accent = PROBLEM_ACCENTS[i % PROBLEM_ACCENTS.length];
            return (
              <div
                key={i}
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '1rem 1.25rem',
                  borderRadius: '0.875rem',
                  background: accent.gradient,
                  border: `1px solid ${accent.border}`,
                }}
              >
                <div>
                  <div style={{ fontSize: '0.92rem', fontWeight: 700, color: 'var(--text-h)' }}>
                    {strategyLabel(r.strategy)}
                  </div>
                  <div style={{ fontSize: '0.78rem', color: 'var(--text-dim)', marginTop: '0.15rem' }}>
                    {r.label || 'Deterministic run'}
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.875rem' }}>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontSize: '1.05rem', fontWeight: 700, color: 'var(--text-h)' }}>
                      {r.score != null ? r.score.toFixed(1) : '0.0'}
                    </div>
                    <div style={{ fontSize: '0.7rem', color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                      Score
                    </div>
                  </div>
                  <GradeBadge grade={r.grade || 'F'} />
                </div>
              </div>
            );
          })}
        </div>
      </article>
    </div>
  );
}

/* ── Contestant Submissions Modal (Contest drill-down) ── */
function ContestantSubmissionsModal({ contestant, onClose, onViewSubmission, onViewFinalProblem }) {
  if (!contestant) return null;

  let problems = [];
  const isFinal = (contestant.judging_mode || 'practice') === 'contest_final';

  if (isFinal) {
    const rawRounds = contestant.round_scores || [];
    const groupedMap = {};
    const groupedList = [];
    rawRounds.forEach(r => {
      if (r.problem_code === 'Live' || (r.label && r.label.toLowerCase().includes('best live'))) {
        return;
      }
      const code = r.problem_code || 'Unknown';
      if (!groupedMap[code]) {
        groupedMap[code] = {
          problem_code: code,
          problem_title: r.problem_title || 'Unknown Problem',
          rounds: [],
          key: code,
        };
        groupedList.push(groupedMap[code]);
      }
      groupedMap[code].rounds.push(r);
    });

    groupedList.forEach(g => {
      const sum = g.rounds.reduce((acc, r) => acc + (r.score || 0), 0);
      g.averageScore = g.rounds.length > 0 ? sum / g.rounds.length : 0;
    });

    problems = groupedList;
  } else {
    try {
      const parsed = typeof contestant.raw_metrics === 'string'
        ? JSON.parse(contestant.raw_metrics)
        : contestant.raw_metrics;
      if (Array.isArray(parsed)) {
        problems = parsed;
      }
    } catch (e) {
      console.error("Failed to parse raw_metrics for contestant", e);
    }
  }

  const totalScore = (contestant.total_score || 0).toFixed(1);
  const gradeStyle = gradeColors[contestant.grade] || gradeColors.F;

  return (
    <div className="detail-overlay" onClick={onClose}>
      <article className="detail-card panel" onClick={e => e.stopPropagation()} style={{ maxWidth: '580px' }}>
        {/* Header */}
        <div className="detail-header" style={{ marginBottom: '0' }}>
          <div>
            <span className="section-tag">Contestant Breakdown</span>
            <h3 style={{ fontSize: '1.15rem', margin: '0.3rem 0 0' }}>{displayName(contestant)}</h3>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close">✕</button>
        </div>

        {/* Score Summary Banner */}
        <div style={{
          margin: '1.25rem 0',
          padding: '1.25rem 1.5rem',
          borderRadius: '1rem',
          background: 'linear-gradient(135deg, rgba(99,102,241,0.06), rgba(139,92,246,0.04))',
          border: '1px solid rgba(99,102,241,0.12)',
          display: 'flex',
          alignItems: 'center',
          gap: '1rem',
        }}>
          <GradeBadge grade={contestant.grade} />
          <div>
            <div style={{ fontSize: '2rem', fontWeight: 800, color: 'var(--text-h)', letterSpacing: '-0.03em', lineHeight: 1 }}>
              {totalScore}
            </div>
            <div style={{ fontSize: '0.78rem', fontWeight: 600, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.08em', marginTop: '0.2rem' }}>
              Total Score · {problems.length} Problem{problems.length !== 1 ? 's' : ''}
            </div>
          </div>
        </div>

        {/* Problem Cards */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {problems.length === 0 ? (
            <div style={{
              textAlign: 'center',
              padding: '2.5rem 1rem',
              color: '#94a3b8',
              fontSize: '0.9rem',
              borderRadius: '0.75rem',
              background: 'rgba(15,23,42,0.02)',
              border: '1px dashed rgba(148,163,184,0.25)',
            }}>
              <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>📭</div>
              No problem data available.
            </div>
          ) : (
            problems.map((p, i) => {
              const accent = PROBLEM_ACCENTS[i % PROBLEM_ACCENTS.length];
              const problemName = isFinal
                ? p.problem_code && p.problem_title
                  ? `${p.problem_code} - ${p.problem_title}`
                  : p.problem_title || 'Unknown Problem'
                : p.problem_code && p.problem_title
                  ? `${p.problem_code} - ${p.problem_title}`
                  : (p.problem_id || `Problem ${i + 1}`);
              const score = isFinal ? p.averageScore : p.total_score;

              return (
                <div
                  key={i}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '1rem 1.25rem',
                    borderRadius: '0.875rem',
                    background: accent.gradient,
                    border: `1px solid ${accent.border}`,
                    transition: 'transform 180ms ease, box-shadow 180ms ease',
                    cursor: 'pointer',
                  }}
                  onMouseOver={(e) => {
                    e.currentTarget.style.transform = 'translateY(-2px)';
                    e.currentTarget.style.boxShadow = `0 8px 24px ${accent.border}`;
                  }}
                  onMouseOut={(e) => {
                    e.currentTarget.style.transform = 'translateY(0)';
                    e.currentTarget.style.boxShadow = 'none';
                  }}
                  onClick={() => {
                    if (isFinal) {
                      onViewFinalProblem(p);
                    } else {
                      onViewSubmission(p);
                    }
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.875rem', minWidth: 0 }}>
                    {/* Problem number circle */}
                    {!isFinal && (
                      <div style={{
                        width: '2.25rem',
                        height: '2.25rem',
                        borderRadius: '0.625rem',
                        background: accent.color,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: '#fff',
                        fontWeight: 800,
                        fontSize: '0.85rem',
                        flexShrink: 0,
                        boxShadow: `0 4px 12px ${accent.border}`,
                      }}>
                        {p.problem_code || String.fromCharCode(65 + i)}
                      </div>
                    )}
                    <div style={{ minWidth: 0 }}>
                      <div style={{
                        fontSize: '0.92rem',
                        fontWeight: 700,
                        color: 'var(--text-h)',
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      }}>
                        {problemName}
                      </div>
                      <div style={{
                        fontSize: '0.78rem',
                        fontWeight: 600,
                        color: accent.color,
                        marginTop: '0.15rem',
                        letterSpacing: '0.03em',
                      }}>
                        {isFinal
                          ? `Final Score: ${score != null ? score.toFixed(1) : '—'}`
                          : `Score: ${score != null ? score.toFixed(1) : '—'}`}
                      </div>
                    </div>
                  </div>

                  {isFinal ? (
                    <span style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '0.25rem',
                      fontWeight: 600,
                      fontSize: '0.78rem',
                      color: accent.color,
                    }}>
                      View Details →
                    </span>
                  ) : (
                    <button
                      onClick={(e) => { e.stopPropagation(); onViewSubmission(p); }}
                      style={{
                        padding: '0.4rem 0.85rem',
                        borderRadius: '0.5rem',
                        border: `1px solid ${accent.border}`,
                        background: accent.bg,
                        color: accent.color,
                        fontWeight: 600,
                        fontSize: '0.78rem',
                        cursor: 'pointer',
                        transition: 'all 180ms ease',
                        whiteSpace: 'nowrap',
                        flexShrink: 0,
                      }}
                      onMouseOver={(e) => {
                        e.currentTarget.style.background = accent.color;
                        e.currentTarget.style.color = '#fff';
                        e.currentTarget.style.borderColor = accent.color;
                      }}
                      onMouseOut={(e) => {
                        e.currentTarget.style.background = accent.bg;
                        e.currentTarget.style.color = accent.color;
                        e.currentTarget.style.borderColor = accent.border;
                      }}
                    >
                      View Submission
                    </button>
                  )}
                </div>
              );
            })
          )}
        </div>
      </article>
    </div>
  );
}

/* ── Main Leaderboard Page ── */
export default function Leaderboard() {
  const { user } = useAuth();
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selected, setSelected] = useState(null);
  const [selectedContestant, setSelectedContestant] = useState(null);
  const [selectedFinalProblem, setSelectedFinalProblem] = useState(null);
  const [activeStrategy, setActiveStrategy] = useState('bbo_heavy');
  const [contestName, setContestName] = useState('');
  const [modeFilter, setModeFilter] = useState('all'); // 'all' | 'practice' | 'contest_live' | 'contest_final'
  const [contestPhase, setContestPhase] = useState('');
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
        let fetchedEntries = data.leaderboard || [];
        if (isContestMode && data.type === 'final') {
          fetchedEntries = fetchedEntries.map(e => ({
            ...e,
            judging_mode: 'contest_final',
            total_score: e.avg_score || 0,
            grade: e.final_grade || 'F',
            submission_id: `final-${e.system_name}`,
            latency_score: e.latency_score || 0,
            throughput_score: e.throughput_score || 0,
            correctness_score: e.correctness_score || 0,
            p99_latency_ms: e.p99_latency_ms || 0,
            tps: e.tps || 0,
            cross_events: 0,
            orders_processed: 0,
            strategy: 'Combined Final',
            submitted_at: e.finalized_at,
            round_scores: e.round_scores,
          }));
        }
        setEntries(fetchedEntries);
        if (isContestMode) {
          setContestPhase(data.phase || '');
          if (data.type) {
            setModeFilter(data.type === 'final' ? 'contest_final' : 'contest_live');
          }
        } else {
          setContestPhase('');
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

    if (isContestMode) {
      fetch('/api/contests')
        .then(res => res.ok ? res.json() : Promise.reject())
        .then(data => {
          const contests = data.contests || [];
          const contest = contests.find(c => c.id === contestId);
          if (contest) {
            setContestName(contest.name);
          }
        })
        .catch(() => {});
    }
  }, [activeStrategy, updateTrigger, isContestMode, contestId]);

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
          <h2>{isContestMode ? `Contest Leaderboard: ${contestName || contestId}` : 'Trading Engine Rankings'}</h2>
          <p>
            {isContestMode
              ? 'Real-time scores for this specific contest. Ranks are determined by the contest rules.'
              : 'Real-time scores from benchmarked submissions. Engines are ranked by composite score across latency, throughput, and correctness.'}
          </p>
        </div>
        <div className="leaderboard-status">
          <span
            className={`status-pill ${
              isContestMode && contestPhase === 'finalizing'
                ? ''
                : modeFilter === 'contest_final'
                  ? 'offline'
                  : (connected ? 'success' : 'offline')
            }`}
            style={
              isContestMode && contestPhase === 'finalizing'
                ? { background: 'rgba(245, 158, 11, 0.18)', color: '#b45309' }
                : {}
            }
          >
            {isContestMode && contestPhase === 'finalizing'
              ? '● Finalizing...'
              : modeFilter === 'contest_final'
                ? '● Finalized'
                : (connected ? '● Live' : '○ Offline')}
          </span>
          <span className="entry-count">
            {isContestMode && contestPhase === 'finalizing' ? '0' : displayEntries.length} submissions
          </span>
        </div>
      </div>

      {!isContestMode && (
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
      )}

      <details className="scoring-info panel">
        <summary>How is the Total Score calculated?</summary>
        <div className="scoring-details">
          <div className="math-formula">
            <span>S<sub>Total</sub></span> = S<sub>L</sub> + S<sub>T</sub> + S<sub>C</sub>
          </div>
          <div className="math-formula">
            <span>S<sub>L</sub> (Latency)</span> = 25 × max(0, min(1, (100 - P99) / 95))
          </div>
          <div className="math-formula">
            <span>S<sub>T</sub> (Throughput)</span> = 25 × max(0, min(1, (TPS - 100) / 4900))
          </div>
          <div className="math-formula">
            <span>S<sub>C</sub> (Correctness)</span> = 50 × max(0, 1 - (Crosses / Orders))
          </div>
        </div>
      </details>

      {error && (
        <div className="feedback error">
          Backend unavailable: {error}. Showing cached data if available.
        </div>
      )}

      {isContestMode && contestPhase === 'finalizing' ? (
        <div className="leaderboard-empty panel" style={{
          background: 'linear-gradient(135deg, rgba(99,102,241,0.03), rgba(139,92,246,0.02))',
          border: '1px solid rgba(99,102,241,0.15)',
          padding: '5rem 2rem',
          textAlign: 'center',
          borderRadius: '1.25rem'
        }}>
          <div className="spinner" style={{ margin: '0 auto 1.5rem', width: '3rem', height: '3rem', borderWidth: '3.5px' }} />
          <h3 style={{ fontSize: '1.4rem', color: 'var(--text-h)', margin: '0 0 0.5rem 0', fontWeight: '700' }}>Finalizing Standings</h3>
          <p style={{ color: '#64748b', maxWidth: '500px', margin: '0 auto', fontSize: '0.92rem', lineHeight: '1.6' }}>
            The contest has ended! The host is currently running the final post-contest evaluation rounds. 
            The finalized leaderboard standings will appear here shortly.
          </p>
        </div>
      ) : displayEntries.length === 0 && !error ? (
        <div className="leaderboard-empty panel">
          <div className="empty-icon">🏁</div>
          <h3>No submissions yet</h3>
          <p>
            {isContestMode
              ? 'No submissions found for this contest yet.'
              : activeStrategy
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
                  onClick={() => {
                    if (isContestMode) {
                      setSelectedContestant(entry);
                    } else {
                      setSelected(entry);
                    }
                  }}
                >
                  <div className="podium-rank">{getRankEmoji(i)}</div>
                  <GradeBadge grade={entry.grade} />
                  <strong className="podium-score">{entry.total_score.toFixed(1)}</strong>
                  <div className="podium-meta">
                    <span>P99 {formatLatency(entry.p99_latency_ms)}</span>
                    <span>{formatTPS(entry.tps)} TPS</span>
                  </div>
                  <span className="podium-id">{displayName(entry)}</span>
                  {isContestMode && modeFilter === 'contest_final' && (
                    <div style={{ fontSize: '0.75rem', color: '#4f46e5', fontWeight: 600, marginTop: '0.25rem', marginBottom: '-0.15rem' }}>
                      Best Live: {(() => {
                        const bestLiveRound = (entry.round_scores || []).find(r => (r.problem_code === 'Live') || (r.label && r.label.startsWith('Best Live')));
                        return bestLiveRound && bestLiveRound.score != null ? bestLiveRound.score.toFixed(1) : '—';
                      })()}
                    </div>
                  )}
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
                  <th>{isContestMode ? 'Username' : 'Engine'}</th>
                  {!isContestMode && <th>Mode</th>}
                  <th>Strategy</th>
                  <th>Grade</th>
                  <th>Score</th>
                  {isContestMode && modeFilter === 'contest_final' && <th>Best Live</th>}
                  <th>Latency</th>
                  <th>Throughput</th>
                  <th>Correctness</th>
                  <th>P99</th>
                  <th>TPS</th>
                </tr>
              </thead>
              <tbody>
                {displayEntries.map((entry, i) => {
                  const entryName = entry.username || entry.system_name;
                  const isMe = user && entryName === user.username;
                  const mode = getModeInfo(entry.judging_mode || 'practice');
                  return (
                    <tr
                      key={entry.submission_id}
                      className={`lb-row ${i < 3 ? 'lb-row-top' : ''} ${isMe ? 'lb-row-me' : ''}`}
                      onClick={() => {
                        if (isContestMode) {
                          setSelectedContestant(entry);
                        } else {
                          setSelected(entry);
                        }
                      }}
                    >
                      <td className="rank-cell">{getRankEmoji(i)}</td>
                      <td className="id-cell" title={entry.submission_id}>
                        {displayName(entry)}
                      </td>
                      {!isContestMode && (
                        <td>
                          <span style={{
                            fontSize: '0.72rem',
                            padding: '0.2rem 0.5rem',
                            borderRadius: '6px',
                            fontWeight: 600,
                            background: mode.bg,
                            color: mode.color,
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '0.25rem',
                          }}>
                            {mode.emoji}
                          </span>
                        </td>
                      )}
                      <td className="strategy-cell">{strategyLabel(entry.strategy)}</td>
                      <td><GradeBadge grade={entry.grade} /></td>
                      <td className="score-cell">{entry.total_score.toFixed(1)}</td>
                      {isContestMode && modeFilter === 'contest_final' && (
                        <td style={{ fontWeight: 600, color: 'var(--accent, #6366f1)' }}>
                          {(() => {
                            const bestLiveRound = (entry.round_scores || []).find(r => (r.problem_code === 'Live') || (r.label && r.label.startsWith('Best Live')));
                            return bestLiveRound && bestLiveRound.score != null ? bestLiveRound.score.toFixed(1) : '—';
                          })()}
                        </td>
                      )}
                      <td>{entry.latency_score.toFixed(1)}<span className="dim">/25</span></td>
                      <td>{entry.throughput_score.toFixed(1)}<span className="dim">/25</span></td>
                      <td>{entry.correctness_score.toFixed(1)}<span className="dim">/50</span></td>
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

      <ContestantSubmissionsModal
        contestant={selectedContestant}
        onClose={() => setSelectedContestant(null)}
        onViewSubmission={(sub) => {
          setSelected(sub);
        }}
        onViewFinalProblem={(prob) => {
          setSelectedFinalProblem(prob);
        }}
      />
      <SubmissionDetail submission={selected} onClose={() => setSelected(null)} />
      {selectedFinalProblem && (
        <FinalProblemDetailModal
          problem={selectedFinalProblem}
          onClose={() => setSelectedFinalProblem(null)}
        />
      )}
    </section>
  );
}
