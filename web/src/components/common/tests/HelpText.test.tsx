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
  describe('Text truncation for default text', () => {
    it('does not truncate short text', () => {
      const shortText = 'This is a short help text';
      render(<HelpText helpText={shortText} />);

      expect(screen.getByText(shortText)).toBeInTheDocument();
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('does not truncate long help text when no default value is present', () => {
      const longText = 'This is a very long help text that exceeds the maximum length of 80 characters and should not be truncated when there is no default value present';
      render(<HelpText helpText={longText} />);

      // Should show the full text without truncation
      expect(screen.getByText(longText)).toBeInTheDocument();
      // Should not show "Show more" button
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
      // Should not show ellipsis
      expect(screen.queryByText(/\.\.\./)).not.toBeInTheDocument();
    });

    it('expands text when "Show more" is clicked for long default values', () => {
      const helpText = 'This is help text';
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-80-characters-and-should-be-truncated-when-combined-with-help-text';
      render(<HelpText helpText={helpText} defaultValue={longDefaultValue} />);

      const showMoreButton = screen.getByText('Show more');
      fireEvent.click(showMoreButton);

      // Should show full text after clicking
      expect(screen.getByText(longDefaultValue)).toBeInTheDocument();
      // Button should change to "Show less"
      expect(screen.getByText('Show less')).toBeInTheDocument();
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('collapses text when "Show less" is clicked for long default values', () => {
      const helpText = 'This is help text';
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-80-characters-and-should-be-truncated-when-combined-with-help-text';
      render(<HelpText helpText={helpText} defaultValue={longDefaultValue} />);

      const showMoreButton = screen.getByText('Show more');
      fireEvent.click(showMoreButton);

      const showLessButton = screen.getByText('Show less');
      fireEvent.click(showLessButton);

      // Should show truncated text again
      expect(screen.getByText(/\.\.\./)).toBeInTheDocument();
      expect(screen.getByText('Show more')).toBeInTheDocument();
      expect(screen.queryByText('Show less')).not.toBeInTheDocument();
    });

    it('truncates text when default value exceeds 80 characters', () => {
      const helpText = 'This is help text';
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-80-characters-and-should-be-truncated-making-the-combined-text-too-long';

      render(<HelpText helpText={helpText} defaultValue={longDefaultValue} />);

      // Should show "Show more" button when default value exceeds 80 chars
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

    it('preserves markdown formatting in help text with long default value', () => {
      const helpTextWithMarkdown = 'This help text has `code formatting` and **bold text**';
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-80-characters-and-should-be-truncated-properly-while-preserving-markdown';
      render(<HelpText helpText={helpTextWithMarkdown} defaultValue={longDefaultValue} />);

      expect(screen.getByText('Show more')).toBeInTheDocument();

      // Click to expand and check markdown is preserved
      fireEvent.click(screen.getByText('Show more'));
      expect(screen.getByText('code formatting')).toHaveClass('font-mono');
      expect(screen.getByText('bold text')).toBeInTheDocument();
      expect(screen.getByText(longDefaultValue)).toBeInTheDocument();
    });

    it('preserves markdown formatting in help text with code blocks and long default value', () => {
      const helpTextWithCodeBlock = 'To configure TLS:\n\n```\nssl_cert=cert.pem\nssl_key=key.pem\n```';
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-80-characters-and-should-be-truncated-making-the-combined-text-too-long';
      const { container } = render(<HelpText helpText={helpTextWithCodeBlock} defaultValue={longDefaultValue} />);

      expect(screen.getByText('Show more')).toBeInTheDocument();

      // Click to expand and check code block markdown is preserved
      fireEvent.click(screen.getByText('Show more'));

      // Check that certificate content is present
      expect(screen.getByText(/ssl_cert=cert.pem/)).toBeInTheDocument();
      expect(screen.getByText(/ssl_key=key.pem/)).toBeInTheDocument();

      // Check that pre/code elements exist for code block
      const preElements = container.querySelectorAll('pre');
      const codeElements = container.querySelectorAll('code');
      expect(preElements.length).toBeGreaterThan(0);
      expect(codeElements.length).toBeGreaterThan(0);
    });

    it('applies correct CSS classes to show more/less button', () => {
      const helpText = 'Help text';
      // Need a much longer default value to exceed helpText.length + 80 + threshold
      const longDefaultValue = 'a'.repeat(150); // This will definitely exceed the threshold
      render(<HelpText helpText={helpText} defaultValue={longDefaultValue} />);

      const showMoreButton = screen.getByText('Show more');
      expect(showMoreButton).toHaveClass('ml-1', 'text-blue-600', 'hover:text-blue-800', 'text-xs', 'cursor-pointer');
      expect(showMoreButton).toHaveAttribute('type', 'button');
    });

    it('does not truncate default value exactly at 80 characters', () => {
      const helpText = 'Help text';
      // Create default value that's exactly 80 characters
      const exactMaxDefaultValue = 'a'.repeat(80);
      render(<HelpText helpText={helpText} defaultValue={exactMaxDefaultValue} />);

      // Should not show "Show more" button at exactly 80 chars for default value
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('does not truncate default value within the truncation threshold', () => {
      const helpText = 'Help text';
      // Create default value that's 100 characters (over 80 but within threshold of 40)
      const withinThresholdDefaultValue = 'a'.repeat(100);
      render(<HelpText helpText={helpText} defaultValue={withinThresholdDefaultValue} />);

      // Should not show "Show more" button when within threshold (80 + 40 = 120)
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('does not truncate default value at the edge of truncation threshold', () => {
      const helpText = 'Help text'; // 9 characters
      // maxTextLength = 9 + 80 = 89
      // combinedText = "Help text (Default: `...defaultValue...`)"
      // For threshold edge: combinedText.length should be maxTextLength + 40 = 129
      // "Help text (Default: `...`)" = 9 + " (Default: `" + defaultValue + "`)" = 9 + 13 + defaultValue.length + 2 = 24 + defaultValue.length
      // So we need: 24 + defaultValue.length = 129, therefore defaultValue.length = 105
      const atThresholdEdgeDefaultValue = 'a'.repeat(105);
      render(<HelpText helpText={helpText} defaultValue={atThresholdEdgeDefaultValue} />);

      // Should not show "Show more" button when exactly at threshold edge
      expect(screen.queryByText('Show more')).not.toBeInTheDocument();
    });

    it('truncates default value just over the truncation threshold', () => {
      const helpText = 'Help text';
      // Create default value that's 121 characters (over 80 + threshold = 80 + 40)
      const justOverThresholdDefaultValue = 'a'.repeat(121);
      render(<HelpText helpText={helpText} defaultValue={justOverThresholdDefaultValue} />);

      // Should show "Show more" button when over threshold limit
      expect(screen.getByText('Show more')).toBeInTheDocument();
    });

    it('maintains proper data-testid when truncated', () => {
      const helpText = 'Help text';
      const longDefaultValue = 'this-is-a-very-long-default-value-that-exceeds-80-characters-and-the-threshold-so-it-should-be-truncated';
      const { container } = render(<HelpText helpText={helpText} defaultValue={longDefaultValue} dataTestId="custom-test-id" />);

      const helpDiv = container.querySelector('[data-testid="help-text-custom-test-id"]');
      expect(helpDiv).toBeInTheDocument();
    });
  });
});
