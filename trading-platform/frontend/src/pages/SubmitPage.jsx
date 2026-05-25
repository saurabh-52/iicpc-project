import React, { useState } from 'react';
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
    protocol: 'websocket',
    strategy: 'bbo_heavy',
    file: null
  });
  const [submitState, setSubmitState] = useState({ type: '', message: '' });
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleChange = (e) => {
    const { name, value, files } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: files ? files[0] : value
    }));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsSubmitting(true);
    setSubmitState({ type: '', message: '' });

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

      setSubmitState({
        type: 'success',
        message: result?.message || 'Engine submitted successfully.',
      });
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
                <option value="websocket">WebSocket</option>
                <option value="tcp">Raw TCP</option>
                <option value="grpc">gRPC</option>
                <option value="http">HTTP/REST</option>
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

          <button type="submit" className="button button-primary submit-button" disabled={isSubmitting}>
            {isSubmitting ? 'Submitting...' : 'Deploy and test'}
          </button>
        </form>
      </section>
    </section>
  );
}