import React, { useState } from 'react';
import { formatFieldName, formatValue, getTableColumns, isComplexValue } from '../utils/resultFormatter';
import NestedResultDisplay from './NestedResultDisplay';

interface ResultTableProps {
  data: any[];
  title?: string;
}

const ResultTable: React.FC<ResultTableProps> = ({ data, title }) => {
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [sortColumn, setSortColumn] = useState<string | null>(null);
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('asc');

  if (!data || data.length === 0) {
    return (
      <div className="result-table empty">
        <h4>{title || 'Results'}</h4>
        <p>No data to display</p>
      </div>
    );
  }

  const columns = getTableColumns(data);

  const toggleRowExpansion = (index: number) => {
    const newExpanded = new Set(expandedRows);
    if (newExpanded.has(index)) {
      newExpanded.delete(index);
    } else {
      newExpanded.add(index);
    }
    setExpandedRows(newExpanded);
  };

  const handleSort = (column: string) => {
    if (sortColumn === column) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortDirection('asc');
    }
  };

  const sortedData = [...data].sort((a, b) => {
    if (!sortColumn) return 0;

    const aVal = a[sortColumn];
    const bVal = b[sortColumn];

    if (aVal === bVal) return 0;

    let comparison = 0;
    if (typeof aVal === 'string' && typeof bVal === 'string') {
      comparison = aVal.localeCompare(bVal);
    } else if (typeof aVal === 'number' && typeof bVal === 'number') {
      comparison = aVal - bVal;
    } else {
      comparison = String(aVal).localeCompare(String(bVal));
    }

    return sortDirection === 'asc' ? comparison : -comparison;
  });

  return (
    <div className="result-table">
      <div className="table-header">
        <h4>{title || 'Results'}</h4>
        <span className="row-count">{data.length} row{data.length !== 1 ? 's' : ''}</span>
      </div>

      <div className="table-container">
        <table>
          <thead>
            <tr>
              {columns.map((column) => (
                <th 
                  key={column} 
                  onClick={() => handleSort(column)}
                  className={`sortable ${sortColumn === column ? `sorted-${sortDirection}` : ''}`}
                >
                  {formatFieldName(column)}
                  <span className="sort-icon">
                    {sortColumn === column ? (sortDirection === 'asc' ? '↑' : '↓') : '↕'}
                  </span>
                </th>
              ))}
              <th className="actions-header">Actions</th>
            </tr>
          </thead>
          <tbody>
            {sortedData.map((row, index) => (
              <React.Fragment key={index}>
                <tr className={expandedRows.has(index) ? 'expanded' : ''}>
                  {columns.map((column) => {
                    const value = row[column];
                    const hasComplexValue = isComplexValue(value);

                    return (
                      <td key={column} className={hasComplexValue ? 'complex-value' : ''}>
                        {hasComplexValue ? (
                          <span className="complex-indicator">
                            {formatValue(value)}
                          </span>
                        ) : (
                          formatValue(value)
                        )}
                      </td>
                    );
                  })}
                  <td className="actions">
                    {/* Check if row has any complex values to show expand button */}
                    {columns.some(col => isComplexValue(row[col])) && (
                      <button
                        type="button"
                        onClick={() => toggleRowExpansion(index)}
                        className="expand-btn"
                        title={expandedRows.has(index) ? 'Collapse details' : 'Expand details'}
                      >
                        {expandedRows.has(index) ? '−' : '+'}
                      </button>
                    )}
                  </td>
                </tr>
                {expandedRows.has(index) && (
                  <tr className="expanded-details">
                    <td colSpan={columns.length + 1}>
                      <div className="expanded-content">
                        <h5>Detailed View:</h5>
                        <div className="expanded-fields">
                          {columns.map((column) => {
                            const value = row[column];
                            if (!isComplexValue(value)) return null;

                            return (
                              <div key={column} className="expanded-field">
                                <NestedResultDisplay 
                                  data={value}
                                  fieldName={column}
                                  level={1}
                                  isExpanded={true}
                                />
                              </div>
                            );
                          })}
                        </div>
                      </div>
                    </td>
                  </tr>
                )}
              </React.Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default ResultTable;