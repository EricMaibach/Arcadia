export interface FormattedResult {
  type: 'array' | 'object' | 'primitive' | 'empty' | 'error';
  data: any;
  isEmpty: boolean;
  error?: string;
  statusInfo?: {
    status: string;
    success: boolean;
  };
}

export function analyzeResult(result: any): FormattedResult {
  if (result === null || result === undefined) {
    return { type: 'empty', data: null, isEmpty: true };
  }

  // Check if this is the expected wrapper format
  if (typeof result === 'object' && result.hasOwnProperty('output') && result.hasOwnProperty('status')) {
    const status = result.status;
    let output = result.output;
    
    // The output field might be a JSON string that needs parsing
    if (typeof output === 'string') {
      try {
        output = JSON.parse(output);
      } catch (e) {
        // If parsing fails, treat the string as an error message
        return {
          type: 'error',
          data: output,
          isEmpty: false,
          error: 'Failed to parse output JSON',
          statusInfo: {
            status: status,
            success: false
          }
        };
      }
    }
    
    // Check if the operation was successful
    if (status === 'success' && output && typeof output === 'object' && output.success === true) {
      // Extract ONLY the data field - this is what we want to display
      const actualData = output.data;
      return analyzeActualData(actualData);
    } else {
      // Handle error case
      let errorMessage = 'Operation failed';
      
      if (output && output.error) {
        errorMessage = output.error;
      } else if (output && typeof output === 'string') {
        errorMessage = output;
      } else if (typeof result.error === 'string') {
        errorMessage = result.error;
      }
      
      return {
        type: 'error',
        data: errorMessage,
        isEmpty: false,
        error: errorMessage,
        statusInfo: {
          status: status,
          success: output ? output.success || false : false
        }
      };
    }
  }

  // Fallback: analyze the result directly if it doesn't match the wrapper format
  return analyzeActualData(result);
}

export function analyzeActualData(data: any): FormattedResult {
  if (data === null || data === undefined) {
    return { type: 'empty', data: null, isEmpty: true };
  }

  if (Array.isArray(data)) {
    return { 
      type: 'array', 
      data: data, 
      isEmpty: data.length === 0 
    };
  }

  if (typeof data === 'object') {
    const keys = Object.keys(data);
    return { 
      type: 'object', 
      data: data, 
      isEmpty: keys.length === 0 
    };
  }

  return { 
    type: 'primitive', 
    data: data, 
    isEmpty: false 
  };
}

export function formatFieldName(fieldName: string): string {
  return fieldName
    .replace(/_/g, ' ')
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .split(' ')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

export function formatValue(value: any): string {
  if (value === null || value === undefined) {
    return 'N/A';
  }

  if (typeof value === 'boolean') {
    return value ? 'Yes' : 'No';
  }

  if (typeof value === 'string') {
    // Check if it looks like a date
    if (value.match(/^\d{4}-\d{2}-\d{2}/) || value.match(/^\d{2}\/\d{2}\/\d{4}/)) {
      try {
        const date = new Date(value);
        if (!isNaN(date.getTime())) {
          return date.toLocaleDateString();
        }
      } catch (e) {
        // Fall through to return original string
      }
    }
    return value;
  }

  if (typeof value === 'number') {
    // Format numbers with appropriate precision
    if (Number.isInteger(value)) {
      return value.toString();
    }
    return value.toFixed(2);
  }

  if (Array.isArray(value)) {
    return `[${value.length} items]`;
  }

  if (typeof value === 'object') {
    return `{${Object.keys(value).length} fields}`;
  }

  return String(value);
}

export function getTableColumns(data: any[]): string[] {
  if (data.length === 0) {
    return [];
  }

  // Get all unique keys from all objects
  const allKeys = new Set<string>();
  
  for (const item of data) {
    if (typeof item === 'object' && item !== null) {
      Object.keys(item).forEach(key => allKeys.add(key));
    }
  }

  return Array.from(allKeys).sort();
}

export function isComplexValue(value: any): boolean {
  return Array.isArray(value) || (typeof value === 'object' && value !== null);
}