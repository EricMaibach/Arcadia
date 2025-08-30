export interface FieldSchema {
  name: string;
  type: 'string' | 'number' | 'boolean';
  required: boolean;
  label?: string;
}

export interface ParsedSchema {
  fields: FieldSchema[];
  isEmpty: boolean;
}

export function parseInputFormat(inputFormat: string): ParsedSchema {
  try {
    // Handle empty object case
    if (!inputFormat || inputFormat.trim() === '{}') {
      return { fields: [], isEmpty: true };
    }

    // Parse the JSON-like format
    const parsed = JSON.parse(inputFormat);
    
    if (typeof parsed !== 'object' || parsed === null) {
      return { fields: [], isEmpty: true };
    }

    const fields: FieldSchema[] = [];

    for (const [fieldName, typeSpec] of Object.entries(parsed)) {
      if (typeof typeSpec !== 'string') continue;

      // Check if field is optional (ends with ?)
      const isOptional = typeSpec.endsWith('?');
      const type = isOptional ? typeSpec.slice(0, -1) : typeSpec;

      // Validate and normalize type
      let normalizedType: 'string' | 'number' | 'boolean';
      switch (type.toLowerCase()) {
        case 'string':
          normalizedType = 'string';
          break;
        case 'number':
        case 'int':
        case 'integer':
        case 'float':
          normalizedType = 'number';
          break;
        case 'boolean':
        case 'bool':
          normalizedType = 'boolean';
          break;
        default:
          normalizedType = 'string'; // Default to string for unknown types
      }

      fields.push({
        name: fieldName,
        type: normalizedType,
        required: !isOptional,
        label: formatFieldLabel(fieldName)
      });
    }

    return { fields, isEmpty: fields.length === 0 };

  } catch (error) {
    console.warn('Failed to parse input format:', inputFormat, error);
    return { fields: [], isEmpty: true };
  }
}

function formatFieldLabel(fieldName: string): string {
  return fieldName
    .replace(/_/g, ' ')
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .split(' ')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

export function buildJsonFromFormData(fields: FieldSchema[], formData: Record<string, any>): object {
  const result: Record<string, any> = {};

  for (const field of fields) {
    const value = formData[field.name];

    // Skip empty optional fields
    if (!field.required && (value === '' || value === null || value === undefined)) {
      continue;
    }

    // Convert and validate the value based on type
    switch (field.type) {
      case 'string':
        result[field.name] = String(value || '');
        break;
      case 'number':
        const numValue = Number(value);
        if (!isNaN(numValue)) {
          result[field.name] = numValue;
        } else if (field.required) {
          result[field.name] = 0; // Default for required number fields
        }
        break;
      case 'boolean':
        result[field.name] = Boolean(value);
        break;
    }
  }

  return result;
}