import React, { useState } from 'react';
import { App, RunToolRequest, appApi } from '../services/api';
import DynamicForm from './DynamicForm';
import ResultDisplay from './ResultDisplay';

interface AppToolRunnerProps {
  app: App;
  toolName: string;
}

const AppToolRunner: React.FC<AppToolRunnerProps> = ({ app, toolName }) => {
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  const formatToolName = (name: string) => {
    return name.split('_').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  };

  const tool = app.tools.find(t => t.name === toolName);
  
  const handleRun = async (jsonData: object) => {
    try {
      setRunning(true);
      setError(null);
      setResult(null);

      const request: RunToolRequest = {
        appId: app.appId,
        toolName: toolName,
        input: jsonData
      };

      const response = await appApi.runTool(request);
      setResult(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to run tool');
    } finally {
      setRunning(false);
    }
  };

  if (!tool) {
    return (
      <div className="app-tool-runner">
        <div className="error-section">
          <h4>Error:</h4>
          <div className="error-message">Tool "{toolName}" not found</div>
        </div>
      </div>
    );
  }

  return (
    <div className="app-tool-runner">
      <div className="tool-header">
        <h3>{formatToolName(toolName)}</h3>
      </div>

      <DynamicForm 
        inputFormat={tool.inputFormat}
        onSubmit={handleRun}
        isSubmitting={running}
        toolName={toolName}
      />

      {error && (
        <div className="error-section">
          <h4>Error:</h4>
          <div className="error-message">{error}</div>
        </div>
      )}
      
      {result && (
        <ResultDisplay result={result} title={`${formatToolName(toolName)} Result`} />
      )}
    </div>
  );
};

export default AppToolRunner;