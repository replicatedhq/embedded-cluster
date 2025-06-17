import React from 'react';
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../test/setup.tsx';
import ValidationStep from '../ValidationStep';
import { MOCK_INSTALL_CONFIG } from '../../../test/testData';

const server = setupServer(
  // Mock installation status endpoint
  http.get('*/api/install/installation/status', () => {
    return HttpResponse.json({
      state: 'Succeeded',
      description: 'Installation initialized',
      lastUpdated: new Date().toISOString()
    });
  }),

  // Mock preflight run endpoint
  http.post('*/api/install/host-preflights/run', () => {
    return HttpResponse.json({ success: true });
  }),

  // Mock preflight status endpoint - returns failures
  http.get('*/api/install/host-preflights/status', () => {
    return HttpResponse.json({
      status: { state: 'Failed' },
      output: {
        fail: [
          { title: 'Disk Space', message: 'Not enough disk space available' }
        ],
        warn: [],
        pass: []
      }
    });
  }),

  // Mock installation config endpoint
  http.get('*/api/install/installation/config', () => {
    return HttpResponse.json({
      ...MOCK_INSTALL_CONFIG,
      ignoreHostPreflights: false
    });
  }),

  // Mock infra setup endpoint
  http.post('*/api/install/infra/setup', () => {
    return HttpResponse.json({ success: true });
  })
);

describe('ValidationStep', () => {
  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();

  beforeAll(() => {
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it('disables button when preflights fail and ignoreHostPreflights is false', async () => {
    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            config: {
              ...MOCK_INSTALL_CONFIG,
              ignoreHostPreflights: false
            }
          }
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be disabled - user cannot proceed with failures by default
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).toBeDisabled();

    // Try to click disabled button - nothing should happen
    fireEvent.click(nextButton);

    // No modal should appear and onNext should not be called
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('shows modal when preflights fail and ignoreHostPreflights is true', async () => {
    // Mock config with ignoreHostPreflights = true
    server.use(
      http.get('*/api/install/installation/config', () => {
        return HttpResponse.json({
          ...MOCK_INSTALL_CONFIG,
          ignoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            config: {
              ...MOCK_INSTALL_CONFIG,
              ignoreHostPreflights: true
            }
          }
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be enabled when CLI flag is used
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).not.toBeDisabled();

    // Click Next button - should show modal
    fireEvent.click(nextButton);

    // Modal should appear
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    expect(screen.getByText(/Some validation checks have failed/)).toBeInTheDocument();
    expect(screen.getByText('Continue Anyway')).toBeInTheDocument();
    expect(screen.getByText('Cancel')).toBeInTheDocument();
  });

  it('allows user to cancel in modal', async () => {
    // Mock config with ignoreHostPreflights = true
    server.use(
      http.get('*/api/install/installation/config', () => {
        return HttpResponse.json({
          ...MOCK_INSTALL_CONFIG,
          ignoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            config: {
              ...MOCK_INSTALL_CONFIG,
              ignoreHostPreflights: true
            }
          }
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Click Next to show modal
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    // Click Cancel
    const cancelButton = screen.getByText('Cancel');
    fireEvent.click(cancelButton);

    // Modal should close and onNext should not be called
    await waitFor(() => {
      expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();
    });

    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('allows user to continue anyway in modal', async () => {
    // Mock config with ignoreHostPreflights = true
    server.use(
      http.get('*/api/install/installation/config', () => {
        return HttpResponse.json({
          ...MOCK_INSTALL_CONFIG,
          ignoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            config: {
              ...MOCK_INSTALL_CONFIG,
              ignoreHostPreflights: true
            }
          }
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Click Next to show modal
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    // Click Continue Anyway
    const continueButton = screen.getByText('Continue Anyway');
    fireEvent.click(continueButton);

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });
}); 