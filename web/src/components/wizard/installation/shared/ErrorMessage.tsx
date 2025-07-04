import React from 'react';
import { XCircle } from 'lucide-react';

interface ErrorMessageProps {
  error: string;
}

const ErrorMessage: React.FC<ErrorMessageProps> = ({ error }) => (
  <div className="mt-6 p-4 bg-red-50 text-red-800 rounded-md" data-testid="error-message">
    <div className="flex">
      <div className="flex-shrink-0">
        <XCircle className="h-5 w-5 text-red-400" />
      </div>
      <div className="ml-3">
        <h3 className="text-sm font-medium">Installation Error</h3>
        <div className="mt-2 text-sm">
          <p>{error}</p>
        </div>
      </div>
    </div>
  </div>
);

export default ErrorMessage;
