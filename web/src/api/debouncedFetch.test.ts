import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDebouncedFetch } from './debouncedFetch';

describe('useDebouncedFetch', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('should debounce multiple calls', async () => {
    const mockResponse = new Response('{}', { status: 200 });
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(mockResponse);
    
    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    // Make multiple rapid calls
    act(() => {
      result.current.debouncedFetch('/api/test1');
      result.current.debouncedFetch('/api/test2');
      result.current.debouncedFetch('/api/test3');
    });

    // Fast-forward time by less than debounce delay
    act(() => {
      vi.advanceTimersByTime(50);
    });

    expect(fetchSpy).not.toHaveBeenCalled();

    // Fast-forward time to complete the debounce
    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Should only be called once due to debouncing (with the last URL)
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy).toHaveBeenCalledWith('/api/test3', expect.objectContaining({
      signal: expect.any(AbortSignal)
    }));
  });

  it('should pass through fetch options with abort signal', async () => {
    const mockResponse = new Response('{}', { status: 200 });
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(mockResponse);
    
    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    const fetchOptions = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ data: 'test' })
    };

    act(() => {
      result.current.debouncedFetch('/api/test', fetchOptions);
    });

    act(() => {
      vi.advanceTimersByTime(100);
    });

    expect(fetchSpy).toHaveBeenCalledWith('/api/test', expect.objectContaining({
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ data: 'test' }),
      signal: expect.any(AbortSignal)
    }));
  });

  it('should return null for AbortError', async () => {
    const abortError = new Error('AbortError');
    abortError.name = 'AbortError';
    vi.spyOn(global, 'fetch').mockRejectedValue(abortError);

    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    let response: Response | null | undefined;
    act(() => {
      result.current.debouncedFetch('/api/test').then((res) => {
        response = res;
      });
    });

    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Wait for promise to resolve
    await act(async () => {
      await Promise.resolve();
    });

    expect(response).toBe(null);
  });

  it('should propagate non-abort errors', async () => {
    const networkError = new Error('Network error');
    vi.spyOn(global, 'fetch').mockRejectedValue(networkError);

    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    let caughtError: Error | null = null;
    act(() => {
      result.current.debouncedFetch('/api/test').catch((err) => {
        caughtError = err;
      });
    });

    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Wait for promise to resolve
    await act(async () => {
      await Promise.resolve();
    });

    expect(caughtError).toBe(networkError);
  });

  it('should return successful responses', async () => {
    const mockResponse = new Response('{"success": true}', { status: 200 });
    vi.spyOn(global, 'fetch').mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    let response: Response | null | undefined;
    act(() => {
      result.current.debouncedFetch('/api/test').then((res) => {
        response = res;
      });
    });

    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Wait for promise to resolve
    await act(async () => {
      await Promise.resolve();
    });

    expect(response).toBe(mockResponse);
    expect(response?.status).toBe(200);
  });

  it('should cleanup properly', () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(new Response('{}'));
    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    act(() => {
      result.current.debouncedFetch('/api/test');
    });

    // Cleanup should clear the timeout
    act(() => {
      result.current.cleanup();
    });

    act(() => {
      vi.advanceTimersByTime(100);
    });

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('should abort previous requests when new ones are made', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(new Response('{}'));
    
    // Track abort calls with a simple spy
    const abortSpy = vi.fn();
    vi.spyOn(global, 'AbortController').mockImplementation(() => ({
      abort: abortSpy,
      signal: { aborted: false }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any));

    const { result } = renderHook(() => useDebouncedFetch({ debounceMs: 100 }));

    // First call
    act(() => {
      result.current.debouncedFetch('/api/test1');
    });

    // Second call should abort the first
    act(() => {
      result.current.debouncedFetch('/api/test2');
    });

    // Verify the first request was aborted
    expect(abortSpy).toHaveBeenCalledTimes(1);

    // Complete the debounce
    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Only the second call should execute
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy).toHaveBeenCalledWith('/api/test2', expect.any(Object));

    // Verify the second call was not aborted
    expect(abortSpy).toHaveBeenCalledTimes(1);
  });
});
