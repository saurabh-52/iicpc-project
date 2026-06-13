import { useState, useEffect, useMemo } from 'react';
import { Link, useLocation } from 'react-router-dom';
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
  const displayName = (entry) => {
    if (entry.system_name && entry.system_name.trim()) return entry.system_name;
    return entry.submission_id?.slice(0, 18) || '—';
  };
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
              <strong>{submission.total_score?.toFixed(1) || '0.0'}</strong>
              <span>/100</span>
            </div>
          </div>

          <div className="detail-breakdown">
            <ScoreBar label="Latency" value={submission.latency_score || 0} max={25} color="#3b82f6" />
            <ScoreBar label="Throughput" value={submission.throughput_score || 0} max={25} color="#10b981" />
            <ScoreBar label="Correctness" value={submission.correctness_score || 0} max={50} color="#f59e0b" />
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

function HistoryModal({ history, systemName, onClose, onSelect }) {
  return (
    <div className="detail-overlay" onClick={onClose}>
      <div className="detail-card panel" style={{ maxWidth: '52rem', width: '92%', maxHeight: '85vh', display: 'flex', flexDirection: 'column', padding: '2rem' }} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem', flexShrink: 0 }}>
          <div style={{ textAlign: 'left' }}>
            <span className="section-tag">Performance Archive</span>
            <h2 style={{ fontSize: '1.5rem', margin: '0.2rem 0 0', letterSpacing: '-0.02em', color: 'var(--text-h)' }}>
              Submission History
            </h2>
            <p style={{ margin: '0.2rem 0 0', fontSize: '0.85rem', color: 'var(--muted)' }}>
              Showing runs for system "{systemName}"
            </p>
          </div>
          <button className="detail-close" onClick={onClose} aria-label="Close">✕</button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', minHeight: 0, marginBottom: '0.5rem' }}>
          {history.length === 0 ? (
            <div style={{ padding: '3rem 1rem', textAlign: 'center', background: 'rgba(15, 23, 42, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(148, 163, 184, 0.25)' }}>
              <p style={{ color: 'var(--muted)', margin: 0, fontSize: '0.95rem' }}>No practice submissions found for "{systemName}".</p>
            </div>
          ) : (
            <div className="leaderboard-table-wrap" style={{ border: '1px solid rgba(148, 163, 184, 0.15)', background: '#fff' }}>
              <table className="leaderboard-table">
                <thead>
                  <tr>
                    <th style={{ padding: '0.85rem 1rem' }}>Date</th>
                    <th style={{ padding: '0.85rem 1rem' }}>Context</th>
                    <th style={{ padding: '0.85rem 1rem' }}>Strategy</th>
                    <th style={{ padding: '0.85rem 1rem' }}>Grade</th>
                    <th style={{ padding: '0.85rem 1rem' }}>Score</th>
                    <th style={{ padding: '0.85rem 1rem' }}>TPS</th>
                    <th style={{ padding: '0.85rem 1rem' }}>P99 Latency</th>
                  </tr>
                </thead>
                <tbody>
                  {history.map(run => (
                    <tr 
                      key={run.submission_id} 
                      className="lb-row" 
                      onClick={() => onSelect(run)}
                      style={{ cursor: 'pointer' }}
                      title="Click to view details"
                    >
                      <td style={{ padding: '0.85rem 1rem', color: '#64748b', fontSize: '0.85rem' }}>
                        {new Date(run.submitted_at).toLocaleDateString()} {new Date(run.submitted_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                      </td>
                      <td style={{ padding: '0.85rem 1rem', fontSize: '0.85rem' }}>
                        {run.contest_id ? (
                          <span style={{ color: '#ef4444', fontWeight: '600' }}>Contest: {run.contest_id}</span>
                        ) : run.judging_mode === 'practice' ? (
                          <span style={{ color: 'var(--accent)', fontWeight: '600' }}>Practice</span>
                        ) : (
                          <span style={{ color: '#94a3b8', fontStyle: 'italic' }}>Unknown</span>
                        )}
                      </td>
                      <td className="strategy-cell" style={{ padding: '0.85rem 1rem' }}>
                        {strategyLabel(run.strategy)}
                      </td>
                      <td style={{ padding: '0.6rem 1rem' }}>
                        <GradeBadge grade={run.grade} />
                      </td>
                      <td className="score-cell" style={{ padding: '0.85rem 1rem' }}>
                        {run.total_score?.toFixed(1)}
                      </td>
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600 }}>
                        {formatTPS(run.tps)}
                      </td>
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600 }}>
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

export default function SubmitPage() {
  const { user, authHeaders } = useAuth();
  const languageExtensions = {
    cpp: ['cpp', 'cc', 'cxx'],
    go: ['go'],
    rust: ['rs'],
    python: ['py'],
  };

  const location = useLocation();
  const queryParams = new URLSearchParams(location.search);
  const urlContestId = queryParams.get('contest_id') || '';
  const urlStrategy = queryParams.get('strategy') || '';

  const [formData, setFormData] = useState({
    systemName: user?.username || '',
    port: '8080',
    language: 'cpp',
    protocol: 'http',
    strategy: urlStrategy || 'bbo_heavy',
    rampUpSeconds: '0',
    file: null,
    contestId: urlContestId
  });
  const [submitState, setSubmitState] = useState({ type: '', message: '' });
  const [executionResult, setExecutionResult] = useState(null);
  const [stressTestResult, setStressTestResult] = useState(null);
  const [stressTestMeta, setStressTestMeta] = useState(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [history, setHistory] = useState([]);
  const [showHistory, setShowHistory] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [selectedSubmission, setSelectedSubmission] = useState(null);
  const [contestData, setContestData] = useState(null);
  const [activeProblemIdx, setActiveProblemIdx] = useState(0);

  useEffect(() => {
    if (urlContestId) {
      fetch(`/api/contests/${urlContestId}/public`)
        .then(res => res.json())
        .then(data => {
          if (data && data.details) {
            setContestData(data);
            if (data.details.strategy) {
              setFormData(prev => ({ ...prev, strategy: data.details.strategy }));
            } else if (data.problems && data.problems.length > 0) {
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
          }
        })
        .catch(console.error);
    }
  }, [urlContestId]);

  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  const isContestUpcoming = useMemo(() => {
    if (!contestData || !contestData.details) return false;
    const start = new Date(contestData.details.startTime);
    return new Date() < start;
  }, [contestData]);

  const isContestEnded = useMemo(() => {
    if (!contestData || !contestData.details) return false;
    const end = new Date(new Date(contestData.details.startTime).getTime() + (contestData.details.durationMinutes || 60) * 60000);
    return new Date() >= end;
  }, [contestData]);

  const activeProblem = useMemo(() => {
    if (!contestData?.problems || contestData.problems.length === 0) return null;
    return contestData.problems[activeProblemIdx] || contestData.problems[0];
  }, [contestData, activeProblemIdx]);

  const getRemainingTime = () => {
    if (!contestData?.details?.startTime) return '';
    const start = new Date(contestData.details.startTime);
    const end = new Date(start.getTime() + (contestData.details.durationMinutes || 60) * 60000);
    
    if (now < start.getTime()) {
      const diff = start.getTime() - now;
      const d = Math.floor(diff / 86400000);
      const h = Math.floor((diff % 86400000) / 3600000);
      const m = Math.floor((diff % 3600000) / 60000);
      const s = Math.floor((diff % 60000) / 1000);
      if (d > 0) return `Starts in ${d}d ${h}h`;
      return `Starts in ${h}h ${m}m ${s}s`;
    }

    const diff = end.getTime() - now;
    if (diff <= 0) return 'Ended';
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    const s = Math.floor((diff % 60000) / 1000);
    return `${h}h ${m}m ${s}s`;
  };

  // Auto-update systemName when user changes
  useEffect(() => {
    if (user?.username && !formData.systemName) {
      setFormData(prev => ({ ...prev, systemName: user.username }));
    }
  }, [user]);

  const fetchHistory = async () => {
    setLoadingHistory(true);
    setShowHistory(true);
    try {
      const res = await fetch('/api/history/me?limit=20&judging_mode=practice', { headers: authHeaders() });
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

  const handleChange = (e) => {
    const { name, value, files } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: files ? files[0] : value
    }));
  };

  const handleCleanup = async () => {
    if (!executionResult?.pod_id) return;
    try {
      await fetch(`/api/sandbox/${executionResult.pod_id}`, { method: 'DELETE' });
      setSubmitState({ type: 'success', message: 'Sandbox cleaned up.' });
      setExecutionResult(null);
      setStressTestResult(null);
    } catch {
      setSubmitState({ type: 'error', message: 'Cleanup failed.' });
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
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
        body: payload, // Do NOT set Content-Type header, the browser handles the boundary
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

      // Trigger stress test when the engine is actively Running (not Succeeded).
      // Trading engines are long-running servers — they never "complete".
      // The backend now returns a target_url based on the NodePort.
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
            judging_mode: formData.contestId ? 'contest_live' : 'practice',
            contest_id: formData.contestId || undefined,
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
          judgingMode: stressResult?.rounds?.[0]?.judging_mode || 'practice',
          seedUsed: stressResult?.rounds?.[0]?.seed_used || null,
          correctnessHint: stressResult?.rounds?.[0]?.correctness_hint || null,
        });

        const roundCount = stressResult?.rounds?.length || 1;
        const strategies = (stressResult?.rounds || []).map(r => r.strategy).join(' + ');
        setSubmitState({
          type: 'success',
          message: `${result?.message || 'Engine submitted successfully.'} Stress test completed: ${roundCount} round(s) [${strategies || formData.strategy}].`,
        });
      } else if (sandboxExecution) {
        setSubmitState({
          type: 'success',
          message: `${result?.message || 'Engine submitted successfully.'} Stress test skipped because the sandbox did not report success (phase: ${sandboxExecution.phase}).`,
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

  return (
    <section className="submit-layout">
      <aside className="panel submit-aside">
        <span className="section-tag">Submit engine</span>
        {contestData?.details ? (
          <div style={{ marginBottom: '1.5rem', padding: '1rem', background: 'rgba(255,255,255,0.03)', borderRadius: '8px', border: '1px solid rgba(255,255,255,0.1)' }}>
            <h2 style={{ marginTop: 0, marginBottom: '0.5rem', fontSize: '1.4rem' }}>{contestData.details.name}</h2>
            <h3 style={{ margin: '0 0 0.5rem 0', fontSize: '1rem', color: 'var(--text-muted)' }}>Task Description</h3>
            <div style={{ fontSize: '0.9rem', color: 'var(--text)', whiteSpace: 'pre-wrap' }}>
              {contestData.details.description || "No description provided by host."}
            </div>
          </div>
        ) : (
          <>
            <h2>Upload a trading system for sandbox execution.</h2>
            <p>
              Provide a source file, expose the port, and choose a stress profile so the platform can
              package and run the engine in isolation.
            </p>
          </>
        )}

        {formData.contestId ? (
          <div style={{
            background: 'rgba(239, 68, 68, 0.1)',
            border: '1px solid rgba(239, 68, 68, 0.3)',
            borderRadius: '8px',
            padding: '0.75rem 1rem',
            marginBottom: '1rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}>
            <span style={{ fontSize: '1.2rem' }}>🔴</span>
            <div>
              <strong style={{ color: '#ef4444', fontSize: '0.85rem' }}>Contest Live Mode</strong>
              <p style={{ margin: 0, fontSize: '0.78rem', opacity: 0.8 }}>Submitting for contest: {formData.contestId}. Scores will have 20% random variance.</p>
            </div>
          </div>
        ) : (
          <div style={{
            background: 'rgba(99, 102, 241, 0.1)',
            border: '1px solid rgba(99, 102, 241, 0.3)',
            borderRadius: '8px',
            padding: '0.75rem 1rem',
            marginBottom: '1rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}>
            <span style={{ fontSize: '1.2rem' }}>⚡</span>
            <div>
              <strong style={{ color: 'var(--accent)', fontSize: '0.85rem' }}>Practice Mode</strong>
              <p style={{ margin: 0, fontSize: '0.78rem', opacity: 0.8 }}>Deterministic results — same code, same input, same output every time.</p>
            </div>
          </div>
        )}

        {formData.contestId && contestData?.details && (
          <div style={{
            background: 'rgba(15, 23, 42, 0.02)',
            border: '1px solid rgba(148, 163, 184, 0.2)',
            borderRadius: '8px',
            padding: '1rem',
            marginBottom: '1.5rem',
            textAlign: 'center'
          }}>
            <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '0.25rem', textTransform: 'uppercase', letterSpacing: '0.05em', fontWeight: '600' }}>
              {isContestUpcoming ? 'Contest Status' : isContestEnded ? 'Contest Status' : 'Time Remaining'}
            </div>
            <div style={{ fontSize: '1.75rem', fontWeight: '700', color: isContestEnded ? '#ef4444' : 'var(--text-h)', fontVariantNumeric: 'tabular-nums' }}>
              {getRemainingTime()}
            </div>
          </div>
        )}

        <div style={{ display: 'flex', gap: '0.5rem', flexDirection: 'column' }}>
          {formData.contestId && (
            <Link className="button button-primary" style={{ textAlign: 'center' }} to={`/leaderboard?contest_id=${formData.contestId}`}>
              Go to Leaderboard
            </Link>
          )}
          <Link className="button button-secondary" style={{ textAlign: 'center' }} to="/">
            Back to dashboard
          </Link>
        </div>
      </aside>

      {formData.contestId ? (
        contestData?.problems && contestData.problems.length > 0 ? (
          <section className="panel form-card" style={{ maxWidth: '64rem' }}>
            <div className="section-header" style={{ borderBottom: '1px solid rgba(148,163,184,0.12)', paddingBottom: '1rem', marginBottom: '1.5rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%', flexWrap: 'wrap', gap: '1rem' }}>
                <div>
                  <span className="section-tag">Contest Workspace</span>
                  <h3 style={{ margin: '0.25rem 0 0 0' }}>Problems & Submissions</h3>
                </div>
                {/* Tabs for problems */}
                <div className="cw-tab-group" style={{ display: 'flex', gap: '8px', background: 'rgba(15,23,42,0.03)', padding: '4px', borderRadius: '12px' }}>
                  {contestData.problems.map((p, idx) => (
                    <button
                      key={p.ID}
                      type="button"
                      className={`cp-tab ${activeProblemIdx === idx ? 'active' : ''}`}
                      onClick={() => setActiveProblemIdx(idx)}
                      style={{
                        padding: '6px 16px',
                        borderRadius: '8px',
                        border: 'none',
                        cursor: 'pointer',
                        fontSize: '0.88rem',
                        fontWeight: '600',
                        background: activeProblemIdx === idx ? '#fff' : 'transparent',
                        color: activeProblemIdx === idx ? 'var(--text-h, #0f172a)' : 'var(--muted, #64748b)',
                        boxShadow: activeProblemIdx === idx ? '0 1px 3px rgba(0,0,0,0.05)' : 'none',
                        transition: 'all 0.2s'
                      }}
                    >
                      Problem {p.Code || String.fromCharCode(65 + idx)}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr', gap: '2rem' }}>
              {/* Left side: Selected Problem Statement */}
              <div style={{ borderRight: '1px solid rgba(148, 163, 184, 0.12)', paddingRight: '2rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '0.5rem' }}>
                  <span style={{ fontSize: '1.5rem' }}>🧩</span>
                  <h4 style={{ margin: 0, fontSize: '1.2rem', color: 'var(--text-h)' }}>
                    {activeProblem?.Code ? `${activeProblem.Code} — ` : ''}{activeProblem?.Title || 'Untitled Problem'}
                  </h4>
                </div>
                
                <div style={{ display: 'flex', gap: '12px', marginBottom: '1.25rem', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '0.82rem', background: 'rgba(99, 102, 241, 0.06)', color: '#6366f1', padding: '4px 8px', borderRadius: '6px', fontWeight: '500' }}>
                    ⏱️ Time Limit: {activeProblem?.TimeLimit || 1}s
                  </span>
                  <span style={{ fontSize: '0.82rem', background: 'rgba(16, 185, 129, 0.06)', color: '#10b981', padding: '4px 8px', borderRadius: '6px', fontWeight: '500' }}>
                    💾 Memory Limit: {activeProblem?.MemoryLimit || 256}MB
                  </span>
                </div>

                <div style={{ 
                  fontSize: '0.92rem', 
                  color: 'var(--text)', 
                  lineHeight: '1.6', 
                  whiteSpace: 'pre-wrap', 
                  background: 'rgba(15, 23, 42, 0.01)',
                  border: '1px solid rgba(148, 163, 184, 0.1)',
                  padding: '1.25rem',
                  borderRadius: '12px',
                  maxHeight: '380px',
                  overflowY: 'auto',
                  marginBottom: '1.5rem'
                }}>
                  {activeProblem?.Statement || 'No statement details.'}
                </div>

                {/* Strategies details */}
                <div style={{ 
                  background: 'rgba(37, 99, 235, 0.02)',
                  border: '1px solid rgba(37, 99, 235, 0.1)',
                  padding: '1rem',
                  borderRadius: '8px',
                  fontSize: '0.85rem'
                }}>
                  <h5 style={{ margin: '0 0 6px 0', color: 'var(--accent)', fontWeight: '600' }}>🎯 Baseline Testing Profile</h5>
                  <p style={{ margin: 0, color: 'var(--text-muted)' }}>
                    This problem runs with baseline strategies: <strong>{[...(activeProblem?.SampleStrategies || []).map(s => s.replace(/_/g, ' ')), ...(activeProblem?.SampleBotFilesJSON ? JSON.parse(activeProblem.SampleBotFilesJSON).map(f => f.name) : [])].join(', ') || 'None'}</strong>.
                  </p>
                </div>
              </div>

              {/* Right side: Submission form */}
              <div>
                {isContestUpcoming ? (
                  <div style={{ padding: '2.5rem 1rem', textAlign: 'center', background: 'rgba(15, 23, 42, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(148, 163, 184, 0.25)' }}>
                    <span style={{ fontSize: '2.5rem', display: 'block', marginBottom: '12px' }}>🔒</span>
                    <h4 style={{ margin: '0 0 8px 0', color: 'var(--text-h)' }}>Submission Locked</h4>
                    <p style={{ margin: 0, fontSize: '0.88rem', color: 'var(--muted)' }}>
                      The contest has not started yet. Submissions will open automatically when the countdown reaches zero.
                    </p>
                  </div>
                ) : isContestEnded ? (
                  <div style={{ padding: '2.5rem 1rem', textAlign: 'center', background: 'rgba(239, 68, 68, 0.02)', borderRadius: '1rem', border: '1px dashed rgba(239, 68, 68, 0.25)' }}>
                    <span style={{ fontSize: '2.5rem', display: 'block', marginBottom: '12px' }}>🏁</span>
                    <h4 style={{ margin: '0 0 8px 0', color: '#ef4444' }}>Contest Ended</h4>
                    <p style={{ margin: 0, fontSize: '0.88rem', color: 'var(--muted)' }}>
                      This contest has closed. Submissions are locked and final scores are being computed.
                    </p>
                  </div>
                ) : (
                  <form onSubmit={handleSubmit} className="submit-form">
                    {/* Submission fields */}
                    <div className="field-grid" style={{ gridTemplateColumns: '1fr', gap: '1rem' }}>
                      <label className="field">
                        <span>Exposed Port</span>
                        <input required type="number" name="port" value={formData.port} onChange={handleChange} />
                      </label>

                      <label className="field">
                        <span>Language</span>
                        <select name="language" value={formData.language} onChange={handleChange}>
                          <option value="cpp">C++</option>
                          <option value="go">Go</option>
                          <option value="rust">Rust</option>
                          <option value="python">Python</option>
                        </select>
                      </label>

                      <label className="field">
                        <span>Protocol</span>
                        <select name="protocol" value={formData.protocol} onChange={handleChange}>
                          <option value="http">HTTP / REST</option>
                          <option value="tcp">Raw TCP</option>
                          <option value="fix">FIX Protocol</option>
                        </select>
                      </label>
                    </div>

                    <div style={{
                      marginTop: '0.5rem',
                      marginBottom: '1rem',
                      padding: '0.65rem 0.85rem',
                      background: 'rgba(99, 102, 241, 0.05)',
                      borderLeft: '3px solid #6366f1',
                      borderRadius: '0.5rem',
                      fontSize: '0.78rem',
                      color: '#4f46e5',
                      lineHeight: '1.4'
                    }}>
                      <span>ℹ️ <strong>Contest Mode:</strong> Bots fire using <strong>80% fixed and 20% random (time-based) seeds</strong> to simulate live market fluctuations.</span>
                    </div>

                    <div className="field-grid" style={{ gridTemplateColumns: '1fr' }}>
                      <label className="field">
                        <span>Ramp-Up (seconds)</span>
                        <input type="number" name="rampUpSeconds" value={formData.rampUpSeconds} min="0" max="30" onChange={handleChange} />
                      </label>
                    </div>

                    <label className="field upload-field">
                      <span>Source Code</span>
                      <input required type="file" name="file" accept=".cpp,.cc,.cxx,.rs,.go,.py" onChange={handleChange} />
                    </label>

                    {submitState.message ? (
                      <div className={`feedback ${submitState.type}`} style={{ marginBottom: '1rem' }}>{submitState.message}</div>
                    ) : null}

                    <div style={{ display: 'flex', gap: '1rem', marginTop: '1.5rem', flexWrap: 'wrap' }}>
                      <button type="submit" className="button button-primary submit-button" disabled={isSubmitting} style={{ flex: 2 }}>
                        {isSubmitting ? 'Submitting...' : 'Submit Trading System →'}
                      </button>
                      <button type="button" className="button button-secondary submit-button" disabled={loadingHistory} onClick={fetchHistory} style={{ flex: 1, padding: '1rem 0' }}>
                        {loadingHistory ? 'Loading...' : 'View History'}
                      </button>
                    </div>
                  </form>
                )}

                {/* Execution outcome display */}
                {executionResult && (
                  <section className="result-panel" style={{ marginTop: '1.5rem', padding: '1rem', borderRadius: '8px', background: 'rgba(15, 23, 42, 0.02)', border: '1px solid rgba(148, 163, 184, 0.15)' }}>
                    <div className="result-row">
                      <span>Pod ID</span>
                      <strong>{executionResult.pod_id}</strong>
                    </div>
                    <div className="result-row">
                      <span>Service</span>
                      <strong>{executionResult.service_name}</strong>
                    </div>
                    <div className="result-row">
                      <span>Phase</span>
                      <strong>{executionResult.phase}</strong>
                    </div>
                    {executionResult.node_port && (
                      <div className="result-row">
                        <span>NodePort</span>
                        <strong>{executionResult.node_port}</strong>
                      </div>
                    )}
                    {executionResult.target_url && (
                      <div className="result-row">
                        <span>Target</span>
                        <strong>{executionResult.target_url}</strong>
                      </div>
                    )}
                    {executionResult.output && (
                      <div className="result-output" style={{ marginTop: '0.5rem', maxHeight: '150px', overflowY: 'auto', fontSize: '0.78rem', background: '#0f172a', color: '#cbd5e1', padding: '8px', borderRadius: '4px', fontFamily: 'monospace' }}>
                        <span>Execution Output / Logs</span>
                        <pre style={{ margin: '4px 0 0 0', whiteSpace: 'pre-wrap' }}>{executionResult.output}</pre>
                      </div>
                    )}
                    <button
                      type="button"
                      className="button button-secondary"
                      style={{ marginTop: '0.5rem', fontSize: '0.85rem', padding: '0.6rem 1rem' }}
                      onClick={handleCleanup}
                    >
                      🗑️ Cleanup sandbox
                    </button>
                  </section>
                )}

                {stressTestResult && (
                  <section className="result-panel" style={{ marginTop: '1.5rem' }}>
                    {stressTestMeta ? (
                      <div style={{
                        display: 'flex',
                        gap: '0.5rem',
                        marginBottom: '0.75rem',
                        flexWrap: 'wrap',
                      }}>
                        <span style={{
                          background: 'rgba(99, 102, 241, 0.15)',
                          color: 'var(--accent)',
                          padding: '0.25rem 0.6rem',
                          borderRadius: '6px',
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          textTransform: 'uppercase',
                          letterSpacing: '0.5px',
                        }}>
                          ⚡ {stressTestMeta.judgingMode === 'practice' ? 'Practice' : stressTestMeta.judgingMode}
                        </span>
                        {stressTestMeta.seedUsed != null ? (
                          <span style={{
                            background: 'rgba(255, 255, 255, 0.05)',
                            color: 'var(--muted)',
                            padding: '0.25rem 0.6rem',
                            borderRadius: '6px',
                            fontSize: '0.75rem',
                            fontFamily: 'monospace',
                          }}>
                            seed: 0x{stressTestMeta.seedUsed.toString(16)}
                          </span>
                        ) : null}
                      </div>
                    ) : null}
                    <div className="result-row">
                      <span>Strategy</span>
                      <strong>{stressTestResult.strategy}</strong>
                    </div>
                    <div className="result-row">
                      <span>Target</span>
                      <strong>{stressTestResult.target}</strong>
                    </div>
                    <div className="result-row">
                      <span>Requests</span>
                      <strong>{stressTestResult.requests}</strong>
                    </div>
                    <div className="result-row">
                      <span>Successes</span>
                      <strong>{stressTestResult.successes}</strong>
                    </div>
                    <div className="result-row">
                      <span>Failures</span>
                      <strong>{stressTestResult.failures}</strong>
                    </div>
                    <div className="result-row">
                      <span>TPS</span>
                      <strong>
                        {stressTestResult.requests_per_second != null
                          ? stressTestResult.requests_per_second.toFixed(1)
                          : '—'}
                      </strong>
                    </div>
                    <div className="result-output">
                      <span>Latency summary</span>
                      <pre>
                        {stressTestResult.min_latency_ms != null
                          ? `min: ${stressTestResult.min_latency_ms.toFixed(2)}ms\n`
                          : ''}
                        {stressTestResult.avg_latency_ms != null
                          ? `avg: ${stressTestResult.avg_latency_ms.toFixed(2)}ms\n`
                          : ''}
                        {stressTestResult.p50_latency_ms != null
                          ? `p50: ${stressTestResult.p50_latency_ms.toFixed(2)}ms\n`
                          : ''}
                        {stressTestResult.p90_latency_ms != null
                          ? `p90: ${stressTestResult.p90_latency_ms.toFixed(2)}ms\n`
                          : ''}
                        {stressTestResult.p99_latency_ms != null
                          ? `p99: ${stressTestResult.p99_latency_ms.toFixed(2)}ms\n`
                          : ''}
                        {stressTestResult.max_latency_ms != null
                          ? `max: ${stressTestResult.max_latency_ms.toFixed(2)}ms\n`
                          : ''}
                        {stressTestResult.stddev_latency_ms != null
                          ? `σ:   ${stressTestResult.stddev_latency_ms.toFixed(2)}ms`
                          : ''}
                      </pre>
                    </div>

                    {stressTestResult.error_breakdown && Object.keys(stressTestResult.error_breakdown).length > 0 ? (
                      <div className="result-output">
                        <span>Error breakdown</span>
                        <pre>
                          {Object.entries(stressTestResult.error_breakdown)
                            .map(([kind, count]) => `${kind}: ${count}`)
                            .join('\n')}
                        </pre>
                      </div>
                    ) : null}

                    {stressTestMeta?.correctnessHint ? (
                      <div style={{
                        background: 'rgba(245, 158, 11, 0.1)',
                        border: '1px solid rgba(245, 158, 11, 0.3)',
                        borderRadius: '8px',
                        padding: '0.75rem 1rem',
                        marginTop: '0.75rem',
                        fontSize: '0.85rem',
                        lineHeight: '1.5',
                        color: '#fbbf24',
                      }}>
                        <strong style={{ display: 'block', marginBottom: '0.25rem' }}>⚠️ Correctness Hint</strong>
                        {stressTestMeta.correctnessHint}
                      </div>
                    ) : null}
                  </section>
                )}
              </div>
            </div>
          </section>
        ) : (
          <section className="panel form-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '300px' }}>
            <div style={{ textAlign: 'center', color: 'var(--muted)' }}>
              <span style={{ fontSize: '2rem' }}>🔄</span>
              <p>Loading contest details and problems...</p>
            </div>
          </section>
        )
      ) : (
        <section className="panel form-card">
          <div className="section-header">
            <div>
              <span className="section-tag">Submission form</span>
              <h3>Engine details</h3>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="submit-form">
            <div className="field-grid">
              <label className="field">
                <span>Exposed Port</span>
                <input required type="number" name="port" value={formData.port} onChange={handleChange} />
              </label>

              <label className="field">
                <span>Language</span>
                <select name="language" value={formData.language} onChange={handleChange}>
                  <option value="cpp">C++</option>
                  <option value="go">Go</option>
                  <option value="rust">Rust</option>
                  <option value="python">Python</option>
                </select>
              </label>

              <label className="field">
                <span>Protocol</span>
                <select name="protocol" value={formData.protocol} onChange={handleChange}>
                  <option value="http">HTTP / REST</option>
                  <option value="tcp">Raw TCP</option>
                  <option value="fix">FIX Protocol</option>
                </select>
              </label>
            </div>

            <label className="field">
              <span>Stress Test Strategy</span>
              <select
                name="strategy"
                value={formData.strategy}
                onChange={handleChange}
                disabled={isSubmitting}
              >
                <option value="bbo_heavy">BBO Heavy (Common)</option>
                <option value="flash_crash">Flash Crash</option>
                <option value="high_cancel">High Cancel Rate</option>
                <option value="wide_spread">Wide Spread</option>
                <option value="market_maker">Market Maker</option>
                <option value="iceberg">Iceberg Orders</option>
                <option value="momentum_burst">Momentum Burst</option>
              </select>
            </label>

            <div style={{
              marginTop: '-0.5rem',
              marginBottom: '1rem',
              padding: '0.65rem 0.85rem',
              background: 'rgba(99, 102, 241, 0.05)',
              borderLeft: '3px solid #6366f1',
              borderRadius: '0.5rem',
              fontSize: '0.78rem',
              color: '#4f46e5',
              lineHeight: '1.4'
            }}>
              <span>ℹ️ <strong>Practice Mode:</strong> Bots fire using <strong>100% fixed, deterministic seeds</strong> for fully reproducible test runs.</span>
            </div>

            <div className="field-grid" style={{ alignItems: 'center' }}>
              <label className="field">
                <span>Ramp-Up (seconds)</span>
                <input type="number" name="rampUpSeconds" value={formData.rampUpSeconds} min="0" max="30" onChange={handleChange} />
              </label>

              <div style={{
                gridColumn: 'span 2',
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
            </div>

            <label className="field upload-field">
              <span>Source Code</span>
              <input required type="file" name="file" accept=".cpp,.cc,.cxx,.rs,.go,.py" onChange={handleChange} />
            </label>

            {submitState.message ? (
              <div className={`feedback ${submitState.type}`}>{submitState.message}</div>
            ) : null}

            {executionResult ? (
              <section className="result-panel">
                <div className="result-row">
                  <span>Pod ID</span>
                  <strong>{executionResult.pod_id}</strong>
                </div>
                <div className="result-row">
                  <span>Service</span>
                  <strong>{executionResult.service_name}</strong>
                </div>
                <div className="result-row">
                  <span>Phase</span>
                  <strong>{executionResult.phase}</strong>
                </div>
                {executionResult.node_port ? (
                  <div className="result-row">
                    <span>NodePort</span>
                    <strong>{executionResult.node_port}</strong>
                  </div>
                ) : null}
                {executionResult.target_url ? (
                  <div className="result-row">
                    <span>Target</span>
                    <strong>{executionResult.target_url}</strong>
                  </div>
                ) : null}
                {executionResult.output ? (
                  <div className="result-output" style={{ marginTop: '0.5rem' }}>
                    <span>Execution Output / Logs</span>
                    <pre>{executionResult.output}</pre>
                  </div>
                ) : null}
                <button
                  type="button"
                  className="button button-secondary"
                  style={{ marginTop: '0.5rem', fontSize: '0.85rem', padding: '0.6rem 1rem' }}
                  onClick={handleCleanup}
                >
                  🗑️ Cleanup sandbox
                </button>
              </section>
            ) : null}

            {stressTestResult ? (
              <section className="result-panel">
                {stressTestMeta ? (
                  <div style={{
                    display: 'flex',
                    gap: '0.5rem',
                    marginBottom: '0.75rem',
                    flexWrap: 'wrap',
                  }}>
                    <span style={{
                      background: 'rgba(99, 102, 241, 0.15)',
                      color: 'var(--accent)',
                      padding: '0.25rem 0.6rem',
                      borderRadius: '6px',
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.5px',
                    }}>
                      ⚡ Practice
                    </span>
                    {stressTestMeta.seedUsed != null ? (
                      <span style={{
                        background: 'rgba(255, 255, 255, 0.05)',
                        color: 'var(--muted)',
                        padding: '0.25rem 0.6rem',
                        borderRadius: '6px',
                        fontSize: '0.75rem',
                        fontFamily: 'monospace',
                      }}>
                        seed: 0x{stressTestMeta.seedUsed.toString(16)}
                      </span>
                    ) : null}
                  </div>
                ) : null}
                <div className="result-row">
                  <span>Strategy</span>
                  <strong>{stressTestResult.strategy}</strong>
                </div>
                <div className="result-row">
                  <span>Target</span>
                  <strong>{stressTestResult.target}</strong>
                </div>
                <div className="result-row">
                  <span>Requests</span>
                  <strong>{stressTestResult.requests}</strong>
                </div>
                <div className="result-row">
                  <span>Successes</span>
                  <strong>{stressTestResult.successes}</strong>
                </div>
                <div className="result-row">
                  <span>Failures</span>
                  <strong>{stressTestResult.failures}</strong>
                </div>
                <div className="result-row">
                  <span>TPS</span>
                  <strong>
                    {stressTestResult.requests_per_second != null
                      ? stressTestResult.requests_per_second.toFixed(1)
                      : '—'}
                  </strong>
                </div>
                <div className="result-output">
                  <span>Latency summary</span>
                  <pre>
                    {stressTestResult.min_latency_ms != null
                      ? `min: ${stressTestResult.min_latency_ms.toFixed(2)}ms\n`
                      : ''}
                    {stressTestResult.avg_latency_ms != null
                      ? `avg: ${stressTestResult.avg_latency_ms.toFixed(2)}ms\n`
                      : ''}
                    {stressTestResult.p50_latency_ms != null
                      ? `p50: ${stressTestResult.p50_latency_ms.toFixed(2)}ms\n`
                      : ''}
                    {stressTestResult.p90_latency_ms != null
                      ? `p90: ${stressTestResult.p90_latency_ms.toFixed(2)}ms\n`
                      : ''}
                    {stressTestResult.p99_latency_ms != null
                      ? `p99: ${stressTestResult.p99_latency_ms.toFixed(2)}ms\n`
                      : ''}
                    {stressTestResult.max_latency_ms != null
                      ? `max: ${stressTestResult.max_latency_ms.toFixed(2)}ms\n`
                      : ''}
                    {stressTestResult.stddev_latency_ms != null
                      ? `σ:   ${stressTestResult.stddev_latency_ms.toFixed(2)}ms`
                      : ''}
                  </pre>
                </div>

                {stressTestResult.error_breakdown && Object.keys(stressTestResult.error_breakdown).length > 0 ? (
                  <div className="result-output">
                    <span>Error breakdown</span>
                    <pre>
                      {Object.entries(stressTestResult.error_breakdown)
                        .map(([kind, count]) => `${kind}: ${count}`)
                        .join('\n')}
                    </pre>
                  </div>
                ) : null}

                {stressTestMeta?.correctnessHint ? (
                  <div style={{
                    background: 'rgba(245, 158, 11, 0.1)',
                    border: '1px solid rgba(245, 158, 11, 0.3)',
                    borderRadius: '8px',
                    padding: '0.75rem 1rem',
                    marginTop: '0.75rem',
                    fontSize: '0.85rem',
                    lineHeight: '1.5',
                    color: '#fbbf24',
                  }}>
                    <strong style={{ display: 'block', marginBottom: '0.25rem' }}>⚠️ Correctness Hint</strong>
                    {stressTestMeta.correctnessHint}
                  </div>
                ) : null}
              </section>
            ) : null}

            <div style={{ display: 'flex', gap: '1rem', marginTop: '1.5rem', flexWrap: 'wrap' }}>
              <button type="submit" className="button button-primary submit-button" disabled={isSubmitting} style={{ flex: 2 }}>
                {isSubmitting ? 'Submitting...' : 'Deploy and launch stress test'}
              </button>
              <button type="button" className="button button-secondary submit-button" disabled={loadingHistory} onClick={fetchHistory} style={{ flex: 1, padding: '1rem 0' }}>
                {loadingHistory ? 'Loading...' : 'View My History'}
              </button>
            </div>
          </form>
        </section>
      )}

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
    </section>
  );
}