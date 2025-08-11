import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import HelpText from '../HelpText';

describe('HelpText', () => {
  it('renders help text without default value', () => {
    render(<HelpText helpText="This is help text" />);
    expect(screen.getByText('This is help text')).toBeInTheDocument();
  });

  it('renders default value without help text', () => {
    const { container } = render(<HelpText defaultValue="default-value" />);
    expect(container).toHaveTextContent('(Default: default-value)');
    expect(screen.getByText('default-value')).toBeInTheDocument();
  });

  it('renders help text with default value inline', () => {
    const { container } = render(<HelpText helpText="This is help text" defaultValue="default-value" />);
    expect(container).toHaveTextContent('This is help text (Default: default-value)');
    expect(screen.getByText('default-value')).toBeInTheDocument();
  });

  it('does not render when both helpText and defaultValue are missing', () => {
    const { container } = render(<HelpText />);
    expect(container.firstChild).toBeNull();
  });

  it('does not render when error is present', () => {
    const { container } = render(
      <HelpText helpText="Help text" defaultValue="default-value" error="Error message" />
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders with proper CSS classes', () => {
    const { container } = render(<HelpText helpText="Help text" />);
    const helpDiv = container.querySelector('div');
    expect(helpDiv).toHaveClass('mt-1', 'text-sm', 'text-gray-500');
  });

  it('renders default values with code styling', () => {
    render(<HelpText defaultValue="default-value" />);
    const codeElement = screen.getByText('default-value');
    expect(codeElement).toHaveClass('font-mono', 'text-xs', 'bg-gray-100', 'px-1', 'py-0.5', 'rounded');
  });

  it('handles complex default values with special characters', () => {
    const { container } = render(<HelpText defaultValue="special-!@#$%^&*()-value" />);
    expect(container).toHaveTextContent('(Default: special-!@#$%^&*()-value)');
    expect(screen.getByText('special-!@#$%^&*()-value')).toBeInTheDocument();
  });

  it('renders markdown in help text properly', () => {
    render(<HelpText helpText="This has `code` in it" />);
    const codeElement = screen.getByText('code');
    expect(codeElement).toHaveClass('font-mono', 'text-xs', 'bg-gray-100', 'px-1', 'py-0.5', 'rounded');
  });

  it('renders only helpText when defaultValue is empty string', () => {
    const { container } = render(<HelpText helpText="Help text" defaultValue="" />);
    expect(screen.getByText('Help text')).toBeInTheDocument();
    expect(container).not.toHaveTextContent('(Default:');
  });

  it('renders only defaultValue when helpText is empty string', () => {
    const { container } = render(<HelpText helpText="" defaultValue="default-value" />);
    expect(container).toHaveTextContent('(Default: default-value)');
    expect(screen.getByText('default-value')).toBeInTheDocument();
  });

  it('renders paragraphs with inline styling', () => {
    const { container } = render(<HelpText helpText="Help text" />);
    // The ReactMarkdown should render p tags with inline styling via CSS
    const paragraphs = container.querySelectorAll('p');
    expect(paragraphs.length).toBeGreaterThan(0);
    // Check that the container has the inline styling classes
    const helpDiv = container.querySelector('div');
    expect(helpDiv).toHaveClass('[&_p]:inline', '[&_p]:mb-0');
  });

  // Truncation functionality tests
  describe('Text truncation', () => {
    it('does not truncate short text', () => {
      const shortText = 'This is a short help text';
      render(<HelpText helpText={shortText} />);

      expect(screen.getByText(shortText)).toBeInTheDocument();
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

        it('truncates long text and shows "Show more" button', () => {
      const longText = 'This is a very long help text that exceeds the maximum length of 80 characters and should be truncated with an ellipsis and show more button because it is longer than the threshold';
      render(<HelpText helpText={longText} />);

      // Should show truncated text with ellipsis
      expect(screen.getByText(/\.\.\./)).toBeInTheDocument();
      // Should show "Show more" button
      expect(screen.getByText('Show more')).toBeInTheDocument();
      // Should not show the full text initially
      expect(screen.queryByText(longText)).not.toBeInTheDocument();
    });

        it('expands text when "Show more" is clicked', () => {
      const longText = 'This is a very long help text that exceeds the maximum length of 80 characters and should be truncated with an ellipsis and show more button because it is longer than the threshold';
      render(<HelpText helpText={longText} />);

      const showMoreButton = screen.getByText('Show more');
      fireEvent.click(showMoreButton);

      // Should show full text after clicking
      expect(screen.getByText(longText)).toBeInTheDocument();
      // Button should change to "Show less"
      expect(screen.getByText('Show less')).toBeInTheDocument();
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

        it('collapses text when "Show less" is clicked', () => {
      const longText = 'This is a very long help text that exceeds the maximum length of 80 characters and should be truncated with an ellipsis and show more button because it is longer than the threshold';
      render(<HelpText helpText={longText} />);

      const showMoreButton = screen.getByText('Show more');
      fireEvent.click(showMoreButton);

      const showLessButton = screen.getByText('Show less');
      fireEvent.click(showLessButton);

      // Should show truncated text again
      expect(screen.getByText(/\.\.\./)).toBeInTheDocument();
      expect(screen.getByText('Show more')).toBeInTheDocument();
      expect(screen.queryByText('Show less')).not.toBeInTheDocument();
    });

    it('truncates combined text with default value when total length exceeds limit', () => {
      const longHelpText = 'This is a moderately long help text that when combined with default value it exceeds the limit';
      const defaultValue = 'very-long-default-value-that-makes-total-exceed-limit';

      render(<HelpText helpText={longHelpText} defaultValue={defaultValue} />);

      // Should show "Show more" button when combined text is too long
      expect(screen.getByText('Show more')).toBeInTheDocument();
      expect(screen.getByText(/\.\.\./)).toBeInTheDocument();
    });

    it('does not truncate when only default value is present and under limit', () => {
      const shortDefaultValue = 'short-default';
      render(<HelpText defaultValue={shortDefaultValue} />);

      expect(screen.getByText(shortDefaultValue)).toBeInTheDocument();
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

        it('truncates when only default value is present and exceeds limit', () => {
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-the-maximum-character-limit-of-80-characters-and-the-threshold-so-it-should-be-truncated';
      render(<HelpText defaultValue={longDefaultValue} />);

      expect(screen.getByText('Show more')).toBeInTheDocument();
      expect(screen.getByText(/\.\.\./)).toBeInTheDocument();
    });

            it('preserves markdown formatting in truncated text', () => {
      const longTextWithMarkdown = 'This is a very long help text with `code formatting` and **bold text** that exceeds the maximum length of 80 characters and the threshold so it should be truncated properly';
      render(<HelpText helpText={longTextWithMarkdown} />);

      expect(screen.getByText('Show more')).toBeInTheDocument();

      // Click to expand and check markdown is preserved
      fireEvent.click(screen.getByText('Show more'));
      expect(screen.getByText('code formatting')).toHaveClass('font-mono');
      expect(screen.getByText('bold text')).toBeInTheDocument();
    });

    it('preserves markdown formatting in truncated text with code blocks', () => {
      // Create text with TLS certificate that will be truncated in the middle of the certificate
      const longTextWithCodeBlock = 'To configure TLS, provide your certificate:\n\n```\n-----BEGIN CERTIFICATE-----\nMIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV\nBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX\naWRnaXRzIFB0eSBMdGQwHhcNMTcwODI3MjM1NzU5WhcNMTgwODI3MjM1NzU5WjBF\nMQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50\nZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB\nCgKCAQEAuuB5/8GvxwqhkzCQF22XA0cGRUbKCzlJHoFGELqoZKxSV8j9C7sW7A==\n-----END CERTIFICATE-----\n```\n\nEnsure the certificate is valid.';
      const { container } = render(<HelpText helpText={longTextWithCodeBlock} />);

      expect(screen.getByText('Show more')).toBeInTheDocument();

      // Click to expand and check code block markdown is preserved
      fireEvent.click(screen.getByText('Show more'));

      // Check that certificate content is present (may be in different elements)
      expect(screen.getByText(/BEGIN CERTIFICATE/)).toBeInTheDocument();
      expect(screen.getByText(/END CERTIFICATE/)).toBeInTheDocument();

      // Check that pre/code elements exist for code block
      const preElements = container.querySelectorAll('pre');
      const codeElements = container.querySelectorAll('code');
      expect(preElements.length).toBeGreaterThan(0);
      expect(codeElements.length).toBeGreaterThan(0);
    });

        it('applies correct CSS classes to show more/less button', () => {
      const longText = 'This is a very long help text that exceeds the maximum length of 80 characters and should be truncated with an ellipsis and show more button because it is longer than the threshold';
      render(<HelpText helpText={longText} />);

      const showMoreButton = screen.getByText('Show more');
      expect(showMoreButton).toHaveClass('ml-1', 'text-blue-600', 'hover:text-blue-800', 'text-xs', 'cursor-pointer');
      expect(showMoreButton).toHaveAttribute('type', 'button');
    });

            it('handles text exactly at maxTextLength', () => {
      // Create text that's exactly 80 characters (the maxTextLength)
      const exactMaxText = 'a'.repeat(80);
      render(<HelpText helpText={exactMaxText} />);

      // Should not show "Show more" button at exactly maxTextLength
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('handles text within the truncation threshold', () => {
      // Create text that's 100 characters (over maxTextLength of 80 but within threshold of 40)
      const withinThresholdText = 'a'.repeat(100);
      render(<HelpText helpText={withinThresholdText} />);

      // Should not show "Show more" button when within threshold (80 + 40 = 120)
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('handles text at the edge of truncation threshold', () => {
      // Create text that's exactly 120 characters (maxTextLength + threshold = 80 + 40)
      const atThresholdEdgeText = 'a'.repeat(120);
      render(<HelpText helpText={atThresholdEdgeText} />);

      // Should not show "Show more" button when exactly at threshold edge
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('handles text just over the truncation threshold', () => {
      // Create text that's 121 characters (over maxTextLength + threshold = 80 + 40)
      const justOverThresholdText = 'a'.repeat(121);
      render(<HelpText helpText={justOverThresholdText} />);

      // Should show "Show more" button when over threshold limit
      expect(screen.getByText('Show more')).toBeInTheDocument();
    });

        it('maintains proper data-testid when truncated', () => {
      const longText = 'This is a very long help text that exceeds the maximum length of 80 characters and the threshold so it should be truncated';
      const { container } = render(<HelpText helpText={longText} dataTestId="custom-test-id" />);

      const helpDiv = container.querySelector('[data-testid="help-text-custom-test-id"]');
      expect(helpDiv).toBeInTheDocument();
    });
  });
});
