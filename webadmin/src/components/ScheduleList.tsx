import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { scheduleApi, AppSchedule } from '../services/api';

const ScheduleList: React.FC = () => {
  const [schedules, setSchedules] = useState<AppSchedule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterAppId, setFilterAppId] = useState('');

  useEffect(() => {
    loadSchedules();
  }, [filterAppId]);

  const loadSchedules = async () => {
    try {
      setLoading(true);
      setError(null);
      const scheduleList = await scheduleApi.listSchedules(filterAppId || undefined);
      setSchedules(scheduleList);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load schedules');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this schedule?')) {
      return;
    }

    try {
      await scheduleApi.deleteSchedule(id);
      setSchedules(schedules.filter(s => s.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete schedule');
    }
  };

  const toggleActive = async (schedule: AppSchedule) => {
    try {
      await scheduleApi.updateSchedule(schedule.id, { isActive: !schedule.isActive });
      setSchedules(schedules.map(s => 
        s.id === schedule.id ? { ...s, isActive: !s.isActive } : s
      ));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update schedule');
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  if (loading) return <div className="loading">Loading schedules...</div>;

  return (
    <div className="schedule-list">
      <div className="header">
        <h2>App Schedules</h2>
        <Link to="/admin/schedule/new" className="new-btn">New Schedule</Link>
      </div>

      <div className="filters">
        <input
          type="text"
          placeholder="Filter by App ID"
          value={filterAppId}
          onChange={(e) => setFilterAppId(e.target.value)}
          className="filter-input"
        />
        <button onClick={loadSchedules} className="refresh-btn">Refresh</button>
      </div>

      {error && <div className="error">Error: {error}</div>}
      
      {schedules.length === 0 ? (
        <p>No schedules found.</p>
      ) : (
        <div className="schedules-table">
          <table>
            <thead>
              <tr>
                <th>App ID</th>
                <th>Tool</th>
                <th>Type</th>
                <th>Next Run</th>
                <th>Status</th>
                <th>Run Count</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {schedules.map((schedule) => (
                <tr key={schedule.id}>
                  <td>{schedule.appId}</td>
                  <td>{schedule.toolName}</td>
                  <td>{schedule.scheduleType}</td>
                  <td>
                    {schedule.nextRun ? formatDate(schedule.nextRun) : 'N/A'}
                  </td>
                  <td>
                    <span className={`status ${schedule.isActive ? 'active' : 'inactive'}`}>
                      {schedule.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td>{schedule.runCount}</td>
                  <td className="actions">
                    <Link to={`/admin/schedule/edit/${schedule.id}`} className="edit-btn">
                      Edit
                    </Link>
                    <button
                      onClick={() => toggleActive(schedule)}
                      className={`toggle-btn ${schedule.isActive ? 'deactivate' : 'activate'}`}
                    >
                      {schedule.isActive ? 'Deactivate' : 'Activate'}
                    </button>
                    <button
                      onClick={() => handleDelete(schedule.id)}
                      className="delete-btn"
                    >
                      Delete
                    </button>
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

export default ScheduleList;