import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import './App.css';
import Home from './components/Home';
import AppPage from './components/AppPage';
import Admin from './components/Admin';
import { appApi, App as AppType } from './services/api';

function App() {
  const [apps, setApps] = useState<AppType[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadApps();
  }, []);

  const loadApps = async () => {
    try {
      const appList = await appApi.listApps();
      setApps(appList);
    } catch (err) {
      console.error('Failed to load apps:', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Router>
      <div className="App">
        <header className="App-header">
          <Link to="/" className="logo-link">
            <h1>Arcadia</h1>
          </Link>
          <nav className="nav">
            <Link to="/" className="nav-link">Home</Link>
            {!loading && apps.map((app) => (
              <Link 
                key={app.appId} 
                to={`/app/${encodeURIComponent(app.appId)}`} 
                className="nav-link app-nav-link"
              >
                {app.appId}
              </Link>
            ))}
            <Link to="/admin" className="nav-link admin-link">Admin</Link>
          </nav>
        </header>
        
        <main className="main-content">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/app/:appId" element={<AppPage />} />
            <Route path="/admin/*" element={<Admin />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}

export default App;
