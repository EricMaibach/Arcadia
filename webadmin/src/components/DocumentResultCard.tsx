import React, { useState } from 'react';
import { DocumentSearchResult, EnhancedDocumentSearchResult } from '../services/api';

interface DocumentResultCardProps {
  result: DocumentSearchResult | EnhancedDocumentSearchResult;
  searchType: 'basic' | 'enhanced';
  searchTimeMs: number;
}

const DocumentResultCard: React.FC<DocumentResultCardProps> = ({ result, searchType, searchTimeMs }) => {
  const [copied, setCopied] = useState(false);
  const [showRawJson, setShowRawJson] = useState(false);

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  const formatScore = (score: number): string => {
    return (score * 100).toFixed(1);
  };

  const getFileIcon = (path: string): string => {
    const ext = path.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'pdf':
        return '📄';
      case 'md':
      case 'markdown':
        return '📝';
      case 'txt':
        return '📃';
      case 'json':
        return '📋';
      case 'js':
      case 'ts':
      case 'tsx':
      case 'jsx':
        return '📜';
      case 'py':
        return '🐍';
      case 'go':
        return '🔷';
      default:
        return '📄';
    }
  };

  const renderBasicResult = (basicResult: DocumentSearchResult) => (
    <>
      {basicResult.chunks.slice(0, 3).map((chunk, idx) => (
        <div key={idx} className="document-chunk">
          <div className="chunk-score">
            Score: {formatScore(chunk.score)}%
          </div>
          <div className="chunk-content">
            "{chunk.content}"
          </div>
          <div className="chunk-index">
            Chunk #{chunk.chunk_index}
          </div>
        </div>
      ))}
      {basicResult.chunks.length > 3 && (
        <div className="more-chunks">
          + {basicResult.chunks.length - 3} more chunks
        </div>
      )}
    </>
  );

  const renderEnhancedResult = (enhancedResult: EnhancedDocumentSearchResult) => (
    <>
      {enhancedResult.context_highlights.slice(0, 5).map((highlight, idx) => (
        <div key={idx} className="document-highlight">
          <div className="highlight-content">
            "{highlight}"
          </div>
        </div>
      ))}
      {enhancedResult.context_highlights.length > 5 && (
        <div className="more-highlights">
          + {enhancedResult.context_highlights.length - 5} more highlights
        </div>
      )}
      {enhancedResult.content_preview && (
        <div className="content-preview">
          <strong>Preview:</strong> {enhancedResult.content_preview}
          {enhancedResult.is_truncated && <span className="truncated-badge">Truncated</span>}
        </div>
      )}
    </>
  );

  return (
    <div className="document-result-card">
      <div className="document-header">
        <div className="document-title">
          <span className="file-icon">{getFileIcon(result.document.file_path)}</span>
          <span className="file-path">{result.document.file_path}</span>
        </div>
        <div className="document-actions">
          <button
            className="copy-path-btn"
            onClick={() => copyToClipboard(result.document.file_path)}
            title="Copy file path"
          >
            {copied ? (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            ) : (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
              </svg>
            )}
          </button>
          <button
            className="toggle-json-btn"
            onClick={() => setShowRawJson(!showRawJson)}
            title="Toggle raw JSON view"
          >
            {showRawJson ? (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polyline points="9 11 12 14 15 11"></polyline>
                <polyline points="9 17 12 20 15 17"></polyline>
                <line x1="12" y1="20" x2="12" y2="4"></line>
              </svg>
            ) : (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polyline points="16 18 22 12 16 6"></polyline>
                <polyline points="8 6 2 12 8 18"></polyline>
              </svg>
            )}
          </button>
        </div>
      </div>

      <div className="document-metadata">
        <span className="metadata-item">
          <strong>Score:</strong> {formatScore(result.best_score)}%
        </span>
        <span className="metadata-item">
          <strong>Rank:</strong> #{result.relevance_rank}
        </span>
        <span className="metadata-item">
          <strong>Chunks:</strong> {searchType === 'basic' ? (result as DocumentSearchResult).total_chunks : result.document.chunk_count}
        </span>
        <span className="metadata-item">
          <strong>Search Time:</strong> {searchTimeMs}ms
        </span>
      </div>

      {showRawJson ? (
        <div className="raw-json-container">
          <div className="raw-json-header">
            <strong>Raw JSON Response</strong>
            <button
              className="copy-json-btn"
              onClick={() => copyToClipboard(JSON.stringify(result, null, 2))}
              title="Copy JSON to clipboard"
            >
              {copied ? 'Copied!' : 'Copy JSON'}
            </button>
          </div>
          <pre className="raw-json-content">
            {JSON.stringify(result, null, 2)}
          </pre>
        </div>
      ) : (
        <>
          <div className="document-content">
            {searchType === 'basic'
              ? renderBasicResult(result as DocumentSearchResult)
              : renderEnhancedResult(result as EnhancedDocumentSearchResult)
            }
          </div>

          {result.document.metadata && Object.keys(result.document.metadata).length > 0 && (
            <details className="document-metadata-details">
              <summary>Document Metadata</summary>
              <pre>{JSON.stringify(result.document.metadata, null, 2)}</pre>
            </details>
          )}
        </>
      )}
    </div>
  );
};

export default DocumentResultCard;
