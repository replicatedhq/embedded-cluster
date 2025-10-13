import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import InfraUpgradePhase from '../phases/InfraUpgradePhase.tsx';
import { withTestButton } from './TestWrapper.tsx';

const TestInfraUpgradePhase = withTestButton(InfraUpgradePhase);

const createServer = (target: 'linux' | 'kubernetes', mode: 'install' | 'upgrade' = 'upgrade') => setupServer(
  // Mock infrastructure upgrade status endpoint
  http.get(`*/api/${target}/${mode}/infra/status`, () => {
    return HttpResponse.json({
      components: [],
      logs: '',
      status: {
        state: 'Running',
        description: 'Upgrading infrastructure components...',
        lastUpdated: new Date().toISOString()
      }
    });
  }),
  // Mock infrastructure upgrade start endpoint
  http.post(`*/api/${target}/${mode}/infra/upgrade`, () => {
    return HttpResponse.json({
      components: [],
      logs: '',
      status: {
        state: 'Running',
        description: 'Starting infrastructure upgrade...',
        lastUpdated: new Date().toISOString()
      }
    });
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('InfraUpgradePhase - $displayName', ({ target }) => {
  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it('calls onStateChange with "Pending" immediately when component mounts', async () => {
    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should call onStateChange with "Pending" immediately on mount
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toContain('Pending');
    });
  });

  it('shows ready state before upgrade starts', async () => {
    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should show ready state
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-ready')).toBeInTheDocument();
      expect(screen.getByText(/Click "Next" to start the infrastructure upgrade/i)).toBeInTheDocument();
    });

    // Next button should be enabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    });
  });

  it('hides back button before upgrade starts', async () => {
    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Back button should not be present
    await waitFor(() => {
      expect(screen.queryByTestId('back-button')).not.toBeInTheDocument();
    });
  });

  it('starts upgrade when next button is clicked', async () => {
    let upgradeStarted = false;
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        upgradeStarted = true;
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting infrastructure upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Click next to start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should have called the upgrade API
    await waitFor(() => {
      expect(upgradeStarted).toBe(true);
    });
  });

  it('shows running state during upgrade', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Upgrading Kubernetes...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        return HttpResponse.json({
          components: [
            {
              name: 'k0s',
              status: {
                state: 'Running',
                description: 'Upgrading k0s...',
                lastUpdated: new Date().toISOString()
              }
            }
          ],
          logs: 'Upgrading k0s to version 1.30...',
          status: {
            state: 'Running',
            description: 'Upgrading Kubernetes...',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should show running state
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-running')).toBeInTheDocument();
      expect(screen.getByTestId('infra-upgrade-description')).toHaveTextContent('Upgrading Kubernetes...');
    });

    // Should show component status
    await waitFor(() => {
      expect(screen.getByText('k0s')).toBeInTheDocument();
      expect(screen.getByText('Running')).toBeInTheDocument();
    });

    // Next button should be disabled during upgrade
    await waitFor(() => {
      expect(nextButton).toBeDisabled();
    });

    // Should call onStateChange with "Running"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toContain('Running');
    });
  });

  it('shows success state when upgrade completes successfully', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        return HttpResponse.json({
          components: [
            {
              name: 'k0s',
              status: {
                state: 'Succeeded',
                description: 'Upgraded successfully',
                lastUpdated: new Date().toISOString()
              }
            }
          ],
          logs: 'Upgrade completed successfully',
          status: {
            state: 'Succeeded',
            description: 'Infrastructure upgraded successfully',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should show success state
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-success')).toBeInTheDocument();
      expect(screen.getByText('Infrastructure Upgraded Successfully')).toBeInTheDocument();
    });

    // Next button should be enabled after successful upgrade
    await waitFor(() => {
      expect(nextButton).not.toBeDisabled();
    });

    // Should call onStateChange with "Succeeded"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toContain('Succeeded');
    });
  });

  it('shows error state when upgrade fails', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        return HttpResponse.json({
          components: [
            {
              name: 'k0s',
              status: {
                state: 'Failed',
                description: 'Upgrade failed',
                lastUpdated: new Date().toISOString()
              }
            }
          ],
          logs: 'Error: Failed to upgrade k0s',
          status: {
            state: 'Failed',
            description: 'Infrastructure upgrade failed due to k0s upgrade error',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should show error state
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-error')).toBeInTheDocument();
      expect(screen.getByTestId('infra-upgrade-error-message')).toHaveTextContent('Infrastructure upgrade failed due to k0s upgrade error');
    });

    // Next button should be disabled after failed upgrade
    await waitFor(() => {
      expect(nextButton).toBeDisabled();
    });

    // Should call onStateChange with "Failed"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toContain('Failed');
    });
  });

  it('handles API error when starting upgrade', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json(
          {
            statusCode: 500,
            message: 'Internal server error'
          },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Try to start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should show error message
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });
  });

  it('handles API error when polling status', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        return HttpResponse.json(
          {
            statusCode: 500,
            message: 'Failed to get upgrade status'
          },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should show error message
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });
  });

  it('stops polling when upgrade succeeds', async () => {
    let callCount = 0;
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        callCount++;
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Succeeded',
            description: 'Infrastructure upgraded successfully',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-success')).toBeInTheDocument();
    });

    const initialCallCount = callCount;

    // Wait a bit more and ensure no additional calls are made
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Should not make additional API calls after success
    expect(callCount).toBeLessThanOrEqual(initialCallCount + 1);
  });

  it('stops polling when upgrade fails', async () => {
    let callCount = 0;
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        callCount++;
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Failed',
            description: 'Infrastructure upgrade failed',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Wait for failure state
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-error')).toBeInTheDocument();
    });

    const initialCallCount = callCount;

    // Wait a bit more and ensure no additional calls are made
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Should not make additional API calls after failure
    expect(callCount).toBeLessThanOrEqual(initialCallCount + 1);
  });

  it('displays logs when available', async () => {
    const testLogs = 'Downloading k0s binary...\nStarting k0s upgrade...\nCompleted successfully';

    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      }),
      http.get(`*/api/${target}/upgrade/infra/status`, () => {
        return HttpResponse.json({
          components: [],
          logs: testLogs,
          status: {
            state: 'Running',
            description: 'Upgrading infrastructure...',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Should display logs
    await waitFor(() => {
      expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
    });
  });

  it('hides back button after upgrade starts', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/infra/upgrade`, () => {
        return HttpResponse.json({
          components: [],
          logs: '',
          status: {
            state: 'Running',
            description: 'Starting upgrade...',
            lastUpdated: new Date().toISOString()
          }
        });
      })
    );

    renderWithProviders(
      <TestInfraUpgradePhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Back button should not be visible initially
    expect(screen.queryByTestId('back-button')).not.toBeInTheDocument();

    // Start upgrade
    const nextButton = await screen.findByTestId('next-button');
    fireEvent.click(nextButton);

    // Back button should still not be visible after upgrade starts
    await waitFor(() => {
      expect(screen.getByTestId('infra-upgrade-running')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('back-button')).not.toBeInTheDocument();
  });
});
