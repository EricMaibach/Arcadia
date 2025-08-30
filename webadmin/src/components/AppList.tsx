import React, { useState, useEffect } from 'react';
import { appApi, App } from '../services/api';

const AppList: React.FC = () => {
  const [apps, setApps] = useState<App[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadApps();
  }, []);

  const loadApps = async () => {
    try {
      setLoading(true);
      setError(null);
      const appList = await appApi.listApps();
      setApps(appList);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load apps');
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div className="loading">Loading apps...</div>;
  if (error) return <div className="error">Error: {error}</div>;

  return (
    <div className="app-list">
      <h2>Registered Apps</h2>
      <button onClick={loadApps} className="refresh-btn">Refresh</button>
      
      {apps.length === 0 ? (
        <p>No apps registered yet.</p>
      ) : (
        <div className="apps-grid">
          {apps.map((app) => (
            <div key={app.appId} className="app-card">
              <h3>{app.appId}</h3>
              <p><strong>Version:</strong> {app.version}</p>
              <p><strong>Runtime:</strong> {app.runtime}</p>
              <p><strong>Source Language:</strong> {app.sourceLanguage || 'N/A'}</p>
              <p><strong>Artifact:</strong> {app.artifactUri}</p>
              
              <div className="tools-section">
                <h4>Available Tools:</h4>
                {app.tools.length === 0 ? (
                  <p>No tools available</p>
                ) : (
                  <ul className="tools-list">
                    {app.tools.map((tool, index) => (
                      <li key={index} className="tool-item">
                        <strong>{tool.name}</strong>
                        <br />
                        <small>Input Format: {tool.inputFormat}</small>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              
              {app.files && app.files.length > 0 && (
                <div className="files-section">
                  <h4>Files:</h4>
                  <ul className="files-list">
                    {app.files.map((file, index) => (
                      <li key={index}>{file.name}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default AppList;