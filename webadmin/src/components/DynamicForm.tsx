import React, { useState, useEffect } from 'react';
import { FieldSchema, parseInputFormat, buildJsonFromFormData } from '../utils/schemaParser';

interface DynamicFormProps {
  inputFormat: string;
  onSubmit: (jsonData: object) => void;
  isSubmitting?: boolean;
  toolName: string;
}

const DynamicForm: React.FC<DynamicFormProps> = ({ 
  inputFormat, 
  onSubmit, 
  isSubmitting = false, 
  toolName 
}) => {
  const [schema, setSchema] = useState<{ fields: FieldSchema[], isEmpty: boolean }>({ fields: [], isEmpty: true });
  const [formData, setFormData] = useState<Record<string, any>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showRawEditor, setShowRawEditor] = useState(false);
  const [rawJson, setRawJson] = useState('{}');

  useEffect(() => {
    const parsedSchema = parseInputFormat(inputFormat);
    setSchema(parsedSchema);
    
    // Initialize form data with default values
    const initialData: Record<string, any> = {};
    parsedSchema.fields.forEach(field => {
      switch (field.type) {
        case 'string':
          initialData[field.name] = '';
          break;
        case 'number':
          initialData[field.name] = field.required ? 0 : '';
          break;
        case 'boolean':
          initialData[field.name] = false;
          break;
      }
    });
    setFormData(initialData);
    setErrors({});
  }, [inputFormat]);

  const handleFieldChange = (fieldName: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      [fieldName]: value
    }));
    
    // Clear error for this field
    if (errors[fieldName]) {
      setErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[fieldName];
        return newErrors;
      });
    }
  };

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};
    
    schema.fields.forEach(field => {
      const value = formData[field.name];
      
      const fieldDisplayName = field.label || field.name;
      
      // Check required fields
      if (field.required) {
        if (value === '' || value === null || value === undefined) {
          newErrors[field.name] = `${fieldDisplayName} is required`;
        } else if (field.type === 'number' && isNaN(Number(value))) {
          newErrors[field.name] = `${fieldDisplayName} must be a valid number`;
        }
      }
      
      // Validate number fields
      if (field.type === 'number' && value !== '' && value !== null && value !== undefined) {
        if (isNaN(Number(value))) {
          newErrors[field.name] = `${fieldDisplayName} must be a valid number`;
        }
      }
    });
    
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    
    if (showRawEditor) {
      try {
        const jsonData = JSON.parse(rawJson);
        onSubmit(jsonData);
      } catch (err) {
        setErrors({ json: 'Invalid JSON format' });
      }
      return;
    }

    if (!validateForm()) {
      return;
    }

    const jsonData = buildJsonFromFormData(schema.fields, formData);
    onSubmit(jsonData);
  };

  const clearForm = () => {
    const clearedData: Record<string, any> = {};
    schema.fields.forEach(field => {
      switch (field.type) {
        case 'string':
          clearedData[field.name] = '';
          break;
        case 'number':
          clearedData[field.name] = field.required ? 0 : '';
          break;
        case 'boolean':
          clearedData[field.name] = false;
          break;
      }
    });
    setFormData(clearedData);
    setErrors({});
  };

  const loadSampleData = () => {
    const sampleData: Record<string, any> = {};
    schema.fields.forEach(field => {
      switch (field.type) {
        case 'string':
          // Generate sample strings based on field name
          if (field.name.includes('date')) {
            sampleData[field.name] = new Date().toISOString().split('T')[0];
          } else if (field.name.includes('email')) {
            sampleData[field.name] = 'user@example.com';
          } else if (field.name.includes('name')) {
            sampleData[field.name] = 'Sample Name';
          } else if (field.name.includes('id')) {
            sampleData[field.name] = 'sample-id-123';
          } else {
            sampleData[field.name] = `sample ${field.name}`;
          }
          break;
        case 'number':
          // Generate sample numbers based on field name
          if (field.name.includes('calorie')) {
            sampleData[field.name] = 250;
          } else if (field.name.includes('protein') || field.name.includes('carb') || field.name.includes('fat')) {
            sampleData[field.name] = 15;
          } else if (field.name.includes('quantity') || field.name.includes('amount')) {
            sampleData[field.name] = 1;
          } else {
            sampleData[field.name] = 42;
          }
          break;
        case 'boolean':
          sampleData[field.name] = true;
          break;
      }
    });
    setFormData(sampleData);
  };

  if (schema.isEmpty) {
    return (
      <div className="dynamic-form">
        <div className="empty-form">
          <p>This tool doesn't require any input parameters.</p>
          <button 
            type="button" 
            onClick={() => onSubmit({})}
            disabled={isSubmitting}
            className={`run-btn ${isSubmitting ? 'running' : ''}`}
          >
            {isSubmitting ? 'Running...' : `Run ${toolName}`}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="dynamic-form">
      <form onSubmit={handleSubmit} className="tool-form">
        <div className="form-header">
          <h4>Tool Parameters</h4>
          <div className="form-actions-header">
            <button 
              type="button" 
              onClick={() => setShowRawEditor(!showRawEditor)} 
              className={showRawEditor ? "format-btn" : "sample-btn"}
            >
              {showRawEditor ? "Use Form" : "Raw JSON"}
            </button>
            {!showRawEditor && (
              <>
                <button type="button" onClick={loadSampleData} className="sample-btn">
                  Load Sample Data
                </button>
                <button type="button" onClick={clearForm} className="clear-btn">
                  Clear Form
                </button>
              </>
            )}
          </div>
        </div>

        {showRawEditor ? (
          <div className="raw-json-editor">
            <label htmlFor="raw-json">JSON Input:</label>
            <textarea
              id="raw-json"
              value={rawJson}
              onChange={(e) => setRawJson(e.target.value)}
              className={`input-textarea ${errors.json ? 'error' : ''}`}
              rows={10}
              placeholder="Enter JSON input for the tool"
            />
            {errors.json && (
              <div className="field-error">{errors.json}</div>
            )}
          </div>
        ) : (
          <div className="form-fields">
            {schema.fields.map((field) => (
            <div key={field.name} className="form-field">
              <label htmlFor={field.name}>
                {field.label || field.name}
                {field.required && <span className="required">*</span>}
              </label>
              
              {field.type === 'string' && (
                <input
                  type={field.name.includes('date') ? 'date' : 
                        field.name.includes('email') ? 'email' : 'text'}
                  id={field.name}
                  value={formData[field.name] || ''}
                  onChange={(e) => handleFieldChange(field.name, e.target.value)}
                  className={errors[field.name] ? 'error' : ''}
                  placeholder={`Enter ${field.label?.toLowerCase() || field.name}`}
                />
              )}

              {field.type === 'number' && (
                <input
                  type="number"
                  id={field.name}
                  value={formData[field.name] || ''}
                  onChange={(e) => handleFieldChange(field.name, e.target.value)}
                  className={errors[field.name] ? 'error' : ''}
                  placeholder={`Enter ${field.label?.toLowerCase() || field.name}`}
                  step={field.name.includes('calorie') || field.name.includes('protein') || 
                        field.name.includes('carb') || field.name.includes('fat') ? '0.1' : '1'}
                />
              )}

              {field.type === 'boolean' && (
                <div className="checkbox-field">
                  <input
                    type="checkbox"
                    id={field.name}
                    checked={formData[field.name] || false}
                    onChange={(e) => handleFieldChange(field.name, e.target.checked)}
                  />
                  <label htmlFor={field.name} className="checkbox-label">
                    {field.label || field.name}
                  </label>
                </div>
              )}

              {errors[field.name] && (
                <div className="field-error">{errors[field.name]}</div>
              )}
            </div>
          ))}
          </div>
        )}

        <div className="form-footer">
          <button 
            type="submit" 
            disabled={isSubmitting}
            className={`run-btn ${isSubmitting ? 'running' : ''}`}
          >
            {isSubmitting ? 'Running...' : `Run ${toolName}`}
          </button>
        </div>
      </form>
    </div>
  );
};

export default DynamicForm;