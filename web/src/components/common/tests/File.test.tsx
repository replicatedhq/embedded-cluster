import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import File from '../file/File';

describe('File', () => {
  const defaultProps = {
    dataTestId: 'test-file',
    filename: 'test-file.txt',
    handleDownload: vi.fn(),
    handleRemove: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders with basic props', () => {
      render(<File {...defaultProps} />);

      expect(screen.getByTestId('test-file-filename')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-filename')).toHaveTextContent('test-file.txt');
      expect(screen.getByTestId('test-file-filename')).toBeInTheDocument();

      // Icons should be present
      expect(screen.getByTestId('test-file-check-icon')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-file-icon')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-filename')).toHaveAttribute('title', 'Download file');
    });

    it('shows remove button when allowRemove is true', () => {
      render(<File {...defaultProps} allowRemove />);

      expect(screen.getByTestId('test-file-remove')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-remove')).toHaveAttribute('title', 'Remove file');
      expect(screen.getByTestId('test-file-remove-icon')).toBeInTheDocument();
    });

    it('hides remove button when allowRemove is false', () => {
      render(<File {...defaultProps} allowRemove={false} />);

      expect(screen.queryByTestId('test-file-remove')).not.toBeInTheDocument();
      expect(screen.queryByTestId('test-file-remove-icon')).not.toBeInTheDocument();
    });

    it('hides remove button when allowRemove is not provided', () => {
      render(<File {...defaultProps} />);

      expect(screen.queryByTestId('test-file-remove')).not.toBeInTheDocument();
      expect(screen.queryByTestId('test-file-remove-icon')).not.toBeInTheDocument();
    });
  });

  describe('Download functionality', () => {
    it('calls handleDownload when filename is clicked', async () => {
      const mockHandleDownload = vi.fn();
      const user = userEvent.setup();

      render(<File {...defaultProps} handleDownload={mockHandleDownload} />);

      const filename = screen.getByTestId('test-file-filename');
      await user.click(filename);

      expect(mockHandleDownload).toHaveBeenCalledTimes(1);
      expect(mockHandleDownload).toHaveBeenCalledWith(expect.any(Object));
    });

    it('does not call handleDownload when disabled and filename is clicked', async () => {
      const mockHandleDownload = vi.fn();
      const user = userEvent.setup();

      render(<File {...defaultProps} handleDownload={mockHandleDownload} disabled />);

      const filename = screen.getByTestId('test-file-filename');
      await user.click(filename);

      expect(mockHandleDownload).not.toHaveBeenCalled();
    });

    it('has correct styling and attributes for filename', () => {
      render(<File {...defaultProps} />);

      const filename = screen.getByTestId('test-file-filename');
      expect(filename).toHaveClass(
        'text-sm',
        'text-green-700',
        'font-medium',
        'hover:underline',
        'cursor-pointer'
      );
      expect(filename).toHaveAttribute('title', 'Download file');
    });
  });

  describe('Remove functionality', () => {
    it('calls handleRemove when remove button is clicked', async () => {
      const mockHandleRemove = vi.fn();
      const user = userEvent.setup();

      render(<File {...defaultProps} handleRemove={mockHandleRemove} allowRemove />);

      const removeButton = screen.getByTestId('test-file-remove');
      await user.click(removeButton);

      expect(mockHandleRemove).toHaveBeenCalledTimes(1);
      expect(mockHandleRemove).toHaveBeenCalledWith(expect.any(Object));
    });

    it('does not call handleRemove when disabled and remove button is clicked', async () => {
      const mockHandleRemove = vi.fn();
      const user = userEvent.setup();

      render(<File {...defaultProps} handleRemove={mockHandleRemove} allowRemove disabled />);

      const removeButton = screen.getByTestId('test-file-remove');
      await user.click(removeButton);

      expect(mockHandleRemove).not.toHaveBeenCalled();
    });

    it('remove button is disabled when disabled prop is true', () => {
      render(<File {...defaultProps} allowRemove disabled />);

      const removeButton = screen.getByTestId('test-file-remove');
      expect(removeButton).toBeDisabled();
    });

    it('remove button has correct styling and attributes', () => {
      render(<File {...defaultProps} allowRemove />);

      const removeButton = screen.getByTestId('test-file-remove');
      expect(removeButton).toHaveClass(
        'ml-2',
        'p-1',
        'rounded-full',
        'hover:bg-green-100',
        'transition-colors',
        'opacity-0',
        'group-hover:opacity-100'
      );
      expect(removeButton).toHaveAttribute('title', 'Remove file');
    });
  });

  describe('Visual styling', () => {
    it('has correct container styling', () => {
      render(<File {...defaultProps} />);

      const container = screen.getByTestId('test-file-container');
      expect(container).toHaveClass(
        'flex',
        'items-center',
        'space-x-2',
        'px-3',
        'py-2',
        'bg-green-50',
        'border',
        'border-green-200',
        'rounded-md',
        'group',
        'ml-3'
      );
    });

    it('displays correct icons with proper styling', () => {
      render(<File {...defaultProps} />);

      expect(screen.getByTestId('test-file-check-icon')).toHaveClass('w-4', 'h-4', 'text-green-500');
      expect(screen.getByTestId('test-file-file-icon')).toHaveClass('w-4', 'h-4', 'text-green-600');

      expect(screen.getByTestId('test-file-filename')).toHaveAttribute('title', 'Download file');
    });
  });

  describe('Integration scenarios', () => {
    it('works correctly with all props provided', async () => {
      const mockHandleDownload = vi.fn();
      const mockHandleRemove = vi.fn();
      const user = userEvent.setup();

      render(
        <File
          {...defaultProps}
          dataTestId="integration-test"
          handleDownload={mockHandleDownload}
          handleRemove={mockHandleRemove}
          allowRemove
          filename="integration-test.pdf"
        />
      );

      // Check filename display
      expect(screen.getByTestId('integration-test-filename')).toHaveTextContent('integration-test.pdf');

      // Test download functionality
      const filename = screen.getByTestId('integration-test-filename');
      await user.click(filename);
      expect(mockHandleDownload).toHaveBeenCalledTimes(1);

      // Test remove functionality
      const removeButton = screen.getByTestId('integration-test-remove');
      await user.click(removeButton);
      expect(mockHandleRemove).toHaveBeenCalledTimes(1);
    });

    it('handles long filenames correctly', () => {
      const longFilename = 'this-is-a-very-long-filename-that-might-cause-layout-issues.extension';

      render(<File {...defaultProps} filename={longFilename} />);

      expect(screen.getByTestId('test-file-filename')).toHaveTextContent(longFilename);
      expect(screen.getByTestId('test-file-filename')).toBeInTheDocument();
    });

    it('handles special characters in filename', () => {
      const specialFilename = 'file with spaces & special-chars (1).txt';

      render(<File {...defaultProps} filename={specialFilename} />);

      expect(screen.getByTestId('test-file-filename')).toHaveTextContent(specialFilename);
      expect(screen.getByTestId('test-file-filename')).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('provides proper titles for interactive elements', () => {
      render(<File {...defaultProps} allowRemove />);

      expect(screen.getByTestId('test-file-filename')).toHaveAttribute('title', 'Download file');
      expect(screen.getByTestId('test-file-remove')).toHaveAttribute('title', 'Remove file');
    });

    it('maintains proper keyboard accessibility for remove button', () => {
      render(<File {...defaultProps} allowRemove />);

      const removeButton = screen.getByTestId('test-file-remove');
      expect(removeButton.tagName).toBe('BUTTON');
    });

    it('handles disabled state correctly for screen readers', () => {
      render(<File {...defaultProps} allowRemove disabled />);

      const removeButton = screen.getByTestId('test-file-remove');
      expect(removeButton).toHaveAttribute('disabled');
    });
  });
});
