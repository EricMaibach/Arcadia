import React, { useState, useEffect } from 'react';
import { fileWatcherApi } from '../services/api';

const FileWatcher: React.FC = () => {
  const [watchedDirectories, setWatchedDirectories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newDirectoryPath, setNewDirectoryPath] = useState('');
  const [isAdding, setIsAdding] = useState(false);

  useEffect(() => {
    loadWatchedDirectories();
  }, []);

  const loadWatchedDirectories = async () => {
    try {
      setLoading(true);
      setError(null);
      const directories = await fileWatcherApi.listWatchedDirectories();
      setWatchedDirectories(directories || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load watched directories');
      setWatchedDirectories([]); // Set empty array on error
    } finally {
      setLoading(false);
    }
  };

  const handleAddDirectory = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDirectoryPath.trim()) return;

    try {
      setIsAdding(true);
      setError(null);
      await fileWatcherApi.addWatchedDirectory(newDirectoryPath.trim());
      setNewDirectoryPath('');
      await loadWatchedDirectories(); // Refresh the list
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add directory');
    } finally {
      setIsAdding(false);
    }
  };

  const handleRemoveDirectory = async (path: string) => {
    if (!window.confirm(`Are you sure you want to stop watching "${path}"?`)) {
      return;
    }

    try {
      setError(null);
      await fileWatcherApi.removeWatchedDirectory(path);
      await loadWatchedDirectories(); // Refresh the list
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove directory');
    }
  };

  if (loading) return <div className="loading">Loading watched directories...</div>;

  return (
    <div className="file-watcher">
      <h2>File Watcher Management</h2>
      <p>Manage directories being monitored for file changes.</p>

      {error && (
        <div className="error">
          Error: {error}
          <button onClick={() => setError(null)} className="error-dismiss">×</button>
        </div>
      )}

      <div className="add-directory-section">
        <h3>Add Directory to Watch</h3>
        <form onSubmit={handleAddDirectory} className="add-directory-form">
          <div className="form-group">
            <label htmlFor="directoryPath">Directory Path:</label>
            <input
              type="text"
              id="directoryPath"
              value={newDirectoryPath}
              onChange={(e) => setNewDirectoryPath(e.target.value)}
              placeholder="/path/to/directory"
              required
              className="directory-input"
            />
          </div>
          <button 
            type="submit" 
            disabled={isAdding || !newDirectoryPath.trim()}
            className="add-btn"
          >
            {isAdding ? 'Adding...' : 'Add Directory'}
          </button>
        </form>
      </div>

      <div className="watched-directories-section">
        <div className="section-header">
          <h3>Watched Directories ({watchedDirectories?.length || 0})</h3>
          <button onClick={loadWatchedDirectories} className="refresh-btn">
            Refresh
          </button>
        </div>

        {!watchedDirectories || watchedDirectories.length === 0 ? (
          <div className="empty-state">
            <p>No directories are currently being watched.</p>
            <p>Add a directory above to start monitoring file changes.</p>
          </div>
        ) : (
          <div className="directories-list">
            {watchedDirectories?.map((directory) => (
              <div key={directory} className="directory-item">
                <div className="directory-info">
                  <span className="directory-path" title={directory}>
                    {directory}
                  </span>
                </div>
                <div className="directory-actions">
                  <button
                    onClick={() => handleRemoveDirectory(directory)}
                    className="remove-btn"
                    title={`Stop watching ${directory}`}
                  >
                    Remove
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="info-section">
        <h3>How It Works</h3>
        <ul>
          <li>Add directories to monitor for file changes (create, modify, delete)</li>
          <li>File events are logged to the server console in real-time</li>
          <li>Watched directories persist across server restarts</li>
          <li>Use absolute paths for best results</li>
        </ul>
      </div>
    </div>
  );
};

export default FileWatcher;