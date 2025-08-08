import React, { useEffect, useState, useCallback } from 'react';

const RETRY_INTERVAL = 10000; // 10 seconds

// Reusable spinner component
const Spinner: React.FC = () => (
  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
);

// Connection modal component
const ConnectionModal: React.FC<{ 
  nextRetryTime?: number;
}> = ({ nextRetryTime }) => {
  const [secondsUntilRetry, setSecondsUntilRetry] = useState(0);

  useEffect(() => {
    if (!nextRetryTime) return;
    
    const updateCountdown = () => {
      const now = Date.now();
      const remaining = Math.max(0, Math.floor((nextRetryTime - now) / 1000));
      setSecondsUntilRetry(remaining);
    };
    
    // Update immediately
    updateCountdown();
    
    // Update every second
    const interval = setInterval(updateCountdown, 1000);
    return () => clearInterval(interval);
  }, [nextRetryTime]);

  return (
    <div className="fixed inset-0 z-50 bg-black bg-opacity-50 flex items-center justify-center">
      <div className="bg-white rounded-lg p-6 max-w-md mx-4 shadow-xl">
        <div className="flex items-center justify-center mb-4">
          <div className="w-12 h-12 bg-red-100 rounded-full flex items-center justify-center mr-4">
            <svg className="w-6 h-6 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z" />
            </svg>
          </div>
        </div>
        
        <h2 className="text-xl font-bold text-gray-900 text-center mb-2">
          Cannot connect
        </h2>
        
        <p className="text-gray-700 text-center mb-6">
          We're unable to reach the server right now. Please check that the 
          installer is running and accessible.
        </p>
        
        <div className="flex items-center justify-center">
          <div className="flex items-center text-sm font-semibold text-gray-600">
            {secondsUntilRetry > 0 ? (
              <>
                <Spinner />
                Retrying in {secondsUntilRetry} second{secondsUntilRetry !== 1 ? 's' : ''}
              </>
            ) : (
              <>
                <Spinner />
                Retrying now...
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

// Custom hook for connection monitoring logic
const useConnectionMonitor = () => {
  const [isConnected, setIsConnected] = useState(true);
  const [nextRetryTime, setNextRetryTime] = useState<number | undefined>();
  const [checkInterval, setCheckInterval] = useState<NodeJS.Timeout | null>(null);

  const checkConnection = useCallback(async () => {
    try {
      const timeoutPromise = new Promise((_, reject) => 
        setTimeout(() => reject(new Error('Timeout')), 5000)
      );
      
      const fetchPromise = fetch('/api/health', {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
      });
      
      const response = await Promise.race([fetchPromise, timeoutPromise]) as Response;
      
      if (response.ok) {
        setIsConnected(true);
        setNextRetryTime(undefined);
      } else {
        throw new Error(`HTTP ${response.status}`);
      }
    } catch {
      // Connection failed - set up countdown for next retry
      setIsConnected(false);
      const retryTime = Date.now() + RETRY_INTERVAL;
      setNextRetryTime(retryTime);
    }
  }, []);

  useEffect(() => {
    // Initial check
    checkConnection();
    
    // Set up regular interval checks
    const interval = setInterval(checkConnection, RETRY_INTERVAL);
    setCheckInterval(interval);
    
    // Cleanup on unmount
    return () => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, []); // Empty dependency array to prevent infinite loops

  // Cleanup interval when it changes
  useEffect(() => {
    return () => {
      if (checkInterval) {
        clearInterval(checkInterval);
      }
    };
  }, [checkInterval]);

  return {
    isConnected,
    nextRetryTime,
  };
};

const ConnectionMonitor: React.FC = () => {
  const { isConnected, nextRetryTime } = useConnectionMonitor();

  return (
    <>
      {!isConnected && (
        <ConnectionModal 
          nextRetryTime={nextRetryTime}
        />
      )}
    </>
  );
};

export default ConnectionMonitor;
