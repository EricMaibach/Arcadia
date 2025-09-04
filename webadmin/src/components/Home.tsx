import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { appApi, App } from '../services/api';
import ArcadiaIcon from '../ArcadiaIcon.jpg';

interface ChatMessage {
  id: string;
  content: string;
  isUser: boolean;
  timestamp: Date;
}

interface ContextStats {
  message_count: number;
  total_tokens: number;
}

const Home: React.FC = () => {
  const [apps, setApps] = useState<App[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentView, setCurrentView] = useState<'chat' | 'apps'>('chat');
  
  // Chat state
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [inputMessage, setInputMessage] = useState('');
  const [chatLoading, setChatLoading] = useState(false);
  const [contextStats, setContextStats] = useState<ContextStats | null>(null);
  
  // Generate a session ID that persists for this session
  const [sessionId] = useState(() => {
    // Try to get existing session from sessionStorage or generate new one
    let id = sessionStorage.getItem('arcadia-session-id');
    if (!id) {
      id = `session-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      sessionStorage.setItem('arcadia-session-id', id);
    }
    return id;
  });

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

  const sendMessage = async () => {
    if (!inputMessage.trim() || chatLoading) return;
    
    console.log('Sending message to Arcadia:', inputMessage.trim());

    const userMessage: ChatMessage = {
      id: Date.now().toString() + '-user',
      content: inputMessage.trim(),
      isUser: true,
      timestamp: new Date()
    };

    setMessages(prev => [...prev, userMessage]);
    setInputMessage('');
    setChatLoading(true);

    try {
      const response = await fetch('http://localhost:8080/claude', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ 
          message: userMessage.content,
          session_id: sessionId 
        }),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = await response.json();
      
      // Update context stats if provided
      if (data.context_stats) {
        setContextStats(data.context_stats);
      }
      
      const claudeMessage: ChatMessage = {
        id: Date.now().toString() + '-claude',
        content: data.response || 'No response received',
        isUser: false,
        timestamp: new Date()
      };

      setMessages(prev => [...prev, claudeMessage]);
    } catch (err) {
      const errorMessage: ChatMessage = {
        id: Date.now().toString() + '-error',
        content: `Error: ${err instanceof Error ? err.message : 'Failed to get response from Arcadia'}`,
        isUser: false,
        timestamp: new Date()
      };
      setMessages(prev => [...prev, errorMessage]);
    } finally {
      setChatLoading(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  if (loading) return <div className="loading">Loading apps...</div>;
  if (error) return <div className="error">Error: {error}</div>;

  return (
    <div className="home">
      {/* View Toggle Button */}
      <div className="view-toggle">
        <button 
          className={`view-toggle-btn ${currentView === 'apps' ? 'active' : ''}`}
          onClick={() => setCurrentView(currentView === 'chat' ? 'apps' : 'chat')}
          title={currentView === 'chat' ? 'View Apps' : 'Back to Chat'}
        >
          {currentView === 'chat' ? (
            <svg width="36" height="36" viewBox="0 0 36 36" fill="none" stroke="currentColor" strokeWidth="1.2">
              <path d="M10 2c0 0-6 2-6 8s6 6 6 6c2-1 4-3 4-6s-4-8-4-8z" fill="rgba(127,176,105,0.35)" strokeLinejoin="round"/>
              <path d="M10 2c1 2 2 4 2 8" strokeLinecap="round" opacity="0.7"/>
              
              <path d="M26 2c0 0-6 2-6 8s6 6 6 6c2-1 4-3 4-6s-4-8-4-8z" fill="rgba(127,176,105,0.35)" strokeLinejoin="round"/>
              <path d="M26 2c1 2 2 4 2 8" strokeLinecap="round" opacity="0.7"/>
              
              <path d="M10 20c0 0-6 2-6 8s6 6 6 6c2-1 4-3 4-6s-4-8-4-8z" fill="rgba(127,176,105,0.35)" strokeLinejoin="round"/>
              <path d="M10 20c1 2 2 4 2 8" strokeLinecap="round" opacity="0.7"/>
              
              <path d="M26 20c0 0-6 2-6 8s6 6 6 6c2-1 4-3 4-6s-4-8-4-8z" fill="rgba(127,176,105,0.35)" strokeLinejoin="round"/>
              <path d="M26 20c1 2 2 4 2 8" strokeLinecap="round" opacity="0.7"/>
            </svg>
          ) : (
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
            </svg>
          )}
        </button>
      </div>

      {currentView === 'chat' ? (
        /* Chat Interface */
        <div className="claude-chat">
          {contextStats && (
            <div className="context-stats" style={{
              padding: '0.5rem 1rem',
              background: 'linear-gradient(135deg, rgba(74, 124, 89, 0.1), rgba(127, 176, 105, 0.1))',
              borderBottom: '1px solid rgba(244, 211, 94, 0.3)',
              fontSize: '0.85rem',
              color: '#6c757d',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center'
            }}>
              <div>
                <span>Context: {contextStats.message_count} messages</span>
                {contextStats.total_tokens > 0 && (
                  <span style={{ marginLeft: '1rem' }}>
                    • {contextStats.total_tokens.toLocaleString()} tokens
                  </span>
                )}
              </div>
              <div style={{ fontSize: '0.75rem', opacity: 0.7 }}>
                Session: {sessionId.slice(-8)}
              </div>
            </div>
          )}
          <div className="chat-messages">
              {messages.length === 0 && (
                <div className="chat-placeholder">
                  <div className="hero-icon">
                    <img src={ArcadiaIcon} alt="Arcadia Tree" className="hero-tree" />
                  </div>
                  <h2>Welcome to Arcadia</h2>
                  <p>A digital ecosystem where applications grow and flourish together.</p>
                  <p>Ask me anything to get started!</p>
                </div>
              )}
              {messages.map((message) => (
                <div 
                  key={message.id} 
                  className={`chat-message ${message.isUser ? 'user' : 'claude'}`}
                >
                  <div className="message-content">
                    {message.content}
                  </div>
                  <div className="message-timestamp">
                    {message.timestamp.toLocaleTimeString()}
                  </div>
                </div>
              ))}
              {chatLoading && (
                <div className="chat-message claude loading">
                  <div className="message-content">
                    <div className="typing-indicator">
                      <span></span>
                      <span></span>
                      <span></span>
                    </div>
                  </div>
                </div>
              )}
            </div>
            <div className="chat-input">
              <textarea
                value={inputMessage}
                onChange={(e) => setInputMessage(e.target.value)}
                onKeyDown={handleKeyPress}
                placeholder="Ask Arcadia anything..."
                rows={2}
                disabled={chatLoading}
              />
              <button 
                onClick={sendMessage}
                disabled={!inputMessage.trim() || chatLoading}
                className="arcadia-send-btn"
                title="Send message to Arcadia"
                style={{
                  padding: '0.5rem',
                  borderRadius: '50%',
                  width: '48px',
                  height: '48px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: !inputMessage.trim() || chatLoading 
                    ? 'linear-gradient(135deg, #adb5bd, #ced4da)' 
                    : 'linear-gradient(135deg, #4a7c59, #7fb069)',
                  border: !inputMessage.trim() || chatLoading 
                    ? '1px solid #dee2e6' 
                    : '1px solid #f4d35e',
                  boxShadow: !inputMessage.trim() || chatLoading 
                    ? 'none' 
                    : '0 4px 12px rgba(74, 124, 89, 0.4), 0 0 20px rgba(244, 211, 94, 0.3)',
                  cursor: !inputMessage.trim() || chatLoading ? 'not-allowed' : 'pointer',
                  fontSize: 0,
                  color: 'transparent',
                  transition: 'all 0.3s ease'
                }}
              >
                <img 
                  src={ArcadiaIcon} 
                  alt="Send to Arcadia" 
                  className="arcadia-send-icon"
                  style={{
                    width: '36px',
                    height: '36px',
                    borderRadius: '50%',
                    filter: !inputMessage.trim() || chatLoading 
                      ? 'grayscale(100%) brightness(0.8)' 
                      : 'brightness(1.2) contrast(1.1)',
                    opacity: !inputMessage.trim() || chatLoading ? 0.5 : 1,
                    animation: !inputMessage.trim() || chatLoading 
                      ? 'none' 
                      : 'arcadia-breathe 4s ease-in-out infinite'
                  }}
                />
              </button>
            </div>
          </div>
      ) : (
        /* Apps View */
        <div className="apps-view">
          <div className="apps-header">
            <h2>Available Apps</h2>
            <p>Discover and interact with apps in the Arcadia ecosystem</p>
          </div>
          
          {loading ? (
            <div className="loading">Loading apps...</div>
          ) : error ? (
            <div className="error">Error: {error}</div>
          ) : apps.length === 0 ? (
            <div className="no-apps">
              <div className="empty-state">
                <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                  <line x1="9" y1="9" x2="15" y2="15"/>
                  <line x1="15" y1="9" x2="9" y2="15"/>
                </svg>
                <h3>No Apps Available</h3>
                <p>No apps are currently registered in the system.</p>
                <Link to="/admin/submit" className="submit-app-link">
                  Submit an App
                </Link>
              </div>
            </div>
          ) : (
            <div className="apps-grid">
              {apps.map((app) => (
                <Link 
                  key={app.appId} 
                  to={`/app/${encodeURIComponent(app.appId)}`} 
                  className="app-card-link"
                >
                  <div className="app-card">
                    <div className="app-icon">
                      <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                        <rect x="3" y="3" width="7" height="7"/>
                        <rect x="14" y="3" width="7" height="7"/>
                        <rect x="14" y="14" width="7" height="7"/>
                        <rect x="3" y="14" width="7" height="7"/>
                      </svg>
                    </div>
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
          )}
        </div>
      )}
    </div>
  );
};

export default Home;