import React from 'react';
import { FileText, CheckCircle, X } from 'lucide-react';

interface FileProps {
  id: string;
  dataTestId?: string;
  handleDownload: (e: React.MouseEvent) => void;
  handleRemove: (e: React.MouseEvent) => void;
  allowRemove?: boolean;
  filename: string;
  disabled?: boolean;
}

const File: React.FC<FileProps> = ({ id, dataTestId, handleDownload, handleRemove, filename, allowRemove, disabled }) => {
  return (
    <div className="flex items-center space-x-2 px-3 py-2 bg-green-50 border border-green-200 rounded-md group ml-3">
      <CheckCircle className="w-4 h-4 text-green-500" />
      <FileText className="w-4 h-4 text-green-600" />
      <span
        onClick={disabled ? undefined : handleDownload}
        className="text-sm text-green-700 font-medium hover:underline cursor-pointer"
        title="Download file"
        data-testid={`${dataTestId || id}-filename`}
      >
        {filename}
      </span>
      {allowRemove &&
        <button
          onClick={handleRemove}
          disabled={disabled}
          className="ml-2 p-1 rounded-full hover:bg-green-100 transition-colors opacity-0 group-hover:opacity-100"
          title="Remove file"
          data-testid={`${dataTestId || id}-remove`}
        >
          <X className="w-3 h-3 text-green-600 hover:text-green-800" />
        </button>
      }
    </div>
  )
}

export default File;
