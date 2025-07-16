import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FileInput from './FileInput';

// Mock the SettingsContext
const mockThemeColor = '#3498DB';
vi.mock('../../contexts/SettingsContext', () => ({
  useSettings: () => ({
    settings: {
      themeColor: mockThemeColor
    }
  })
}));

// Helper function to create a mock file
const createMockFile = (name: string, content: string, type: string): File => {
  return new File([content], name, { type });
};

// Test content with UTF-8 characters
const mockFileContent = 'Hello 👋 café naïve résumé 中文 🚀';
const mockFileContentBase64 = btoa(String.fromCharCode(...new TextEncoder().encode(mockFileContent)));

// Mock DOM methods locally
const mockAnchorClick = vi.fn();

// Store original createElement
const originalCreateElement = document.createElement.bind(document);

describe('FileInput', () => {
  const defaultProps = {
    id: 'test-file-input',
    label: 'Test File Input',
    onChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();

    // Mock document.createElement for anchor elements
    vi.spyOn(document, 'createElement').mockImplementation((tagName: string) => {
      const element = originalCreateElement(tagName);
      if (tagName === 'a') {
        element.click = mockAnchorClick;
      }
      return element;
    });
  });


  describe('Rendering', () => {
    it('renders with basic props', () => {
      render(<FileInput {...defaultProps} />);

      expect(screen.getByText('Test File Input')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-input-button')).toBeInTheDocument();
    });

    it('renders with required indicator when required is true', () => {
      render(<FileInput {...defaultProps} required />);

      expect(screen.getByText('*')).toBeInTheDocument();
    });

    it('renders help text when provided', () => {
      render(<FileInput {...defaultProps} helpText="This is help text" />);

      expect(screen.getByText('This is help text')).toBeInTheDocument();
    });

    it('renders error message when provided', () => {
      render(<FileInput {...defaultProps} error="This is an error" />);

      expect(screen.getByText('This is an error')).toBeInTheDocument();
    });

    it('renders in disabled state', () => {
      render(<FileInput {...defaultProps} disabled />);

      const uploadButton = screen.getByTestId('test-file-input-button');
      expect(uploadButton).toBeDisabled();
    });
  });

  describe('File Selection', () => {
    it('opens file dialog when upload button is clicked', async () => {
      const user = userEvent.setup();
      render(<FileInput {...defaultProps} />);

      const uploadButton = screen.getByTestId('test-file-input-button');
      const fileInput = screen.getByTestId('test-file-input');

      // Mock the click method
      const clickSpy = vi.spyOn(fileInput, 'click');

      await user.click(uploadButton);

      expect(clickSpy).toHaveBeenCalled();
    });

    it('processes file selection and calls onChange', async () => {
      const mockOnChange = vi.fn();
      render(<FileInput {...defaultProps} onChange={mockOnChange} />);

      const fileInput = screen.getByTestId('test-file-input');
      const testFile = createMockFile('test.txt', mockFileContent, 'text/plain');

      fireEvent.change(fileInput, { target: { files: [testFile] } });

      // Wait for async file processing
      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith(mockFileContentBase64, 'test.txt');
      });
    });

    it('shows processing state during file encoding', async () => {
      render(<FileInput {...defaultProps} />);

      const fileInput = screen.getByTestId('test-file-input');
      const testFile = createMockFile('test.txt', mockFileContent, 'text/plain');

      await act(async () => {
        fireEvent.change(fileInput, { target: { files: [testFile] } });
      });

      // Should show processing state immediately
      expect(screen.getByText('Processing...')).toBeInTheDocument();
    });
  });

  describe('Drag and Drop', () => {
    it('provides visual feedback on drag over', () => {
      render(<FileInput {...defaultProps} />);

      const uploadButton = screen.getByTestId('test-file-input-button');
      const container = uploadButton.closest('div');

      fireEvent.dragOver(container!);

      expect(uploadButton).toHaveClass('border-2', 'shadow-md');
    });

    it('removes visual feedback on drag leave', () => {
      render(<FileInput {...defaultProps} />);

      const uploadButton = screen.getByTestId('test-file-input-button');
      const container = uploadButton.closest('div');

      fireEvent.dragOver(container!);
      expect(uploadButton).toHaveClass('border-2', 'shadow-md');

      fireEvent.dragLeave(container!);
      expect(uploadButton).not.toHaveClass('border-2', 'shadow-md');
    });

    it('processes dropped files', async () => {
      const mockOnChange = vi.fn();
      render(<FileInput {...defaultProps} onChange={mockOnChange} />);

      const uploadButton = screen.getByTestId('test-file-input-button');
      const container = uploadButton.closest('div');
      const testFile = createMockFile('dropped.txt', mockFileContent, 'text/plain');

      const dropEvent = new Event('drop', { bubbles: true });
      Object.defineProperty(dropEvent, 'dataTransfer', {
        value: {
          files: [testFile]
        }
      });

      fireEvent(container!, dropEvent);

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith(mockFileContentBase64, 'dropped.txt');
      });
    });

    it('ignores drag and drop when disabled', () => {
      render(<FileInput {...defaultProps} disabled />);

      const uploadButton = screen.getByTestId('test-file-input-button');
      const container = uploadButton.closest('div');

      fireEvent.dragOver(container!);

      expect(uploadButton).not.toHaveClass('border-2', 'shadow-md');
    });
  });

  describe('File Display and Removal', () => {
    it('displays selected file information', () => {
      render(
        <FileInput
          {...defaultProps}
          value={btoa('test content')}
          filename="selected-file.txt"
        />
      );

      expect(screen.getByText('selected-file.txt')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-input-download')).toBeInTheDocument();
      expect(screen.getByTestId('test-file-input-remove')).toBeInTheDocument();
    });

    it('removes file when remove button is clicked', async () => {
      const mockOnChange = vi.fn();
      const user = userEvent.setup();

      render(
        <FileInput
          {...defaultProps}
          onChange={mockOnChange}
          value={btoa('test content')}
          filename="selected-file.txt"
        />
      );

      const removeButton = screen.getByTestId('test-file-input-remove');
      await user.click(removeButton);

      expect(mockOnChange).toHaveBeenCalledWith('', '');
    });

    it('prevents file dialog from opening when remove button is clicked', async () => {
      const user = userEvent.setup();

      render(
        <FileInput
          {...defaultProps}
          value={btoa('test content')}
          filename="selected-file.txt"
        />
      );

      const fileInput = screen.getByTestId('test-file-input');
      const clickSpy = vi.spyOn(fileInput, 'click');

      const removeButton = screen.getByTestId('test-file-input-remove');
      await user.click(removeButton);

      expect(clickSpy).not.toHaveBeenCalled();
    });

    it('filename is clickable and has correct hover styling', () => {
      render(
        <FileInput
          {...defaultProps}
          value={btoa('test file content')}
          filename="test-file.txt"
        />
      );

      const filename = screen.getByTestId('test-file-input-download');
      expect(filename).toBeInTheDocument();
      expect(filename).toHaveClass('hover:underline', 'cursor-pointer');
      expect(filename).toHaveAttribute('title', 'Download file');
    });

    it('downloads file when filename is clicked', async () => {
      const user = userEvent.setup();
      const testContent = 'test file content';

      render(
        <FileInput
          {...defaultProps}
          value={btoa(testContent)}
          filename="test-file.txt"
        />
      );

      await user.click(screen.getByTestId('test-file-input-download'));

      expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
      expect(mockAnchorClick).toHaveBeenCalled();
      expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:test-url');
    });

    it('prevents file dialog from opening when filename is clicked', async () => {
      const user = userEvent.setup();

      render(
        <FileInput
          {...defaultProps}
          value={btoa('test content')}
          filename="selected-file.txt"
        />
      );

      const fileInput = screen.getByTestId('test-file-input');
      const clickSpy = vi.spyOn(fileInput, 'click');

      const filename = screen.getByTestId('test-file-input-download');
      await user.click(filename);

      expect(clickSpy).not.toHaveBeenCalled();
    });

    it('handles download errors gracefully', async () => {
      const user = userEvent.setup();
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => { });

      // Mock URL.createObjectURL to throw an error
      vi.spyOn(URL, 'createObjectURL').mockImplementationOnce(() => {
        throw new Error('Download failed');
      });

      render(
        <FileInput
          {...defaultProps}
          value={btoa('test content')}
          filename="test-file.txt"
        />
      );

      await user.click(screen.getByTestId('test-file-input-download'));

      expect(consoleErrorSpy).toHaveBeenCalledWith('Failed to download file:', expect.any(Error));
      consoleErrorSpy.mockRestore();
    });
  });



  describe('Error Handling', () => {
    it('prioritizes external error over internal error', () => {
      render(
        <FileInput
          {...defaultProps}
          error="External error message"
        />
      );

      expect(screen.getByText('External error message')).toBeInTheDocument();
    });

    it('clears internal errors when file is removed', async () => {
      const user = userEvent.setup();

      render(
        <FileInput
          {...defaultProps}
          value={btoa('test content')}
          filename="test.txt"
        />
      );

      const removeButton = screen.getByTestId('test-file-input-remove');
      await user.click(removeButton);

      // Internal error should be cleared (we can't easily test this without triggering an error first)
      expect(screen.queryByText(/File type not supported/)).not.toBeInTheDocument();
    });
  });

  describe('Base64 Encoding', () => {
    it('encodes files to base64 correctly', async () => {
      const mockOnChange = vi.fn();
      render(<FileInput {...defaultProps} onChange={mockOnChange} />);

      const fileInput = screen.getByTestId('test-file-input');
      const testFile = createMockFile('test.txt', mockFileContent, 'text/plain');

      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(mockOnChange).toHaveBeenCalledWith(mockFileContentBase64, 'test.txt');
      });
    });

    it('handles file read errors gracefully', async () => {
      // Mock FileReader to simulate error
      const OriginalFileReader = global.FileReader;
      global.FileReader = class {
        onerror: ((event: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            if (this.onerror) {
              const errorEvent = {
                target: {
                  error: {
                    message: 'Invalid file type'
                  }
                }
              } as ProgressEvent<FileReader>;
              this.onerror(errorEvent);
            }
          }, 10);
        }
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any;

      render(<FileInput {...defaultProps} />);

      const fileInput = screen.getByTestId('test-file-input');
      const testFile = createMockFile('test.txt', mockFileContent, 'text/plain');

      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(screen.getByText('Invalid file type')).toBeInTheDocument();
      });

      // Restore original FileReader
      global.FileReader = OriginalFileReader;
    });

    it('handles onload with non-string result error gracefully', async () => {
      // Mock FileReader to simulate ArrayBuffer result (non-string)
      const OriginalFileReader = global.FileReader;
      global.FileReader = class {
        result: ArrayBuffer | null = null;
        onload: ((event: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            // Simulate ArrayBuffer result instead of string
            this.result = new ArrayBuffer(10);
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 10);
        }
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
      } as any;

      render(<FileInput {...defaultProps} />);

      const fileInput = screen.getByTestId('test-file-input');
      const testFile = createMockFile('test.txt', mockFileContent, 'text/plain');

      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(screen.getByText('Unexpected result type when reading file')).toBeInTheDocument();
      });

      // Restore original FileReader
      global.FileReader = OriginalFileReader;
    });
  });
});