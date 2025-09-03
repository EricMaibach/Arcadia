import React, { useState } from 'react';
import { appApi, ToolInfo, AppRequest } from '../services/api';

const AppSubmit: React.FC = () => {
  const [appId, setAppId] = useState('');
  const [version, setVersion] = useState('');
  const [runtime, setRuntime] = useState('wasm');
  const [appSrc, setAppSrc] = useState('');
  const [tools, setTools] = useState<ToolInfo[]>([{ name: '', inputFormat: '' }]);
  const [dependencies, setDependencies] = useState<{ name: string; version: string }[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);
  const [fullResponse, setFullResponse] = useState<any>(null);

  const addTool = () => {
    setTools([...tools, { name: '', inputFormat: '' }]);
  };

  const removeTool = (index: number) => {
    setTools(tools.filter((_, i) => i !== index));
  };

  const updateTool = (index: number, field: keyof ToolInfo, value: string) => {
    const updatedTools = tools.map((tool, i) => 
      i === index ? { ...tool, [field]: value } : tool
    );
    setTools(updatedTools);
  };

  const addDependency = () => {
    setDependencies([...dependencies, { name: '', version: '' }]);
  };

  const removeDependency = (index: number) => {
    setDependencies(dependencies.filter((_, i) => i !== index));
  };

  const updateDependency = (index: number, field: 'name' | 'version', value: string) => {
    const updatedDependencies = dependencies.map((dep, i) => 
      i === index ? { ...dep, [field]: value } : dep
    );
    setDependencies(updatedDependencies);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Validation
    if (!appId || !version || !appSrc) {
      setError('App ID, version, and app source are required');
      return;
    }

    const validTools = tools.filter(tool => tool.name.trim() && tool.inputFormat.trim());
    if (validTools.length === 0) {
      setError('At least one tool with name and input format is required');
      return;
    }

    try {
      setSubmitting(true);
      setError(null);
      setResult(null);
      setFullResponse(null);
      
      // Process dependencies
      const validDependencies = dependencies.filter(dep => dep.name.trim() && dep.version.trim());
      const dependenciesMap: { [key: string]: string } = {};
      validDependencies.forEach(dep => {
        dependenciesMap[dep.name.trim()] = dep.version.trim();
      });
      
      const request: AppRequest = {
        appId: appId.trim(),
        version: version.trim(),
        runtime,
        tools: validTools,
        appSrc: appSrc.trim(),
        ...(Object.keys(dependenciesMap).length > 0 && { dependencies: dependenciesMap })
      };

      console.log('Submitting request:', request);
      const response = await appApi.submitAppSrc(request);
      console.log('Full response received:', response);
      
      setResult(response);
      setFullResponse({
        status: 'success',
        data: response,
        timestamp: new Date().toISOString()
      });
      
      // Reset form on success
      setAppId('');
      setVersion('');
      setAppSrc('');
      setTools([{ name: '', inputFormat: '' }]);
      setDependencies([]);
    } catch (err: any) {
      console.error('Error submitting app:', err);
      
      // Capture full error details
      const errorDetails: any = {
        status: 'error',
        timestamp: new Date().toISOString(),
        error: {
          message: err.message || 'Unknown error',
          name: err.name || 'Error',
          stack: err.stack,
        }
      };

      // If it's an axios error, capture response details
      if (err.response) {
        errorDetails.error = {
          ...errorDetails.error,
          httpStatus: err.response.status,
          httpStatusText: err.response.statusText,
          responseData: err.response.data,
          responseHeaders: err.response.headers,
          config: {
            method: err.config?.method,
            url: err.config?.url,
            baseURL: err.config?.baseURL
          }
        };
      } else if (err.request) {
        // Network error - no response received
        errorDetails.error = {
          ...errorDetails.error,
          type: 'network_error',
          request: 'Request was made but no response received'
        };
      }

      setFullResponse(errorDetails);
      setError(err.response?.data?.message || err.message || 'Failed to submit app');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="app-submit">
      <h2>Submit New App</h2>
      
      <form onSubmit={handleSubmit} className="submit-form">
        <div className="form-group">
          <label htmlFor="appId">App ID:</label>
          <input
            type="text"
            id="appId"
            value={appId}
            onChange={(e) => setAppId(e.target.value)}
            placeholder="e.g., my-app"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="version">Version:</label>
          <input
            type="text"
            id="version"
            value={version}
            onChange={(e) => setVersion(e.target.value)}
            placeholder="e.g., 1.0.0"
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="runtime">Runtime:</label>
          <select
            id="runtime"
            value={runtime}
            onChange={(e) => setRuntime(e.target.value)}
          >
            <option value="wasm">WASM</option>
          </select>
        </div>

        <div className="form-group">
          <label>Tools:</label>
          {tools.map((tool, index) => (
            <div key={index} className="tool-input">
              <input
                type="text"
                value={tool.name}
                onChange={(e) => updateTool(index, 'name', e.target.value)}
                placeholder="Tool name"
              />
              <input
                type="text"
                value={tool.inputFormat}
                onChange={(e) => updateTool(index, 'inputFormat', e.target.value)}
                placeholder="Input format (JSON schema)"
              />
              {tools.length > 1 && (
                <button type="button" onClick={() => removeTool(index)} className="remove-btn">
                  Remove
                </button>
              )}
            </div>
          ))}
          <button type="button" onClick={addTool} className="add-btn">
            Add Tool
          </button>
        </div>

        <div className="form-group">
          <label>Dependencies (optional):</label>
          <div className="dependencies-info">
            <p>Common dependencies like chrono, regex, rand are auto-detected from 'use' statements. 
               Add explicit dependencies here to override versions or include additional crates.</p>
          </div>
          {dependencies.length > 0 && dependencies.map((dep, index) => (
            <div key={index} className="dependency-input">
              <input
                type="text"
                value={dep.name}
                onChange={(e) => updateDependency(index, 'name', e.target.value)}
                placeholder="Crate name (e.g., chrono)"
              />
              <input
                type="text"
                value={dep.version}
                onChange={(e) => updateDependency(index, 'version', e.target.value)}
                placeholder='Version (e.g., 0.4 or { version = "1.0", features = ["json"] })'
              />
              <button type="button" onClick={() => removeDependency(index)} className="remove-btn">
                Remove
              </button>
            </div>
          ))}
          <button type="button" onClick={addDependency} className="add-btn">
            Add Dependency
          </button>
        </div>

        <div className="form-group">
          <label htmlFor="appSrc">App Source (Rust trait implementation):</label>
          <textarea
            id="appSrc"
            value={appSrc}
            onChange={(e) => setAppSrc(e.target.value)}
            placeholder="Paste your Rust trait implementation here..."
            rows={15}
            required
          />
        </div>

        <button type="submit" disabled={submitting} className="submit-btn">
          {submitting ? 'Submitting...' : 'Submit App'}
        </button>
      </form>

      {error && <div className="error">Error: {error}</div>}
      
      {fullResponse && (
        <div className={fullResponse.status === 'success' ? 'result' : 'error'}>
          <h3>
            {fullResponse.status === 'success' ? 'Success!' : 'Full Error Response'}
          </h3>
          <div className="response-details">
            <p><strong>Status:</strong> {fullResponse.status}</p>
            <p><strong>Timestamp:</strong> {new Date(fullResponse.timestamp).toLocaleString()}</p>
          </div>
          <details className="response-data">
            <summary>
              <strong>
                {fullResponse.status === 'success' ? 'Response Data' : 'Error Details'} 
                (click to expand)
              </strong>
            </summary>
            <pre>{JSON.stringify(fullResponse, null, 2)}</pre>
          </details>
        </div>
      )}
    </div>
  );
};

export default AppSubmit;