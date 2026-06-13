import { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

// Helper: GradeBadge style
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
  if (ms == null) return '—';
  if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`;
  if (ms < 100) return `${ms.toFixed(2)}ms`;
  return `${ms.toFixed(0)}ms`;
}

function formatTPS(tps) {
  if (tps == null) return '—';
  if (tps >= 1000) return `${(tps / 1000).toFixed(1)}K`;
  return tps.toFixed(0);
}

function formatTimeAgo(dateInput) {
  if (!dateInput) return '—';
  const date = new Date(dateInput);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 5) return 'Just now';
  if (seconds < 60) return `${seconds}s ago`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;

  return date.toLocaleDateString();
}

const strategyLabels = {
  bbo_heavy: 'Common',
  flash_crash: 'Flash Crash',
  high_cancel: 'High Cancel',
  wide_spread: 'Wide Spread',
  market_maker: 'Market Maker',
  iceberg: 'Iceberg',
  momentum_burst: 'Momentum Burst',
};

function strategyLabel(strategy) {
  if (!strategy) return 'Common';
  return strategyLabels[strategy] || strategy || '—';
}

function ScoreBar({ label, value, max, color }) {
  const pct = Math.min((value / max) * 100, 100);
  return (
    <div className="score-bar-container" style={{ marginBottom: '0.75rem' }}>
      <div className="score-bar-label" style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.82rem', marginBottom: '0.25rem', color: 'var(--text)' }}>
        <span>{label}</span>
        <strong>{value.toFixed(1)}/{max}</strong>
      </div>
      <div className="score-bar-track" style={{ height: '6px', background: 'rgba(148, 163, 184, 0.15)', borderRadius: '3px', overflow: 'hidden' }}>
        <div
          className="score-bar-fill"
          style={{ width: `${pct}%`, background: color, height: '100%', borderRadius: '3px', transition: 'width 0.3s ease' }}
        />
      </div>
    </div>
  );
}

function SubmissionDetail({ submission, onClose }) {
  if (!submission) return null;

  const [activeTab, setActiveTab] = useState('average');

  const rawMetricsObj = useMemo(() => {
    if (!submission.raw_metrics) return null;
    try {
      if (typeof submission.raw_metrics === 'string') {
        return JSON.parse(submission.raw_metrics);
      }
      return submission.raw_metrics;
    } catch {
      return null;
    }
  }, [submission.raw_metrics]);

  const isMulti = rawMetricsObj?.is_multi_strategy;
  const rounds = rawMetricsObj?.rounds || [];

  const displaySource = useMemo(() => {
    if (activeTab === 'average' || !isMulti) {
      return {
        strategy: submission.strategy,
        grade: submission.grade,
        total_score: submission.total_score,
        latency_score: submission.latency_score,
        throughput_score: submission.throughput_score,
        correctness_score: submission.correctness_score,
        p99_latency_ms: submission.p99_latency_ms,
        tps: submission.tps,
        orders_processed: submission.orders_processed,
        cross_events: submission.cross_events,
      };
    }
    const r = rounds[activeTab];
    return {
      strategy: r?.strategy,
      grade: r?.score?.grade || 'F',
      total_score: r?.score?.total_score || 0,
      latency_score: r?.score?.latency_score || 0,
      throughput_score: r?.score?.throughput_score || 0,
      correctness_score: r?.score?.correctness_score || 0,
      p99_latency_ms: r?.perf_metrics?.p99_latency_ms || r?.metrics?.p99_latency_ms || 0,
      tps: r?.perf_metrics?.tps || r?.metrics?.requests_per_second || 0,
      orders_processed: r?.val_result?.orders_processed || r?.metrics?.successes || 0,
      cross_events: r?.val_result?.cross_events || 0,
    };
  }, [activeTab, submission, rounds, isMulti]);

  const displayName = (entry) => {
    if (entry.system_name && entry.system_name.trim()) return entry.system_name;
    return entry.submission_id?.slice(0, 18) || '—';
  };

  return (
    <div className="detail-overlay" onClick={onClose} style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(15, 23, 42, 0.4)', backdropFilter: 'blur(4px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '1rem'
    }}>
      <article className="detail-card panel" onClick={e => e.stopPropagation()} style={{
        background: '#fff', padding: '2rem', borderRadius: '16px', maxWidth: '36rem', width: '100%',
        boxShadow: 'var(--shadow)', border: '1px solid rgba(148, 163, 184, 0.2)'
      }}>
        <div className="detail-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '1.25rem' }}>
          <div>
            <span className="section-tag" style={{ fontSize: '0.7rem', color: 'var(--muted)', display: 'block', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Submission Detail</span>
            <h3 style={{ margin: '0.25rem 0 0 0', fontSize: '1.35rem', color: 'var(--text-h)' }}>{displayName(submission)}</h3>
            <span className="detail-strategy-tag" style={{ display: 'inline-block', fontSize: '0.78rem', background: 'rgba(99,102,241,0.08)', color: '#6366f1', padding: '2px 8px', borderRadius: '6px', marginTop: '0.5rem', fontWeight: 600 }}>
              {strategyLabel(displaySource.strategy)}
            </span>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close" style={{ background: 'none', border: 'none', fontSize: '1.25rem', cursor: 'pointer', color: 'var(--text)' }}>✕</button>
        </div>

        {isMulti && (
          <div className="multi-strategy-tabs" style={{ display: 'flex', gap: '0.35rem', marginBottom: '1.25rem', overflowX: 'auto', paddingBottom: '6px', borderBottom: '1px solid rgba(148, 163, 184, 0.12)' }}>
            <button
              onClick={() => setActiveTab('average')}
              style={{
                padding: '6px 12px',
                borderRadius: '6px',
                border: 'none',
                cursor: 'pointer',
                fontSize: '0.78rem',
                fontWeight: '600',
                background: activeTab === 'average' ? 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)' : 'rgba(15,23,42,0.03)',
                color: activeTab === 'average' ? '#fff' : 'var(--text-h)',
                transition: 'all 0.2s',
                boxShadow: activeTab === 'average' ? '0 2px 6px rgba(99, 102, 241, 0.2)' : 'none',
              }}
            >
              Summary (Average)
            </button>
            {rounds.map((round, idx) => (
              <button
                key={idx}
                onClick={() => setActiveTab(idx)}
                style={{
                  padding: '6px 12px',
                  borderRadius: '6px',
                  border: 'none',
                  cursor: 'pointer',
                  fontSize: '0.78rem',
                  fontWeight: '600',
                  background: activeTab === idx ? 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)' : 'rgba(15,23,42,0.03)',
                  color: activeTab === idx ? '#fff' : 'var(--text-h)',
                  transition: 'all 0.2s',
                  boxShadow: activeTab === idx ? '0 2px 6px rgba(99, 102, 241, 0.2)' : 'none',
                  whiteSpace: 'nowrap'
                }}
              >
                {strategyLabel(round.strategy)}
              </button>
            ))}
          </div>
        )}

        <div className="detail-scores" style={{ display: 'flex', gap: '2rem', alignItems: 'center', marginBottom: '1.5rem', padding: '1rem', background: 'rgba(15,23,42,0.02)', borderRadius: '12px' }}>
          <div className="detail-total" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.5rem', borderRight: '1px solid rgba(148,163,184,0.15)', paddingRight: '1.5rem' }}>
            <GradeBadge grade={displaySource.grade} />
            <div className="detail-total-num" style={{ fontSize: '1.5rem', color: 'var(--text-h)', fontWeight: '800' }}>
              <strong>{displaySource.total_score?.toFixed(1) || '0.0'}</strong>
              <span style={{ fontSize: '0.9rem', color: 'var(--muted)', fontWeight: '400' }}>/100</span>
            </div>
          </div>

          <div className="detail-breakdown" style={{ flex: 1 }}>
            <ScoreBar label="Latency" value={displaySource.latency_score || 0} max={25} color="#3b82f6" />
            <ScoreBar label="Throughput" value={displaySource.throughput_score || 0} max={25} color="#10b981" />
            <ScoreBar label="Correctness" value={displaySource.correctness_score || 0} max={50} color="#f59e0b" />
          </div>
        </div>

        <div className="detail-metrics" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', background: 'rgba(15,23,42,0.01)', border: '1px solid rgba(148, 163, 184, 0.1)', padding: '1rem', borderRadius: '12px' }}>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>P99 Latency</span>
            <strong style={{ fontSize: '0.95rem', color: 'var(--text-h)' }}>{formatLatency(displaySource.p99_latency_ms)}</strong>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>TPS</span>
            <strong style={{ fontSize: '0.95rem', color: 'var(--text-h)' }}>{formatTPS(displaySource.tps)}</strong>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>Orders</span>
            <strong style={{ fontSize: '0.95rem', color: 'var(--text-h)' }}>{(displaySource.orders_processed || 0).toLocaleString()}</strong>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>Crosses</span>
            <strong style={{ fontSize: '0.95rem', color: displaySource.cross_events > 0 ? '#ef4444' : 'var(--text-h)' }}>
              {displaySource.cross_events || 0}
            </strong>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>Submitted</span>
            <strong style={{ fontSize: '0.82rem', color: 'var(--text-h)' }}>{new Date(submission.submitted_at).toLocaleString()}</strong>
          </div>
        </div>
      </article>
    </div>
  );
}

function HistoryModal({ history, systemName, onClose, onSelect }) {
  return (
    <div className="detail-overlay" onClick={onClose} style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(15, 23, 42, 0.4)', backdropFilter: 'blur(4px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '1rem'
    }}>
      <div className="detail-card panel" style={{
        background: '#fff', maxWidth: '52rem', width: '100%', maxHeight: '85vh',
        display: 'flex', flexDirection: 'column', padding: '2rem', borderRadius: '16px',
        boxShadow: 'var(--shadow)', border: '1px solid rgba(148, 163, 184, 0.2)'
      }} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem', flexShrink: 0 }}>
          <div style={{ textAlign: 'left' }}>
            <span className="section-tag" style={{ fontSize: '0.7rem', color: 'var(--muted)' }}>Performance Archive</span>
            <h2 style={{ fontSize: '1.4rem', margin: '0.2rem 0 0', letterSpacing: '-0.02em', color: 'var(--text-h)' }}>
              Submission History
            </h2>
            <p style={{ margin: '0.2rem 0 0', fontSize: '0.85rem', color: 'var(--muted)' }}>
              Showing runs for system "{systemName}" in this contest
            </p>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close" style={{ background: 'none', border: 'none', fontSize: '1.25rem', cursor: 'pointer', color: 'var(--text)' }}>✕</button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, marginBottom: '0.5rem' }}>
          {history.length === 0 ? (
            <div style={{ padding: '3rem 1rem', textAlign: 'center', background: 'rgba(15, 23, 42, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(148, 163, 184, 0.25)' }}>
              <p style={{ color: 'var(--muted)', margin: 0, fontSize: '0.95rem' }}>No submissions found for "{systemName}" in this contest.</p>
            </div>
          ) : (
            <div className="leaderboard-table-wrap" style={{ border: '1px solid rgba(148, 163, 184, 0.15)', background: '#fff', borderRadius: '8px', overflow: 'hidden' }}>
              <table className="leaderboard-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
                <thead>
                  <tr style={{ background: 'rgba(15,23,42,0.02)', borderBottom: '1px solid rgba(148,163,184,0.1)' }}>
                    <th style={{ padding: '0.85rem 1rem', fontSize: '0.8rem', color: 'var(--text-h)' }}>Submitted File</th>
                    <th style={{ padding: '0.85rem 1rem', fontSize: '0.8rem', color: 'var(--text-h)' }}>Submitted At</th>
                    <th style={{ padding: '0.85rem 1rem', fontSize: '0.8rem', color: 'var(--text-h)' }}>Grade</th>
                    <th style={{ padding: '0.85rem 1rem', fontSize: '0.8rem', color: 'var(--text-h)' }}>Score</th>
                    <th style={{ padding: '0.85rem 1rem', fontSize: '0.8rem', color: 'var(--text-h)' }}>TPS</th>
                    <th style={{ padding: '0.85rem 1rem', fontSize: '0.8rem', color: 'var(--text-h)' }}>P99 Latency</th>
                  </tr>
                </thead>
                <tbody>
                  {history.map((run, idx) => (
                    <tr 
                      key={run.submission_id} 
                      className="lb-row" 
                      onClick={() => onSelect(run)}
                      style={{ cursor: 'pointer', borderBottom: '1px solid rgba(148,163,184,0.08)', transition: 'background 0.2s' }}
                      onMouseEnter={(e) => e.currentTarget.style.backgroundColor = 'rgba(15,23,42,0.02)'}
                      onMouseLeave={(e) => e.currentTarget.style.backgroundColor = 'transparent'}
                      title="Click to view details"
                    >
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600, color: 'var(--text-h)', fontSize: '0.85rem' }}>
                        {run.filename || `Submission ${history.length - idx}`}
                      </td>
                      <td style={{ padding: '0.85rem 1rem', color: '#64748b', fontSize: '0.85rem' }}>
                        {formatTimeAgo(run.submitted_at)}
                      </td>
                      <td style={{ padding: '0.6rem 1rem' }}>
                        <GradeBadge grade={run.grade} />
                      </td>
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600, color: 'var(--text-h)' }}>
                        {run.total_score?.toFixed(1)}
                      </td>
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600, color: 'var(--text-h)' }}>
                        {formatTPS(run.tps)}
                      </td>
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600, color: 'var(--text-h)' }}>
                        {formatLatency(run.p99_latency_ms)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function ContestArenaPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user, authHeaders, logout } = useAuth();

  const languageExtensions = {
    cpp: ['cpp', 'cc', 'cxx'],
    go: ['go'],
    rust: ['rs'],
    python: ['py'],
  };

  const [contestData, setContestData] = useState(null);
  const [loadingContest, setLoadingContest] = useState(true);
  const [activeProblemIdx, setActiveProblemIdx] = useState(0);

  const activeProblem = useMemo(() => {
    if (!contestData?.problems || contestData.problems.length === 0) return null;
    return contestData.problems[activeProblemIdx] || contestData.problems[0];
  }, [contestData, activeProblemIdx]);

  // Registration states
  const [isRegistered, setIsRegistered] = useState(false);
  const [checkingRegistration, setCheckingRegistration] = useState(true);
  const [regSystemName, setRegSystemName] = useState(user?.username || '');
  const [regCode, setRegCode] = useState('');
  const [regMessage, setRegMessage] = useState('');
  const [regSubmitting, setRegSubmitting] = useState(false);

  // Submission Form states
  const [formData, setFormData] = useState({
    systemName: user?.username || '',
    port: '8080',
    language: 'cpp',
    protocol: 'http',
    strategy: '',
    rampUpSeconds: '0',
    file: null,
    contestId: id
  });

  const [submitState, setSubmitState] = useState({ type: '', message: '' });
  const [executionResult, setExecutionResult] = useState(null);
  const [stressTestResult, setStressTestResult] = useState(null);
  const [stressTestMeta, setStressTestMeta] = useState(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // History states
  const [history, setHistory] = useState([]);
  const [showHistory, setShowHistory] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [selectedSubmission, setSelectedSubmission] = useState(null);

  // Timer state
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  // Fetch contest data and registration status
  const loadPageData = async () => {
    setLoadingContest(true);
    setCheckingRegistration(true);
    try {
      // 1. Fetch contest info
      const contestRes = await fetch(`/api/contests/${id}/public`);
      if (!contestRes.ok) {
        throw new Error("Contest not found or inaccessible.");
      }
      const data = await contestRes.json();
      setContestData(data);

      if (data?.details?.strategy) {
        setFormData(prev => ({ ...prev, strategy: data.details.strategy }));
      } else if (data?.problems && data.problems.length > 0) {
        let foundStrat = '';
        for (const p of data.problems) {
          if (p.sampleStrategies && p.sampleStrategies.length > 0) {
            foundStrat = p.sampleStrategies[0];
            break;
          }
        }
        setFormData(prev => ({ ...prev, strategy: foundStrat || 'bbo_heavy' }));
      } else {
        setFormData(prev => ({ ...prev, strategy: 'bbo_heavy' }));
      }

      // 2. Fetch my registrations to see if user has entered
      const hdrs = authHeaders();
      if (hdrs.Authorization) {
        const regRes = await fetch('/api/contests/my-registrations', { headers: hdrs });
        if (regRes.ok) {
          const regData = await regRes.json();
          if (Array.isArray(regData.contest_ids)) {
            const hasReg = regData.contest_ids.includes(id);
            setIsRegistered(hasReg);
          }
        }
      }
    } catch (e) {
      console.error(e);
      setSubmitState({ type: 'error', message: e.message || 'Failed to load contest workspace.' });
    } finally {
      setLoadingContest(false);
      setCheckingRegistration(false);
    }
  };

  useEffect(() => {
    if (id) {
      loadPageData();
    }
  }, [id]);

  // Autofill username when loaded
  useEffect(() => {
    if (user?.username) {
      setRegSystemName(user.username);
      setFormData(prev => ({ ...prev, systemName: user.username }));
    }
  }, [user]);

  // Sync strategy and protocol with active problem
  useEffect(() => {
    if (activeProblem) {
      const pStrat = activeProblem.sampleStrategies?.[0] || activeProblem.SampleStrategies?.[0];
      const pProto = activeProblem.sampleProtocol || activeProblem.SampleProtocol || 'http';
      setFormData(prev => ({
        ...prev,
        strategy: pStrat || contestData?.details?.strategy || 'bbo_heavy',
        protocol: pProto
      }));
      // Clear sandbox deployment logs, stress test reports, and submit feedback when active problem changes
      setExecutionResult(null);
      setStressTestResult(null);
      setStressTestMeta(null);
      setSubmitState({ type: '', message: '' });
    }
  }, [activeProblem, contestData]);

  // Computed state for contest timing
  const contestTimes = useMemo(() => {
    if (!contestData?.details) return { isUpcoming: false, isEnded: false, remainingText: '' };
    
    const start = new Date(contestData.details.startTime);
    const duration = contestData.details.durationMinutes || 60;
    const end = new Date(start.getTime() + duration * 60000);

    const isUpcoming = now < start.getTime();
    const isEnded = now >= end.getTime();

    let remainingText = '';
    if (isUpcoming) {
      const diff = start.getTime() - now;
      const d = Math.floor(diff / 86400000);
      const h = Math.floor((diff % 86400000) / 3600000);
      const m = Math.floor((diff % 3600000) / 60000);
      const s = Math.floor((diff % 60000) / 1000);
      remainingText = d > 0 ? `Starts in ${d}d ${h}h` : `Starts in ${h}h ${m}m ${s}s`;
    } else if (isEnded) {
      remainingText = 'Ended';
    } else {
      const diff = end.getTime() - now;
      const h = Math.floor(diff / 3600000);
      const m = Math.floor((diff % 3600000) / 60000);
      const s = Math.floor((diff % 60000) / 1000);
      remainingText = `${h}h ${m}m ${s}s`;
    }

    return { isUpcoming, isEnded, remainingText, start, end };
  }, [contestData, now]);

  const [wasLive, setWasLive] = useState(false);

  useEffect(() => {
    if (contestData?.details) {
      const start = new Date(contestData.details.startTime).getTime();
      const duration = contestData.details.durationMinutes || 60;
      const end = start + duration * 60000;
      const nowTime = Date.now();
      if (nowTime >= start && nowTime < end) {
        setWasLive(true);
      }
    }
  }, [contestData]);

  // Automatically redirect registrants out of the arena to the leaderboard once the contest ends
  useEffect(() => {
    if (wasLive && contestTimes.isEnded) {
      navigate(`/leaderboard?contest_id=${id}`, { replace: true });
    }
  }, [wasLive, contestTimes.isEnded, id, navigate]);

  const handleChange = (e) => {
    const { name, value, files } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: files ? files[0] : value
    }));
  };

  const handleRegisterInline = async (e) => {
    e.preventDefault();
    setRegSubmitting(true);
    setRegMessage('Registering...');
    try {
      const res = await fetch(`/api/contests/${id}/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ systemName: regSystemName, code: regCode })
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.message || data.error || 'Registration failed');
      }
      setRegMessage('Registered successfully! Loading workspace...');
      setTimeout(() => {
        setIsRegistered(true);
        loadPageData();
      }, 1000);
    } catch (err) {
      setRegMessage('⚠ ' + err.message);
    } finally {
      setRegSubmitting(false);
    }
  };

  const fetchHistory = async () => {
    setLoadingHistory(true);
    setShowHistory(true);
    try {
      const probId = activeProblem?.ID || '';
      const res = await fetch(`/api/history/me?limit=50&contest_id=${id}&problem_id=${probId}`, { headers: authHeaders() });
      if (!res.ok) throw new Error("Failed to fetch history");
      const data = await res.json();
      setHistory(data.history || []);
    } catch (e) {
      console.error(e);
      alert("Failed to load history.");
      setShowHistory(false);
    } finally {
      setLoadingHistory(false);
    }
  };

  const handleCleanup = async () => {
    if (!executionResult?.pod_id) return;
    try {
      await fetch(`/api/sandbox/${executionResult.pod_id}`, { method: 'DELETE' });
      setSubmitState({ type: 'success', message: 'Sandbox cleaned up successfully.' });
      setExecutionResult(null);
      setStressTestResult(null);
    } catch {
      setSubmitState({ type: 'error', message: 'Cleanup failed.' });
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!isRegistered) {
      alert("You must be registered to submit.");
      return;
    }
    if (contestTimes.isUpcoming) {
      alert("Submissions are not open yet.");
      return;
    }
    if (contestTimes.isEnded) {
      alert("Contest has ended. Submissions are closed.");
      return;
    }

    setIsSubmitting(true);
    setSubmitState({ type: '', message: '' });
    setExecutionResult(null);
    setStressTestResult(null);
    setStressTestMeta(null);

    const selectedFile = formData.file;
    if (!selectedFile) {
      setSubmitState({
        type: 'error',
        message: 'Please choose a source file before submitting.',
      });
      setIsSubmitting(false);
      return;
    }

    const fileExtension = selectedFile.name.split('.').pop()?.toLowerCase();
    const allowedExtensions = languageExtensions[formData.language] || [];

    if (!allowedExtensions.includes(fileExtension)) {
      setSubmitState({
        type: 'error',
        message: `Selected language ${formData.language.toUpperCase()} requires one of: ${allowedExtensions
          .map((ext) => `.${ext}`)
          .join(', ')}.`,
      });
      setIsSubmitting(false);
      return;
    }

    const payload = new FormData();
    Object.entries(formData).forEach(([key, value]) => {
      if (key === 'file') {
        payload.append('source_code', value);
        return;
      }
      if (key === 'language') {
        payload.append('language', value);
        return;
      }
      payload.append(key, value);
    });

    try {
      const hdrs = authHeaders();
      const response = await fetch('/api/submit', {
        method: 'POST',
        headers: hdrs,
        body: payload,
      });
      const responseText = await response.text();
      let result = null;

      if (responseText) {
        try {
          result = JSON.parse(responseText);
        } catch {
          result = { message: responseText };
        }
      }

      if (!response.ok) {
        if (result?.execution_result) {
          setExecutionResult(result.execution_result);
        }
        throw new Error(result?.error || result?.message || `Submission failed with status ${response.status}`);
      }

      const sandboxExecution = result?.execution_result || null;
      setSubmitState({
        type: 'success',
        message: result?.message || 'Engine submitted successfully.',
      });
      setExecutionResult(sandboxExecution);

      if (sandboxExecution?.target_url && sandboxExecution?.phase === 'Running') {
        const stressResponse = await fetch('/api/stress-test', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...authHeaders(),
          },
          body: JSON.stringify({
            target: sandboxExecution.target_url,
            protocol: formData.protocol,
            strategy: formData.strategy,
            system_name: formData.systemName,
            language: formData.language,
            bots: 16,
            requests: 0,
            duration_seconds: 10,
            timeout_ms: 2000,
            method: 'POST',
            path: '/',
            expect_reply: formData.protocol === 'tcp' || formData.protocol === 'fix',
            ramp_up_seconds: parseInt(formData.rampUpSeconds) || 0,
            judging_mode: 'contest_live',
            contest_id: id,
            problem_id: activeProblem?.ID || '',
            filename: formData.file?.name || '',
          }),
        });

        const stressText = await stressResponse.text();
        let stressResult = null;

        if (stressText) {
          try {
            stressResult = JSON.parse(stressText);
          } catch {
            stressResult = { message: stressText };
          }
        }

        if (!stressResponse.ok) {
          throw new Error(stressResult?.error || stressResult?.message || `Stress test failed with status ${stressResponse.status}`);
        }

        setStressTestResult(stressResult?.rounds?.[0]?.metrics || stressResult?.metrics || null);
        setStressTestMeta({
          judgingMode: stressResult?.rounds?.[0]?.judging_mode || 'contest_live',
          seedUsed: stressResult?.rounds?.[0]?.seed_used || null,
          correctnessHint: stressResult?.rounds?.[0]?.correctness_hint || null,
        });

        const roundCount = stressResult?.rounds?.length || 1;
        const strategies = (stressResult?.rounds || []).map(r => r.strategy).join(' + ');
        setSubmitState({
          type: 'success',
          message: `Engine submitted successfully. Stress test completed: ${roundCount} round(s) [${strategies || formData.strategy}].`,
        });
      } else if (sandboxExecution) {
        setSubmitState({
          type: 'success',
          message: `Engine submitted successfully. Stress test skipped (phase: ${sandboxExecution.phase}).`,
        });
      }
    } catch (error) {
      setSubmitState({
        type: 'error',
        message: error.message || 'Upload failed. Please try again.',
      });
      console.error('Upload failed:', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Rendering Loader
  if (loadingContest || checkingRegistration) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '80vh', gap: '1rem' }}>
        <div className="auth-loading-spinner" style={{ width: '40px', height: '40px', border: '3px solid rgba(99, 102, 241, 0.2)', borderTopColor: '#6366f1', borderRadius: '50%', animation: 'spin 1s linear infinite' }} />
        <p style={{ color: 'var(--text-muted)' }}>Entering Contest Workspace...</p>
      </div>
    );
  }

  // Access check: User is not registered
  if (!isRegistered) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '80vh', padding: '1rem' }}>
        <div className="panel" style={{
          maxWidth: '28rem', width: '100%', padding: '2.5rem', borderRadius: '16px',
          textAlign: 'center', border: '1px solid rgba(239, 68, 68, 0.2)',
          background: 'rgba(255,255,255,0.95)', boxShadow: 'var(--shadow)'
        }}>
          <span style={{ fontSize: '3.5rem', display: 'block', marginBottom: '1rem' }}>🚫</span>
          <h2 style={{ fontSize: '1.5rem', fontWeight: '700', color: '#ef4444', marginBottom: '0.5rem' }}>Access Restricted</h2>
          <p style={{ color: 'var(--text)', fontSize: '0.92rem', lineHeight: '1.6', marginBottom: '1.75rem' }}>
            You are not registered for the contest <strong>"{contestData?.details?.name || 'this contest'}"</strong>. Only registered candidates can access the Contest Arena.
          </p>

          <form onSubmit={handleRegisterInline} style={{ display: 'flex', flexDirection: 'column', gap: '1rem', textAlign: 'left', background: 'rgba(15,23,42,0.02)', padding: '1.25rem', borderRadius: '12px', border: '1px solid rgba(148,163,184,0.1)', marginBottom: '1.5rem' }}>
            <h4 style={{ margin: 0, fontSize: '0.9rem', color: 'var(--text-h)', fontWeight: '600' }}>Register to join immediately</h4>
            
            <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
              System Name
              <input 
                required 
                type="text" 
                value={regSystemName} 
                onChange={(e) => setRegSystemName(e.target.value)} 
                style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #dbe4f0', fontSize: '0.85rem' }} 
              />
            </label>

            {contestData?.details?.visibility === 'private' && (
              <label style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
                Private Entry Code
                <input 
                  required 
                  type="password" 
                  value={regCode} 
                  placeholder="Enter access code" 
                  onChange={(e) => setRegCode(e.target.value)} 
                  style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #dbe4f0', fontSize: '0.85rem' }} 
                />
              </label>
            )}

            {regMessage && (
              <div style={{ fontSize: '0.8rem', color: regMessage.includes('successfully') ? '#22c55e' : '#ef4444', fontWeight: 500 }}>
                {regMessage}
              </div>
            )}

            <button 
              type="submit" 
              disabled={regSubmitting} 
              className="button button-primary" 
              style={{ width: '100%', padding: '10px 0', fontSize: '0.88rem' }}
            >
              {regSubmitting ? 'Registering...' : 'Register & Enter Arena'}
            </button>
          </form>

          <Link to="/contests" className="button button-secondary" style={{ width: '100%', textAlign: 'center', fontSize: '0.88rem' }}>
            Back to Contests
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', width: '100%' }}>
      
      {/* Premium Header Banner (Full Size Header) */}
      <header style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 1000,
        background: 'linear-gradient(135deg, #1e1b4b 0%, #311042 50%, #4c1d95 100%)',
        color: '#fff',
        padding: '1.25rem 2rem',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        flexWrap: 'wrap',
        gap: '1.5rem',
        boxShadow: '0 4px 20px rgba(0, 0, 0, 0.15)',
        borderBottom: '1px solid rgba(255,255,255,0.08)',
        boxSizing: 'border-box'
      }}>
        <div style={{ textAlign: 'left' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem' }}>
            <span style={{ fontSize: '0.75rem', fontWeight: 800, textTransform: 'uppercase', letterSpacing: '0.1em', background: 'rgba(255,255,255,0.15)', padding: '2px 8px', borderRadius: '4px' }}>
              Contest Workspace
            </span>
            {contestTimes.isUpcoming ? (
              <span style={{ fontSize: '0.75rem', fontWeight: 700, color: '#fbbf24', background: 'rgba(251,191,36,0.12)', padding: '2px 8px', borderRadius: '4px' }}>
                ⏳ Upcoming
              </span>
            ) : contestTimes.isEnded ? (
              <span style={{ fontSize: '0.75rem', fontWeight: 700, color: '#f87171', background: 'rgba(248,113,113,0.12)', padding: '2px 8px', borderRadius: '4px' }}>
                🏁 Ended
              </span>
            ) : (
              <span style={{
                fontSize: '0.75rem', fontWeight: 700, color: '#34d399', background: 'rgba(52,211,153,0.12)', padding: '2px 8px', borderRadius: '4px',
                animation: 'pulse 2s ease-in-out infinite', display: 'flex', alignItems: 'center', gap: '4px'
              }}>
                <span style={{ width: '6px', height: '6px', background: '#34d399', borderRadius: '50%' }} />
                🔴 Live
              </span>
            )}
          </div>
          <h2 style={{ fontSize: '1.8rem', fontWeight: '800', margin: 0, letterSpacing: '-0.02em', textShadow: '0 2px 4px rgba(0,0,0,0.2)' }}>
            {contestData?.details?.name}
          </h2>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '1.5rem' }}>
          {/* Live Leaderboard Link */}
          <Link
            to={`/leaderboard?contest_id=${id}`}
            style={{
              textDecoration: 'none',
              fontSize: '0.82rem',
              fontWeight: '600',
              color: '#fff',
              background: 'rgba(255, 255, 255, 0.08)',
              border: '1px solid rgba(255, 255, 255, 0.12)',
              padding: '0.55rem 1.05rem',
              borderRadius: '10px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '6px',
              transition: 'all 200ms'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = 'rgba(255, 255, 255, 0.15)';
              e.currentTarget.style.borderColor = 'rgba(255, 255, 255, 0.2)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = 'rgba(255, 255, 255, 0.08)';
              e.currentTarget.style.borderColor = 'rgba(255, 255, 255, 0.12)';
            }}
          >
            {contestData?.details?.phase === 'completed'
              ? '📊 Final Leaderboard'
              : contestData?.details?.phase === 'finalizing'
                ? '📊 Finalizing Leaderboard'
                : '📊 Live Leaderboard'}
          </Link>

          {/* Time Remaining block */}
          <div style={{
            background: 'rgba(255, 255, 255, 0.08)',
            backdropFilter: 'blur(8px)',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            padding: '0.5rem 1.25rem',
            borderRadius: '10px',
            display: 'flex',
            alignItems: 'center',
            gap: '12px'
          }}>
            <span style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'rgba(255,255,255,0.7)', fontWeight: '600' }}>
              {contestTimes.isUpcoming ? 'Countdown to Start' : contestTimes.isEnded ? 'Contest Ended' : 'Time Remaining'}
            </span>
            <strong style={{
              fontSize: '1.35rem',
              fontWeight: '750',
              color: contestTimes.isEnded ? '#f87171' : '#fff',
              fontVariantNumeric: 'tabular-nums',
              fontFamily: 'monospace'
            }}>
              {contestTimes.remainingText}
            </strong>
          </div>

          {/* Sign Out Option */}
          {user && (
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '0.6rem',
              padding: '0.35rem 0.5rem 0.35rem 0.35rem',
              borderRadius: '999px',
              background: 'rgba(255, 255, 255, 0.08)',
              border: '1px solid rgba(255, 255, 255, 0.12)',
            }}>
              <div style={{
                width: '28px',
                height: '28px',
                borderRadius: '50%',
                background: 'linear-gradient(135deg, #38bdf8, #0284c7)',
                color: '#fff',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '0.78rem',
                fontWeight: '700',
                flexShrink: 0
              }}>
                {user?.username?.charAt(0)?.toUpperCase() || '?'}
              </div>
              <span style={{
                fontSize: '0.78rem',
                fontWeight: '600',
                color: '#fff',
                maxWidth: '100px',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap'
              }}>
                {user?.username}
              </span>
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  logout();
                }}
                style={{
                  background: 'none',
                  border: 'none',
                  color: 'rgba(255, 255, 255, 0.6)',
                  cursor: 'pointer',
                  fontSize: '0.9rem',
                  padding: '0.15rem',
                  borderRadius: '50%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: '24px',
                  height: '24px',
                  transition: 'all 150ms'
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = '#f87171';
                  e.currentTarget.style.backgroundColor = 'rgba(239, 68, 68, 0.15)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = 'rgba(255, 255, 255, 0.6)';
                  e.currentTarget.style.backgroundColor = 'transparent';
                }}
                title="Sign out"
              >
                ↪
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Main Grid: Sidebar + Main Content (Wrapped in page padding & centering) */}
      <div style={{
        padding: '7rem 2rem 2rem',
        maxWidth: '85rem',
        width: '100%',
        margin: '0 auto',
        boxSizing: 'border-box',
        display: 'flex',
        flexDirection: 'column',
        gap: '1.5rem'
      }}>
        <div style={{ display: 'grid', gridTemplateColumns: '240px 1fr', gap: '1.5rem', alignItems: 'start' }}>
        
        {/* Left Sidebar */}
        <aside style={{ display: 'flex', flexDirection: 'column', gap: '1rem', position: 'sticky', top: '6rem' }}>
          
          {/* Problems List Card */}
          <div className="panel" style={{ padding: '1.25rem', borderRadius: '12px', background: '#fff' }}>
            <h3 style={{ fontSize: '0.85rem', fontWeight: '700', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-h)', borderBottom: '1px solid rgba(148,163,184,0.12)', paddingBottom: '0.75rem', marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '6px' }}>
              🧩 Problems
            </h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              {contestData?.problems && contestData.problems.length > 0 ? (
                contestData.problems.map((p, idx) => (
                  <button
                    key={p.ID}
                    onClick={() => setActiveProblemIdx(idx)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      width: '100%',
                      padding: '10px 12px',
                      borderRadius: '8px',
                      border: 'none',
                      cursor: 'pointer',
                      fontSize: '0.88rem',
                      fontWeight: '600',
                      transition: 'all 0.2s',
                      background: activeProblemIdx === idx ? 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)' : 'rgba(15,23,42,0.02)',
                      color: activeProblemIdx === idx ? '#fff' : 'var(--text-h)',
                      boxShadow: activeProblemIdx === idx ? '0 4px 12px rgba(99, 102, 241, 0.2)' : 'none',
                      textAlign: 'left'
                    }}
                  >
                    <span>
                      Problem {p.Code || String.fromCharCode(65 + idx)}
                    </span>
                    <span style={{ fontSize: '0.78rem', opacity: 0.8 }}>
                      {p.Title?.slice(0, 12) || 'Statement'}
                    </span>
                  </button>
                ))
              ) : (
                <div style={{ fontSize: '0.85rem', color: 'var(--muted)', textAlign: 'center', padding: '1rem 0' }}>
                  No problems in this contest.
                </div>
              )}
            </div>
          </div>

          {/* Back to Contests Button */}
          <Link
            className="button button-secondary"
            style={{
              textAlign: 'center',
              fontSize: '0.82rem',
              padding: '10px 12px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '6px',
              background: '#fff',
              border: '1px solid rgba(148, 163, 184, 0.25)',
              borderRadius: '12px',
              textDecoration: 'none'
            }}
            to="/contests"
          >
            🔙 Back to Contests
          </Link>

          {/* Important Notice */}
          <div style={{
            padding: '1rem',
            background: 'rgba(245, 158, 11, 0.05)',
            border: '1px solid rgba(245, 158, 11, 0.15)',
            borderLeft: '4px solid #f59e0b',
            borderRadius: '12px',
            fontSize: '0.8rem',
            color: '#d97706',
            lineHeight: '1.5',
            display: 'flex',
            flexDirection: 'column',
            gap: '6px'
          }}>
            <span style={{ fontWeight: '700', display: 'flex', alignItems: 'center', gap: '6px', textTransform: 'uppercase', fontSize: '0.75rem', letterSpacing: '0.03em' }}>
              ⚠️ Important Point
            </span>
            <span>
              The <strong>latest submitted code file</strong> for each problem will be tested for post-contest evaluation.
            </span>
          </div>

        </aside>

        {/* Right Content Area */}
        <main style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', minWidth: 0 }}>
          
          {/* Main Statement & Form Layout */}
          <div style={{ display: 'grid', gridTemplateColumns: '1.3fr 1fr', gap: '1.25rem', alignItems: 'stretch' }}>
            
            {/* Problem Details Panel */}
            <section className="panel" style={{ padding: '1.5rem', borderRadius: '12px', background: '#fff', minHeight: '360px', display: 'flex', flexDirection: 'column' }}>
              {activeProblem ? (
                <>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '0.75rem' }}>
                    <span style={{ fontSize: '1.5rem' }}>🧩</span>
                    <h3 style={{ margin: 0, fontSize: '1.35rem', color: 'var(--text-h)', fontWeight: '700' }}>
                      {activeProblem.Code ? `${activeProblem.Code} — ` : ''}{activeProblem.Title}
                    </h3>
                  </div>

                  <div style={{ display: 'flex', gap: '8px', marginBottom: '1.25rem', flexWrap: 'wrap' }}>
                    <span style={{ fontSize: '0.78rem', background: 'rgba(99, 102, 241, 0.06)', color: '#6366f1', padding: '4px 10px', borderRadius: '6px', fontWeight: '600', border: '1px solid rgba(99, 102, 241, 0.1)' }}>
                      ⏱️ Time Limit: {activeProblem.TimeLimit || 1}s
                    </span>
                    <span style={{ fontSize: '0.78rem', background: 'rgba(16, 185, 129, 0.06)', color: '#10b981', padding: '4px 10px', borderRadius: '6px', fontWeight: '600', border: '1px solid rgba(16, 185, 129, 0.1)' }}>
                      💾 Memory Limit: {activeProblem.MemoryLimit || 256}MB
                    </span>
                  </div>

                  <div style={{
                    fontSize: '0.92rem',
                    color: 'var(--text)',
                    lineHeight: '1.65',
                    whiteSpace: 'pre-wrap',
                    background: 'rgba(15, 23, 42, 0.015)',
                    border: '1px solid rgba(148, 163, 184, 0.1)',
                    padding: '1.25rem',
                    borderRadius: '10px',
                    flex: 1,
                    minHeight: '180px',
                    maxHeight: '380px',
                    overflowY: 'auto',
                    marginBottom: '1.25rem',
                    textAlign: 'left'
                  }}>
                    {activeProblem.Statement || 'No statement details.'}
                  </div>

                  {/* Strategies details */}
                  <div style={{
                    background: 'rgba(37, 99, 235, 0.03)',
                    border: '1px solid rgba(37, 99, 235, 0.1)',
                    padding: '1.15rem',
                    borderRadius: '10px',
                    fontSize: '0.85rem',
                    textAlign: 'left'
                  }}>
                    <h5 style={{ margin: '0 0 6px 0', color: 'var(--accent)', fontWeight: '700', fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: '4px' }}>
                      🎯 Baseline Testing Profile
                    </h5>
                    <p style={{ margin: 0, color: 'var(--text)', lineHeight: '1.4' }}>
                      This problem runs with baseline strategies: <strong style={{ color: 'var(--text-h)' }}>{[...(activeProblem.SampleStrategies || []).map(s => s.replace(/_/g, ' ')), ...(activeProblem.SampleBotFilesJSON ? JSON.parse(activeProblem.SampleBotFilesJSON).map(f => f.name) : [])].join(', ') || 'None'}</strong>.
                    </p>
                  </div>
                </>
              ) : (
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--muted)' }}>
                  Loading problem...
                </div>
              )}
            </section>

            {/* Submission Section */}
            <section className="panel" style={{ padding: '1.5rem', borderRadius: '12px', background: '#fff', display: 'flex', flexDirection: 'column' }}>
              <h3 style={{ fontSize: '1rem', fontWeight: '700', color: 'var(--text-h)', marginBottom: '1.25rem', borderBottom: '1px solid rgba(148,163,184,0.12)', paddingBottom: '0.75rem', textAlign: 'left' }}>
                🚀 Submit Solution
              </h3>

              {contestTimes.isUpcoming ? (
                <div style={{ padding: '2.5rem 1rem', textAlign: 'center', background: 'rgba(15, 23, 42, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(148, 163, 184, 0.25)' }}>
                  <span style={{ fontSize: '2.5rem', display: 'block', marginBottom: '12px' }}>🔒</span>
                  <h4 style={{ margin: '0 0 8px 0', color: 'var(--text-h)', fontWeight: '600' }}>Submission Locked</h4>
                  <p style={{ margin: 0, fontSize: '0.88rem', color: 'var(--muted)' }}>
                    The contest has not started yet. Submissions will open automatically when the countdown reaches zero.
                  </p>
                </div>
              ) : contestTimes.isEnded ? (
                <div style={{ padding: '2.5rem 1rem', textAlign: 'center', background: 'rgba(239, 68, 68, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(239, 68, 68, 0.25)' }}>
                  <span style={{ fontSize: '2.5rem', display: 'block', marginBottom: '12px' }}>🏁</span>
                  <h4 style={{ margin: '0 0 8px 0', color: '#ef4444', fontWeight: '600' }}>Contest Ended</h4>
                  <p style={{ margin: 0, fontSize: '0.88rem', color: 'var(--muted)' }}>
                    This contest has closed. Submissions are locked and final scores are being computed.
                  </p>
                </div>
              ) : (
                <form onSubmit={handleSubmit} className="submit-form" style={{ display: 'flex', flexDirection: 'column', gap: '1.1rem', textAlign: 'left' }}>
                  <div className="field-grid" style={{ gridTemplateColumns: '1fr', gap: '1rem' }}>
                    <label className="field" style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
                      Exposed Port
                      <input required type="number" name="port" value={formData.port} onChange={handleChange} style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #dbe4f0', fontSize: '0.85rem' }} />
                    </label>

                    <label className="field" style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
                      Language
                      <select name="language" value={formData.language} onChange={handleChange} style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #dbe4f0', fontSize: '0.85rem' }}>
                        <option value="cpp">C++</option>
                        <option value="go">Go</option>
                        <option value="rust">Rust</option>
                        <option value="python">Python</option>
                      </select>
                    </label>

                    <label className="field" style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
                      Protocol
                      <select name="protocol" value={formData.protocol} onChange={handleChange} style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #dbe4f0', fontSize: '0.85rem' }}>
                        <option value="http">HTTP / REST</option>
                        <option value="tcp">Raw TCP</option>
                        <option value="fix">FIX Protocol</option>
                      </select>
                    </label>

                    <label className="field" style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
                      Ramp-Up (seconds)
                      <input type="number" name="rampUpSeconds" value={formData.rampUpSeconds} min="0" max="30" onChange={handleChange} style={{ padding: '8px 12px', borderRadius: '6px', border: '1px solid #dbe4f0', fontSize: '0.85rem' }} />
                    </label>
                  </div>

                  <div style={{
                    padding: '0.65rem 0.85rem',
                    background: 'rgba(245, 158, 11, 0.05)',
                    borderLeft: '3px solid #f59e0b',
                    borderRadius: '0.5rem',
                    fontSize: '0.78rem',
                    color: '#d97706',
                    lineHeight: '1.4'
                  }}>
                    <span>⚠️ <strong>Caution:</strong> Your engine responses should include <code>best_bid</code> and <code>best_ask</code> fields. To receive a correctness score, your engine must return JSON like: <code>{'{"status":"accepted","best_bid":99.95,"best_ask":100.05}'}</code> so the validator can verify your orderbook state after each order.</span>
                  </div>

                  <label className="field upload-field" style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text)' }}>
                    Source Code
                    <input required type="file" name="file" accept=".cpp,.cc,.cxx,.rs,.go,.py" onChange={handleChange} style={{ padding: '8px', border: '1px dashed #dbe4f0', borderRadius: '6px', fontSize: '0.85rem' }} />
                  </label>

                  {submitState.message ? (
                    <div className={`feedback ${submitState.type}`} style={{
                      padding: '0.75rem', borderRadius: '6px', fontSize: '0.85rem',
                      background: submitState.type === 'error' ? 'rgba(239, 68, 68, 0.08)' : 'rgba(34, 197, 94, 0.08)',
                      color: submitState.type === 'error' ? '#ef4444' : '#22c55e',
                      border: submitState.type === 'error' ? '1px solid rgba(239,68,68,0.2)' : '1px solid rgba(34,197,94,0.2)'
                    }}>{submitState.message}</div>
                  ) : null}

                  <div style={{ display: 'flex', gap: '1rem', marginTop: '1rem', flexWrap: 'wrap' }}>
                    <button type="submit" className="button button-primary" disabled={isSubmitting} style={{ flex: 2, padding: '10px 0', fontSize: '0.88rem' }}>
                      {isSubmitting ? 'Submitting...' : 'Submit Trading System →'}
                    </button>
                    <button type="button" className="button button-secondary" disabled={loadingHistory} onClick={fetchHistory} style={{ flex: 1, padding: '10px 0', fontSize: '0.88rem' }}>
                      {loadingHistory ? 'Loading...' : 'History'}
                    </button>
                  </div>
                </form>
              )}
            </section>
          </div>

          {/* Sandbox Running Output (Logs) */}
          {executionResult && (
            <section className="panel" style={{
              padding: '1.25rem 1.5rem',
              borderRadius: '12px',
              background: '#fff',
              textAlign: 'left',
              borderLeft: '4px solid #6366f1'
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
                <h4 style={{ margin: 0, fontSize: '0.9rem', color: 'var(--text-h)', fontWeight: '700', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  🖥️ Sandbox Deployment
                </h4>
                <button
                  type="button"
                  className="button button-secondary"
                  style={{ fontSize: '0.72rem', padding: '3px 10px' }}
                  onClick={handleCleanup}
                >
                  🗑️ Cleanup
                </button>
              </div>
              
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem 1.5rem', marginBottom: '0.75rem', fontSize: '0.82rem' }}>
                <div><span style={{ color: 'var(--text-muted)' }}>Pod:</span> <strong style={{ color: 'var(--text-h)', fontFamily: 'monospace', fontSize: '0.78rem' }}>{executionResult.pod_id}</strong></div>
                <div><span style={{ color: 'var(--text-muted)' }}>Service:</span> <strong style={{ color: 'var(--text-h)', fontFamily: 'monospace', fontSize: '0.78rem' }}>{executionResult.service_name}</strong></div>
                <div><span style={{ color: 'var(--text-muted)' }}>Phase:</span> <strong style={{ color: executionResult.phase === 'Running' ? '#22c55e' : '#ef4444', fontSize: '0.82rem' }}>{executionResult.phase}</strong></div>
                {executionResult.target_url && (
                  <div><span style={{ color: 'var(--text-muted)' }}>Target:</span> <strong style={{ color: 'var(--text-h)', fontFamily: 'monospace', fontSize: '0.78rem' }}>{executionResult.target_url}</strong></div>
                )}
              </div>

              {executionResult.output && (
                <div style={{
                  background: '#0f172a',
                  color: '#cbd5e1',
                  padding: '10px 12px',
                  borderRadius: '8px',
                  fontFamily: 'monospace',
                  fontSize: '0.75rem',
                  maxHeight: '140px',
                  overflowY: 'auto',
                  lineHeight: '1.5'
                }}>
                  <span style={{ display: 'block', borderBottom: '1px solid rgba(255,255,255,0.08)', paddingBottom: '4px', marginBottom: '6px', color: '#64748b', fontSize: '0.68rem', textTransform: 'uppercase', letterSpacing: '0.04em' }}>Sandbox logs</span>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{executionResult.output}</pre>
                </div>
              )}
            </section>
          )}

          {/* Stress Test Outcomes Panel */}
          {stressTestResult && (
            <section className="panel" style={{
              padding: '1.25rem 1.5rem',
              borderRadius: '12px',
              background: '#fff',
              textAlign: 'left',
              borderLeft: '4px solid #10b981'
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
                <h4 style={{ margin: 0, fontSize: '0.9rem', color: 'var(--text-h)', fontWeight: '700', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  📊 Stress Test Report
                </h4>
                {stressTestMeta && (
                  <div style={{ display: 'flex', gap: '0.4rem' }}>
                    <span style={{ background: 'rgba(99, 102, 241, 0.1)', color: 'var(--accent)', padding: '2px 8px', borderRadius: '4px', fontSize: '0.7rem', fontWeight: 700, textTransform: 'uppercase' }}>
                      ⚡ {stressTestMeta.judgingMode === 'practice' ? 'Practice' : stressTestMeta.judgingMode}
                    </span>
                    {stressTestMeta.seedUsed != null && (
                      <span style={{ background: 'rgba(15,23,42,0.03)', color: 'var(--muted)', padding: '2px 8px', borderRadius: '4px', fontSize: '0.7rem', fontFamily: 'monospace' }}>
                        seed: 0x{stressTestMeta.seedUsed.toString(16)}
                      </span>
                    )}
                  </div>
                )}
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(120px, 1fr))', gap: '0.6rem', marginBottom: '1rem' }}>
                {[
                  { label: 'Strategy', value: strategyLabel(stressTestResult.strategy), color: 'var(--text-h)' },
                  { label: 'Requests', value: stressTestResult.requests, color: 'var(--text-h)' },
                  { label: 'Successes', value: stressTestResult.successes, color: '#22c55e' },
                  { label: 'Failures', value: stressTestResult.failures, color: stressTestResult.failures > 0 ? '#ef4444' : 'var(--text-h)' },
                  { label: 'TPS', value: stressTestResult.requests_per_second != null ? stressTestResult.requests_per_second.toFixed(1) : '—', color: 'var(--text-h)' },
                ].map((item) => (
                  <div key={item.label} style={{
                    padding: '0.6rem 0.75rem',
                    background: 'rgba(15,23,42,0.015)',
                    border: '1px solid rgba(148,163,184,0.08)',
                    borderRadius: '8px'
                  }}>
                    <span style={{ fontSize: '0.68rem', color: 'var(--text-muted)', display: 'block', textTransform: 'uppercase', letterSpacing: '0.03em', marginBottom: '2px' }}>{item.label}</span>
                    <strong style={{ fontSize: '0.88rem', color: item.color }}>{item.value}</strong>
                  </div>
                ))}
              </div>

              <div style={{
                background: 'rgba(15,23,42,0.02)',
                padding: '0.75rem 1rem',
                borderRadius: '8px',
                border: '1px solid rgba(148,163,184,0.08)',
                fontSize: '0.78rem',
                fontFamily: 'monospace'
              }}>
                <span style={{ display: 'block', fontSize: '0.7rem', fontFamily: 'var(--sans)', color: 'var(--text-muted)', marginBottom: '0.4rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.03em' }}>Latency Breakdown</span>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(100px, 1fr))', gap: '4px 12px' }}>
                  <div>Min: {stressTestResult.min_latency_ms != null ? `${stressTestResult.min_latency_ms.toFixed(2)}ms` : '—'}</div>
                  <div>Avg: {stressTestResult.avg_latency_ms != null ? `${stressTestResult.avg_latency_ms.toFixed(2)}ms` : '—'}</div>
                  <div>P50: {stressTestResult.p50_latency_ms != null ? `${stressTestResult.p50_latency_ms.toFixed(2)}ms` : '—'}</div>
                  <div>P90: {stressTestResult.p90_latency_ms != null ? `${stressTestResult.p90_latency_ms.toFixed(2)}ms` : '—'}</div>
                  <div>P99: {stressTestResult.p99_latency_ms != null ? `${stressTestResult.p99_latency_ms.toFixed(2)}ms` : '—'}</div>
                  <div>Max: {stressTestResult.max_latency_ms != null ? `${stressTestResult.max_latency_ms.toFixed(2)}ms` : '—'}</div>
                </div>
              </div>

              {stressTestResult.error_breakdown && Object.keys(stressTestResult.error_breakdown).length > 0 && (
                <div style={{ marginTop: '0.6rem', background: 'rgba(239, 68, 68, 0.02)', border: '1px solid rgba(239, 68, 68, 0.1)', padding: '0.6rem 0.75rem', borderRadius: '8px', fontSize: '0.78rem', fontFamily: 'monospace' }}>
                  <span style={{ display: 'block', color: '#ef4444', fontFamily: 'var(--sans)', fontWeight: 600, marginBottom: '0.2rem', fontSize: '0.72rem', textTransform: 'uppercase' }}>Error breakdown</span>
                  {Object.entries(stressTestResult.error_breakdown).map(([kind, count]) => (
                    <div key={kind}>{kind}: {count}</div>
                  ))}
                </div>
              )}

              {stressTestMeta?.correctnessHint && (
                <div style={{
                  background: 'rgba(245, 158, 11, 0.06)',
                  border: '1px solid rgba(245, 158, 11, 0.2)',
                  borderRadius: '8px',
                  padding: '0.6rem 0.85rem',
                  marginTop: '0.6rem',
                  fontSize: '0.82rem',
                  lineHeight: '1.45',
                  color: '#d97706',
                }}>
                  <strong style={{ display: 'block', marginBottom: '0.1rem', fontSize: '0.75rem' }}>⚠️ Correctness Hint</strong>
                  {stressTestMeta.correctnessHint}
                </div>
              )}
            </section>
          )}

        </main>
      </div>

      {showHistory && (
        <HistoryModal
          history={history}
          systemName={formData.systemName}
          onClose={() => setShowHistory(false)}
          onSelect={(run) => setSelectedSubmission(run)}
        />
      )}

      {selectedSubmission && (
        <SubmissionDetail
          submission={selectedSubmission}
          onClose={() => setSelectedSubmission(null)}
        />
      )}
      
      </div>
    </div>
  );
}
