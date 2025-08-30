import React, { useState } from 'react';
import { formatFieldName, formatValue, isComplexValue } from '../utils/resultFormatter';
import NestedResultDisplay from './NestedResultDisplay';

interface ResultObjectProps {
  data: Record<string, any>;
  title?: string;
}

const ResultObject: React.FC<ResultObjectProps> = ({ data, title }) => {
  const [expandedFields, setExpandedFields] = useState<Set<string>>(new Set());

  if (!data || typeof data !== 'object' || Object.keys(data).length === 0) {
    return (
      <div className="result-object empty">
        <h4>{title || 'Result'}</h4>
        <p>No data to display</p>
      </div>
    );
  }

  const toggleFieldExpansion = (fieldName: string) => {
    const newExpanded = new Set(expandedFields);
    if (newExpanded.has(fieldName)) {
      newExpanded.delete(fieldName);
    } else {
      newExpanded.add(fieldName);
    }
    setExpandedFields(newExpanded);
  };

  const fields = Object.entries(data).sort(([a], [b]) => a.localeCompare(b));

  return (
    <div className="result-object">
      <div className="object-header">
        <h4>{title || 'Result'}</h4>
        <span className="field-count">{fields.length} field{fields.length !== 1 ? 's' : ''}</span>
      </div>

      <div className="object-content">
        {fields.map(([fieldName, value]) => {
          const hasComplexValue = isComplexValue(value);
          const isExpanded = expandedFields.has(fieldName);

          return (
            <div key={fieldName} className={`field-row ${hasComplexValue ? 'complex' : 'simple'}`}>
              <div className="field-header">
                <div className="field-name">
                  <strong>{formatFieldName(fieldName)}</strong>
                  {hasComplexValue && (
                    <span className="field-type">
                      {Array.isArray(value) ? `Array (${value.length} items)` : 'Object'}
                    </span>
                  )}
                </div>
                <div className="field-actions">
                  {hasComplexValue && (
                    <button
                      type="button"
                      onClick={() => toggleFieldExpansion(fieldName)}
                      className="expand-btn"
                      title={isExpanded ? 'Collapse' : 'Expand'}
                    >
                      {isExpanded ? '−' : '+'}
                    </button>
                  )}
                </div>
              </div>

              <div className="field-value">
                {hasComplexValue ? (
                  <div className="complex-value">
                    {isExpanded ? (
                      <NestedResultDisplay 
                        data={value} 
                        fieldName={fieldName}
                        level={1}
                        isExpanded={true}
                      />
                    ) : (
                      <div className="value-summary">
                        {formatValue(value)}
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="simple-value">
                    {formatValue(value)}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default ResultObject;