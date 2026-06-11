import { useState } from 'react';
import { useAuth } from '../context/AuthContext';

export default function HostDashboard() {
  const { authHeaders } = useAuth();
  
  const [file, setFile] = useState(null);
  const [uploadStatus, setUploadStatus] = useState('');
  
  const [demoConf, setDemoConf] = useState({
    num_bots: 5,
    orders_per_second: 10,
    run_duration_seconds: 30,
    order_size_min: 1,
    order_size_max: 50
  });
  const [demoStatus, setDemoStatus] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleFileChange = (e) => {
    if (e.target.files && e.target.files.length > 0) {
      setFile(e.target.files[0]);
    }
  };

  const handleUpload = async (e) => {
    e.preventDefault();
    if (!file) return;

    setUploadStatus('Uploading...');
    const formData = new FormData();
    formData.append('bot_file', file);

    try {
      const res = await fetch('/api/host/bot-upload', {
        method: 'POST',
        headers: authHeaders(), // don't set Content-Type for FormData, browser does it with boundary
        body: formData,
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'Upload failed');
      setUploadStatus('Upload successful!');
      setFile(null);
      e.target.reset();
    } catch (err) {
      setUploadStatus(`Error: ${err.message}`);
    }
  };

  const handleDemoChange = (e) => {
    const { name, value } = e.target;
    setDemoConf(prev => ({ ...prev, [name]: parseInt(value, 10) || 0 }));
  };

  const handleDemoSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setDemoStatus('Starting demo run...');

    try {
      const res = await fetch('/api/host/demo-run', {
        method: 'POST',
        headers: {
          ...authHeaders(),
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(demoConf)
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'Demo run failed');
      setDemoStatus(`Demo run started successfully. Run ID: ${data.run_id}`);
    } catch (err) {
      setDemoStatus(`Error: ${err.message}`);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="dashboard-grid">
      <div className="dashboard-main" style={{ gridColumn: 'span 2' }}>
        <div className="dashboard-header panel">
          <span className="section-tag">Host Tools</span>
          <h2>Host Dashboard</h2>
          <p>Administrative tools for uploading custom bots and running lightweight demo traffic.</p>
        </div>
      </div>

      <div className="dashboard-main" style={{ gridColumn: 'span 1' }}>
        <div className="panel">
          <h3 style={{ marginBottom: '1rem' }}>Upload Custom Bot</h3>
          <p style={{ marginBottom: '1.5rem', fontSize: '0.9rem', color: 'var(--text-dim)' }}>
            Upload a custom bot binary or script. This will be stored securely in the host workspace.
          </p>
          <form onSubmit={handleUpload} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <input 
              type="file" 
              onChange={handleFileChange} 
              style={{ border: '1px dashed var(--border)', padding: '1rem', borderRadius: '8px', cursor: 'pointer' }}
            />
            <button 
              type="submit" 
              disabled={!file} 
              className="button button-primary"
            >
              Upload Bot
            </button>
            {uploadStatus && (
              <p style={{ fontSize: '0.9rem', color: uploadStatus.startsWith('Error') ? 'var(--error)' : 'var(--success)' }}>
                {uploadStatus}
              </p>
            )}
          </form>
        </div>
      </div>

      <div className="dashboard-main" style={{ gridColumn: 'span 1' }}>
        <div className="panel">
          <h3 style={{ marginBottom: '1rem' }}>Run Demo Traffic</h3>
          <p style={{ marginBottom: '1.5rem', fontSize: '0.9rem', color: 'var(--text-dim)' }}>
            Start a lightweight demo run with custom configuration. Demo runs are explicitly tagged and do not appear on the global leaderboard.
          </p>
          <form onSubmit={handleDemoSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>Number of Bots</span>
              <input 
                type="number" 
                name="num_bots" 
                value={demoConf.num_bots} 
                onChange={handleDemoChange}
                style={{ width: '100px', padding: '0.5rem', borderRadius: '4px', border: '1px solid var(--border)' }}
              />
            </label>
            <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>Orders per Second</span>
              <input 
                type="number" 
                name="orders_per_second" 
                value={demoConf.orders_per_second} 
                onChange={handleDemoChange}
                style={{ width: '100px', padding: '0.5rem', borderRadius: '4px', border: '1px solid var(--border)' }}
              />
            </label>
            <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>Duration (seconds)</span>
              <input 
                type="number" 
                name="run_duration_seconds" 
                value={demoConf.run_duration_seconds} 
                onChange={handleDemoChange}
                style={{ width: '100px', padding: '0.5rem', borderRadius: '4px', border: '1px solid var(--border)' }}
              />
            </label>
            
            <button 
              type="submit" 
              disabled={submitting} 
              className="button button-secondary"
            >
              {submitting ? 'Starting...' : 'Start Demo'}
            </button>
            
            {demoStatus && (
              <p style={{ fontSize: '0.9rem', color: demoStatus.startsWith('Error') ? 'var(--error)' : 'var(--success)' }}>
                {demoStatus}
              </p>
            )}
          </form>
        </div>
      </div>
    </div>
  );
}
