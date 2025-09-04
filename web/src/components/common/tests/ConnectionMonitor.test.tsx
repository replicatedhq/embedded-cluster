import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import ConnectionMonitor from '../ConnectionMonitor';

const server = setupServer(
  http.get('*/api/health', () => {
    return new HttpResponse(JSON.stringify({ status: 'ok' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  })
);

describe('ConnectionMonitor', () => {
  beforeEach(() => {
    server.listen({ onUnhandledRequest: 'warn' });
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  it('should not show modal when API is available', async () => {
    render(<ConnectionMonitor />);

    // Modal should not appear when connected
    await new Promise(resolve => setTimeout(resolve, 100));
    expect(screen.queryByText('Cannot connect')).not.toBeInTheDocument();
  });

  it('should show modal when health check fails', async () => {
    server.use(
      http.get('*/api/health', () => {
        return HttpResponse.error();
      })
    );

    render(<ConnectionMonitor />);

    await waitFor(() => {
      expect(screen.getByText('Cannot connect')).toBeInTheDocument();
    }, { timeout: 4000 });
  }, 6000);

  it('should handle automatic retry', async () => {
    let retryCount = 0;
    
    server.use(
      http.get('*/api/health', () => {
        retryCount++;
        
        // Fail first time, succeed on second automatic retry
        if (retryCount === 1) {
          return HttpResponse.error();
        }
        
        return new HttpResponse(JSON.stringify({ status: 'ok' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      })
    );

    render(<ConnectionMonitor />);

    // Wait for modal to appear after first health check fails
    await waitFor(() => {
      expect(screen.getByText('Cannot connect')).toBeInTheDocument();
    }, { timeout: 6000 });

    // Should show countdown - wait for it to appear since it's async
    await waitFor(() => {
      expect(screen.getByText(/Retrying in \d+ second/)).toBeInTheDocument();
    }, { timeout: 2000 });

    // Modal should disappear when automatic retry succeeds
    // This should happen after the RETRY_INTERVAL (10 seconds)
    await waitFor(() => {
      expect(screen.queryByText('Cannot connect')).not.toBeInTheDocument();
    }, { timeout: 12000 });
  }, 15000);

  it('should show retry countdown timer', async () => {
    server.use(
      http.get('*/api/health', () => {
        return HttpResponse.error();
      })
    );

    render(<ConnectionMonitor />);

    // Wait for modal to appear
    await waitFor(() => {
      expect(screen.getByText('Cannot connect')).toBeInTheDocument();
    }, { timeout: 4000 });

    await waitFor(() => {
      expect(screen.getByText(/Retrying in \d+ second/)).toBeInTheDocument();
    }, { timeout: 2000 });

    // Verify the countdown is actually working by checking it's a reasonable value
    const countdownElement = screen.getByText(/Retrying in \d+ second/);
    const countdownText = countdownElement.textContent || '';
    const secondsMatch = countdownText.match(/Retrying in (\d+) second/);
    
    if (secondsMatch) {
      const seconds = parseInt(secondsMatch[1], 10);
      // Should be between 1-10 seconds (since RETRY_INTERVAL is 10 seconds)
      expect(seconds).toBeGreaterThan(0);
      expect(seconds).toBeLessThanOrEqual(10);
    }
  }, 8000);
});
