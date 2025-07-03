import React, { useRef, useEffect } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';

interface LogViewerProps {
  title: string;
  logs: string[];
  isExpanded: boolean;
  onToggle: () => void;
}

const LogViewer: React.FC<LogViewerProps> = ({
  title,
  logs,
  isExpanded,
  onToggle
}) => {
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (logsEndRef.current && isExpanded) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, isExpanded]);

  return (
    <div className="mt-6" data-testid="log-viewer">
      <button
        onClick={onToggle}
        className="flex items-center gap-2 text-sm font-medium text-gray-900 hover:text-gray-700 transition-colors"
        data-testid="log-viewer-toggle"
      >
        <h3>{title}</h3>
        {isExpanded ? (
          <ChevronUp className="w-4 h-4" />
        ) : (
          <ChevronDown className="w-4 h-4" />
        )}
      </button>
      {isExpanded && (
        <div 
          className="bg-gray-900 text-gray-200 rounded-md p-4 h-48 overflow-y-auto font-mono text-xs mt-2"
          data-testid="log-viewer-content"
        >
          {logs.map((log, index) => (
            <div key={index} className="whitespace-pre-wrap pb-1">
              {log}
            </div>
          ))}
          <div ref={logsEndRef} />
        </div>
      )}
    </div>
  );
};

export default LogViewer;
