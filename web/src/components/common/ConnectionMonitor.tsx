import React, { useEffect, useState } from 'react';

// Connection modal component
const ConnectionModal: React.FC<{ onRetry: () => void; isRetrying: boolean }> = ({ onRetry, isRetrying }) => {
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setRetryCount(count => count + 1);
    }, 1000);
    return () => clearInterval(interval);
  }, []);

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
        
        <div className="flex items-center justify-between">
          <div className="flex items-center text-sm font-semibold text-gray-600">
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
            Trying again in {Math.max(1, 10 - (retryCount % 10))} second{Math.max(1, 10 - (retryCount % 10)) !== 1 ? 's' : ''}
          </div>
          <button 
            onClick={onRetry}
            disabled={isRetrying}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isRetrying ? 'Retrying...' : 'Try Now'}
          </button>
        </div>
      </div>
    </div>
  );
};

const ConnectionMonitor: React.FC = () => {
  const [isConnected, setIsConnected] = useState(true);
  const [isChecking, setIsChecking] = useState(false);

  const checkConnection = async () => {
    setIsChecking(true);
    
    try {
      // Try up to 3 times before marking as disconnected
      let attempts = 0;
      const maxAttempts = 3;
      
      while (attempts < maxAttempts) {
        try {
          // Create a timeout promise
          const timeoutPromise = new Promise((_, reject) => 
            setTimeout(() => reject(new Error('Timeout')), 1000)
          );
          
          const fetchPromise = fetch('/api/health', {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' },
          });
          
          const response = await Promise.race([fetchPromise, timeoutPromise]) as Response;
          
          if (response.ok) {
            setIsConnected(true);
            return;
          } else {
            throw new Error(`HTTP ${response.status}`);
          }
        } catch {
          attempts++;
          if (attempts < maxAttempts) {
            await new Promise(resolve => setTimeout(resolve, 500));
          }
        }
      }
      
      // All attempts failed
      setIsConnected(false);
    } catch {
      setIsConnected(false);
    } finally {
      setIsChecking(false);
    }
  };

  useEffect(() => {
    // Initial check
    checkConnection();
    
    // Set up periodic health checks every 10 seconds
    const interval = setInterval(checkConnection, 10000);
    
    return () => clearInterval(interval);
  }, []);

  return (
    <>
      {!isConnected && (
        <ConnectionModal 
          onRetry={checkConnection}
          isRetrying={isChecking}
        />
      )}
    </>
  );
};

export default ConnectionMonitor;
