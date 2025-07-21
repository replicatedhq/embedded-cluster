import React, { useState, useRef, useCallback, useMemo } from 'react';
import Button from '../Button';
import { Upload } from 'lucide-react';
import File from './File';
import HelpText from '../HelpText';

interface FileInputProps {
  id: string;
  label: string;
  value?: string;           // Base64 encoded file content
  filename?: string;        // Original filename
  defaultValue?: string;    // Default base64 content
  defaultFilename: string; // Default filename to be used if no file is selected
  onChange: (value: string, filename: string) => void;
  disabled?: boolean;
  error?: string;
  helpText?: string;
  accept?: string;         // File type restrictions
  required?: boolean;
  className?: string;
  labelClassName?: string;
  dataTestId?: string;
}

interface FileInputState {
  isDragOver: boolean;
  isProcessing: boolean;
  internalError: string | null;
}

const FileInput: React.FC<FileInputProps> = ({
  id,
  label,
  value,
  defaultValue,
  filename,
  defaultFilename,
  onChange,
  disabled = false,
  error,
  helpText,
  accept,
  required = false,
  className = '',
  labelClassName = '',
  dataTestId,
}) => {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [state, setState] = useState<FileInputState>({
    isDragOver: false,
    isProcessing: false,
    internalError: null,
  });

  const hasFile = useMemo(() => value && filename || defaultValue, [value, filename, defaultValue]);
  const downloadValue = useMemo(() => value || defaultValue, [value, defaultValue]);
  const downloadFilename = useMemo(() => filename || defaultFilename, [filename, defaultFilename]);
  const valueIsDefault = useMemo(() => !value && defaultValue, [value, defaultValue]);
  const displayError = useMemo(() => error || state.internalError, [error, state.internalError]);

  const encodeFileToBase64 = (file: File): Promise<string> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();

      reader.onload = () => {
        if (typeof reader.result === 'string') {
          // readAsDataURL returns "data:mime/type;base64,<data>"
          // We need to extract just the base64 part
          const base64 = reader.result.split(',')[1];
          resolve(base64);
        } else {
          reject(new Error('Unexpected result type when reading file'));
        }
      };

      reader.onerror = (e: ProgressEvent<FileReader>) => {
        reject(new Error(e.target?.error?.message || 'Failed to read file'));
      };

      reader.readAsDataURL(file);
    });
  };

  const handleFileSelect = useCallback(async (file: File) => {
    if (disabled) return;

    setState(prev => ({ ...prev, internalError: null, isProcessing: true }));

    try {
      const base64Content = await encodeFileToBase64(file);
      onChange(base64Content, file.name);
      setState(prev => ({ ...prev, isProcessing: false }));
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to process file';
      setState(prev => ({
        ...prev,
        internalError: errorMessage,
        isProcessing: false
      }));
    }
  }, [disabled, onChange]);

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleFileSelect(file);
    }
  }, [handleFileSelect]);

  const handleClick = useCallback(() => {
    if (!disabled && fileInputRef.current) {
      fileInputRef.current.click();
    }
  }, [disabled]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (!disabled) {
      setState(prev => ({ ...prev, isDragOver: true }));
    }
  }, [disabled]);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setState(prev => ({ ...prev, isDragOver: false }));
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setState(prev => ({ ...prev, isDragOver: false }));

    if (disabled) return;

    const files = e.dataTransfer.files;
    if (files.length > 0) {
      handleFileSelect(files[0]);
    }
  }, [disabled, handleFileSelect]);

  const handleRemove = useCallback((e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent triggering the file dialog
    if (disabled) return;

    onChange('', '');
    setState(prev => ({ ...prev, internalError: null }));

    // Reset the file input
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  }, [disabled, onChange]);

  const handleDownload = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (disabled || !downloadValue || !downloadFilename) return;

    try {
      // Convert base64 to blob for download
      const content = atob(downloadValue);
      const bytes = new Uint8Array(content.length);
      for (let i = 0; i < content.length; i++) {
        bytes[i] = content.charCodeAt(i);
      }
      const blob = new Blob([bytes]);

      // Create URL and trigger download
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = downloadFilename;
      link.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Failed to download file:', error);
    }
  }, [disabled, downloadValue, downloadFilename]);

  return (
    <div className={`mb-4 ${className}`}>
      <label htmlFor={id} className={`block text-sm font-medium text-gray-700 mb-1 ${labelClassName}`}>
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>

      <div
        className="mt-1"
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        {/* Hidden file input */}
        <input
          ref={fileInputRef}
          id={id}
          type="file"
          accept={accept}
          disabled={disabled}
          onChange={handleInputChange}
          className="hidden"
          data-testid={dataTestId || id}
        />

        <div className="flex items-center">
          <Button
            variant="outline"
            onClick={handleClick}
            disabled={disabled || state.isProcessing}
            icon={state.isProcessing ? (
              <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-gray-900"></div>
            ) : (
              <Upload className="w-4 h-4" />
            )}
            className={`w-80 transition-all duration-200 ${state.isDragOver ? 'border-2 shadow-md' : ''}`}
            dataTestId={`${dataTestId || id}-button`}
          >
            {state.isProcessing ? 'Processing...' : 'Upload File'}
          </Button>

          {hasFile && <File
            id={id}
            dataTestId={dataTestId}
            handleDownload={handleDownload}
            handleRemove={handleRemove}
            filename={downloadFilename}
            allowRemove={!valueIsDefault} // Only allow remove if not using default value
            disabled={disabled} />
          }
        </div>
      </div>

      {displayError && <p id={`${id}-error`} className="mt-1 text-sm text-red-500">{displayError}</p>}
      {/* Don't show the default value for file in the help text, instead just highlight there is something */}
      <HelpText helpText={helpText} defaultValue='File provided ' error={displayError || undefined} />
    </div>
  );
};

export default FileInput;
