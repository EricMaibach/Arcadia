import React, { useState } from 'react';
import { analyzeResult, FormattedResult } from '../utils/resultFormatter';
import ResultTable from './ResultTable';
import ResultObject from './ResultObject';

interface ResultDisplayProps {
  result: any;
  title?: string;
}

const ResultDisplay: React.FC<ResultDisplayProps> = ({ result, title = 'Result' }) => {
  const [showRaw, setShowRaw] = useState(false);
  const formattedResult: FormattedResult = analyzeResult(result);

  const renderFormattedResult = () => {
    switch (formattedResult.type) {
      case 'error':
        return (
          <div className="result-error">
            <div className="error-header">
              <h4>{title}</h4>
              <span className="error-status">
                {formattedResult.statusInfo?.status || 'Error'}
              </span>
            </div>
            <div className="error-content">
              <div className="error-message">
                {formattedResult.error || 'An error occurred'}
              </div>
              {formattedResult.statusInfo && (
                <div className="error-details">
                  <strong>Status:</strong> {formattedResult.statusInfo.status}<br />
                  <strong>Success:</strong> {formattedResult.statusInfo.success ? 'Yes' : 'No'}
                </div>
              )}
            </div>
          </div>
        );

      case 'empty':
        return (
          <div className="result-empty">
            <h4>{title}</h4>
            <p>No result returned</p>
          </div>
        );

      case 'array':
        if (formattedResult.isEmpty) {
          return (
            <div className="result-empty">
              <h4>{title}</h4>
              <p>Empty array returned</p>
            </div>
          );
        }
        return <ResultTable data={formattedResult.data} title={title} />;

      case 'object':
        if (formattedResult.isEmpty) {
          return (
            <div className="result-empty">
              <h4>{title}</h4>
              <p>Empty object returned</p>
            </div>
          );
        }
        return <ResultObject data={formattedResult.data} title={title} />;

      case 'primitive':
        return (
          <div className="result-primitive">
            <div className="primitive-header">
              <h4>{title}</h4>
              <span className="primitive-type">
                {typeof formattedResult.data}
              </span>
            </div>
            <div className="primitive-value">
              {String(formattedResult.data)}
            </div>
          </div>
        );

      default:
        return (
          <div className="result-unknown">
            <h4>{title}</h4>
            <p>Unknown result format</p>
          </div>
        );
    }
  };

  return (
    <div className="result-display">
      <div className="result-controls">
        <button 
          type="button"
          onClick={() => setShowRaw(!showRaw)}
          className={`toggle-view-btn ${showRaw ? 'active' : ''}`}
          title={showRaw ? 'Show formatted view' : 'Show raw JSON'}
        >
          {showRaw ? '📊 Formatted' : '{ } Raw JSON'}
        </button>
      </div>

      {showRaw ? (
        <div className="result-raw">
          <h4>{title} (Raw JSON)</h4>
          <pre className="raw-json">{JSON.stringify(result, null, 2)}</pre>
        </div>
      ) : (
        <div className="result-formatted">
          {renderFormattedResult()}
        </div>
      )}
    </div>
  );
};

export default ResultDisplay;