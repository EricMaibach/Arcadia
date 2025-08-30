import React, { useState } from 'react';
import { analyzeActualData } from '../utils/resultFormatter';
import { formatFieldName, formatValue } from '../utils/resultFormatter';
import ResultTable from './ResultTable';
import ResultObject from './ResultObject';

interface NestedResultDisplayProps {
  data: any;
  fieldName?: string;
  level?: number;
  isExpanded?: boolean;
}

const NestedResultDisplay: React.FC<NestedResultDisplayProps> = ({ 
  data, 
  fieldName = '',
  level = 0,
  isExpanded = false
}) => {
  const [expanded, setExpanded] = useState(isExpanded);
  const maxLevel = 3; // Prevent infinite nesting
  
  if (level > maxLevel) {
    return <pre className="max-depth-reached">{JSON.stringify(data, null, 2)}</pre>;
  }

  const analyzed = analyzeActualData(data);

  const renderNestedContent = () => {
    switch (analyzed.type) {
      case 'array':
        if (analyzed.isEmpty) {
          return <div className="nested-empty">Empty array</div>;
        }
        return <ResultTable data={analyzed.data} title={fieldName ? formatFieldName(fieldName) : 'Items'} />;

      case 'object':
        if (analyzed.isEmpty) {
          return <div className="nested-empty">Empty object</div>;
        }
        return <ResultObject data={analyzed.data} title={fieldName ? formatFieldName(fieldName) : 'Object'} />;

      case 'primitive':
        return (
          <div className="nested-primitive">
            <span className="primitive-value">{formatValue(analyzed.data)}</span>
          </div>
        );

      case 'empty':
        return <div className="nested-empty">No data</div>;

      default:
        return <pre className="nested-fallback">{JSON.stringify(data, null, 2)}</pre>;
    }
  };

  // For primitive values, just return the formatted value
  if (analyzed.type === 'primitive') {
    return <span className="inline-primitive">{formatValue(data)}</span>;
  }

  // For complex structures, show collapsible content
  return (
    <div className={`nested-display level-${level}`}>
      <div className="nested-header" onClick={() => setExpanded(!expanded)}>
        <span className="expand-indicator">
          {expanded ? '▼' : '▶'}
        </span>
        <span className="nested-title">
          {fieldName ? formatFieldName(fieldName) : 
           analyzed.type === 'array' ? `Array (${analyzed.data.length} items)` : 
           'Object'}
        </span>
        {!expanded && (
          <span className="nested-preview">
            {analyzed.type === 'array' ? 
              `${analyzed.data.length} items` : 
              `${Object.keys(analyzed.data || {}).length} fields`}
          </span>
        )}
      </div>
      
      {expanded && (
        <div className="nested-content">
          {renderNestedContent()}
        </div>
      )}
    </div>
  );
};

export default NestedResultDisplay;