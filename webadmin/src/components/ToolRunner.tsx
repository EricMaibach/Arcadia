import React, { useState, useEffect } from 'react';
import { appApi, App, RunToolRequest } from '../services/api';
import ResultDisplay from './ResultDisplay';

const ToolRunner: React.FC = () => {
  const [apps, setApps] = useState<App[]>([]);
  const [selectedAppId, setSelectedAppId] = useState('');
  const [selectedTool, setSelectedTool] = useState('');
  const [input, setInput] = useState('{}');
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadApps();
  }, []);

  const loadApps = async () => {
    try {
      const appList = await appApi.listApps();
      setApps(appList);
    } catch (err) {
      setError('Failed to load apps');
    }
  };

  const selectedApp = apps.find(app => app.appId === selectedAppId);
  const availableTools = selectedApp?.tools || [];

  const handleRun = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!selectedAppId || !selectedTool) {
      setError('Please select an app and tool');
      return;
    }

    let parsedInput;
    try {
      parsedInput = JSON.parse(input);
    } catch (err) {
      setError('Invalid JSON input');
      return;
    }

    try {
      setRunning(true);
      setError(null);
      setResult(null);

      const request: RunToolRequest = {
        appId: selectedAppId,
        toolName: selectedTool,
        input: parsedInput
      };

      const response = await appApi.runTool(request);
      setResult(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to run tool');
    } finally {
      setRunning(false);
    }
  };

  const getToolInputFormat = (toolName: string): string => {
    const tool = availableTools.find(t => t.name === toolName);
    return tool?.inputFormat || '{}';
  };

  return (
    <div className="tool-runner">
      <h2>Run Tool</h2>
      
      <form onSubmit={handleRun} className="runner-form">
        <div className="form-group">
          <label htmlFor="appSelect">Select App:</label>
          <select
            id="appSelect"
            value={selectedAppId}
            onChange={(e) => {
              setSelectedAppId(e.target.value);
              setSelectedTool('');
            }}
          >
            <option value="">-- Select App --</option>
            {apps.map(app => (
              <option key={app.appId} value={app.appId}>
                {app.appId} (v{app.version})
              </option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="toolSelect">Select Tool:</label>
          <select
            id="toolSelect"
            value={selectedTool}
            onChange={(e) => setSelectedTool(e.target.value)}
            disabled={!selectedAppId}
          >
            <option value="">-- Select Tool --</option>
            {availableTools.map(tool => (
              <option key={tool.name} value={tool.name}>
                {tool.name}
              </option>
            ))}
          </select>
        </div>

        {selectedTool && (
          <div className="form-group">
            <label>Expected Input Format:</label>
            <pre className="input-format">
              {getToolInputFormat(selectedTool)}
            </pre>
          </div>
        )}

        <div className="form-group">
          <label htmlFor="input">Input (JSON):</label>
          <textarea
            id="input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Enter JSON input for the tool"
            rows={8}
          />
        </div>

        <button type="submit" disabled={running || !selectedAppId || !selectedTool} className="run-btn">
          {running ? 'Running...' : 'Run Tool'}
        </button>
      </form>

      {error && <div className="error">Error: {error}</div>}
      
      {result && (
        <ResultDisplay result={result} title="Tool Execution Result" />
      )}
    </div>
  );
};

export default ToolRunner;