import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { ConnectionProvider, useConnection } from '../ConnectionContext';

const TestComponent = () => {
  const { isConnected } = useConnection();
  return (
    <div data-testid="connection-status">
      {isConnected ? 'Connected' : 'Disconnected'}
    </div>
  );
};

const server = setupServer(
  http.get('*/api/health', () => {
    return new HttpResponse(JSON.stringify({ status: 'ok' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  })
);

describe('ConnectionContext', () => {
  beforeEach(() => {
    server.listen({ onUnhandledRequest: 'warn' });
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  it('should initialize as connected when API is available', async () => {
    render(
      <ConnectionProvider>
        <TestComponent />
      </ConnectionProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('connection-status')).toHaveTextContent('Connected');
    }, { timeout: 10000 });
  });

  it('should show disconnected and modal when health check fails', async () => {
    server.use(
      http.get('*/api/health', () => {
        return HttpResponse.error();
      })
    );

    render(
      <ConnectionProvider>
        <TestComponent />
      </ConnectionProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('connection-status')).toHaveTextContent('Disconnected');
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

    render(
      <ConnectionProvider>
        <TestComponent />
      </ConnectionProvider>
    );

    await waitFor(() => {
      expect(screen.getByText('Try Now')).toBeInTheDocument();
    }, { timeout: 10000 });

    fireEvent.click(screen.getByText('Try Now'));

    await waitFor(() => {
      expect(screen.getByTestId('connection-status')).toHaveTextContent('Connected');
    }, { timeout: 20000 });
  });

  it('should throw error when used outside provider', () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    
    expect(() => {
      render(<TestComponent />);
    }).toThrow('useConnection must be used within a ConnectionProvider');
    
    consoleSpy.mockRestore();
  });
});
