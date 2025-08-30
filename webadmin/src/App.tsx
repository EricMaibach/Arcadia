import React from 'react';
import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import './App.css';
import AppList from './components/AppList';
import AppSubmit from './components/AppSubmit';
import ToolRunner from './components/ToolRunner';
import ScheduleList from './components/ScheduleList';
import ScheduleForm from './components/ScheduleForm';
import ScheduledRunsList from './components/ScheduledRunsList';

function App() {
  return (
    <Router>
      <div className="App">
        <header className="App-header">
          <h1>Arcadia App Engine Admin</h1>
          <nav className="nav">
            <Link to="/" className="nav-link">Apps</Link>
            <Link to="/submit" className="nav-link">Submit App</Link>
            <Link to="/run-tool" className="nav-link">Run Tool</Link>
            <Link to="/schedules" className="nav-link">Schedules</Link>
            <Link to="/scheduled-runs" className="nav-link">Scheduled Runs</Link>
          </nav>
        </header>
        
        <main className="main-content">
          <Routes>
            <Route path="/" element={<AppList />} />
            <Route path="/submit" element={<AppSubmit />} />
            <Route path="/run-tool" element={<ToolRunner />} />
            <Route path="/schedules" element={<ScheduleList />} />
            <Route path="/schedule/new" element={<ScheduleForm />} />
            <Route path="/schedule/edit/:id" element={<ScheduleForm />} />
            <Route path="/scheduled-runs" element={<ScheduledRunsList />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}

export default App;
