import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import useWebSocket from '../hooks/useWebSocket';

/* ═══════════════════════════════════════
   CONSTANTS
   ═══════════════════════════════════════ */

const STEP_TITLES = ['Contest Details', 'Review & Publish'];
const STEP_ICONS = ['📋', '🚀'];
const STEP_DESCRIPTIONS = ['Set up your contest info', 'Final check before going live'];
const STRATEGIES = ['bbo_heavy', 'flash_crash', 'high_cancel', 'wide_spread', 'market_maker', 'iceberg', 'momentum_burst'];
const DRAFT_KEY = 'contest_draft_v1';


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
  return `${hours}h ${mins}m`;
}

/* ═══════════════════════════════════════
   CONTEST PAGE — Root
   ═══════════════════════════════════════ */

function normalizeContests(dbContests) {
  return dbContests.map(c => {
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
    };
  });
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

  const normalized = normalizeContests(dbContests);
  const upcoming = normalized
    .filter(c => c.status === 'upcoming')
    .sort((a, b) => new Date(a.startTime) - new Date(b.startTime));
  const past = normalized
    .filter(c => c.status === 'ended')
    .sort((a, b) => new Date(b.startTime) - new Date(a.startTime));

  const handleEditContest = async (contestId) => {
    try {
      const res = await fetch(`/api/contests/${contestId}/full`, { headers: authHeaders() });
      if (!res.ok) throw new Error('Failed to fetch contest details');
      const data = await res.json();
      
      // Transform db models to what wizard expects
      const initialDetails = {
        ...data.details,
        startTime: formatLocalDatetime(data.details.startTime),
        registrationDeadline: formatLocalDatetime(data.details.registrationDeadline)
      };

      setEditingContestData({ details: initialDetails });
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
    </div>
  );
}

/* ═══════════════════════════════════════
   LANDING VIEW
   ═══════════════════════════════════════ */

function LandingView({ onNavigate, upcomingContests = [], pastContests = [], loading, onRegister, registeredContestIds = [], user, onEdit, onDelete, finalizationProgress }) {
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

function ContestTable({ contests, type, onRegister, registeredContestIds = [], user, onEdit, onDelete, finalizationProgress }) {
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

function ContestTableRow({ contest, type, onRegister, isRegistered, user, onEdit, onDelete, finalizationProgress = null }) {
  const isPast = type === 'past';
  const navigate = useNavigate();
  const { authHeaders } = useAuth();
  const now = new Date();
  const start = new Date(contest.startTime);
  const end = new Date(start.getTime() + (contest.duration || 60) * 60000);
  const isLive = now >= start && now < end;
  const isEnded = now >= end;
  const isHost = user && contest.createdBy && user.id === contest.createdBy;

  const [finalizing, setFinalizing] = useState(false);
  const [finalizeMsg, setFinalizeMsg] = useState('');

  const handleFinalize = async () => {
    if (!confirm('Run final evaluation rounds? This will re-evaluate all submissions with 5 deterministic test suites.')) return;
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
            {contest.strategy && (
              <span style={{ fontSize: '0.65rem', color: '#60a5fa', fontWeight: 600, background: 'rgba(96, 165, 250, 0.1)', padding: '2px 6px', borderRadius: '4px' }}>
                🎯 {contest.strategy.replace(/_/g, ' ')}
              </span>
            )}
            {isLive && (
              <span style={{ fontSize: '0.65rem', color: '#ef4444', fontWeight: 600, background: 'rgba(239, 68, 68, 0.1)', padding: '2px 6px', borderRadius: '4px' }}>
                80% Fixed
              </span>
            )}
          </div>
          <p style={{ fontSize: '0.8rem', color: 'var(--text-muted, #888)', margin: '4px 0 0 0', display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{contest.description}</p>
        </div>
      </td>
      <td>
        <div>{formatDate(contest.startTime)}</div>
        <div style={{ fontSize: '0.8rem', color: 'var(--text-muted, #888)', marginTop: '2px' }}>{formatTime(contest.startTime)}</div>
      </td>
      <td>{contest.duration}m</td>
      <td>
        <span style={{ fontWeight: 500, display: 'flex', alignItems: 'center', gap: '4px' }}>
          👤 {contest.participants}
        </span>
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
                <button className="cw-button cw-button-secondary" style={{ padding: '4px 8px', fontSize: '0.75rem' }} onClick={() => onEdit(contest.id)}>Edit</button>
                <button className="cw-button cw-button-secondary" style={{ padding: '4px 8px', fontSize: '0.75rem', color: '#ef4444', borderColor: 'rgba(239, 68, 68, 0.3)' }} onClick={() => onDelete(contest.id)}>Delete</button>
                <button className="cw-button cw-button-secondary" style={{ padding: '4px 8px', fontSize: '0.75rem' }} onClick={() => navigate(`/leaderboard?contest_id=${contest.id}`)}>Rank</button>
                {isEnded && contest.phase !== 'completed' && (
                  <button className="cw-button cw-button-primary" style={{ padding: '4px 8px', fontSize: '0.75rem' }} onClick={handleFinalize} disabled={finalizing || contest.phase === 'finalizing'}>
                    {finalizing || contest.phase === 'finalizing' ? '...' : 'Finalize'}
                  </button>
                )}
                {isLive && (
                  <button className="cw-button cw-button-primary" style={{ padding: '4px 8px', fontSize: '0.75rem', background: '#ef4444' }} onClick={() => navigate(`/submit?contest_id=${contest.id}&strategy=${contest.strategy || 'bbo_heavy'}`)}>
                    Submit
                  </button>
                )}
              </>
            ) : !isPast && !isLive ? (
              isRegistered ? (
                <button className="cw-button cw-button-ghost" disabled style={{ padding: '4px 12px', fontSize: '0.8rem', borderColor: '#22c55e', color: '#22c55e' }}>Registered ✓</button>
              ) : (
                <button className="cw-button cw-button-primary" style={{ padding: '4px 12px', fontSize: '0.8rem' }} onClick={() => onRegister && onRegister(contest)}>Register →</button>
              )
            ) : isLive ? (
              <>
                <button className="cw-button cw-button-primary" style={{ background: '#ef4444', padding: '4px 12px', fontSize: '0.8rem' }} onClick={() => navigate(`/submit?contest_id=${contest.id}&strategy=${contest.strategy || 'bbo_heavy'}`)}>
                  Submit
                </button>
                <button className="cw-button cw-button-secondary" style={{ padding: '4px 12px', fontSize: '0.8rem' }} onClick={() => navigate(`/leaderboard?contest_id=${contest.id}`)}>
                  Rank
                </button>
              </>
            ) : (
              <button className="cw-button cw-button-secondary" style={{ padding: '4px 12px', fontSize: '0.8rem' }} onClick={() => navigate(`/leaderboard?contest_id=${contest.id}`)}>
                Leaderboard
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

function JoinContestView({ onBack, upcomingContests = [], pastContests = [], onRegister, registeredContestIds = [], user, onEdit, onDelete, finalizationProgress }) {
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

      {/* Contest cards */}
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

  const autosave = useRef(null);
  const [message, setMessage] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (initialData) {
      setDetails(initialData.details);
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
      }
    } catch (e) {
    }
  }, [initialData]);

  // Autosave to localStorage (debounced), but only if not editing an existing contest
  useEffect(() => {
    if (initialData) return;
    if (autosave.current) clearTimeout(autosave.current);
    autosave.current = setTimeout(() => {
      try {
        localStorage.setItem(DRAFT_KEY, JSON.stringify({ details }));
        setMessage('Draft auto-saved');
        setTimeout(() => setMessage(''), 1000);
      } catch (e) {
        setMessage('Draft save failed');
      }
    }, 750);
    return () => clearTimeout(autosave.current);
  }, [details, initialData]);

  const validateDetails = () => details.name.trim() !== '' && details.startTime !== '' && (details.visibility !== 'private' || (details.code && details.code.trim() !== '')) && details.strategy && details.strategy.trim() !== '' && details.finalStrategies && details.finalStrategies.length > 0;

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
        body: JSON.stringify({ details: isoDetails }) 
      });
      if (!res.ok) {
        const errData = await res.json().catch(() => null);
        throw new Error(errData?.message || errData?.error || 'Save failed');
      }
      if (!initialData) {
        localStorage.setItem(DRAFT_KEY, JSON.stringify({ details }));
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
        body: JSON.stringify({ details: isoDetails }) 
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
    validateDetails(),
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
            <span>{Math.round((completedSteps.filter(Boolean).length / 2) * 100)}%</span>
          </div>
          <div className="wizard-progress-track">
            <div
              className="wizard-progress-fill"
              style={{ width: `${(completedSteps.filter(Boolean).length / 2) * 100}%` }}
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
            <div className="wizard-step-badge">{step + 1} / 2</div>
            <div>
              <h2 className="wizard-page-title">{STEP_TITLES[step]}</h2>
              <p className="wizard-page-subtitle">{STEP_DESCRIPTIONS[step]}</p>
            </div>
          </div>
          <div className="wizard-topbar-right">
            {message && <div className={`wizard-toast ${message.includes('✓') ? 'success' : message.includes('⚠') ? 'warning' : ''}`}>{message}</div>}
            <div className="wizard-nav-buttons">
              {step > 0 && <button className="cw-button cw-button-ghost" onClick={() => setStep(step - 1)}>← Back</button>}
              {step < 1 && <button className="cw-button cw-button-primary" onClick={() => setStep(step + 1)}>Next →</button>}
              {step === 1 && (
                <button className="cw-button cw-button-publish" onClick={publish} disabled={saving}>
                  {saving ? 'Publishing...' : '🚀 Publish Contest'}
                </button>
              )}
            </div>
          </div>
        </div>

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
                    <input className="cw-input" placeholder="e.g. IICPC Finals 2026" value={details.name} onChange={(e) => setDetails({ ...details, name: e.target.value })} />
                  </label>
                  <label className="cw-field">
                    <span className="cw-label">Description</span>
                    <textarea className="cw-input cw-textarea" rows={4} placeholder="Describe your contest — rules, scoring, theme..." value={details.description} onChange={(e) => setDetails({ ...details, description: e.target.value })} />
                  </label>
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
                  <div className="cw-field" style={{ marginTop: '16px' }}>
                    <span className="cw-label">Sample Test Strategies <span className="cw-required">*</span></span>
                    <span className="cw-field-desc" style={{ fontSize: '0.82rem', color: 'var(--text-muted, #888)', display: 'block', marginBottom: '8px' }}>
                      Select one or more baseline trading profiles for sample runs:
                    </span>
                    <div className="cw-strategy-chips" style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                      {STRATEGIES.map((s) => {
                        const selected = (details.strategy || '').split(',').filter(Boolean);
                        return (
                          <label key={s} className={`cw-chip ${selected.includes(s) ? 'active' : ''}`}>
                            <input type="checkbox" hidden checked={selected.includes(s)} onChange={(e) => {
                              const next = new Set(selected);
                              if (e.target.checked) next.add(s); else next.delete(s);
                              setDetails({ ...details, strategy: Array.from(next).join(',') });
                            }} />
                            {s.replace(/_/g, ' ')}
                          </label>
                        );
                      })}
                    </div>
                  </div>
                  <div className="cw-field" style={{ marginTop: '16px' }}>
                    <span className="cw-label">Hidden Test Strategies <span className="cw-required">*</span></span>
                    <span className="cw-field-desc" style={{ fontSize: '0.82rem', color: 'var(--text-muted, #888)', display: 'block', marginBottom: '8px' }}>
                      Select one or more baseline trading profiles for final grading runs:
                    </span>
                    <div className="cw-strategy-chips" style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                      {STRATEGIES.map((s) => (
                        <label key={s} className={`cw-chip cw-chip-hidden ${(details.finalStrategies || []).includes(s) ? 'active' : ''}`}>
                          <input type="checkbox" hidden checked={(details.finalStrategies || []).includes(s)} onChange={(e) => {
                            const next = new Set(details.finalStrategies || []);
                            if (e.target.checked) next.add(s); else next.delete(s);
                            setDetails({ ...details, finalStrategies: Array.from(next) });
                          }} />
                          {s.replace(/_/g, ' ')}
                        </label>
                      ))}
                    </div>
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
                  <label className="cw-field">
                    <span className="cw-label">Banner Image (optional)</span>
                    <div className="cw-file-upload">
                      <input type="file" accept="image/*" onChange={(e) => {
                        const f = e.target.files?.[0] || null;
                        setDetails({ ...details, bannerPreview: f ? URL.createObjectURL(f) : null });
                      }} />
                      <span className="cw-file-text">📁 Choose file or drag here</span>
                    </div>
                  </label>
                  {details.bannerPreview && (
                    <div className="cw-banner-preview">
                      <img src={details.bannerPreview} alt="banner" />
                    </div>
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
                      <input className="cw-input" type="datetime-local" value={details.startTime} onChange={(e) => setDetails({ ...details, startTime: e.target.value })} />
                    </label>
                    <label className="cw-field">
                      <span className="cw-label">Registration Deadline</span>
                      <input className="cw-input" type="datetime-local" value={details.registrationDeadline} onChange={(e) => setDetails({ ...details, registrationDeadline: e.target.value })} />
                    </label>
                  </div>
                </div>
              </div>
            </div>
          )}

          {step === 1 && (
            <div className="cw-review-layout">
              <div className="cw-card cw-review-card">
                <div className="cw-card-header">
                  <span className="cw-card-icon">📊</span>
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
                  </div>
                  {details.description && (
                    <div className="cw-review-desc">
                      <span className="cw-review-label">Description</span>
                      <p>{details.description}</p>
                    </div>
                  )}
                </div>
              </div>


              <div className="cw-card cw-checklist-card">
                <div className="cw-card-header">
                  <span className="cw-card-icon">✅</span>
                  <h4>Pre-Publish Checklist</h4>
                </div>
                <div className="cw-card-body">
                  <div className="cw-checklist">
                    <div className={`cw-check-item ${validateDetails() ? 'done' : ''}`}>
                      <span className="cw-check-icon">{validateDetails() ? '✓' : '○'}</span>
                      <span>Contest details completed</span>
                    </div>
                  </div>
                </div>
              </div>

              <div className="cw-publish-actions">
                {!initialData && (
                  <button className="cw-button cw-button-outline" onClick={saveDraft} disabled={saving}>💾 Save Draft</button>
                )}
                <button className="cw-button cw-button-outline" onClick={() => window.alert('Preview: open in new tab')}>👁 Preview</button>
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