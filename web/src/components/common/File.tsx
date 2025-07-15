import React, { useRef } from 'react';
import { Upload, CheckCircle, FileText, X, Download } from 'lucide-react';
import Button from './Button';
import HelpText from './HelpText';

interface FileProps {
  id: string;
  label: string;
  helpText?: string;
  error?: string;
  required?: boolean;
  accept?: string;
  fileName?: string;
  value?: string;
  onFileChange: (content: string, filename: string) => void;
  onFileRemove?: () => void;
  disabled?: boolean;
  className?: string;
  labelClassName?: string;
  dataTestId?: string;
  buttonText?: string;
  buttonClassName?: string;
  successMessage?: string;
  placeholder?: string;
  showDownload?: boolean;
  defaultValue?: string;
}

const File: React.FC<FileProps> = ({
  id,
  label,
  helpText,
  error,
  required = false,
  accept,
  fileName,
  value,
  onFileChange,
  onFileRemove,
  disabled = false,
  className = '',
  labelClassName = '',
  dataTestId,
  buttonText = 'Upload File',
  buttonClassName = 'w-80',
  successMessage,
  placeholder,
  showDownload = true,
  defaultValue,
}) => {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileSelect = () => {
    if (disabled) return;
    fileInputRef.current?.click();
  };

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    try {
      const content = await fileToBase64(file);
      onFileChange(content, file.name);
    } catch (error) {
      console.error('Error converting file to base64:', error);
    }
  };

  const fileToBase64 = (file: File): Promise<string> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        if (typeof reader.result === 'string') {
          const base64Content = reader.result.split(',')[1];
          resolve(base64Content);
        } else {
          reject(new Error('Failed to read file as base64'));
        }
      };
      reader.onerror = reject;
      reader.readAsDataURL(file);
    });
  };

  const handleFileRemove = () => {
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
    onFileChange('', '');
    onFileRemove?.();
  };

  const handleDownload = () => {
    if (!value || !fileName) return;

    try {
      const binaryData = atob(value);
      const bytes = new Uint8Array(binaryData.length);
      for (let i = 0; i < binaryData.length; i++) {
        bytes[i] = binaryData.charCodeAt(i);
      }

      const blob = new Blob([bytes]);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = fileName;
      a.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Error downloading file:', error);
    }
  };

  const getHelpText = () => {
    if (fileName) {
      return successMessage || 'File uploaded successfully';
    }
    
    const baseHelpText = helpText || placeholder || '';
    const defaultText = defaultValue ? '(Default: File provided)' : '';
    return [baseHelpText, defaultText].filter(Boolean).join(' ');
  };

  return (
    <div className={`space-y-1 ${className}`}>
      <label htmlFor={id} className={`block text-sm font-medium text-gray-700 ${labelClassName}`}>
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>
      <div className="mt-1 flex items-center">
        <input
          id={id}
          type="file"
          ref={fileInputRef}
          onChange={handleFileChange}
          accept={accept}
          className="hidden"
          disabled={disabled}
          data-testid={dataTestId}
        />
        <Button
          variant="outline"
          onClick={handleFileSelect}
          icon={<Upload className="w-4 h-4" />}
          className={buttonClassName}
          disabled={disabled}
        >
          {buttonText}
        </Button>
        {fileName && (
          <div className="flex items-center space-x-2 px-3 py-2 bg-green-50 border border-green-200 rounded-md group ml-3">
            <CheckCircle className="w-4 h-4 text-green-500" />
            <FileText className="w-4 h-4 text-green-600" />
            <span className="text-sm text-green-700 font-medium">{fileName}</span>
            {showDownload && value && (
              <button
                onClick={handleDownload}
                className="ml-2 p-1 rounded-full hover:bg-green-100 transition-colors opacity-0 group-hover:opacity-100"
                title="Download file"
                disabled={disabled}
              >
                <Download className="w-3 h-3 text-green-600 hover:text-green-800" />
              </button>
            )}
            <button
              onClick={handleFileRemove}
              className="ml-2 p-1 rounded-full hover:bg-green-100 transition-colors opacity-0 group-hover:opacity-100"
              title="Remove file"
              disabled={disabled}
            >
              <X className="w-3 h-3 text-green-600 hover:text-green-800" />
            </button>
          </div>
        )}
      </div>
      {error && <p className="text-sm text-red-500">{error}</p>}
      <HelpText helpText={getHelpText()} error={error} />
    </div>
  );
};

export default File;