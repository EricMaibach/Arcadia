import React from 'react';
import { Routes, Route, Link } from 'react-router-dom';
import AppList from './AppList';
import AppSubmit from './AppSubmit';
import ToolRunner from './ToolRunner';
import ScheduleList from './ScheduleList';
import ScheduleForm from './ScheduleForm';
import ScheduledRunsList from './ScheduledRunsList';

const Admin: React.FC = () => {
  return (
    <div className="admin">
      <nav className="admin-nav">
        <Link to="/admin" className="admin-nav-link">Apps</Link>
        <Link to="/admin/submit" className="admin-nav-link">Submit App</Link>
        <Link to="/admin/run-tool" className="admin-nav-link">Run Tool</Link>
        <Link to="/admin/schedules" className="admin-nav-link">Schedules</Link>
        <Link to="/admin/scheduled-runs" className="admin-nav-link">Scheduled Runs</Link>
      </nav>
      
      <div className="admin-content">
        <Routes>
          <Route index element={<AppList />} />
          <Route path="submit" element={<AppSubmit />} />
          <Route path="run-tool" element={<ToolRunner />} />
          <Route path="schedules" element={<ScheduleList />} />
          <Route path="schedule/new" element={<ScheduleForm />} />
          <Route path="schedule/edit/:id" element={<ScheduleForm />} />
          <Route path="scheduled-runs" element={<ScheduledRunsList />} />
        </Routes>
      </div>
    </div>
  );
};

export default Admin;