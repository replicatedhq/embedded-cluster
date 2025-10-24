import { useCallback, useRef } from 'react';

export interface DebouncedFetchOptions {
  debounceMs?: number;
}

export interface DebouncedFetchHook {
  debouncedFetch: (url: string, options?: RequestInit) => Promise<Response | null>;
  cleanup: () => void;
}

/**
 * Custom hook that provides debounced fetch functionality with automatic request cancellation.
 * 
 * Features:
 * - Debounces multiple rapid calls to prevent excessive API requests
 * - Automatically cancels previous requests when a new one is initiated
 * - Returns null for cancelled requests (no need to handle AbortErrors)
 * - Proper cleanup of timeouts and abort controllers
 * 
 * @param options Configuration options
 * @returns Object with debouncedFetch function and cleanup function
 * 
 * @example
 * ```typescript
 * const { debouncedFetch, cleanup } = useDebouncedFetch({ debounceMs: 100 });
 * 
 * const response = await debouncedFetch('/api/data', {
 *   method: 'POST',
 *   body: JSON.stringify(data)
 * });
 * 
 * if (!response) {
 *   // Request was cancelled - handle gracefully
 *   return;
 * }
 * 
 * // Handle the response (check response.ok for success/error)
 * if (response.ok) {
 *   const result = await response.json();
 * } else {
 *   // Handle error response
 *   console.error('Request failed:', response.status);
 * }
 * ```
 */
export function useDebouncedFetch(options: DebouncedFetchOptions = {}): DebouncedFetchHook {
  const { debounceMs = 100 } = options;
  
  const abortControllerRef = useRef<AbortController | null>(null);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  const debouncedFetch = useCallback((url: string, options: RequestInit = {}): Promise<Response | null> => {
    return new Promise<Response | null>((resolve, reject) => {
      // Abort any existing request
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      
      // Clear any existing timeout
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
      
      // Create new abort controller for this request
      const newController = new AbortController();
      abortControllerRef.current = newController;
      
      // Set up debounced execution
      timeoutRef.current = setTimeout(async () => {
        // Check if request was cancelled before execution
        if (newController.signal.aborted) {
          resolve(null);
          return;
        }
        
        try {
          // Automatically add the abort signal to the fetch options
          const response = await fetch(url, {
            ...options,
            signal: newController.signal,
          });
          resolve(response);
        } catch (error) {
          // AbortErrors resolve to null (request was cancelled)
          if (error instanceof Error && error.name === 'AbortError') {
            resolve(null);
            return;
          }
          reject(error);
        }
      }, debounceMs);
    });
  }, [debounceMs]);

  const cleanup = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
  }, []);

  return { debouncedFetch, cleanup };
}
