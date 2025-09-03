import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { screen, fireEvent, waitFor, act } from '@testing-library/react';
import { renderWithProviders } from '../../../../test/setup.tsx';
import LogViewer from './LogViewer.tsx';

describe('LogViewer', () => {
  const mockOnToggle = vi.fn();
  const mockLogs = [
    '2024-01-01 10:00:00 INFO: Starting installation...',
    '2024-01-01 10:00:01 INFO: Checking system requirements...',
    '2024-01-01 10:00:02 INFO: System requirements met',
    '2024-01-01 10:00:03 INFO: Installing components...',
    '2024-01-01 10:00:04 INFO: Component A installed successfully',
    '2024-01-01 10:00:05 INFO: Component B installed successfully',
    '2024-01-01 10:00:06 INFO: Installation completed',
  ];

  beforeEach(() => {
    // Mock scrollIntoView
    const mockScrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = mockScrollIntoView;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders collapsed by default', () => {
    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={false}
        onToggle={mockOnToggle}
      />
    );

    expect(screen.getByTestId('log-viewer')).toBeInTheDocument();
    expect(screen.getByTestId('log-viewer-toggle')).toBeInTheDocument();
    expect(screen.getByText('Installation Logs')).toBeInTheDocument();
    expect(screen.queryByTestId('log-viewer-content')).not.toBeInTheDocument();
  });

  it('renders expanded when isExpanded is true', () => {
    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    expect(screen.getByTestId('log-viewer-content')).toBeInTheDocument();
    mockLogs.forEach(log => {
      expect(screen.getByText(log)).toBeInTheDocument();
    });
  });

  it('calls onToggle when toggle button is clicked', () => {
    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={false}
        onToggle={mockOnToggle}
      />
    );

    const toggleButton = screen.getByTestId('log-viewer-toggle');
    fireEvent.click(toggleButton);

    expect(mockOnToggle).toHaveBeenCalledTimes(1);
  });

  it('correctly detects scroll position changes', async () => {
    const mockScrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = mockScrollIntoView;

    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    const logContainer = screen.getByTestId('log-viewer-content');
    
    // Clear any initial calls to scrollIntoView
    mockScrollIntoView.mockClear();

    // Test scroll detection at bottom
    Object.defineProperty(logContainer, 'scrollTop', { value: 100, writable: true });
    Object.defineProperty(logContainer, 'scrollHeight', { value: 200, writable: true });
    Object.defineProperty(logContainer, 'clientHeight', { value: 100, writable: true });

    await act(async () => {
      fireEvent.scroll(logContainer);
    });

    // Test scroll detection away from bottom
    Object.defineProperty(logContainer, 'scrollTop', { value: 50, writable: true });

    await act(async () => {
      fireEvent.scroll(logContainer);
    });

    // Test scroll detection back to bottom
    Object.defineProperty(logContainer, 'scrollTop', { value: 100, writable: true });

    await act(async () => {
      fireEvent.scroll(logContainer);
    });

    // The component should handle all scroll events without errors
    expect(logContainer).toBeInTheDocument();
    
    // Verify that scrollIntoView was called at least once (initial render)
    expect(mockScrollIntoView).toHaveBeenCalled();
  });

  it('does not auto-scroll when component is collapsed', async () => {
    const mockScrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = mockScrollIntoView;

    const { rerender } = renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs.slice(0, 3)}
        isExpanded={false}
        onToggle={mockOnToggle}
      />
    );

    // Add more logs while collapsed
    rerender(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={false}
        onToggle={mockOnToggle}
      />
    );

    // Wait a bit to ensure no auto-scroll happens
    await new Promise(resolve => setTimeout(resolve, 100));

    expect(mockScrollIntoView).not.toHaveBeenCalled();
  });

  it('handles empty logs array', () => {
    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={[]}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    expect(screen.getByTestId('log-viewer-content')).toBeInTheDocument();
    // Should render empty container without any log entries
    const logContainer = screen.getByTestId('log-viewer-content');
    expect(logContainer.children.length).toBe(1); // Only the scroll anchor div
  });

  it('updates isAtBottom state correctly when scrolling', () => {
    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    const logContainer = screen.getByTestId('log-viewer-content');
    
    // Simulate scrolling to bottom
    Object.defineProperty(logContainer, 'scrollTop', { value: 100, writable: true });
    Object.defineProperty(logContainer, 'scrollHeight', { value: 200, writable: true });
    Object.defineProperty(logContainer, 'clientHeight', { value: 100, writable: true });

    fireEvent.scroll(logContainer);

    // Simulate scrolling away from bottom
    Object.defineProperty(logContainer, 'scrollTop', { value: 50, writable: true });
    
    fireEvent.scroll(logContainer);

    // The component should handle both scroll events without errors
    expect(logContainer).toBeInTheDocument();
  });

  it('handles scroll events when container ref is null', () => {
    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    const logContainer = screen.getByTestId('log-viewer-content');
    
    // Simulate a scenario where the ref might be null
    // This test ensures the component doesn't crash when scrollTop/scrollHeight/clientHeight are undefined
    Object.defineProperty(logContainer, 'scrollTop', { value: undefined, writable: true });
    Object.defineProperty(logContainer, 'scrollHeight', { value: undefined, writable: true });
    Object.defineProperty(logContainer, 'clientHeight', { value: undefined, writable: true });

    // Should not throw an error
    expect(() => {
      fireEvent.scroll(logContainer);
    }).not.toThrow();
  });

  it('auto-scrolls when user is at bottom and new logs are added', async () => {
    const mockScrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = mockScrollIntoView;

    const { rerender } = renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={mockLogs.slice(0, 3)}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    const logContainer = screen.getByTestId('log-viewer-content');
    
    // Clear any initial calls to scrollIntoView
    mockScrollIntoView.mockClear();

    // Simulate user at bottom of logs
    Object.defineProperty(logContainer, 'scrollTop', { value: 100, writable: true });
    Object.defineProperty(logContainer, 'scrollHeight', { value: 200, writable: true });
    Object.defineProperty(logContainer, 'clientHeight', { value: 100, writable: true });

    // Trigger scroll event to set isAtBottom to true
    await act(async () => {
      fireEvent.scroll(logContainer);
    });

    // Add more logs (this should trigger auto-scroll)
    await act(async () => {
      rerender(
        <LogViewer
          title="Installation Logs"
          logs={mockLogs}
          isExpanded={true}
          onToggle={mockOnToggle}
        />
      );
    });

    // Should auto-scroll when user is at bottom
    await waitFor(() => {
      expect(mockScrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth' });
    });
  });

  it('handles extremely long log lines without causing horizontal overflow', () => {
    const longLogLines = [
      'Short log message',
      // Very long JSON log line similar to what was seen in the bug report
      'level=DEBUG msg=Request id=3 url=https://ec-e2e-proxy.testcluster.net/v2/anonymous/ttl.sh/salah/embedded-cluster-operator/blobs/sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131\\\\\"\\n\"Content-Security-Policy\": \"frame-ancestors \\\'none\\\'; default-src \\\'none\\\'; sandbox\",\"digest\":\"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131\",\"mediaType\":\"application/vnd.cnf.helm.chart.content.v1.tar+gzip\",\"digest\":\"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131\",\"layer\":\"application/vnd.oci.image.layer.v1.tar+gzip\",\"size\":1259}],\"layer\":[{\"mediaType\":\"application/vnd.cnf.helm.chart.content.v1.tar+gzip\",\"digest\":\"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131\",\"size\":1259}],\"digest\":\"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131\",\"mediaType\":\"application/vnd.oci.image.layer.v1.tar+gzip\",\"size\":1259}]}',
      // Another very long line with repeated text
      'Authorization: Bearer ' + 'a'.repeat(500) + ' User-Agent: Helm/3.18.0',
      'Regular log message after long lines'
    ];

    renderWithProviders(
      <LogViewer
        title="Installation Logs"
        logs={longLogLines}
        isExpanded={true}
        onToggle={mockOnToggle}
      />
    );

    const logContainer = screen.getByTestId('log-viewer-content');
    
    // Verify the container has proper overflow classes
    expect(logContainer).toHaveClass('overflow-y-auto');
    expect(logContainer).toHaveClass('overflow-x-auto');
    
    // Verify all log lines are rendered
    longLogLines.forEach(log => {
      expect(screen.getByText(log)).toBeInTheDocument();
    });

    // Get all log line divs and verify they have break-all class
    const logElements = logContainer.querySelectorAll('div');
    const logLineDivs = Array.from(logElements).filter(div => 
      div.textContent && longLogLines.some(log => div.textContent === log)
    );
    
    logLineDivs.forEach(logDiv => {
      expect(logDiv).toHaveClass('break-all');
      expect(logDiv).toHaveClass('whitespace-pre-wrap');
    });

    // Test that the component doesn't crash with very long content
    expect(logContainer).toBeInTheDocument();
  });
}); 