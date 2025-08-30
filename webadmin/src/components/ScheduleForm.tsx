import React, { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { scheduleApi, appApi, App, ScheduleRequest, AppSchedule, RecurrenceRule } from '../services/api';

const ScheduleForm: React.FC = () => {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const isEdit = Boolean(id);

  const [apps, setApps] = useState<App[]>([]);
  const [formData, setFormData] = useState({
    appId: '',
    toolName: '',
    input: '{}',
    scheduleType: 'one-time' as 'one-time' | 'recurring',
    scheduledTime: '',
    recurrence: {
      interval: 1,
      unit: 'hours',
      daysOfWeek: [] as number[],
      endDate: ''
    }
  });
  
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadApps();
    if (isEdit && id) {
      loadSchedule(id);
    }
  }, [isEdit, id]);

  const loadApps = async () => {
    try {
      const appList = await appApi.listApps();
      setApps(appList);
    } catch (err) {
      setError('Failed to load apps');
    }
  };

  const loadSchedule = async (scheduleId: string) => {
    try {
      const schedule = await scheduleApi.getSchedule(scheduleId);
      setFormData({
        appId: schedule.appId,
        toolName: schedule.toolName,
        input: JSON.stringify(schedule.input, null, 2),
        scheduleType: schedule.scheduleType,
        scheduledTime: new Date(schedule.scheduledTime).toISOString().slice(0, 16),
        recurrence: schedule.recurrence ? {
          interval: schedule.recurrence.interval,
          unit: schedule.recurrence.unit,
          daysOfWeek: schedule.recurrence.daysOfWeek || [],
          endDate: schedule.recurrence.endDate ? 
            new Date(schedule.recurrence.endDate).toISOString().slice(0, 16) : ''
        } : {
          interval: 1,
          unit: 'hours',
          daysOfWeek: [],
          endDate: ''
        }
      });
    } catch (err) {
      setError('Failed to load schedule');
    }
  };

  const selectedApp = apps.find(app => app.appId === formData.appId);
  const availableTools = selectedApp?.tools || [];

  const handleInputChange = (field: string, value: any) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  const handleRecurrenceChange = (field: keyof RecurrenceRule, value: any) => {
    setFormData(prev => ({
      ...prev,
      recurrence: { ...prev.recurrence, [field]: value }
    }));
  };

  const toggleDayOfWeek = (day: number) => {
    const daysOfWeek = formData.recurrence.daysOfWeek.includes(day)
      ? formData.recurrence.daysOfWeek.filter(d => d !== day)
      : [...formData.recurrence.daysOfWeek, day].sort();
    
    handleRecurrenceChange('daysOfWeek', daysOfWeek);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!formData.appId || !formData.toolName || !formData.scheduledTime) {
      setError('App, tool, and scheduled time are required');
      return;
    }

    let parsedInput;
    try {
      parsedInput = JSON.parse(formData.input);
    } catch (err) {
      setError('Invalid JSON input');
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const request: ScheduleRequest = {
        appId: formData.appId,
        toolName: formData.toolName,
        input: parsedInput,
        scheduleType: formData.scheduleType,
        scheduledTime: new Date(formData.scheduledTime).toISOString(),
      };

      if (formData.scheduleType === 'recurring') {
        const recurrence: RecurrenceRule = {
          interval: formData.recurrence.interval,
          unit: formData.recurrence.unit,
        };

        if (formData.recurrence.unit === 'weeks' && formData.recurrence.daysOfWeek.length > 0) {
          recurrence.daysOfWeek = formData.recurrence.daysOfWeek;
        }

        if (formData.recurrence.endDate) {
          recurrence.endDate = new Date(formData.recurrence.endDate).toISOString();
        }

        request.recurrence = recurrence;
      }

      if (isEdit && id) {
        await scheduleApi.updateSchedule(id, request as any);
      } else {
        await scheduleApi.createSchedule(request);
      }

      navigate('/schedules');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save schedule');
    } finally {
      setLoading(false);
    }
  };

  const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

  return (
    <div className="schedule-form">
      <h2>{isEdit ? 'Edit Schedule' : 'New Schedule'}</h2>
      
      <form onSubmit={handleSubmit} className="form">
        <div className="form-group">
          <label htmlFor="appId">App:</label>
          <select
            id="appId"
            value={formData.appId}
            onChange={(e) => {
              handleInputChange('appId', e.target.value);
              handleInputChange('toolName', '');
            }}
            required
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
          <label htmlFor="toolName">Tool:</label>
          <select
            id="toolName"
            value={formData.toolName}
            onChange={(e) => handleInputChange('toolName', e.target.value)}
            disabled={!formData.appId}
            required
          >
            <option value="">-- Select Tool --</option>
            {availableTools.map(tool => (
              <option key={tool.name} value={tool.name}>
                {tool.name}
              </option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="input">Input (JSON):</label>
          <textarea
            id="input"
            value={formData.input}
            onChange={(e) => handleInputChange('input', e.target.value)}
            rows={6}
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="scheduleType">Schedule Type:</label>
          <select
            id="scheduleType"
            value={formData.scheduleType}
            onChange={(e) => handleInputChange('scheduleType', e.target.value as 'one-time' | 'recurring')}
          >
            <option value="one-time">One-time</option>
            <option value="recurring">Recurring</option>
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="scheduledTime">Scheduled Time:</label>
          <input
            type="datetime-local"
            id="scheduledTime"
            value={formData.scheduledTime}
            onChange={(e) => handleInputChange('scheduledTime', e.target.value)}
            required
          />
        </div>

        {formData.scheduleType === 'recurring' && (
          <div className="recurrence-section">
            <h3>Recurrence Settings</h3>
            
            <div className="form-group">
              <label htmlFor="interval">Repeat every:</label>
              <div className="interval-input">
                <input
                  type="number"
                  id="interval"
                  value={formData.recurrence.interval}
                  onChange={(e) => handleRecurrenceChange('interval', parseInt(e.target.value))}
                  min="1"
                  required
                />
                <select
                  value={formData.recurrence.unit}
                  onChange={(e) => handleRecurrenceChange('unit', e.target.value)}
                >
                  <option value="minutes">minutes</option>
                  <option value="hours">hours</option>
                  <option value="days">days</option>
                  <option value="weeks">weeks</option>
                  <option value="months">months</option>
                </select>
              </div>
            </div>

            {formData.recurrence.unit === 'weeks' && (
              <div className="form-group">
                <label>Days of week:</label>
                <div className="days-of-week">
                  {dayNames.map((day, index) => (
                    <label key={index} className="day-checkbox">
                      <input
                        type="checkbox"
                        checked={formData.recurrence.daysOfWeek.includes(index)}
                        onChange={() => toggleDayOfWeek(index)}
                      />
                      {day}
                    </label>
                  ))}
                </div>
              </div>
            )}

            <div className="form-group">
              <label htmlFor="endDate">End Date (optional):</label>
              <input
                type="datetime-local"
                id="endDate"
                value={formData.recurrence.endDate}
                onChange={(e) => handleRecurrenceChange('endDate', e.target.value)}
              />
            </div>
          </div>
        )}

        {error && <div className="error">Error: {error}</div>}

        <div className="form-actions">
          <button type="button" onClick={() => navigate('/schedules')} className="cancel-btn">
            Cancel
          </button>
          <button type="submit" disabled={loading} className="submit-btn">
            {loading ? 'Saving...' : isEdit ? 'Update Schedule' : 'Create Schedule'}
          </button>
        </div>
      </form>
    </div>
  );
};

export default ScheduleForm;