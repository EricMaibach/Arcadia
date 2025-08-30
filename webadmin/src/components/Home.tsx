import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { appApi, App } from '../services/api';
import ArcadiaIcon from '../ArcadiaIcon.jpg';

const Home: React.FC = () => {
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
    <div className="home">
      <div className="welcome-section">
        <div className="hero-icon">
          <img src={ArcadiaIcon} alt="Arcadia Tree" className="hero-tree" />
        </div>
        <h2>Welcome to Arcadia</h2>
        <p>A digital ecosystem where applications grow and flourish together.<br />
        Select an app to work with its tools, or use the admin section to manage the platform.</p>
      </div>

      {apps.length === 0 ? (
        <div className="no-apps">
          <p>No apps are currently registered.</p>
          <Link to="/admin/submit" className="submit-app-link">
            Submit an App
          </Link>
        </div>
      ) : (
        <div className="apps-overview">
          <h3>Available Apps</h3>
          <div className="apps-grid">
            {apps.map((app) => (
              <Link 
                key={app.appId} 
                to={`/app/${encodeURIComponent(app.appId)}`} 
                className="app-card-link"
              >
                <div className="app-card">
                  <h4>{app.appId}</h4>
                  <div className="app-meta">
                    <span className="version">v{app.version}</span>
                    <span className="runtime">{app.runtime}</span>
                  </div>
                  <div className="tools-count">
                    {app.tools.length} tool{app.tools.length !== 1 ? 's' : ''}
                  </div>
                  {app.tools.length > 0 && (
                    <div className="tools-preview">
                      {app.tools.slice(0, 3).map((tool, index) => (
                        <span key={index} className="tool-name">
                          {tool.name}
                        </span>
                      ))}
                      {app.tools.length > 3 && (
                        <span className="more-tools">+{app.tools.length - 3} more</span>
                      )}
                    </div>
                  )}
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default Home;