import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import AppInstallationPhase from '../phases/AppInstallationPhase.tsx';
import { withTestButton } from './TestWrapper.tsx';

const TestAppInstallationPhase = withTestButton(AppInstallationPhase);

const createServer = (target: 'linux' | 'kubernetes') => setupServer(
  // Mock app installation status endpoint
  http.get(`*/api/${target}/install/app/status`, () => {
    return HttpResponse.json({
      status: {
        state: 'Running',
        description: 'Installing application components...'
      }
    });
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('AppInstallationPhase - $displayName', ({ target }) => {
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

  it('calls onStateChange with "Running" immediately when component mounts', async () => {
    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should call onStateChange with "Running" immediately on mount
    const calls = mockOnStateChange.mock.calls.map(args => args[0]);
    expect(calls).toEqual(['Running']);
    expect(mockOnStateChange).toHaveBeenCalledTimes(1);
  });

  it('shows loading state during installation', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: 'Installing application components...'
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show installation in progress (log viewer should be present)
    await waitFor(() => {
      expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
      expect(screen.getByText('Installing application components...')).toBeInTheDocument();
    });

    // Next button should be disabled during installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('shows success state when installation completes successfully', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Succeeded',
            description: 'Application installed successfully'
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show success message in progress area
    await waitFor(() => {
      expect(screen.getByText('Application installed successfully')).toBeInTheDocument();
    });

    // Next button should be enabled after successful installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    });

    // Should call onStateChange with "Succeeded"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toEqual(['Running', 'Succeeded']);
      expect(mockOnStateChange).toHaveBeenCalledTimes(2);
    });
  });

  it('shows error state when installation fails', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Failed',
            description: 'Installation failed due to insufficient resources'
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show error message
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
      expect(screen.getAllByText('Installation failed due to insufficient resources')).toHaveLength(2);
    });

    // Next button should be disabled after failed installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });

    // Should call onStateChange with "Failed"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toEqual(['Running', 'Failed']);
      expect(mockOnStateChange).toHaveBeenCalledTimes(2);
    });
  });

  it('handles API error gracefully', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
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
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show error message when API fails
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    // Next button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('handles network error gracefully', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show error message when network fails
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    // Next button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('shows default loading state when no status is available', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({});
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show log viewer (component is rendered)
    await waitFor(() => {
      expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
    });

    // Next button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('stops polling when installation succeeds', async () => {
    let callCount = 0;
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        callCount++;
        return HttpResponse.json({
          status: {
            state: 'Succeeded',
            description: 'Application installed successfully'
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for success message to appear
    await waitFor(() => {
      expect(screen.getByText('Application installed successfully')).toBeInTheDocument();
    });

    const initialCallCount = callCount;

    // Wait a bit more and ensure no additional calls are made
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Should not make additional API calls after success
    expect(callCount).toBeLessThanOrEqual(initialCallCount + 1); // Allow for one potential additional call due to timing
  });

  it('stops polling when installation fails', async () => {
    let callCount = 0;
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        callCount++;
        return HttpResponse.json({
          status: {
            state: 'Failed',
            description: 'Installation failed'
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for error message to appear
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    const initialCallCount = callCount;

    // Wait a bit more and ensure no additional calls are made
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Should not make additional API calls after failure
    expect(callCount).toBeLessThanOrEqual(initialCallCount + 1); // Allow for one potential additional call due to timing
  });

  it('displays custom status description when available', async () => {
    const customDescription = 'Configuring application settings and finalizing setup...';

    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: customDescription
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should display custom description in the progress message
    await waitFor(() => {
      expect(screen.getByText(customDescription)).toBeInTheDocument();
    });
  });

  it('displays fallback message when no status description is available', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running'
          }
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should display fallback message (Preparing installation...)
    await waitFor(() => {
      expect(screen.getByText('Preparing installation...')).toBeInTheDocument();
    });
  });

  it('displays individual component status indicators', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: 'Installing application components...'
          },
          components: [
            {
              name: 'Nginx App',
              status: { state: 'Running', description: 'Installing chart' }
            },
            {
              name: 'Redis App',
              status: { state: 'Succeeded', description: 'Installation complete' }
            }
          ]
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should display component status indicators
    await waitFor(() => {
      expect(screen.getByText('Nginx App')).toBeInTheDocument();
      expect(screen.getByText('Redis App')).toBeInTheDocument();
    });
  });

  it('calculates progress correctly with multiple components', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: 'Installing application components...'
          },
          components: [
            {
              name: 'Component 1',
              status: { state: 'Succeeded', description: 'Complete' }
            },
            {
              name: 'Component 2',
              status: { state: 'Succeeded', description: 'Complete' }
            },
            {
              name: 'Component 3',
              status: { state: 'Running', description: 'Installing' }
            },
            {
              name: 'Component 4',
              status: { state: 'Pending', description: 'Waiting' }
            }
          ]
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show progress (2 out of 4 components completed = 50%)
    await waitFor(() => {
      const progressBar = document.querySelector('[style*="width: 50%"]');
      expect(progressBar).toBeInTheDocument();
    });
  });

  it('shows 0% progress when no components are provided', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: 'Preparing installation...'
          },
          components: []
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show 0% progress when no components
    await waitFor(() => {
      const progressBar = document.querySelector('[style*="width: 0%"]');
      expect(progressBar).toBeInTheDocument();
    });
  });

  it('shows 100% progress when all components are succeeded', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Succeeded',
            description: 'Installation complete'
          },
          components: [
            {
              name: 'Component 1',
              status: { state: 'Succeeded', description: 'Complete' }
            },
            {
              name: 'Component 2',
              status: { state: 'Succeeded', description: 'Complete' }
            }
          ]
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should show 100% progress when all components succeeded
    await waitFor(() => {
      const progressBar = document.querySelector('[style*="width: 100%"]');
      expect(progressBar).toBeInTheDocument();
    });
  });

  it('handles mixed component states correctly', async () => {
    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: 'Installing application components...'
          },
          components: [
            {
              name: 'Database',
              status: { state: 'Succeeded', description: 'Installation complete' }
            },
            {
              name: 'API Server',
              status: { state: 'Running', description: 'Installing...' }
            },
            {
              name: 'Frontend',
              status: { state: 'Failed', description: 'Installation failed' }
            }
          ]
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should display all component names and states
    await waitFor(() => {
      expect(screen.getByText('Database')).toBeInTheDocument();
      expect(screen.getByText('API Server')).toBeInTheDocument();
      expect(screen.getByText('Frontend')).toBeInTheDocument();
    });

    // Progress should be 33% (1 out of 3 succeeded)
    await waitFor(() => {
      const progressBar = document.querySelector('[style*="width: 33%"]');
      expect(progressBar).toBeInTheDocument();
    });
  });

  it('displays logs from app installation', async () => {
    const testLogs = '[app] Installing nginx chart\n[kots] Creating namespace\n[kots] Deploying application';

    server.use(
      http.get(`*/api/${target}/install/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Running',
            description: 'Installing application components...'
          },
          components: [],
          logs: testLogs
        });
      })
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Should have log viewer available
    await waitFor(() => {
      expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
      expect(screen.getByTestId('log-viewer-toggle')).toBeInTheDocument();
    });
  });

});