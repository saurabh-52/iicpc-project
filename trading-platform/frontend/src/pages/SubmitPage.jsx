import { useState } from 'react';
import { Link } from 'react-router-dom';

export default function SubmitPage() {
  const languageExtensions = {
    cpp: ['cpp', 'cc', 'cxx'],
    go: ['go'],
    rust: ['rs'],
    python: ['py'],
  };

  const [formData, setFormData] = useState({
    systemName: '',
    port: '8080',
    language: 'cpp',
    protocol: 'http',
    strategy: 'bbo_heavy',
    file: null
  });
  const [submitState, setSubmitState] = useState({ type: '', message: '' });
  const [executionResult, setExecutionResult] = useState(null);
  const [stressTestResult, setStressTestResult] = useState(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

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
      const response = await fetch('/api/submit', {
        method: 'POST',
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
        throw new Error(result?.message || `Submission failed with status ${response.status}`);
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
          },
          body: JSON.stringify({
            target: sandboxExecution.target_url,
            protocol: formData.protocol,
            strategy: formData.strategy,
            bots: 16,
            requests: 48,
            timeout_ms: 2000,
            method: 'POST',
            path: '/',
            expect_reply: formData.protocol === 'tcp',
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

        setStressTestResult(stressResult?.metrics || null);
        setSubmitState({
          type: 'success',
          message: `${result?.message || 'Engine submitted successfully.'} Stress test launched with ${formData.strategy}.`,
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
              </select>
            </label>
          </div>

          <label className="field">
            <span>Stress Test Strategy</span>
            <select name="strategy" value={formData.strategy} onChange={handleChange}>
              <option value="bbo_heavy">BBO Heavy (High Liquidity)</option>
              <option value="flash_crash">Flash Crash (Volatility)</option>
              <option value="high_cancel">High Cancel Ratio (Spoofing)</option>
              <option value="wide_spread">Wide Spread (Memory Hog)</option>
            </select>
          </label>

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
                    ? `p99: ${stressTestResult.p99_latency_ms.toFixed(2)}ms`
                    : ''}
                </pre>
              </div>
            </section>
          ) : null}

          <button type="submit" className="button button-primary submit-button" disabled={isSubmitting}>
            {isSubmitting ? 'Submitting...' : 'Deploy and launch stress test'}
          </button>
        </form>
      </section>
    </section>
  );
}