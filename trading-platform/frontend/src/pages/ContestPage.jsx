import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import useWebSocket from '../hooks/useWebSocket';

/* ═══════════════════════════════════════
   CONSTANTS
   ═══════════════════════════════════════ */

const STEP_TITLES = ['Contest Details', 'Problems', 'Review & Publish'];
const STEP_ICONS = ['📋', '🧩', '🚀'];
const STEP_DESCRIPTIONS = ['Set up your contest info', 'Add & configure problems', 'Final check before going live'];
const STRATEGIES = ['bbo_heavy', 'flash_crash', 'high_cancel', 'wide_spread', 'market_maker', 'iceberg', 'momentum_burst'];
const DRAFT_KEY = 'contest_draft_v1';

// Mock data for upcoming / past contests (replace with API later)
const MOCK_UPCOMING = [
  {
    id: 'c1',
    name: 'IICPC Qualifier Round',
    description: 'Qualify for the main event — top 50 teams advance.',
    startTime: new Date(Date.now() + 3 * 86400000).toISOString(),
    duration: 120,
    participants: 184,
    visibility: 'public',
    status: 'upcoming',
    registrationDeadline: new Date(Date.now() + 2 * 86400000).toISOString(),
  },
  {
    id: 'c2',
    name: 'Market Maker Challenge',
    description: 'Build the most profitable market-making engine under adversarial conditions.',
    startTime: new Date(Date.now() + 7 * 86400000).toISOString(),
    duration: 90,
    participants: 67,
    visibility: 'public',
    status: 'upcoming',
    registrationDeadline: new Date(Date.now() + 5 * 86400000).toISOString(),
  },
  {
    id: 'c3',
    name: 'Flash Crash Stress Test',
    description: 'Survive 60 minutes of extreme volatility and flash crash scenarios.',
    startTime: new Date(Date.now() + 14 * 86400000).toISOString(),
    duration: 60,
    participants: 42,
    visibility: 'private',
    status: 'upcoming',
    registrationDeadline: new Date(Date.now() + 12 * 86400000).toISOString(),
  },
];

const MOCK_PAST = [
  {
    id: 'p1',
    name: 'IICPC Practice Round #3',
    description: 'Community practice round with 4 problems.',
    startTime: new Date(Date.now() - 5 * 86400000).toISOString(),
    duration: 90,
    participants: 213,
    visibility: 'public',
    status: 'ended',
  },
  {
    id: 'p2',
    name: 'Spread Optimization Sprint',
    description: 'Optimize bid-ask spread in a simulated order book.',
    startTime: new Date(Date.now() - 12 * 86400000).toISOString(),
    duration: 60,
    participants: 98,
    visibility: 'public',
    status: 'ended',
  },
];

/* ═══════════════════════════════════════
   HELPERS
   ═══════════════════════════════════════ */

function uid(prefix = 'id') {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`;
}

function formatLocalDatetime(dateStr) {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function emptyProblem() {
  return {
    id: uid('prob'),
    code: '',
    title: '',
    statement: '',
    timeLimit: 1,
    memoryLimit: 256,
    sampleStrategies: [], // Selected default strategies
    sampleBotFiles: [],   // Uploaded custom Go bot files: { name, content }[]
    sampleShowCustom: false,
    sampleTargetInjection: 'env', // 'env' | 'arg'
    sampleProtocol: 'http',       // 'http' | 'tcp' | 'fix'
    sampleTelemetryFormat: 'stdout', // 'stdout' | 'callback'
    hiddenStrategies: [],  // Selected default strategies
    hiddenBotFiles: [],   // Uploaded custom Go bot files: { name, content }[]
    hiddenShowCustom: false,
    hiddenTargetInjection: 'env', // 'env' | 'arg'
    hiddenProtocol: 'http',       // 'http' | 'tcp' | 'fix'
    hiddenTelemetryFormat: 'stdout', // 'stdout' | 'callback'
  };
}

function formatDate(iso) {
  const d = new Date(iso);
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function formatTime(iso) {
  const d = new Date(iso);
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
}

function timeUntil(iso) {
  const diff = new Date(iso) - Date.now();
  if (diff <= 0) return 'Started';
  const days = Math.floor(diff / 86400000);
  const hours = Math.floor((diff % 86400000) / 3600000);
  if (days > 0) return `${days}d ${hours}h`;
  const mins = Math.floor((diff % 3600000) / 60000);
  if (hours > 0) return `${hours}h ${mins}m`;
  const secs = Math.floor((diff % 60000) / 1000);
  return `${mins}m ${secs}s`;
}

function CustomBotGuidelines() {
  return (
    <div className="custom-bot-guidelines" style={{
      padding: '16px',
      background: 'linear-gradient(135deg, rgba(217, 119, 6, 0.04) 0%, rgba(0, 0, 0, 0) 100%)',
      borderRadius: '8px',
      border: '1px dashed rgba(217, 119, 6, 0.3)',
      marginBottom: '16px',
      fontSize: '0.88rem',
      lineHeight: '1.5',
      color: 'var(--text, #4b5563)',
      textAlign: 'left'
    }}>
      <h5 style={{ margin: '0 0 10px 0', color: '#b45309', fontWeight: '600', display: 'flex', alignItems: 'center', gap: '6px' }}>
        💡 Custom Go Bot Requirements
      </h5>
      <ol style={{ margin: 0, paddingLeft: '16px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
        <li>
          <strong>Sandbox Network Configuration</strong>:
          <ul style={{ margin: '4px 0 0 0', paddingLeft: '16px', color: 'var(--text, #4b5563)', fontSize: '0.85rem', display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <li>
              <strong>Target Address Injection</strong>: Choose how your script discovers the target trading engine address:
              <ul style={{ margin: '2px 0 0 0', paddingLeft: '12px', listStyleType: 'circle' }}>
                <li>As an environment variable (e.g., <code>TARGET_URL</code>)</li>
                <li>Or as a command-line argument (e.g., <code>--target</code>)</li>
              </ul>
            </li>
            <li>
              <strong>Protocol Support</strong>: Select <code>HTTP</code>, <code>TCP</code>, or <code>FIX</code> so our network security policies allow traffic between the bot sandbox and the trading engine sandbox.
            </li>
          </ul>
        </li>
        <li>
          <strong>Self-Termination</strong>:
          <span style={{ display: 'block', color: 'var(--text, #4b5563)', fontSize: '0.85rem', marginTop: '2px' }}>
            The script must manage its own load lifecycle (e.g., duration limit like 60s or request count limit like 10,000 orders) and exit cleanly.
          </span>
        </li>
        <li>
          <strong>Telemetry & Scoring Format</strong>:
          <span style={{ display: 'block', color: 'var(--text, #4b5563)', fontSize: '0.85rem', marginTop: '2px' }}>
            To calculate scores (TPS, latency, correctness), the bot script must feed metrics back to the platform using one of these interfaces:
            <ul style={{ margin: '4px 0 0 0', paddingLeft: '16px', listStyleType: 'circle', marginBottom: '8px' }}>
              <li><strong>Structured stdout</strong>: Print request results to standard output (stdout) as newline-delimited JSON logs (e.g. <code>{"{"}"timestamp": "...", "latency_ms": 1.25, "success": true, "side": "BUY"{"}"}</code>) so our backend can parse logs.</li>
              <li><strong>Telemetry callback</strong>: Accept a callback URL (usually injected as <code>CALLBACK_URL</code> or <code>--callback</code>) where your script POSTs execution summaries.</li>
            </ul>
          </span>
        </li>
        <li>
          <strong>Bot Seeding & Firing (Recommendation)</strong>:
          <span style={{ display: 'block', color: 'var(--text, #4b5563)', fontSize: '0.85rem', marginTop: '2px' }}>
            When configuring bot scripts:
            <ul style={{ margin: '4px 0 0 0', paddingLeft: '16px', listStyleType: 'circle' }}>
              <li><strong>Sample test strategies</strong>: Recommend keeping the <strong>80/20 rule</strong> (80% fixed bots, 20% random bots with time-based seeds) to prevent users from reverse-engineering or hacking the test cases.</li>
              <li><strong>Hidden test strategies</strong>: Recommend enforcing <strong>100% fixed, deterministic seeds</strong> to guarantee absolute scoring fairness.</li>
            </ul>
          </span>
        </li>
      </ol>
    </div>
  );
}

/* ═══════════════════════════════════════
   CONTEST PAGE — Root
   ═══════════════════════════════════════ */

function normalizeContests(dbContests) {
  const normalizedDb = dbContests.map(c => {
    const startTimeStr = typeof c.startTime === 'string' ? c.startTime : new Date(c.startTime).toISOString();
    const start = new Date(startTimeStr);
    const duration = c.durationMinutes || 60;
    const end = new Date(start.getTime() + duration * 60000);
    const isEnded = new Date() > end;

    return {
      id: c.id,
      name: c.name,
      description: c.description,
      startTime: startTimeStr,
      duration: duration,
      participants: c.participants || 0,
      visibility: c.visibility || 'public',
      status: isEnded ? 'ended' : 'upcoming',
      phase: c.phase || 'upcoming',
      strategy: c.strategy || '',
      createdBy: c.createdBy || '',
      registrationDeadline: c.registrationDeadline || '',
      bannerPreview: c.bannerPreview || null,
    };
  });

  const all = [...normalizedDb];
  
  MOCK_UPCOMING.forEach(m => {
    if (!all.some(a => a.id === m.id)) {
      all.push({
        ...m,
        phase: 'upcoming',
        strategy: '',
        createdBy: '',
      });
    }
  });

  MOCK_PAST.forEach(m => {
    if (!all.some(a => a.id === m.id)) {
      all.push({
        ...m,
        phase: 'completed',
        strategy: '',
        createdBy: '',
      });
    }
  });

  const upcoming = all
    .filter(c => c.status === 'upcoming')
    .sort((a, b) => new Date(a.startTime) - new Date(b.startTime));

  const past = all
    .filter(c => c.status === 'ended')
    .sort((a, b) => new Date(b.startTime) - new Date(a.startTime));

  return { upcoming, past };
}

export default function ContestPage() {
  const { user, authHeaders } = useAuth();
  const [view, setView] = useState('landing');
  const [dbContests, setDbContests] = useState([]);
  const [loading, setLoading] = useState(false);
  const { finalizationProgress } = useWebSocket();

  const [registeredContestIds, setRegisteredContestIds] = useState([]);
  const [registeringContest, setRegisteringContest] = useState(null);
  const [editingContestData, setEditingContestData] = useState(null);
  const [regSystemName, setRegSystemName] = useState('');
  const [regCode, setRegCode] = useState('');
  const [regMessage, setRegMessage] = useState('');
  const [regSubmitting, setRegSubmitting] = useState(false);

  const [viewingParticipantsContest, setViewingParticipantsContest] = useState(null);
  const [participantsList, setParticipantsList] = useState([]);
  const [loadingParticipants, setLoadingParticipants] = useState(false);

  const [viewingDescriptionContest, setViewingDescriptionContest] = useState(null);

  const handleViewParticipants = async (contest) => {
    setViewingParticipantsContest(contest);
    setLoadingParticipants(true);
    setParticipantsList([]);
    try {
      const res = await fetch(`/api/contests/${contest.id}/participants`);
      if (res.ok) {
        const data = await res.json();
        setParticipantsList(Array.isArray(data.participants) ? data.participants : (Array.isArray(data) ? data : []));
      } else {
        console.error('Participants API error:', res.status);
        setParticipantsList([]);
      }
    } catch (e) {
      console.error("Failed to fetch participants:", e);
      setParticipantsList([]);
    } finally {
      setLoadingParticipants(false);
    }
  };

  // Auto-fill system name from user
  useEffect(() => {
    if (user?.username) {
      setRegSystemName(user.username);
    }
  }, [user]);

  const fetchContests = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/contests');
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data.contests)) {
          setDbContests(data.contests);
        }
      }
    } catch (e) {
      console.error('Failed to fetch contests:', e);
    } finally {
      setLoading(false);
    }
  };

  // Fetch the user's registered contest IDs from the backend
  const fetchMyRegistrations = async () => {
    try {
      const hdrs = authHeaders();
      if (!hdrs.Authorization) return; // not logged in
      const res = await fetch('/api/contests/my-registrations', { headers: hdrs });
      if (res.ok) {
        const data = await res.json();
        if (Array.isArray(data.contest_ids)) {
          setRegisteredContestIds(data.contest_ids);
        }
      }
    } catch (e) {
      console.error('Failed to fetch registrations:', e);
    }
  };

  useEffect(() => {
    if (view === 'landing' || view === 'join') {
      fetchContests();
      fetchMyRegistrations();
    }
  }, [view]);

  // Re-fetch registrations when user changes (login/logout)
  useEffect(() => {
    if (user) {
      fetchMyRegistrations();
    } else {
      setRegisteredContestIds([]);
    }
  }, [user]);

  const handleRegisterSuccess = (contestId) => {
    setRegisteredContestIds(prev => [...prev, contestId]);
    fetchContests();
  };

  const handleRegisterSubmit = async (e) => {
    e.preventDefault();
    if (!registeringContest) return;
    setRegSubmitting(true);
    setRegMessage('Registering...');
    try {
      const res = await fetch(`/api/contests/${registeringContest.id}/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ systemName: regSystemName, code: regCode })
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.message || data.error || 'Registration failed');
      }
      setRegMessage('Registered successfully ✓');
      
      handleRegisterSuccess(registeringContest.id);

      setTimeout(() => {
        setRegisteringContest(null);
        setRegCode('');
        setRegMessage('');
      }, 1500);
    } catch (err) {
      setRegMessage('⚠ ' + err.message);
    } finally {
      setRegSubmitting(false);
    }
  };

  const { upcoming, past } = normalizeContests(dbContests);

  const handleEditContest = async (contestId) => {
    try {
      const res = await fetch(`/api/contests/${contestId}/full`, { headers: authHeaders() });
      if (!res.ok) throw new Error('Failed to fetch contest details');
      const data = await res.json();
      
      // Transform db models to what wizard expects
      const initialDetails = {
        ...data.details,
        startTime: formatLocalDatetime(data.details.startTime),
        rawStartTime: data.details.startTime,
        registrationDeadline: formatLocalDatetime(data.details.registrationDeadline)
      };

      const initialProblems = data.problems.map(p => ({
        id: p.ID,
        code: p.Code,
        title: p.Title,
        statement: p.Statement,
        timeLimit: p.TimeLimit,
        memoryLimit: p.MemoryLimit,
        sampleStrategies: p.SampleStrategies || [],
        sampleBotFiles: p.SampleBotFilesJSON ? JSON.parse(p.SampleBotFilesJSON) : [],
        sampleShowCustom: p.SampleShowCustom,
        sampleTargetInjection: p.SampleTargetInjection,
        sampleProtocol: p.SampleProtocol,
        sampleTelemetryFormat: p.SampleTelemetryFormat,
        hiddenStrategies: p.HiddenStrategies || [],
        hiddenBotFiles: p.HiddenBotFilesJSON ? JSON.parse(p.HiddenBotFilesJSON) : [],
        hiddenShowCustom: p.HiddenShowCustom,
        hiddenTargetInjection: p.HiddenTargetInjection,
        hiddenProtocol: p.HiddenProtocol,
        hiddenTelemetryFormat: p.HiddenTelemetryFormat
      }));

      setEditingContestData({ details: initialDetails, problems: initialProblems });
      setView('host');
    } catch (e) {
      alert(e.message);
    }
  };

  const handleDeleteContest = async (contestId) => {
    if (!confirm('Are you sure you want to delete this contest? All registrations and problems will be lost.')) return;
    try {
      const res = await fetch(`/api/contests/${contestId}`, {
        method: 'DELETE',
        headers: authHeaders()
      });
      if (!res.ok) throw new Error('Failed to delete contest');
      fetchContests();
    } catch (e) {
      alert(e.message);
    }
  };

  return (
    <div className="cp-root">
      {view === 'landing' && (
        <LandingView 
          onNavigate={(v) => { setEditingContestData(null); setView(v); }} 
          upcomingContests={upcoming} 
          pastContests={past} 
          loading={loading} 
          onRegister={setRegisteringContest}
          registeredContestIds={registeredContestIds}
          user={user}
          onEdit={handleEditContest}
          onDelete={handleDeleteContest}
          finalizationProgress={finalizationProgress}
          onViewParticipants={handleViewParticipants}
          onViewDescription={setViewingDescriptionContest}
        />
      )}
      {view === 'host' && <HostContestWizard initialData={editingContestData} onBack={() => { setEditingContestData(null); setView('landing'); }} />}
      {view === 'join' && (
        <JoinContestView 
          onBack={() => setView('landing')} 
          upcomingContests={upcoming} 
          pastContests={past} 
          onRegister={setRegisteringContest}
          registeredContestIds={registeredContestIds}
          user={user}
          onEdit={handleEditContest}
          onDelete={handleDeleteContest}
          finalizationProgress={finalizationProgress}
          onViewParticipants={handleViewParticipants}
          onViewDescription={setViewingDescriptionContest}
        />
      )}

      {/* Registration Modal Overlay */}
      {registeringContest && (
        <div className="cw-modal-backdrop" onClick={() => setRegisteringContest(null)}>
          <div className="cw-modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="cw-modal-header">
              <h4>Register: {registeringContest.name}</h4>
              <button className="cw-modal-close" onClick={() => setRegisteringContest(null)}>✕</button>
            </div>
            <form onSubmit={handleRegisterSubmit} className="cw-modal-body" style={{ color: '#0f172a' }}>
              {regMessage && (
                <div className={`wizard-toast ${regMessage.includes('✓') ? 'success' : regMessage.includes('⚠') ? 'warning' : ''}`} style={{ position: 'static', marginBottom: '12px', width: '100%', boxSizing: 'border-box' }}>
                  {regMessage}
                </div>
              )}
              
              <label className="cw-field" style={{ textAlign: 'left' }}>
                <span className="cw-label" style={{ color: '#334155', fontWeight: '500' }}>System Name <span className="cw-required">*</span></span>
                <input 
                  className="cw-input" 
                  required 
                  placeholder="e.g. UltraFast-Matcher" 
                  value={regSystemName} 
                  onChange={(e) => setRegSystemName(e.target.value)} 
                  style={{ border: '1px solid #cbd5e1', color: '#0f172a', background: '#fff' }}
                />
              </label>
              
              {registeringContest.visibility === 'private' && (
                <label className="cw-field" style={{ textAlign: 'left' }}>
                  <span className="cw-label" style={{ color: '#334155', fontWeight: '500' }}>Contest Code <span className="cw-required">*</span></span>
                  <input 
                    className="cw-input" 
                    required 
                    placeholder="Enter private access code" 
                    value={regCode} 
                    onChange={(e) => setRegCode(e.target.value.toUpperCase().replace(/\s+/g, ''))} 
                    style={{ border: '1px solid #cbd5e1', color: '#0f172a', background: '#fff' }}
                  />
                </label>
              )}
              
              <div className="cw-modal-actions">
                <button type="button" className="cw-button cw-button-ghost" onClick={() => setRegisteringContest(null)}>Cancel</button>
                <button type="submit" className="cw-button cw-button-primary" disabled={regSubmitting}>
                  {regSubmitting ? 'Registering...' : 'Confirm Registration'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* View Participants Modal Overlay */}
      {viewingParticipantsContest && (
        <div className="cw-modal-backdrop" onClick={() => setViewingParticipantsContest(null)}>
          <div className="cw-modal-card" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '480px' }}>
            <div className="cw-modal-header">
              <h4>👥 Participants — {viewingParticipantsContest.name}</h4>
              <button className="cw-modal-close" onClick={() => setViewingParticipantsContest(null)}>✕</button>
            </div>
            <div className="cw-modal-body" style={{ color: '#0f172a', maxHeight: '400px', overflowY: 'auto' }}>
              {loadingParticipants ? (
                <div style={{ textAlign: 'center', padding: '24px 0', color: '#64748b' }}>🔄 Loading participants...</div>
              ) : participantsList.length > 0 ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  <div style={{ fontSize: '0.85rem', color: '#64748b', marginBottom: '8px', fontWeight: 500 }}>
                    {participantsList.length} registered participant{participantsList.length !== 1 ? 's' : ''}
                  </div>
                  {participantsList.map((name, idx) => (
                    <div key={idx} style={{
                      display: 'flex', alignItems: 'center', gap: '10px',
                      padding: '10px 14px', borderRadius: '8px',
                      background: idx % 2 === 0 ? 'rgba(99, 102, 241, 0.04)' : 'rgba(0,0,0,0.02)',
                      border: '1px solid rgba(0,0,0,0.05)',
                      transition: 'background 0.2s',
                    }}>
                      <span style={{
                        width: '28px', height: '28px', borderRadius: '50%',
                        background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        color: '#fff', fontSize: '0.75rem', fontWeight: 700,
                        flexShrink: 0,
                      }}>
                        {(name || '?')[0].toUpperCase()}
                      </span>
                      <span style={{ fontSize: '0.9rem', fontWeight: 500, color: '#1e293b' }}>{name}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <div style={{ textAlign: 'center', padding: '24px 0', color: '#94a3b8' }}>
                  <span style={{ fontSize: '2rem', display: 'block', marginBottom: '8px' }}>📭</span>
                  No participants registered yet.
                </div>
              )}
            </div>
            <div className="cw-modal-actions" style={{ justifyContent: 'flex-end' }}>
              <button type="button" className="cw-button cw-button-ghost" onClick={() => setViewingParticipantsContest(null)}>Close</button>
            </div>
          </div>
        </div>
      )}

      {/* View Description Modal Overlay */}
      {viewingDescriptionContest && (
        <div className="cw-modal-backdrop" onClick={() => setViewingDescriptionContest(null)}>
          <div className="cw-modal-card" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '560px', overflow: 'hidden' }}>
            {/* Gradient Banner Header */}
            <div style={{
              width: '100%', height: '120px', position: 'relative',
              background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 40%, #a78bfa 70%, #c4b5fd 100%)',
              display: 'flex', alignItems: 'flex-end', padding: '16px 20px',
            }}>
              {viewingDescriptionContest.bannerPreview && (
                <img src={viewingDescriptionContest.bannerPreview} alt="" style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }} />
              )}
              <div style={{ position: 'relative', zIndex: 1 }}>
                <h3 style={{ margin: 0, color: '#fff', fontSize: '1.25rem', fontWeight: 700, textShadow: '0 1px 4px rgba(0,0,0,0.3)' }}>
                  {viewingDescriptionContest.name}
                </h3>
              </div>
              <button className="cw-modal-close" onClick={() => setViewingDescriptionContest(null)} style={{ position: 'absolute', top: '12px', right: '12px', color: '#fff', zIndex: 2 }}>✕</button>
            </div>
            <div className="cw-modal-body" style={{ color: '#1e293b' }}>
              <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '16px' }}>
                <span className={`cp-visibility-tag ${viewingDescriptionContest.visibility}`} style={{ fontSize: '0.72rem', padding: '3px 8px' }}>
                  {viewingDescriptionContest.visibility === 'public' ? 'Public' : '🔒 Private'}
                </span>
                <span style={{ fontSize: '0.72rem', background: 'rgba(99, 102, 241, 0.08)', color: '#6366f1', padding: '3px 8px', borderRadius: '4px', fontWeight: 600 }}>
                  ⏱ {viewingDescriptionContest.duration || viewingDescriptionContest.durationMinutes}m
                </span>
                <span style={{ fontSize: '0.72rem', background: 'rgba(34, 197, 94, 0.08)', color: '#16a34a', padding: '3px 8px', borderRadius: '4px', fontWeight: 600 }}>
                  📅 {formatDate(viewingDescriptionContest.startTime)} · {formatTime(viewingDescriptionContest.startTime)}
                </span>
                <span style={{ fontSize: '0.72rem', background: 'rgba(249, 115, 22, 0.08)', color: '#ea580c', padding: '3px 8px', borderRadius: '4px', fontWeight: 600 }}>
                  👤 {viewingDescriptionContest.participants || 0} participants
                </span>
              </div>
              <p style={{ fontSize: '0.92rem', lineHeight: 1.7, color: '#334155', margin: 0, whiteSpace: 'pre-wrap' }}>
                {viewingDescriptionContest.description || 'No description provided.'}
              </p>
            </div>
            <div className="cw-modal-actions" style={{ justifyContent: 'flex-end' }}>
              <button type="button" className="cw-button cw-button-ghost" onClick={() => setViewingDescriptionContest(null)}>Close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/* ═══════════════════════════════════════
   LANDING VIEW
   ═══════════════════════════════════════ */

function LandingView({ onNavigate, upcomingContests = [], pastContests = [], loading, onRegister, registeredContestIds = [], user, onEdit, onDelete, finalizationProgress, onViewParticipants, onViewDescription }) {
  const registeredContests = upcomingContests.filter(c => registeredContestIds.includes(c.id));

  return (
    <div className="cp-landing">
      {/* Hero section */}
      <div className="cp-hero-section">
        <div className="cp-hero-text">
          <span className="section-tag">Contests</span>
          <h2 className="cp-hero-title">Compete. Learn. Dominate.</h2>
          <p className="cp-hero-desc">
            Test your trading engines against the best. Join a live contest or host your own to challenge the community.
          </p>
        </div>
      </div>

      {/* Action cards */}
      <div className="cp-action-cards">
        <button className="cp-action-card cp-card-join" onClick={() => onNavigate('join')}>
          <div className="cp-action-icon">🏆</div>
          <div className="cp-action-content">
            <h3>Join a Contest</h3>
            <p>Browse upcoming contests, register, and compete with your trading engine against other participants.</p>
          </div>
          <div className="cp-action-arrow">→</div>
        </button>

        <button className="cp-action-card cp-card-host" onClick={() => onNavigate('host')}>
          <div className="cp-action-icon">✦</div>
          <div className="cp-action-content">
            <h3>Host a Contest</h3>
            <p>Create your own contest — define problems, set rules, invite participants, and run the competition.</p>
          </div>
          <div className="cp-action-arrow">→</div>
        </button>
      </div>

      {loading && (
        <div style={{ textAlign: 'center', padding: '24px 0', fontSize: '0.95rem', color: 'var(--text-muted, #888)' }}>
          🔄 Loading contests from server...
        </div>
      )}

      {/* Registered Contests */}
      <section className="cp-section">
        <div className="cp-section-header">
          <div>
            <span className="section-tag">Your Schedule</span>
            <h3 className="cp-section-title">Registered Contests</h3>
          </div>
          <span className="cp-count-badge">{registeredContests.length} contest{registeredContests.length !== 1 ? 's' : ''}</span>
        </div>

        {registeredContests.length > 0 ? (
          <ContestTable 
            contests={registeredContests} 
            type="upcoming" 
            onRegister={onRegister}
            registeredContestIds={registeredContestIds}
            user={user}
            onEdit={onEdit}
            onDelete={onDelete}
            finalizationProgress={finalizationProgress}
            onViewParticipants={onViewParticipants}
            onViewDescription={onViewDescription}
          />
        ) : (
          <div className="cp-empty-state" style={{ padding: '40px', background: 'rgba(255, 255, 255, 0.4)', borderRadius: '12px', border: '1px dashed rgba(0, 0, 0, 0.08)', textAlign: 'center' }}>
            <span className="cp-empty-icon" style={{ fontSize: '2.5rem', marginBottom: '12px', display: 'block' }}>📅</span>
            <h4 style={{ margin: '0 0 8px 0', fontSize: '1.1rem', color: 'var(--text-h, #0f172a)' }}>No registered contests</h4>
            <p style={{ margin: '0 0 16px 0', fontSize: '0.9rem', color: 'var(--text-muted, #64748b)' }}>
              You are not registered for any upcoming contests yet.
            </p>
            <button className="cw-button cw-button-primary" onClick={() => onNavigate('join')}>
              Browse & Register →
            </button>
          </div>
        )}
      </section>

      {/* Past Contests */}
      <section className="cp-section">
        <div className="cp-section-header">
          <div>
            <span className="section-tag">Completed</span>
            <h3 className="cp-section-title">Past Contests</h3>
          </div>
          <span className="cp-count-badge">{pastContests.length} contest{pastContests.length !== 1 ? 's' : ''}</span>
        </div>

        {pastContests.length > 0 ? (
          <ContestTable 
            contests={pastContests} 
            type="past" 
            onRegister={onRegister}
            registeredContestIds={registeredContestIds}
            user={user}
            onEdit={onEdit}
            onDelete={onDelete}
            finalizationProgress={finalizationProgress}
            onViewParticipants={onViewParticipants}
            onViewDescription={onViewDescription}
          />
        ) : (
          <div className="cp-empty-state" style={{ padding: '20px', textAlign: 'center', color: 'var(--text-muted, #888)' }}>
            No past contests.
          </div>
        )}
      </section>
    </div>
  );
}

/* ═══════════════════════════════════════
   CONTEST TABLE
   ═══════════════════════════════════════ */

function ContestTable({ contests, type, onRegister, registeredContestIds = [], user, onEdit, onDelete, finalizationProgress, onViewParticipants, onViewDescription }) {
  if (!contests || contests.length === 0) return null;
  return (
    <div className="cp-contest-table-wrap">
      <table className="cp-contest-table">
        <thead>
          <tr>
            <th style={{ width: '40%' }}>Name</th>
            <th style={{ width: '15%' }}>Start</th>
            <th style={{ width: '10%' }}>Length</th>
            <th style={{ width: '10%' }}>Participants</th>
            <th style={{ width: '25%' }}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {contests.map((c) => (
            <ContestTableRow 
              key={c.id} 
              contest={c} 
              type={type} 
              onRegister={onRegister}
              isRegistered={registeredContestIds.includes(c.id)}
              user={user}
              onEdit={onEdit}
              onDelete={onDelete}
              finalizationProgress={finalizationProgress?.contestId === c.id ? finalizationProgress.progress : null}
              onViewParticipants={onViewParticipants}
              onViewDescription={onViewDescription}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

/* ═══════════════════════════════════════
   CONTEST TABLE ROW
   ═══════════════════════════════════════ */

function ContestTableRow({ contest, type, onRegister, isRegistered, user, onEdit, onDelete, finalizationProgress = null, onViewParticipants, onViewDescription }) {
  const isPast = type === 'past';
  const navigate = useNavigate();
  const { authHeaders } = useAuth();
  const isHost = user && contest.createdBy && user.id === contest.createdBy;
  const regDeadlineStr = contest.registrationDeadline || contest.startTime;

  // Live-ticking clock so countdowns update every second
  const [tick, setTick] = useState(Date.now());
  useEffect(() => {
    const iv = setInterval(() => setTick(Date.now()), 1000);
    return () => clearInterval(iv);
  }, []);

  const now = new Date(tick);
  const start = new Date(contest.startTime);
  const end = new Date(start.getTime() + (contest.duration || 60) * 60000);
  const isLive = now >= start && now < end;
  const isEnded = now >= end;
  const isRegClosed = new Date(regDeadlineStr) <= now;

  const [finalizing, setFinalizing] = useState(false);
  const [finalizeMsg, setFinalizeMsg] = useState('');

  const handleFinalize = async () => {
    if (!confirm('Run final evaluation rounds? This will re-evaluate all submissions with the chosen strategies and average their scores.')) return;
    setFinalizing(true);
    setFinalizeMsg('');
    try {
      const res = await fetch(`/api/contests/${contest.id}/finalize`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ bot_count: 32, duration_seconds: 10 }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'Finalization failed');
      setFinalizeMsg(`✓ Finalization started (${data.teams} teams, ${data.rounds} rounds)`);
    } catch (err) {
      setFinalizeMsg('⚠ ' + err.message);
    } finally {
      setFinalizing(false);
    }
  };

  return (
    <tr className={`cp-contest-row ${isPast ? 'cp-row-past' : ''}`}>
      <td>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
          <strong>{contest.name}</strong>
          <div style={{ display: 'flex', gap: '6px', alignItems: 'center', flexWrap: 'wrap', marginTop: '4px' }}>
            <span className={`cp-status-tag ${isPast ? 'ended' : isLive ? 'live' : 'upcoming'}`} style={{ fontSize: '0.65rem', padding: '2px 6px' }}>
              {contest.phase === 'completed' ? 'Tested' :
               contest.phase === 'finalizing' ? 'Testing' :
               isPast ? 'Ended' : isLive ? '🔴 Live' : timeUntil(contest.startTime)}
            </span>
            <span className={`cp-visibility-tag ${contest.visibility}`} style={{ fontSize: '0.65rem', padding: '2px 6px' }}>
              {contest.visibility === 'public' ? 'Public' : '🔒 Private'}
            </span>
          </div>
          {onViewDescription && (
            <button
              onClick={() => onViewDescription(contest)}
              style={{
                background: 'none', border: 'none', cursor: 'pointer',
                fontSize: '0.78rem', color: '#6366f1', fontWeight: 500,
                padding: '0', marginTop: '2px', textAlign: 'left',
                display: 'inline-flex', alignItems: 'center', gap: '4px',
              }}
            >
              View Description <span style={{ fontSize: '0.7rem' }}>→</span>
            </button>
          )}
        </div>
      </td>
      <td>
        <div>{formatDate(contest.startTime)}</div>
        <div style={{ fontSize: '0.8rem', color: 'var(--text-muted, #888)', marginTop: '2px' }}>{formatTime(contest.startTime)}</div>
        {/* Pre-contest: registration countdown or "Contest starts in" for registered users */}
        {!isPast && !isLive && !isEnded && (
          isRegistered ? (
            <div style={{
              fontSize: '0.72rem', color: '#6366f1', fontWeight: 600, marginTop: '6px',
              background: 'rgba(99, 102, 241, 0.06)', padding: '2px 6px', borderRadius: '4px',
              display: 'inline-block', whiteSpace: 'nowrap'
            }}>
              🚀 Contest starts in: {timeUntil(contest.startTime)}
            </div>
          ) : (
            <div style={{
              fontSize: '0.72rem',
              color: isRegClosed ? '#ef4444' : '#6366f1',
              fontWeight: 600, marginTop: '6px',
              background: isRegClosed ? 'rgba(239, 68, 68, 0.06)' : 'rgba(99, 102, 241, 0.06)',
              padding: '2px 6px', borderRadius: '4px',
              display: 'inline-block', whiteSpace: 'nowrap'
            }}>
              {isRegClosed ? '⏳ Reg Closed' : `⏳ Reg ends in: ${timeUntil(regDeadlineStr)}`}
            </div>
          )
        )}
        {/* Live contest: show "Contest ends in" */}
        {isLive && (
          <div style={{
            fontSize: '0.72rem', color: '#ef4444', fontWeight: 600, marginTop: '6px',
            background: 'rgba(239, 68, 68, 0.06)', padding: '3px 8px', borderRadius: '4px',
            display: 'inline-block', whiteSpace: 'nowrap',
            animation: 'pulse 2s ease-in-out infinite'
          }}>
            🔴 Contest ends in: {timeUntil(end.toISOString())}
          </div>
        )}
      </td>
      <td>{contest.duration}m</td>
      <td>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: '4px' }}>
          <span style={{ fontWeight: 500, display: 'flex', alignItems: 'center', gap: '4px' }}>
            👤 {contest.participants}
          </span>
          {onViewParticipants && contest.participants > 0 && (
            <button
              onClick={() => onViewParticipants(contest)}
              style={{
                background: 'none', border: 'none', cursor: 'pointer',
                fontSize: '0.7rem', color: '#6366f1', fontWeight: 600,
                padding: '0', textDecoration: 'underline',
                textUnderlineOffset: '2px',
              }}
            >
              View Participants
            </button>
          )}
        </div>
      </td>
      <td>
        <div style={{ display: 'flex', gap: '6px', flexDirection: 'column' }}>
          {/* Status Messages */}
          {finalizationProgress !== null && (
            <div style={{ fontSize: '0.75rem', fontWeight: 600, color: '#6366f1' }}>Testing: {finalizationProgress}%</div>
          )}
          {finalizeMsg && (
            <div style={{ fontSize: '0.75rem', color: finalizeMsg.includes('✓') ? '#22c55e' : '#ef4444' }}>{finalizeMsg}</div>
          )}
          {contest.phase === 'completed' && !finalizeMsg && (
            <div style={{ fontSize: '0.75rem', color: '#22c55e' }}>✓ Tested</div>
          )}

          {/* Action Buttons */}
          <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
            {isHost ? (
              <>
                {!isEnded && contest.phase !== 'completed' && contest.phase !== 'finalizing' && (
                  <button className="cw-button cw-button-secondary" style={{ padding: '4px 8px', fontSize: '0.75rem' }} onClick={() => onEdit(contest.id)}>Edit</button>
                )}
                {!isEnded && contest.phase !== 'completed' && contest.phase !== 'finalizing' && (
                  <button className="cw-button cw-button-secondary" style={{ padding: '4px 8px', fontSize: '0.75rem', color: '#ef4444', borderColor: 'rgba(239, 68, 68, 0.3)' }} onClick={() => onDelete(contest.id)}>Delete</button>
                )}
                <button className="cw-button cw-button-secondary" style={{ padding: '4px 8px', fontSize: '0.75rem' }} onClick={() => navigate(`/leaderboard?contest_id=${contest.id}`)}>
                  {contest.phase === 'completed'
                    ? '📊 Final Leaderboard'
                    : contest.phase === 'finalizing'
                      ? '📊 Finalizing Leaderboard'
                      : '📊 Live Leaderboard'}
                </button>
                {isEnded && contest.phase !== 'completed' && (
                  <button className="cw-button cw-button-primary" style={{ padding: '4px 8px', fontSize: '0.75rem' }} onClick={handleFinalize} disabled={finalizing || contest.phase === 'finalizing'}>
                    {finalizing || contest.phase === 'finalizing' ? 'Finalizing...' : 'Finalize'}
                  </button>
                )}
                {isLive && (
                  <button className="cw-button cw-button-primary" style={{ padding: '4px 8px', fontSize: '0.75rem', background: '#ef4444' }} onClick={() => navigate(`/submit?contest_id=${contest.id}&strategy=${contest.strategy || 'bbo_heavy'}`)}>
                    Submit
                  </button>
                )}
              </>
            ) : !isPast && !isLive && !isEnded ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'center' }}>
                {isRegistered ? (
                  <button className="cw-button cw-button-ghost" disabled style={{ padding: '4px 12px', fontSize: '0.8rem', borderColor: '#22c55e', color: '#22c55e' }}>Registered ✓</button>
                ) : (
                  <button className="cw-button cw-button-primary" disabled={isRegClosed} style={{ padding: '4px 12px', fontSize: '0.8rem' }} onClick={() => onRegister && onRegister(contest)}>Register →</button>
                )}
                {isRegistered ? (
                  <span style={{ fontSize: '0.7rem', color: '#6366f1', whiteSpace: 'nowrap', fontWeight: 500 }}>
                    🚀 Starts in: {timeUntil(contest.startTime)}
                  </span>
                ) : (
                  <span style={{ fontSize: '0.7rem', color: isRegClosed ? '#ef4444' : 'var(--text-muted, #888)', whiteSpace: 'nowrap' }}>
                    {isRegClosed ? 'Registration Closed' : `Ends in: ${timeUntil(regDeadlineStr)}`}
                  </span>
                )}
              </div>
            ) : isLive ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', justifyContent: 'space-between', width: '100%' }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'center' }}>
                  {isRegistered ? (
                    <button className="cw-button cw-button-primary" style={{
                      padding: '6px 16px', fontSize: '0.85rem',
                      background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%)',
                      fontWeight: 600, letterSpacing: '0.02em',
                      boxShadow: '0 2px 8px rgba(99, 102, 241, 0.3)',
                    }} onClick={() => navigate(`/contest/${contest.id}`)}>
                      ⚡ Enter Contest
                    </button>
                  ) : (
                    <span style={{
                      padding: '4px 12px', fontSize: '0.78rem', fontWeight: 500,
                      color: '#94a3b8', background: 'rgba(148, 163, 184, 0.08)',
                      borderRadius: '6px', border: '1px solid rgba(148, 163, 184, 0.2)',
                    }}>
                      🚫 Not Registered
                    </span>
                  )}
                  <span style={{ fontSize: '0.68rem', color: '#ef4444', fontWeight: 600, whiteSpace: 'nowrap' }}>
                    ⏱ Ends in: {timeUntil(end.toISOString())}
                  </span>
                </div>
                <button
                  className="cw-button cw-button-secondary"
                  style={{
                    padding: '6px 12px', fontSize: '0.72rem', whiteSpace: 'nowrap',
                    display: 'inline-flex', alignItems: 'center', gap: '4px',
                    flexShrink: 0,
                  }}
                  onClick={() => navigate(`/leaderboard?contest_id=${contest.id}`)}
                >
                  {contest.phase === 'completed'
                    ? '📊 Final Leaderboard'
                    : contest.phase === 'finalizing'
                      ? '📊 Finalizing Leaderboard'
                      : '📊 Live Leaderboard'}
                </button>
              </div>
            ) : (
              <button
                className="cw-button cw-button-secondary"
                style={{ padding: '4px 12px', fontSize: '0.8rem', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
                onClick={() => navigate(`/leaderboard?contest_id=${contest.id}`)}
              >
                {contest.phase === 'completed'
                  ? '📊 Final Leaderboard'
                  : contest.phase === 'finalizing'
                    ? '📊 Finalizing Leaderboard'
                    : '📊 Leaderboard'}
              </button>
            )}
          </div>
        </div>
      </td>
    </tr>
  );
}

/* ═══════════════════════════════════════
   JOIN CONTEST VIEW
   ═══════════════════════════════════════ */

function JoinContestView({ onBack, upcomingContests = [], pastContests = [], onRegister, registeredContestIds = [], user, onEdit, onDelete, finalizationProgress, onViewParticipants, onViewDescription }) {
  const [search, setSearch] = useState('');
  const [tab, setTab] = useState('upcoming'); // 'upcoming' | 'past'

  const contests = tab === 'upcoming' ? upcomingContests : pastContests;
  const filtered = contests.filter(
    (c) =>
      c.name.toLowerCase().includes(search.toLowerCase()) ||
      c.description.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="cp-join-view">
      <div className="cp-view-topbar">
        <button className="cw-button cw-button-ghost" onClick={onBack}>← Back to Contests</button>
        <div className="cp-view-title-block">
          <span className="section-tag">Browse</span>
          <h2 className="cp-view-title">Join a Contest</h2>
        </div>
      </div>

      {/* Search + tabs */}
      <div className="cp-join-controls">
        <div className="cp-search-wrap">
          <span className="cp-search-icon">🔍</span>
          <input
            className="cp-search-input"
            placeholder="Search contests..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <div className="cp-tab-group">
          <button className={`cp-tab ${tab === 'upcoming' ? 'active' : ''}`} onClick={() => setTab('upcoming')}>
            Upcoming ({upcomingContests.length})
          </button>
          <button className={`cp-tab ${tab === 'past' ? 'active' : ''}`} onClick={() => setTab('past')}>
            Past ({pastContests.length})
          </button>
        </div>
      </div>

      {/* Contest list */}
      <div className="cp-contest-table-container" style={{ marginTop: '24px' }}>
        {filtered.length > 0 ? (
          <ContestTable 
            contests={filtered} 
            type={tab} 
            onRegister={onRegister}
            registeredContestIds={registeredContestIds}
            user={user}
            onEdit={onEdit}
            onDelete={onDelete}
            finalizationProgress={finalizationProgress}
            onViewParticipants={onViewParticipants}
            onViewDescription={onViewDescription}
          />
        ) : (
          <div className="cp-empty-state">
            <span className="cp-empty-icon">🔍</span>
            <h4>No contests found</h4>
            <p>Try a different search term or check back later.</p>
          </div>
        )}
      </div>
    </div>
  );
}

/* ═══════════════════════════════════════
   HOST CONTEST WIZARD
   ═══════════════════════════════════════ */

function HostContestWizard({ initialData, onBack }) {
  const { authHeaders } = useAuth();
  const [step, setStep] = useState(0);
  const hasStarted = initialData?.details?.rawStartTime ? new Date(initialData.details.rawStartTime) <= new Date() : false;

  const [details, setDetails] = useState({
    id: uid('contest'),
    name: '',
    description: '',
    bannerPreview: null,
    visibility: 'public',
    code: '',
    startTime: '',
    durationMinutes: 60,
    registrationDeadline: '',
    strategy: 'bbo_heavy',
    finalStrategies: ['bbo_heavy', 'flash_crash', 'high_cancel', 'iceberg', 'momentum_burst'],
  });

  const [problems, setProblems] = useState([]);
  const [selectedProblemId, setSelectedProblemId] = useState(null);

  const dragFrom = useRef(null);
  const autosave = useRef(null);
  const [message, setMessage] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (initialData) {
      setDetails(initialData.details);
      if (initialData.problems && initialData.problems.length > 0) {
        setProblems(initialData.problems);
        setSelectedProblemId(initialData.problems[0].id);
      } else {
        const p = emptyProblem();
        setProblems([p]);
        setSelectedProblemId(p.id);
      }
      return;
    }

    try {
      const raw = localStorage.getItem(DRAFT_KEY);
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed.details) {
          setDetails(prev => ({
            ...prev,
            ...parsed.details,
            id: parsed.details.id || prev.id
          }));
        }
        if (Array.isArray(parsed.problems) && parsed.problems.length) {
          const merged = parsed.problems.map(p => ({
            ...emptyProblem(),
            ...p
          }));
          setProblems(merged);
          setSelectedProblemId(merged[0].id);
        } else {
          const p = emptyProblem();
          setProblems([p]);
          setSelectedProblemId(p.id);
        }
      } else {
        const p = emptyProblem();
        setProblems([p]);
        setSelectedProblemId(p.id);
      }
    } catch (e) {
      const p = emptyProblem();
      setProblems([p]);
      setSelectedProblemId(p.id);
    }
  }, [initialData]);

  // Autosave to localStorage (debounced), but only if not editing an existing contest
  useEffect(() => {
    if (initialData) return;
    if (autosave.current) clearTimeout(autosave.current);
    autosave.current = setTimeout(() => {
      try {
        localStorage.setItem(DRAFT_KEY, JSON.stringify({ details, problems }));
        setMessage('Draft auto-saved');
        setTimeout(() => setMessage(''), 1000);
      } catch (e) {
        setMessage('Draft save failed');
      }
    }, 750);
    return () => clearTimeout(autosave.current);
  }, [details, problems, initialData]);

  const addProblem = () => {
    const p = emptyProblem();
    setProblems((s) => [...s, p]);
    setSelectedProblemId(p.id);
  };

  const updateProblem = (id, patch) => {
    setProblems((s) => s.map((p) => (p.id === id ? { ...p, ...patch } : p)));
  };

  const handleBotFileChange = (problemId, type, file) => {
    if (!file) return;
    const fileExt = file.name.split('.').pop()?.toLowerCase();
    if (fileExt !== 'go') {
      window.alert('Only Go (.go) files are allowed for custom bot generation strategy!');
      return;
    }
    const reader = new FileReader();
    reader.onload = (e) => {
      const content = e.target.result;
      const newFileObj = { name: file.name, content: content };
      
      setProblems((s) => s.map((p) => {
        if (p.id !== problemId) return p;
        if (type === 'sample') {
          const files = p.sampleBotFiles || [];
          if (files.some(f => f.name === file.name)) {
            window.alert('A file with this name is already uploaded!');
            return p;
          }
          return { ...p, sampleBotFiles: [...files, newFileObj] };
        } else {
          const files = p.hiddenBotFiles || [];
          if (files.some(f => f.name === file.name)) {
            window.alert('A file with this name is already uploaded!');
            return p;
          }
          return { ...p, hiddenBotFiles: [...files, newFileObj] };
        }
      }));
    };
    reader.readAsText(file);
  };

  const handleRemoveBotFile = (problemId, type, fileName) => {
    setProblems((s) => s.map((p) => {
      if (p.id !== problemId) return p;
      if (type === 'sample') {
        const files = p.sampleBotFiles || [];
        return { ...p, sampleBotFiles: files.filter(f => f.name !== fileName) };
      } else {
        const files = p.hiddenBotFiles || [];
        return { ...p, hiddenBotFiles: files.filter(f => f.name !== fileName) };
      }
    }));
  };

  const removeProblem = (id) => {
    if (!confirm('Delete problem?')) return;
    setProblems((s) => s.filter((p) => p.id !== id));
    if (selectedProblemId === id) setSelectedProblemId(problems[0]?.id || null);
  };

  const onDragStart = (e, idx) => {
    dragFrom.current = idx;
    e.dataTransfer?.setData('text/plain', String(idx));
  };

  const onDrop = (e, toIdx) => {
    e.preventDefault();
    const from = dragFrom.current;
    if (from == null) return;
    if (from === toIdx) return;
    setProblems((prev) => {
      const copy = [...prev];
      const [item] = copy.splice(from, 1);
      copy.splice(toIdx, 0, item);
      return copy;
    });
    dragFrom.current = null;
  };

  const onDragOver = (e) => e.preventDefault();

  const validateDetails = () => details.name.trim() !== '' && details.startTime !== '' && (details.visibility !== 'private' || (details.code && details.code.trim() !== ''));
  const validateProblems = () => problems.length > 0 && problems.every((p) => p.title.trim() !== '' && p.statement.trim() !== '');

  const saveDraft = async () => {
    setSaving(true);
    setMessage('Saving draft...');
    try {
      const isoDetails = { ...details };
      if (isoDetails.startTime) isoDetails.startTime = new Date(isoDetails.startTime).toISOString();
      if (isoDetails.registrationDeadline) isoDetails.registrationDeadline = new Date(isoDetails.registrationDeadline).toISOString();

      const res = await fetch('/api/contests/draft', { 
        method: 'POST', 
        headers: { 'Content-Type': 'application/json', ...authHeaders() }, 
        body: JSON.stringify({ details: isoDetails, problems }) 
      });
      if (!res.ok) {
        const errData = await res.json().catch(() => null);
        throw new Error(errData?.message || errData?.error || 'Save failed');
      }
      if (!initialData) {
        localStorage.setItem(DRAFT_KEY, JSON.stringify({ details, problems }));
      }
      setMessage('Draft saved ✓');
    } catch (e) {
      setMessage('⚠ ' + (e.message || 'Save failed'));
    } finally {
      setSaving(false);
      setTimeout(() => setMessage(''), 2000);
    }
  };

  const publish = async () => {
    if (!validateDetails()) {
      setMessage('⚠ Complete contest details first');
      setStep(0);
      return;
    }
    if (!validateProblems()) {
      setMessage('⚠ Ensure all problems have title and statement');
      setStep(1);
      return;
    }
    setSaving(true);
    setMessage('Publishing...');
    let success = false;
    try {
      const isoDetails = { ...details };
      if (isoDetails.startTime) isoDetails.startTime = new Date(isoDetails.startTime).toISOString();
      if (isoDetails.registrationDeadline) isoDetails.registrationDeadline = new Date(isoDetails.registrationDeadline).toISOString();

      const res = await fetch('/api/contests/publish', { 
        method: 'POST', 
        headers: { 'Content-Type': 'application/json', ...authHeaders() }, 
        body: JSON.stringify({ details: isoDetails, problems }) 
      });
      if (!res.ok) {
        const errData = await res.json().catch(() => null);
        throw new Error(errData?.message || errData?.error || 'Publish failed');
      }
      setMessage('Published ✓');
      if (!initialData) {
        localStorage.removeItem(DRAFT_KEY);
      }
      success = true;
      setTimeout(() => {
        onBack();
      }, 1500);
    } catch (e) {
      setMessage('⚠ ' + (e.message || 'Publish failed'));
    } finally {
      if (!success) {
        setSaving(false);
        setTimeout(() => setMessage(''), 4000);
      }
    }
  };

  const completedSteps = [
    validateDetails(),
    validateProblems(),
    validateDetails() && validateProblems(),
  ];

  return (
    <div className="contest-wizard">
      {/* ─── Sidebar ─── */}
      <aside className="wizard-sidebar">
        <div className="wizard-sidebar-header">
          <button className="cw-back-btn" onClick={onBack} title="Back to Contests">←</button>
          <div>
            <span className="section-tag">Host</span>
            <h3>Create Contest</h3>
          </div>
        </div>

        <nav className="wizard-steps">
          {STEP_TITLES.map((t, i) => (
            <button
              key={t}
              className={`wizard-step ${i === step ? 'active' : ''} ${completedSteps[i] ? 'completed' : ''}`}
              onClick={() => setStep(i)}
            >
              <span className="step-num">
                {completedSteps[i] ? '✓' : STEP_ICONS[i]}
              </span>
              <div className="step-info">
                <span className="step-text">{t}</span>
                <span className="step-desc">{STEP_DESCRIPTIONS[i]}</span>
              </div>
            </button>
          ))}
        </nav>

        {/* Progress */}
        <div className="wizard-progress-section">
          <div className="wizard-progress-label">
            <span>Progress</span>
            <span>{Math.round((completedSteps.filter(Boolean).length / 3) * 100)}%</span>
          </div>
          <div className="wizard-progress-track">
            <div
              className="wizard-progress-fill"
              style={{ width: `${(completedSteps.filter(Boolean).length / 3) * 100}%` }}
            />
          </div>
        </div>

        <div className="wizard-sidebar-footer">
          <div className="cp-wizard-actions">
            {!initialData && (
              <button className="cw-button cw-button-secondary" onClick={saveDraft} disabled={saving}>
                {saving ? 'Saving...' : '💾 Save Draft'}
              </button>
            )}
            <button className="cw-button cw-button-primary" onClick={publish} disabled={saving}>
              {saving ? 'Publishing...' : (initialData ? '🚀 Update Contest' : '🚀 Publish Contest')}
            </button>
          </div>
        </div>
      </aside>

      {/* ─── Main ─── */}
      <main className="wizard-main">
        {/* Top bar */}
        <div className="wizard-topbar">
          <div className="wizard-topbar-left">
            <div className="wizard-step-badge">{step + 1} / 3</div>
            <div>
              <h2 className="wizard-page-title">{STEP_TITLES[step]}</h2>
              <p className="wizard-page-subtitle">{STEP_DESCRIPTIONS[step]}</p>
            </div>
          </div>
          <div className="wizard-topbar-right">
            {message && <div className={`wizard-toast ${message.includes('✓') ? 'success' : message.includes('⚠') ? 'warning' : ''}`}>{message}</div>}
            <div className="wizard-nav-buttons">
              {step > 0 && <button className="cw-button cw-button-ghost" onClick={() => setStep(step - 1)}>← Back</button>}
              {step < 2 && <button className="cw-button cw-button-primary" onClick={() => setStep(step + 1)}>Next →</button>}
              {step === 2 && (
                <button className="cw-button cw-button-publish" onClick={publish} disabled={saving}>
                  {saving ? 'Publishing...' : '🚀 Publish Contest'}
                </button>
              )}
            </div>
          </div>
        </div>

        {hasStarted && (
          <div style={{
            margin: '0 24px 20px 24px',
            padding: '12px 16px',
            background: 'rgba(239, 68, 68, 0.08)',
            borderLeft: '4px solid #ef4444',
            borderRadius: '8px',
            fontSize: '0.88rem',
            color: '#b91c1c',
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }}>
            <span>⚠️</span>
            <span><strong>Contest has started:</strong> Adding/deleting problems and changing basic info are disabled. You can edit problem details, sample/hidden test cases, and other settings.</span>
          </div>
        )}

        {/* ─── Step content ─── */}
        <div className="wizard-body" key={step}>
          {step === 0 && (
            <div className="cw-details-grid">
              <div className="cw-card cw-card-main">
                <div className="cw-card-header">
                  <span className="cw-card-icon">📝</span>
                  <h4>Basic Information</h4>
                </div>
                <div className="cw-card-body">
                  <label className="cw-field">
                    <span className="cw-label">Contest Name <span className="cw-required">*</span></span>
                    <input className="cw-input" placeholder="e.g. IICPC Finals 2026" value={details.name} onChange={(e) => setDetails({ ...details, name: e.target.value })} disabled={hasStarted} />
                  </label>
                  <label className="cw-field">
                    <span className="cw-label">Description</span>
                    <textarea className="cw-input cw-textarea" rows={4} placeholder="Describe your contest — rules, scoring, theme..." value={details.description} onChange={(e) => setDetails({ ...details, description: e.target.value })} disabled={hasStarted} />
                  </label>
                  <label className="cw-field">
                    <span className="cw-label">Banner Image (optional)</span>
                    <div className="cw-file-upload" style={hasStarted ? { opacity: 0.65, cursor: 'not-allowed', pointerEvents: 'none' } : {}}>
                      <input type="file" accept="image/*" disabled={hasStarted} onChange={(e) => {
                        const f = e.target.files?.[0] || null;
                        if (f) {
                          const reader = new FileReader();
                          reader.onload = (evt) => {
                            setDetails({ ...details, bannerPreview: evt.target.result });
                          };
                          reader.readAsDataURL(f);
                        } else {
                          setDetails({ ...details, bannerPreview: null });
                        }
                      }} />
                      <span className="cw-file-text">{hasStarted ? '🔒 Banner cannot be changed after start' : '📁 Choose file or drag here'}</span>
                    </div>
                  </label>
                  {details.bannerPreview && (
                    <div className="cw-banner-preview">
                      <img src={details.bannerPreview} alt="banner" />
                    </div>
                  )}
                </div>
              </div>

              <div className="cw-card">
                <div className="cw-card-header">
                  <span className="cw-card-icon">⚙️</span>
                  <h4>Settings</h4>
                </div>
                <div className="cw-card-body">
                  <div className="cw-field-row cw-field-row-2">
                    <label className="cw-field">
                      <span className="cw-label">Visibility</span>
                      <select className="cw-input cw-select" value={details.visibility} onChange={(e) => {
                        const val = e.target.value;
                        setDetails({ ...details, visibility: val, code: val === 'public' ? '' : details.code });
                      }}>
                        <option value="public">🌍 Public</option>
                        <option value="private">🔒 Private</option>
                      </select>
                    </label>
                    <label className="cw-field">
                      <span className="cw-label">Duration (minutes)</span>
                      <input className="cw-input" type="number" min={5} value={details.durationMinutes} onChange={(e) => setDetails({ ...details, durationMinutes: Number(e.target.value) })} />
                    </label>
                  </div>

                  {details.visibility === 'private' && (
                    <label className="cw-field">
                      <span className="cw-label">Contest Code <span className="cw-required">*</span></span>
                      <input
                        required
                        className="cw-input"
                        placeholder="e.g. SECRET-CODE-123"
                        value={details.code || ''}
                        onChange={(e) => setDetails({ ...details, code: e.target.value.toUpperCase().replace(/\s+/g, '') })}
                      />
                      <span className="cw-field-desc" style={{ fontSize: '0.85rem', color: 'var(--text-muted, #888)', marginTop: '4px' }}>
                        Contestants must enter this code to join or register for this private contest.
                      </span>
                    </label>
                  )}
                </div>
              </div>

              <div className="cw-card cw-card-schedule">
                <div className="cw-card-header">
                  <span className="cw-card-icon">📅</span>
                  <h4>Schedule</h4>
                </div>
                <div className="cw-card-body">
                  <div className="cw-field-row cw-field-row-2">
                    <label className="cw-field">
                      <span className="cw-label">Start Time <span className="cw-required">*</span></span>
                      <input className="cw-input" type="datetime-local" value={details.startTime} onChange={(e) => setDetails({ ...details, startTime: e.target.value })} disabled={hasStarted} />
                    </label>
                    <label className="cw-field">
                      <span className="cw-label">Registration Deadline</span>
                      <input className="cw-input" type="datetime-local" value={details.registrationDeadline} onChange={(e) => setDetails({ ...details, registrationDeadline: e.target.value })} disabled={hasStarted} />
                    </label>
                  </div>
                </div>
              </div>
            </div>
          )}

          {step === 1 && (
            <div className="cw-problems-layout">
              <div className="cw-problems-sidebar">
                <div className="cw-problems-sidebar-header">
                  <div>
                    <h4>Problems</h4>
                    <span className="cw-problem-count">{problems.length} problem{problems.length !== 1 ? 's' : ''}</span>
                  </div>
                  {!hasStarted && <button className="cw-button-add" onClick={addProblem}>+ Add</button>}
                </div>
                <div className="cw-problem-list">
                  {problems.map((p, idx) => (
                    <div
                      key={p.id}
                      className={`cw-problem-item ${selectedProblemId === p.id ? 'selected' : ''}`}
                      draggable={!hasStarted}
                      onDragStart={(e) => !hasStarted && onDragStart(e, idx)}
                      onDragOver={onDragOver}
                      onDrop={(e) => !hasStarted && onDrop(e, idx)}
                      onClick={() => setSelectedProblemId(p.id)}
                    >
                      {!hasStarted && <div className="cw-problem-drag">⠿</div>}
                      <div className="cw-problem-info" style={{ marginLeft: hasStarted ? '6px' : '0' }}>
                        <div className="cw-problem-idx">{String.fromCharCode(65 + idx)}</div>
                        <div>
                          <strong>{p.title || 'Untitled Problem'}</strong>
                          <span className="cw-problem-meta">{p.timeLimit}s · {p.memoryLimit}MB</span>
                        </div>
                      </div>
                      {!hasStarted && <button className="cw-problem-delete" onClick={(e) => { e.stopPropagation(); removeProblem(p.id); }} title="Delete problem">✕</button>}
                    </div>
                  ))}
                </div>
              </div>

              <div className="cw-problem-editor-wrap">
                {(() => {
                  const active = problems.find((x) => x.id === selectedProblemId) || problems[0];
                  if (!active) return (
                    <div className="cw-empty-editor">
                      <span className="cw-empty-icon">🧩</span>
                      <p>Select or add a problem to start editing</p>
                    </div>
                  );
                  return (
                    <form className="cw-problem-editor" onSubmit={(e) => e.preventDefault()}>
                      <div className="cw-editor-header">
                        <h4>Editing: {active.title || 'Untitled'}</h4>
                      </div>
                      
                      {/* Section 1: Problem Details */}
                      <div className="cw-card" style={{ marginBottom: '24px' }}>
                        <div className="cw-card-header">
                          <span className="cw-card-icon">⚙️</span>
                          <h4>Problem Details</h4>
                        </div>
                        <div className="cw-card-body" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                          <div className="cw-field-row cw-field-row-2">
                        <label className="cw-field">
                          <span className="cw-label">Problem Code</span>
                          <input className="cw-input" placeholder="e.g. A" value={active.code} onChange={(e) => updateProblem(active.id, { code: e.target.value })} />
                        </label>
                        <label className="cw-field">
                          <span className="cw-label">Title <span className="cw-required">*</span></span>
                          <input className="cw-input" placeholder="Problem title" value={active.title} onChange={(e) => updateProblem(active.id, { title: e.target.value })} />
                        </label>
                      </div>
                      <label className="cw-field">
                        <span className="cw-label">Problem Statement <span className="cw-required">*</span></span>
                        <textarea className="cw-input cw-textarea cw-textarea-tall" rows={8} placeholder="Describe the problem..." value={active.statement} onChange={(e) => updateProblem(active.id, { statement: e.target.value })} />
                      </label>
                      <div className="cw-field-row cw-field-row-2">
                        <label className="cw-field">
                          <span className="cw-label">Time Limit (s)</span>
                          <input className="cw-input" type="number" min={1} value={active.timeLimit} onChange={(e) => updateProblem(active.id, { timeLimit: Number(e.target.value) })} />
                        </label>
                        <label className="cw-field">
                          <span className="cw-label">Memory Limit (MB)</span>
                          <input className="cw-input" type="number" min={32} value={active.memoryLimit} onChange={(e) => updateProblem(active.id, { memoryLimit: Number(e.target.value) })} />
                        </label>
                      </div>
                        </div>
                      </div>

                      {/* Section 2: Live Contest Evaluation */}
                    <div className="cw-card" style={{ marginBottom: '24px' }}>
                      <div className="cw-card-header">
                        <span className="cw-card-icon">⚡</span>
                        <h4>Live Contest Evaluation</h4>
                      </div>
                      <div className="cw-card-body" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                        {/* Sample Test Strategies Selector */}
                        <div className="cw-field" style={{ textAlign: 'left' }}>
                          <span className="cw-label" style={{ display: 'block', marginBottom: '8px', fontWeight: '500' }}>Sample Test Strategies</span>
                          <span className="cw-field-desc" style={{ fontSize: '0.82rem', color: 'var(--text-muted, #888)', display: 'block', marginBottom: '10px' }}>
                            Select one or more baseline trading profiles for sample runs:
                          </span>
                          <div style={{
                            marginBottom: '10px',
                            padding: '8px 12px',
                            background: 'rgba(37, 99, 235, 0.04)',
                            borderLeft: '3px solid var(--accent, #2563eb)',
                            borderRadius: '4px',
                            fontSize: '0.8rem',
                            color: 'var(--text, #1e293b)',
                            lineHeight: '1.4'
                          }}>
                            🛡️ <strong>Seed Info:</strong> Sample test runs use <strong>80% fixed and 20% random (time-based) seeds</strong> during the contest to prevent users from reverse-engineering or hacking the test cases. (Sample strategies are not evaluated post-contest).
                          </div>
                          {((active.sampleStrategies || []).length + (active.sampleBotFiles || []).length > 1) && (
                            <div style={{
                              marginBottom: '10px',
                              padding: '8px 12px',
                              background: 'rgba(217, 119, 6, 0.05)',
                              borderLeft: '3px solid #d97706',
                              borderRadius: '4px',
                              fontSize: '0.82rem',
                              color: '#b45309',
                              textAlign: 'left'
                            }}>
                              ℹ️ In the evaluation phase the answer from all the chosen strategies will be averaged for scoring purposes.
                            </div>
                          )}
                          <div className="cw-strategy-chips" style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginBottom: '16px' }}>
                            {STRATEGIES.map((s) => (
                              <label key={s} className={`cw-chip ${(active.sampleStrategies || []).includes(s) ? 'active' : ''}`}>
                                <input type="checkbox" hidden checked={(active.sampleStrategies || []).includes(s)} onChange={(e) => {
                                  const next = new Set(active.sampleStrategies || []);
                                  if (e.target.checked) next.add(s); else next.delete(s);
                                  updateProblem(active.id, { sampleStrategies: Array.from(next) });
                                }} />
                                {s.replace(/_/g, ' ')}
                              </label>
                            ))}
                            <label className={`cw-chip ${active.sampleShowCustom ? 'active' : ''}`} style={{ border: '1px dashed rgba(224, 169, 109, 0.4)' }}>
                              <input type="checkbox" hidden checked={active.sampleShowCustom || false} onChange={(e) => {
                                updateProblem(active.id, { sampleShowCustom: e.target.checked });
                              }} />
                              ➕ Add Custom Bot
                            </label>
                          </div>
                          
                          {/* Custom Bot Files for Sample Tests */}
                          {active.sampleShowCustom && (
                            <div style={{ marginTop: '16px', borderTop: '1px solid rgba(255,255,255,0.06)', paddingTop: '16px' }}>
                              <CustomBotGuidelines />

                              {/* Custom Bot Configuration Form */}
                              <div style={{ 
                                background: 'rgba(0, 0, 0, 0.01)',
                                border: '1px solid var(--border, #dbe4f0)',
                                borderRadius: '8px',
                                padding: '16px',
                                marginBottom: '16px'
                              }}>
                                <h5 style={{ margin: '0 0 12px 0', color: 'var(--text-h, #0f172a)', fontSize: '0.9rem', fontWeight: '600' }}>
                                  ⚙️ Custom Bot Settings
                                </h5>
                                <div className="cw-field-row cw-field-row-3" style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px' }}>
                                  <label className="cw-field">
                                    <span className="cw-label">Target Address Injection</span>
                                    <select 
                                      className="cw-input cw-select" 
                                      value={active.sampleTargetInjection || 'env'} 
                                      onChange={(e) => updateProblem(active.id, { sampleTargetInjection: e.target.value })}
                                    >
                                      <option value="env">Environment Variable (TARGET_URL)</option>
                                      <option value="arg">Command-line Argument (--target)</option>
                                    </select>
                                  </label>
                                  <label className="cw-field">
                                    <span className="cw-label">Network Protocol</span>
                                    <select 
                                      className="cw-input cw-select" 
                                      value={active.sampleProtocol || 'http'} 
                                      onChange={(e) => updateProblem(active.id, { sampleProtocol: e.target.value })}
                                    >
                                      <option value="http">HTTP</option>
                                      <option value="tcp">TCP</option>
                                      <option value="fix">FIX</option>
                                    </select>
                                  </label>
                                  <label className="cw-field">
                                    <span className="cw-label">Telemetry Format</span>
                                    <select 
                                      className="cw-input cw-select" 
                                      value={active.sampleTelemetryFormat || 'stdout'} 
                                      onChange={(e) => updateProblem(active.id, { sampleTelemetryFormat: e.target.value })}
                                    >
                                      <option value="stdout">Structured stdout (JSON Logs)</option>
                                      <option value="callback">Telemetry Callback (POST)</option>
                                    </select>
                                  </label>
                                </div>
                              </div>
                              
                              {active.sampleBotFiles && active.sampleBotFiles.length > 0 && (
                                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginBottom: '12px' }}>
                                  <span style={{ fontSize: '0.85rem', fontWeight: '500', color: 'var(--text, #4b5563)' }}>Uploaded Custom Sample Bots:</span>
                                  {active.sampleBotFiles.map((file) => (
                                    <div key={file.name} style={{ 
                                      display: 'flex', 
                                      alignItems: 'center', 
                                      justifyContent: 'space-between', 
                                      padding: '8px 12px', 
                                      background: 'rgba(0, 0, 0, 0.02)', 
                                      border: '1px solid rgba(0, 0, 0, 0.08)', 
                                      borderRadius: '6px' 
                                    }}>
                                      <span style={{ fontSize: '0.88rem', color: 'var(--text-h, #0f172a)', display: 'flex', alignItems: 'center', gap: '8px' }}>
                                        📄 {file.name}
                                      </span>
                                      <button 
                                        className="cw-button cw-button-ghost" 
                                        style={{ padding: '2px 6px', minWidth: 'auto', fontSize: '0.82rem', color: '#ff6b6b' }}
                                        onClick={() => handleRemoveBotFile(active.id, 'sample', file.name)}
                                      >
                                        Remove
                                      </button>
                                    </div>
                                  ))}
                                </div>
                              )}

                              <div className="cw-file-upload">
                                <input 
                                  type="file" 
                                  accept=".go" 
                                  onChange={(e) => {
                                    handleBotFileChange(active.id, 'sample', e.target.files?.[0]);
                                    e.target.value = '';
                                  }} 
                                />
                                <span className="cw-file-text">➕ Upload Custom Go Bot File (.go)</span>
                              </div>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Section 3: Post Contest Evaluation */}
                    <div className="cw-card" style={{ marginBottom: '24px' }}>
                      <div className="cw-card-header">
                        <span className="cw-card-icon">🛡️</span>
                        <h4>Post Contest Evaluation</h4>
                      </div>
                      <div className="cw-card-body" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                        {/* Hidden Test Strategies Selector */}
                        <div className="cw-field" style={{ textAlign: 'left' }}>
                          <span className="cw-label" style={{ display: 'block', marginBottom: '8px', fontWeight: '500' }}>Hidden Test Strategies</span>
                          <span className="cw-field-desc" style={{ fontSize: '0.82rem', color: 'var(--text-muted, #888)', display: 'block', marginBottom: '10px' }}>
                            Select one or more baseline trading profiles for final grading runs:
                          </span>
                          <div style={{
                            marginBottom: '10px',
                            padding: '8px 12px',
                            background: 'rgba(37, 99, 235, 0.04)',
                            borderLeft: '3px solid var(--accent, #2563eb)',
                            borderRadius: '4px',
                            fontSize: '0.8rem',
                            color: 'var(--text, #1e293b)',
                            lineHeight: '1.4'
                          }}>
                            🛡️ <strong>Seed Info:</strong> Hidden evaluations (finalization runs) use <strong>100% fixed, deterministic seeds</strong> to guarantee absolute scoring fairness.
                          </div>
                          {((active.hiddenStrategies || []).length + (active.hiddenBotFiles || []).length > 1) && (
                            <div style={{
                              marginBottom: '10px',
                              padding: '8px 12px',
                              background: 'rgba(217, 119, 6, 0.05)',
                              borderLeft: '3px solid #d97706',
                              borderRadius: '4px',
                              fontSize: '0.82rem',
                              color: '#b45309',
                              textAlign: 'left'
                            }}>
                              ℹ️ In the evaluation phase the answer from all the chosen strategies will be averaged for scoring purposes.
                            </div>
                          )}
                          <div className="cw-strategy-chips" style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginBottom: '16px' }}>
                            {STRATEGIES.map((s) => (
                              <label key={s} className={`cw-chip cw-chip-hidden ${(active.hiddenStrategies || []).includes(s) ? 'active' : ''}`}>
                                <input type="checkbox" hidden checked={(active.hiddenStrategies || []).includes(s)} onChange={(e) => {
                                  const next = new Set(active.hiddenStrategies || []);
                                  if (e.target.checked) next.add(s); else next.delete(s);
                                  updateProblem(active.id, { hiddenStrategies: Array.from(next) });
                                }} />
                                {s.replace(/_/g, ' ')}
                              </label>
                            ))}
                            <label className={`cw-chip ${active.hiddenShowCustom ? 'active' : ''}`} style={{ border: '1px dashed rgba(224, 169, 109, 0.4)' }}>
                              <input type="checkbox" hidden checked={active.hiddenShowCustom || false} onChange={(e) => {
                                updateProblem(active.id, { hiddenShowCustom: e.target.checked });
                              }} />
                              ➕ Add Custom Bot
                            </label>
                          </div>

                          {/* Custom Bot Files for Hidden Tests */}
                          {active.hiddenShowCustom && (
                            <div style={{ marginTop: '16px', borderTop: '1px solid rgba(255,255,255,0.06)', paddingTop: '16px' }}>
                              <CustomBotGuidelines />

                              {/* Custom Bot Configuration Form */}
                              <div style={{ 
                                background: 'rgba(0, 0, 0, 0.01)',
                                border: '1px solid var(--border, #dbe4f0)',
                                borderRadius: '8px',
                                padding: '16px',
                                marginBottom: '16px'
                              }}>
                                <h5 style={{ margin: '0 0 12px 0', color: 'var(--text-h, #0f172a)', fontSize: '0.9rem', fontWeight: '600' }}>
                                  ⚙️ Custom Bot Settings
                                </h5>
                                <div className="cw-field-row cw-field-row-3" style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px' }}>
                                  <label className="cw-field">
                                    <span className="cw-label">Target Address Injection</span>
                                    <select 
                                      className="cw-input cw-select" 
                                      value={active.hiddenTargetInjection || 'env'} 
                                      onChange={(e) => updateProblem(active.id, { hiddenTargetInjection: e.target.value })}
                                    >
                                      <option value="env">Environment Variable (TARGET_URL)</option>
                                      <option value="arg">Command-line Argument (--target)</option>
                                    </select>
                                  </label>
                                  <label className="cw-field">
                                    <span className="cw-label">Network Protocol</span>
                                    <select 
                                      className="cw-input cw-select" 
                                      value={active.hiddenProtocol || 'http'} 
                                      onChange={(e) => updateProblem(active.id, { hiddenProtocol: e.target.value })}
                                    >
                                      <option value="http">HTTP</option>
                                      <option value="tcp">TCP</option>
                                      <option value="fix">FIX</option>
                                    </select>
                                  </label>
                                  <label className="cw-field">
                                    <span className="cw-label">Telemetry Format</span>
                                    <select 
                                      className="cw-input cw-select" 
                                      value={active.hiddenTelemetryFormat || 'stdout'} 
                                      onChange={(e) => updateProblem(active.id, { hiddenTelemetryFormat: e.target.value })}
                                    >
                                      <option value="stdout">Structured stdout (JSON Logs)</option>
                                      <option value="callback">Telemetry Callback (POST)</option>
                                    </select>
                                  </label>
                                </div>
                              </div>
                              
                              {active.hiddenBotFiles && active.hiddenBotFiles.length > 0 && (
                                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginBottom: '12px' }}>
                                  <span style={{ fontSize: '0.85rem', fontWeight: '500', color: 'var(--text, #4b5563)' }}>Uploaded Custom Hidden Bots:</span>
                                  {active.hiddenBotFiles.map((file) => (
                                    <div key={file.name} style={{ 
                                      display: 'flex', 
                                      alignItems: 'center', 
                                      justifyContent: 'space-between', 
                                      padding: '8px 12px', 
                                      background: 'rgba(0, 0, 0, 0.02)', 
                                      border: '1px solid rgba(0, 0, 0, 0.08)', 
                                      borderRadius: '6px' 
                                    }}>
                                      <span style={{ fontSize: '0.88rem', color: 'var(--text-h, #0f172a)', display: 'flex', alignItems: 'center', gap: '8px' }}>
                                        📄 {file.name}
                                      </span>
                                      <button 
                                        className="cw-button cw-button-ghost" 
                                        style={{ padding: '2px 6px', minWidth: 'auto', fontSize: '0.82rem', color: '#ff6b6b' }}
                                        onClick={() => handleRemoveBotFile(active.id, 'hidden', file.name)}
                                      >
                                        Remove
                                      </button>
                                    </div>
                                  ))}
                                </div>
                              )}

                              <div className="cw-file-upload">
                                <input 
                                  type="file" 
                                  accept=".go" 
                                  onChange={(e) => {
                                    handleBotFileChange(active.id, 'hidden', e.target.files?.[0]);
                                    e.target.value = '';
                                  }} 
                                />
                                <span className="cw-file-text">➕ Upload Custom Go Bot File (.go)</span>
                              </div>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  </form>
                  );
                })()}
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="cw-review-layout">
              <div className="cw-card cw-review-card">
                <div className="cw-card-header">
                  <span className="cw-card-icon">📋</span>
                  <h4>Contest Summary</h4>
                </div>
                <div className="cw-card-body">
                  <div className="cw-review-grid">
                    <div className="cw-review-item">
                      <span className="cw-review-label">Name</span>
                      <strong>{details.name || '—'}</strong>
                    </div>
                    <div className="cw-review-item">
                      <span className="cw-review-label">Visibility</span>
                      <strong>{details.visibility === 'public' ? '🌍 Public' : '🔒 Private'}</strong>
                    </div>
                    <div className="cw-review-item">
                      <span className="cw-review-label">Start Time</span>
                      <strong>{details.startTime ? new Date(details.startTime).toLocaleString() : '—'}</strong>
                    </div>
                    <div className="cw-review-item">
                      <span className="cw-review-label">Duration</span>
                      <strong>{details.durationMinutes} min</strong>
                    </div>
                    <div className="cw-review-item">
                      <span className="cw-review-label">Registration Deadline</span>
                      <strong>{details.registrationDeadline ? new Date(details.registrationDeadline).toLocaleString() : '—'}</strong>
                    </div>
                    <div className="cw-review-item">
                      <span className="cw-review-label">Problems</span>
                      <strong>{problems.length}</strong>
                    </div>
                  </div>
                  {details.description && (
                    <div className="cw-review-desc">
                      <span className="cw-review-label">Description</span>
                      <p>{details.description}</p>
                    </div>
                  )}
                </div>
              </div>

              <div className="cw-card">
                <div className="cw-card-header">
                  <span className="cw-card-icon">🧩</span>
                  <h4>Problems ({problems.length})</h4>
                </div>
                <div className="cw-card-body">
                  <div className="cw-review-problems">
                    {problems.map((p, idx) => (
                      <div key={p.id} className="cw-review-problem-row">
                        <div className="cw-problem-idx">{String.fromCharCode(65 + idx)}</div>
                        <div className="cw-review-problem-info">
                          <strong>{p.code ? `${p.code} — ` : ''}{p.title || '(untitled)'}</strong>
                          <span className="cw-problem-meta">
                            {p.timeLimit}s · {p.memoryLimit}MB
                            {` · Sample: ${[...(p.sampleStrategies || []).map(s => s.replace(/_/g, ' ')), ...(p.sampleBotFiles || []).map(f => f.name)].join(', ') || 'None'}`}
                            {p.sampleBotFiles && p.sampleBotFiles.length > 0 && ` (Injection: ${p.sampleTargetInjection || 'env'}, Protocol: ${(p.sampleProtocol || 'http').toUpperCase()}, Telemetry: ${p.sampleTelemetryFormat || 'stdout'})`}
                            {` · Hidden: ${[...(p.hiddenStrategies || []).map(s => s.replace(/_/g, ' ')), ...(p.hiddenBotFiles || []).map(f => f.name)].join(', ') || 'None'}`}
                            {p.hiddenBotFiles && p.hiddenBotFiles.length > 0 && ` (Injection: ${p.hiddenTargetInjection || 'env'}, Protocol: ${(p.hiddenProtocol || 'http').toUpperCase()}, Telemetry: ${p.hiddenTelemetryFormat || 'stdout'})`}
                          </span>
                        </div>
                        <div className={`cw-review-status ${p.title && p.statement ? 'ready' : 'incomplete'}`}>
                          {p.title && p.statement ? '✓ Ready' : '⚠ Incomplete'}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              <div className="cw-card cw-checklist-card">
                <div className="cw-card-header">
                  <span className="cw-card-icon">✓</span>
                  <h4>Pre-Publish Checklist</h4>
                </div>
                <div className="cw-card-body">
                  <div className="cw-checklist">
                    <div className={`cw-check-item ${validateDetails() ? 'done' : ''}`}>
                      <span className="cw-check-icon">{validateDetails() ? '✓' : '○'}</span>
                      <span>Contest details completed</span>
                    </div>
                    <div className={`cw-check-item ${validateProblems() ? 'done' : ''}`}>
                      <span className="cw-check-icon">{validateProblems() ? '✓' : '○'}</span>
                      <span>All problems have title & statement</span>
                    </div>
                    <div className={`cw-check-item ${problems.length >= 1 ? 'done' : ''}`}>
                      <span className="cw-check-icon">{problems.length >= 1 ? '✓' : '○'}</span>
                      <span>At least one problem added</span>
                    </div>
                  </div>
                </div>
              </div>

              <div className="cw-publish-actions">
                {!initialData && (
                  <button className="cw-button cw-button-outline" onClick={saveDraft} disabled={saving}>💾 Save Draft</button>
                )}
                <button className="cw-button cw-button-outline" onClick={() => window.alert('Preview: open in new tab')}>👁️ Preview</button>
                <button className="cw-button cw-button-publish cw-button-lg" onClick={publish} disabled={saving}>
                  {saving ? 'Publishing...' : (initialData ? '🚀 Update Contest' : '🚀 Publish Contest')}
                </button>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}