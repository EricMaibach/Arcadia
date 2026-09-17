import React, { useState, useEffect } from 'react';
import { documentApi, BasicSearchResponse, EnhancedSearchResponse } from '../services/api';
import DocumentResultCard from './DocumentResultCard';

const DocumentSearch: React.FC = () => {
  const [query, setQuery] = useState('');
  const [searchType, setSearchType] = useState<'basic' | 'enhanced'>('enhanced');
  const [topK, setTopK] = useState(5);
  const [maxHighlights, setMaxHighlights] = useState(10);
  const [includeFullContent, setIncludeFullContent] = useState(false);

  const [searching, setSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [basicResults, setBasicResults] = useState<BasicSearchResponse | null>(null);
  const [enhancedResults, setEnhancedResults] = useState<EnhancedSearchResponse | null>(null);

  // Load search history from localStorage
  const [searchHistory, setSearchHistory] = useState<string[]>(() => {
    const saved = localStorage.getItem('arcadia-search-history');
    return saved ? JSON.parse(saved) : [];
  });

  // Save search history to localStorage
  useEffect(() => {
    localStorage.setItem('arcadia-search-history', JSON.stringify(searchHistory));
  }, [searchHistory]);

  const addToHistory = (searchQuery: string) => {
    if (searchQuery.trim() && !searchHistory.includes(searchQuery.trim())) {
      const newHistory = [searchQuery.trim(), ...searchHistory].slice(0, 10); // Keep last 10
      setSearchHistory(newHistory);
    }
  };

  const handleSearch = async () => {
    if (!query.trim()) {
      setError('Please enter a search query');
      return;
    }

    if (topK < 1 || topK > 10) {
      setError('Top K must be between 1 and 10');
      return;
    }

    setSearching(true);
    setError(null);
    setBasicResults(null);
    setEnhancedResults(null);

    try {
      if (searchType === 'basic') {
        const response = await documentApi.searchDocuments({
          query: query.trim(),
          top_k: topK,
        });
        setBasicResults(response);
      } else {
        const response = await documentApi.searchDocumentsEnhanced({
          query: query.trim(),
          top_k: topK,
          config: {
            max_highlights: maxHighlights,
            include_full_content: includeFullContent,
          },
        });
        setEnhancedResults(response);
      }

      addToHistory(query.trim());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      console.error('Search error:', err);
    } finally {
      setSearching(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSearch();
    }
  };

  const clearResults = () => {
    setBasicResults(null);
    setEnhancedResults(null);
    setError(null);
  };

  const results = searchType === 'basic' ? basicResults : enhancedResults;
  const hasResults = results && results.results.length > 0;

  return (
    <div className="document-search">
      <div className="document-search-header">
        <h2>Document Search</h2>
        <p>Search through indexed documents using vector similarity</p>
      </div>

      <div className="search-container">
        <div className="search-input-group">
          <input
            type="text"
            className="search-input"
            placeholder="Search documents... (e.g., 'marketplace promos issue')"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyPress}
            disabled={searching}
          />
          <button
            className="search-button"
            onClick={handleSearch}
            disabled={searching || !query.trim()}
          >
            {searching ? (
              <>
                <div className="spinner"></div>
                Searching...
              </>
            ) : (
              <>
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="11" cy="11" r="8"></circle>
                  <path d="m21 21-4.35-4.35"></path>
                </svg>
                Search
              </>
            )}
          </button>
        </div>

        {searchHistory.length > 0 && !searching && !results && (
          <div className="search-history">
            <span className="history-label">Recent:</span>
            {searchHistory.slice(0, 5).map((item, idx) => (
              <button
                key={idx}
                className="history-item"
                onClick={() => setQuery(item)}
                title={item}
              >
                {item}
              </button>
            ))}
          </div>
        )}

        <div className="search-options">
          <div className="option-group">
            <label className="option-label">Search Type:</label>
            <div className="radio-group">
              <label className="radio-label">
                <input
                  type="radio"
                  value="basic"
                  checked={searchType === 'basic'}
                  onChange={(e) => setSearchType(e.target.value as 'basic')}
                  disabled={searching}
                />
                <span>Basic</span>
              </label>
              <label className="radio-label">
                <input
                  type="radio"
                  value="enhanced"
                  checked={searchType === 'enhanced'}
                  onChange={(e) => setSearchType(e.target.value as 'enhanced')}
                  disabled={searching}
                />
                <span>Enhanced</span>
              </label>
            </div>
          </div>

          <div className="option-group">
            <label className="option-label">
              Top K: <span className="option-value">{topK}</span> documents
            </label>
            <input
              type="range"
              min="1"
              max="10"
              value={topK}
              onChange={(e) => setTopK(parseInt(e.target.value))}
              className="range-input"
              disabled={searching}
            />
          </div>

          {searchType === 'enhanced' && (
            <>
              <div className="option-group">
                <label className="option-label">
                  Max Highlights: <span className="option-value">{maxHighlights}</span> snippets
                </label>
                <input
                  type="range"
                  min="1"
                  max="20"
                  value={maxHighlights}
                  onChange={(e) => setMaxHighlights(parseInt(e.target.value))}
                  className="range-input"
                  disabled={searching}
                />
              </div>

              <div className="option-group">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    checked={includeFullContent}
                    onChange={(e) => setIncludeFullContent(e.target.checked)}
                    disabled={searching}
                  />
                  <span>Include full document content</span>
                </label>
              </div>
            </>
          )}
        </div>
      </div>

      {error && (
        <div className="search-error">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="12"></line>
            <line x1="12" y1="16" x2="12.01" y2="16"></line>
          </svg>
          {error}
        </div>
      )}

      {hasResults && (
        <div className="search-results">
          <div className="results-header">
            <div className="results-info">
              <h3>Search Results</h3>
              <span className="results-meta">
                Found {results.total_documents} documents in {results.search_time_ms}ms
              </span>
            </div>
            <button className="clear-button" onClick={clearResults}>
              Clear Results
            </button>
          </div>

          <div className="results-list">
            {results.results.map((result, idx) => (
              <DocumentResultCard
                key={idx}
                result={result}
                searchType={searchType}
                searchTimeMs={results.search_time_ms}
              />
            ))}
          </div>
        </div>
      )}

      {!searching && !results && !error && (
        <div className="search-empty-state">
          <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1">
            <circle cx="11" cy="11" r="8"></circle>
            <path d="m21 21-4.35-4.35"></path>
          </svg>
          <h3>Search Documents</h3>
          <p>Enter a query to search through your indexed documents.</p>
          <div className="empty-state-tips">
            <h4>Tips:</h4>
            <ul>
              <li>Use natural language queries like "marketplace promos issue"</li>
              <li>Enhanced search provides context highlights and previews</li>
              <li>Adjust Top K to control the number of results</li>
              <li>Results are ranked by semantic similarity</li>
            </ul>
          </div>
        </div>
      )}

      {searching && (
        <div className="search-loading">
          <div className="loading-spinner"></div>
          <p>Searching through documents...</p>
        </div>
      )}

      {results && results.results.length === 0 && !searching && (
        <div className="search-no-results">
          <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="8" y1="15" x2="16" y2="15"></line>
            <line x1="9" y1="9" x2="9.01" y2="9"></line>
            <line x1="15" y1="9" x2="15.01" y2="9"></line>
          </svg>
          <h3>No Results Found</h3>
          <p>No documents matched your query "{results.query}"</p>
          <p className="suggestion">Try different keywords or check if documents have been indexed.</p>
        </div>
      )}
    </div>
  );
};

export default DocumentSearch;
