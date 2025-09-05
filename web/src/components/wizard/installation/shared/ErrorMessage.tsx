import React, { useState } from 'react';
import { XCircle, ChevronDown, ChevronUp } from 'lucide-react';

interface ErrorMessageProps {
  error: string;
  maxLength?: number;
  expandedMaxLength?: number;
}

const ErrorMessage: React.FC<ErrorMessageProps> = ({ 
  error, 
  maxLength = 250,
  expandedMaxLength = 1000 
}) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const shouldTruncate = error.length > maxLength;
  const shouldTruncateExpanded = error.length > expandedMaxLength;
  
  let displayError: string;
  if (!shouldTruncate) {
    displayError = error;
  } else if (isExpanded) {
    displayError = shouldTruncateExpanded 
      ? error.substring(0, expandedMaxLength) + '...'
      : error;
  } else {
    displayError = error.substring(0, maxLength) + '...';
  }

  return (
    <div className="mt-6 p-4 bg-red-50 text-red-800 rounded-md" data-testid="error-message">
      <div className="flex">
        <div className="flex-shrink-0">
          <XCircle className="h-5 w-5 text-red-400" />
        </div>
        <div className="ml-3 flex-1">
          <h3 className="text-sm font-medium">Installation Error</h3>
          <div className="mt-2 text-sm">
            <p className="whitespace-pre-wrap break-words">{displayError}</p>
            {shouldTruncate && (
              <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="mt-2 flex items-center gap-1 text-xs font-medium text-red-600 hover:text-red-800 transition-colors"
                data-testid="error-toggle"
              >
                {isExpanded ? 'Show less' : 'Show more'}
                {isExpanded ? (
                  <ChevronUp className="w-3 h-3" />
                ) : (
                  <ChevronDown className="w-3 h-3" />
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default ErrorMessage;
