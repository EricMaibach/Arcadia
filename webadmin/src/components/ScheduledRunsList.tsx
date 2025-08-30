import React, { useState, useEffect } from 'react';
import { scheduleApi, ScheduledRun } from '../services/api';

const ScheduledRunsList: React.FC = () => {
  const [runs, setRuns] = useState<ScheduledRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterScheduleId, setFilterScheduleId] = useState('');

  useEffect(() => {
    loadRuns();
  }, [filterScheduleId]);

  const loadRuns = async () => {
    try {
      setLoading(true);
      setError(null);
      const runsList = await scheduleApi.listScheduledRuns(filterScheduleId || undefined);
      setRuns(runsList);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load scheduled runs');
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const formatDuration = (startedAt: string, completedAt?: string) => {
    const start = new Date(startedAt);
    const end = completedAt ? new Date(completedAt) : new Date();
    const duration = end.getTime() - start.getTime();
    
    if (duration < 1000) return `${duration}ms`;
    if (duration < 60000) return `${(duration / 1000).toFixed(1)}s`;
    if (duration < 3600000) return `${(duration / 60000).toFixed(1)}m`;
    return `${(duration / 3600000).toFixed(1)}h`;
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed': return 'green';
      case 'failed': return 'red';
      case 'running': return 'blue';
      default: return 'gray';
    }
  };

  if (loading) return <div className="loading">Loading scheduled runs...</div>;

  return (
    <div className="scheduled-runs-list">
      <h2>Scheduled Runs</h2>

      <div className="filters">
        <input
          type="text"
          placeholder="Filter by Schedule ID"
          value={filterScheduleId}
          onChange={(e) => setFilterScheduleId(e.target.value)}
          className="filter-input"
        />
        <button onClick={loadRuns} className="refresh-btn">Refresh</button>
      </div>

      {error && <div className="error">Error: {error}</div>}
      
      {runs.length === 0 ? (
        <p>No scheduled runs found.</p>
      ) : (
        <div className="runs-table">
          <table>
            <thead>
              <tr>
                <th>Run ID</th>
                <th>Schedule ID</th>
                <th>App ID</th>
                <th>Tool</th>
                <th>Status</th>
                <th>Started At</th>
                <th>Duration</th>
                <th>Output/Error</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((run) => (
                <tr key={run.id}>
                  <td className="run-id">{run.id}</td>
                  <td className="schedule-id">{run.scheduleId}</td>
                  <td>{run.appId}</td>
                  <td>{run.toolName}</td>
                  <td>
                    <span 
                      className="status"
                      style={{ color: getStatusColor(run.status) }}
                    >
                      {run.status}
                    </span>
                  </td>
                  <td>{formatDate(run.startedAt)}</td>
                  <td>
                    {run.status === 'running' ? (
                      <span className="running">Running...</span>
                    ) : (
                      formatDuration(run.startedAt, run.completedAt)
                    )}
                  </td>
                  <td className="output-cell">
                    {run.status === 'failed' && run.error ? (
                      <details>
                        <summary className="error-summary">Error (click to expand)</summary>
                        <pre className="error-details">{run.error}</pre>
                      </details>
                    ) : run.output ? (
                      <details>
                        <summary>Output (click to expand)</summary>
                        <pre className="output-details">{run.output}</pre>
                      </details>
                    ) : (
                      <span className="no-output">No output</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default ScheduledRunsList;