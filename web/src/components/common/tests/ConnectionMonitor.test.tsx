import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
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
    }, { timeout: 10000 });
  });

  it('should handle manual retry', async () => {
    let callCount = 0;
    server.use(
      http.get('*/api/health', () => {
        callCount++;
        if (callCount <= 6) {
          return HttpResponse.error();
        }
        return new HttpResponse(JSON.stringify({ status: 'ok' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      })
    );

    render(<ConnectionMonitor />);

    // Wait for modal to appear
    await waitFor(() => {
      expect(screen.getByText('Try Now')).toBeInTheDocument();
    }, { timeout: 10000 });

    // Click retry button
    fireEvent.click(screen.getByText('Try Now'));

    // Modal should disappear when connection is restored
    await waitFor(() => {
      expect(screen.queryByText('Cannot connect')).not.toBeInTheDocument();
    }, { timeout: 20000 });
  });

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
    }, { timeout: 10000 });

    // Should show countdown
    expect(screen.getByText(/Trying again in \d+ second/)).toBeInTheDocument();
  });
}); 