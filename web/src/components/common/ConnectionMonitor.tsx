import React, { useEffect, useState, useCallback } from 'react';

// Connection modal component
<<<<<<< HEAD
const ConnectionModal: React.FC<{ 
  onRetry: () => void; 
  isRetrying: boolean;
  nextRetryTime?: number;
  retryInterval: number;
}> = ({ onRetry, isRetrying, nextRetryTime, retryInterval }) => {
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
=======
const ConnectionModal: React.FC<{ onRetry: () => void; isRetrying: boolean }> = ({ onRetry, isRetrying }) => {
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setRetryCount(count => count + 1);
    }, 1000);
    return () => clearInterval(interval);
  }, []);
>>>>>>> main

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
<<<<<<< HEAD
            {isRetrying ? (
              <>
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
                Retrying now...
              </>
            ) : secondsUntilRetry > 0 ? (
              <>
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
                Trying again in {secondsUntilRetry} second{secondsUntilRetry !== 1 ? 's' : ''}
              </>
            ) : (
              <>
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
                Retrying...
              </>
            )}
=======
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
            Trying again in {Math.max(1, 10 - (retryCount % 10))} second{Math.max(1, 10 - (retryCount % 10)) !== 1 ? 's' : ''}
>>>>>>> main
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
<<<<<<< HEAD
  const [nextRetryTime, setNextRetryTime] = useState<number | undefined>();
  const [currentInterval, setCurrentInterval] = useState<NodeJS.Timeout | null>(null);
  
  const RETRY_INTERVAL = 10000; // 10 seconds

  const checkConnection = useCallback(async () => {
    setIsChecking(true);
    setNextRetryTime(undefined); // Clear countdown while checking
    
    let connectionFailed = false;
=======

  const checkConnection = useCallback(async () => {
    setIsChecking(true);
>>>>>>> main
    
    try {
      // Try up to 3 times before marking as disconnected
      let attempts = 0;
      const maxAttempts = 3;
      
      while (attempts < maxAttempts) {
        try {
          // Create a timeout promise
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
<<<<<<< HEAD
            setNextRetryTime(undefined);
=======
>>>>>>> main
            return;
          } else {
            throw new Error(`HTTP ${response.status}`);
          }
        } catch {
          attempts++;
          if (attempts < maxAttempts) {
            await new Promise(resolve => setTimeout(resolve, 1000));
          }
        }
      }
      
<<<<<<< HEAD
      // All attempts failed
      connectionFailed = true;
      setIsConnected(false);
      
    } catch {
      connectionFailed = true;
      setIsConnected(false);
    } finally {
      setIsChecking(false);
      
      // Synchronize countdown and retry timing
      if (connectionFailed) {
        // Clear existing interval to prevent unsynchronized retries
        if (currentInterval) {
          clearInterval(currentInterval);
          setCurrentInterval(null);
        }
        
        // Set countdown timer for exactly RETRY_INTERVAL milliseconds
        const now = Date.now();
        // Add small buffer to ensure countdown starts at full interval
        const retryTime = now + RETRY_INTERVAL + 100;
        setNextRetryTime(retryTime);
        
        // Set synchronized timeout that will retry exactly when countdown reaches 0
        const syncedTimeout = setTimeout(() => {
          checkConnection();
          // After this retry, resume regular interval
          const newInterval = setInterval(checkConnection, RETRY_INTERVAL);
          setCurrentInterval(newInterval);
        }, RETRY_INTERVAL + 100);
        
        setCurrentInterval(syncedTimeout);
      }
    }
  }, [RETRY_INTERVAL, currentInterval]);
=======
      // All attempts failed - show modal immediately
      setIsConnected(false);
      
    } catch {
      setIsConnected(false);
    } finally {
      setIsChecking(false);
    }
  }, []);
>>>>>>> main

  useEffect(() => {
    // Initial check
    checkConnection();
    
<<<<<<< HEAD
    // Set up initial periodic health checks only if no interval exists
    if (!currentInterval) {
      const interval = setInterval(checkConnection, RETRY_INTERVAL);
      setCurrentInterval(interval);
    }
    
    // Cleanup on unmount
    return () => {
      if (currentInterval) {
        clearInterval(currentInterval);
      }
    };
  }, []); // Empty dependency array to prevent infinite loops

  // Separate effect to handle interval reference updates
  useEffect(() => {
    return () => {
      if (currentInterval) {
        clearInterval(currentInterval);
      }
    };
  }, [currentInterval]);
=======
    // Set up periodic health checks every 5 seconds
    const interval = setInterval(checkConnection, 5000);
    
    return () => clearInterval(interval);
  }, [checkConnection]);
>>>>>>> main

  return (
    <>
      {!isConnected && (
        <ConnectionModal 
          onRetry={checkConnection}
          isRetrying={isChecking}
<<<<<<< HEAD
          nextRetryTime={nextRetryTime}
          retryInterval={RETRY_INTERVAL}
=======
>>>>>>> main
        />
      )}
    </>
  );
};

export default ConnectionMonitor;
