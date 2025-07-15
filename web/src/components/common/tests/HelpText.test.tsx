import React from 'react';
import { render, screen } from '@testing-library/react';
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

  it('renders as span instead of paragraph', () => {
    const { container } = render(<HelpText helpText="Help text" />);
    // The ReactMarkdown should render p tags as spans based on our custom components
    const spans = container.querySelectorAll('span');
    expect(spans.length).toBeGreaterThan(0);
    const paragraphs = container.querySelectorAll('p');
    expect(paragraphs.length).toBe(0);
  });
}); 