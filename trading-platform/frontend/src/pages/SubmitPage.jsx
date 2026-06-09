import { useState, useEffect } from 'react';
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
            <ScoreBar label="Latency" value={submission.latency_score || 0} max={50} color="#3b82f6" />
            <ScoreBar label="Throughput" value={submission.throughput_score || 0} max={30} color="#10b981" />
            <ScoreBar label="Correctness" value={submission.correctness_score || 0} max={20} color="#f59e0b" />
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
      const res = await fetch('/api/history/me?limit=20', { headers: authHeaders() });
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
            bots: 16,
            requests: 48,
            timeout_ms: 2000,
            method: 'POST',
            path: '/',
            expect_reply: formData.protocol === 'tcp' || formData.protocol === 'fix',
            ramp_up_seconds: parseInt(formData.rampUpSeconds) || 0,
            judging_mode: formData.contestId ? 'contest_live' : 'practice',
            contest_id: formData.contestId || undefined,
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
        <h2>Upload a trading system for sandbox execution.</h2>
        <p>
          Provide a source file, expose the port, and choose a stress profile so the platform can
          package and run the engine in isolation.
        </p>

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

        <div className="checklist">
          <div>
            <strong>Accepted files</strong>
            <span>C++, Rust, Go, Python</span>
          </div>
          <div>
            <strong>Backend route</strong>
            <span>/submit</span>
          </div>
          <div>
            <strong>Upload method</strong>
            <span>Multipart form data</span>
          </div>
          <div>
            <strong>Judging mode</strong>
            <span>100% fixed seed (reproducible)</span>
          </div>
        </div>

        <Link className="button button-secondary" to="/">
          Back to dashboard
        </Link>
      </aside>

      <section className="panel form-card">
        <div className="section-header">
          <div>
            <span className="section-tag">Submission form</span>
            <h3>Engine details</h3>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="submit-form">
          <label className="field">
            <span>System Name</span>
            <input
              required
              type="text"
              name="systemName"
              value={formData.systemName}
              onChange={handleChange}
              placeholder="e.g., UltraFast-Matcher"
            />
          </label>

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
              disabled={!!formData.contestId || isSubmitting}
            >
              <option value="bbo_heavy">BBO Heavy (Common)</option>
              <option value="flash_crash">Flash Crash</option>
              <option value="high_cancel">High Cancel Rate</option>
              <option value="wide_spread">Wide Spread</option>
              <option value="market_maker">Market Maker</option>
              <option value="iceberg">Iceberg Orders</option>
              <option value="momentum_burst">Momentum Burst</option>
            </select>
            {formData.contestId && (
              <span style={{ fontSize: '0.75rem', color: '#888', marginTop: '4px', display: 'block' }}>
                Locked to contest strategy
              </span>
            )}
          </label>

          <div className="field-grid">
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
            </section>
          ) : null}

          <div style={{ display: 'flex', gap: '1rem', marginTop: '1.5rem' }}>
            <button type="submit" className="button button-primary submit-button" disabled={isSubmitting} style={{ flex: 2 }}>
              {isSubmitting ? 'Submitting...' : 'Deploy and launch stress test'}
            </button>
            <button type="button" className="button button-secondary submit-button" disabled={loadingHistory} onClick={fetchHistory} style={{ flex: 1, padding: '1rem 0' }}>
              {loadingHistory ? 'Loading...' : 'View My History'}
            </button>
          </div>
        </form>
      </section>

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