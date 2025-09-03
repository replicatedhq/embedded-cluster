import { describe, it, expect } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { renderWithProviders } from '../../../../test/setup.tsx';
import ErrorMessage from './ErrorMessage.tsx';

describe('ErrorMessage', () => {
  it('renders short error messages without truncation', () => {
    const shortError = 'This is a short error message';
    
    renderWithProviders(<ErrorMessage error={shortError} />);
    
    expect(screen.getByTestId('error-message')).toBeInTheDocument();
    expect(screen.getByText('Installation Error')).toBeInTheDocument();
    expect(screen.getByText(shortError)).toBeInTheDocument();
    expect(screen.queryByTestId('error-toggle')).not.toBeInTheDocument();
  });

  it('truncates long error messages by default (250 chars)', () => {
    const longError = 'A'.repeat(300); // 300 character error message
    
    renderWithProviders(<ErrorMessage error={longError} />);
    
    const errorElement = screen.getByTestId('error-message');
    expect(errorElement).toBeInTheDocument();
    
    // Should show truncated version with ellipsis (250 chars default)
    const truncatedText = 'A'.repeat(250) + '...';
    expect(screen.getByText(truncatedText)).toBeInTheDocument();
    
    // Should show toggle button
    expect(screen.getByTestId('error-toggle')).toBeInTheDocument();
    expect(screen.getByText('Show more')).toBeInTheDocument();
    
    // Should not show the full error
    expect(screen.queryByText(longError)).not.toBeInTheDocument();
  });

  it('expands to show more content when "Show more" is clicked', () => {
    const longError = 'A'.repeat(300); // 300 character error message
    
    renderWithProviders(<ErrorMessage error={longError} />);
    
    // Initially truncated
    expect(screen.getByText('A'.repeat(250) + '...')).toBeInTheDocument();
    expect(screen.getByText('Show more')).toBeInTheDocument();
    
    // Click to expand
    fireEvent.click(screen.getByTestId('error-toggle'));
    
    // Should show full content (less than 1000 chars)
    expect(screen.getByText(longError)).toBeInTheDocument();
    expect(screen.getByText('Show less')).toBeInTheDocument();
    
    // Click to collapse
    fireEvent.click(screen.getByTestId('error-toggle'));
    
    // Should be truncated again
    expect(screen.getByText('A'.repeat(250) + '...')).toBeInTheDocument();
    expect(screen.getByText('Show more')).toBeInTheDocument();
  });

  it('truncates even expanded content when it exceeds 1000 characters', () => {
    const veryLongError = 'A'.repeat(1500); // 1500 character error message
    
    renderWithProviders(<ErrorMessage error={veryLongError} />);
    
    // Initially truncated to 250
    expect(screen.getByText('A'.repeat(250) + '...')).toBeInTheDocument();
    
    // Click to expand
    fireEvent.click(screen.getByTestId('error-toggle'));
    
    // Should be truncated to 1000 chars even when expanded
    expect(screen.getByText('A'.repeat(1000) + '...')).toBeInTheDocument();
    expect(screen.getByText('Show less')).toBeInTheDocument();
  });

  it('respects custom maxLength and expandedMaxLength props', () => {
    const longError = 'A'.repeat(100);
    
    renderWithProviders(
      <ErrorMessage 
        error={longError} 
        maxLength={20} 
        expandedMaxLength={50} 
      />
    );
    
    // Initially truncated to custom maxLength
    expect(screen.getByText('A'.repeat(20) + '...')).toBeInTheDocument();
    
    // Click to expand
    fireEvent.click(screen.getByTestId('error-toggle'));
    
    // Should be truncated to custom expandedMaxLength
    expect(screen.getByText('A'.repeat(50) + '...')).toBeInTheDocument();
  });

  it('does not truncate when error is exactly at maxLength', () => {
    const exactLengthError = 'A'.repeat(250); // Exactly 250 characters
    
    renderWithProviders(<ErrorMessage error={exactLengthError} />);
    
    expect(screen.getByText(exactLengthError)).toBeInTheDocument();
    expect(screen.queryByText(/\.\.\./)).not.toBeInTheDocument();
    expect(screen.queryByTestId('error-toggle')).not.toBeInTheDocument();
  });

  it('handles very long error messages similar to those in bug reports', () => {
    const veryLongError = `level=DEBUG msg=Request id=3 url=https://ec-e2e-proxy.testcluster.net/v2/anonymous/ttl.sh/salah/embedded-cluster-operator/blobs/sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131\\"\\n"Content-Security-Policy": "frame-ancestors \\'none\\'; default-src \\'none\\'; sandbox","digest":"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131","mediaType":"application/vnd.cnf.helm.chart.content.v1.tar+gzip","digest":"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131","layer":"application/vnd.oci.image.layer.v1.tar+gzip","size":1259}],"layer":[{"mediaType":"application/vnd.cnf.helm.chart.content.v1.tar+gzip","digest":"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131","size":1259}],"digest":"sha256:4b2ac16cacd8d47216406c3d0061666949203030c2c74ccst1756913131","mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","size":1259}]}`;
    
    renderWithProviders(<ErrorMessage error={veryLongError} />);
    
    const errorElement = screen.getByTestId('error-message');
    expect(errorElement).toBeInTheDocument();
    
    // Should be truncated to 250 characters + ellipsis
    const truncatedText = veryLongError.substring(0, 250) + '...';
    expect(screen.getByText(truncatedText)).toBeInTheDocument();
    
    // Should have expand button
    expect(screen.getByTestId('error-toggle')).toBeInTheDocument();
    
    // Should not show the full error initially
    expect(screen.queryByText(veryLongError)).not.toBeInTheDocument();
  });

  it('applies proper CSS classes for text wrapping', () => {
    const longError = 'A'.repeat(300);
    
    renderWithProviders(<ErrorMessage error={longError} />);
    
    const errorParagraph = screen.getByText(/A+\.{3}/);
    expect(errorParagraph).toHaveClass('whitespace-pre-wrap');
    expect(errorParagraph).toHaveClass('break-words');
  });
});