import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { appApi, App } from '../services/api';
import AppToolRunner from './AppToolRunner';

const AppPage: React.FC = () => {
  const { appId } = useParams<{ appId: string }>();
  const [app, setApp] = useState<App | null>(null);
  const [selectedTool, setSelectedTool] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadApp();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appId]);

  const loadApp = async () => {
    if (!appId) return;
    
    try {
      setLoading(true);
      setError(null);
      const apps = await appApi.listApps();
      const foundApp = apps.find(a => a.appId === appId);
      
      if (!foundApp) {
        setError(`App '${appId}' not found`);
        return;
      }
      
      setApp(foundApp);
      if (foundApp.tools.length > 0) {
        setSelectedTool(foundApp.tools[0].name);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load app');
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div className="loading">Loading app...</div>;
  if (error) return <div className="error">Error: {error}</div>;
  if (!app) return <div className="error">App not found</div>;

  return (
    <div className="app-page">
      <div className="app-header">
        <h2>{app.appId}</h2>
        <div className="app-info">
          <span className="app-version">v{app.version}</span>
          <span className="app-runtime">{app.runtime}</span>
          {app.sourceLanguage && <span className="app-language">{app.sourceLanguage}</span>}
        </div>
      </div>

      {app.tools.length === 0 ? (
        <div className="no-tools">
          <p>No tools available for this app.</p>
        </div>
      ) : (
        <div className="app-tools">
          <div className="tool-tabs">
            {app.tools.map((tool) => (
              <button
                key={tool.name}
                className={`tool-tab ${selectedTool === tool.name ? 'active' : ''}`}
                onClick={() => setSelectedTool(tool.name)}
              >
                {tool.name}
              </button>
            ))}
          </div>

          {selectedTool && (
            <AppToolRunner
              app={app}
              toolName={selectedTool}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default AppPage;