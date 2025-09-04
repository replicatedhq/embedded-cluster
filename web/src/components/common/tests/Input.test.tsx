import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import Input from '../Input';

// Mock the SettingsContext
const mockThemeColor = '#3498DB';
vi.mock('../../../contexts/SettingsContext', () => ({
  useSettings: () => ({
    settings: {
      themeColor: mockThemeColor
    }
  })
}));

describe('Input', () => {
  const defaultProps = {
    id: 'test-input',
    label: 'Test Input',
    value: '',
    dataTestId: 'test-input',
    onChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Basic Rendering', () => {
    it('renders with basic props', () => {
      render(<Input {...defaultProps} />);

      expect(screen.getByLabelText('Test Input')).toBeInTheDocument();
      expect(screen.getByTestId('test-input')).toBeInTheDocument();
    });

    it('renders with required indicator when required is true', () => {
      render(<Input {...defaultProps} required />);

      expect(screen.getByText('*')).toBeInTheDocument();
    });

    it('renders help text when provided', () => {
      render(<Input {...defaultProps} helpText="This is help text" />);

      expect(screen.getByText('This is help text')).toBeInTheDocument();
    });

    it('renders error message when provided', () => {
      render(<Input {...defaultProps} error="This is an error" />);

      expect(screen.getByText('This is an error')).toBeInTheDocument();
    });

    it('renders with icon when provided', () => {
      const iconTestId = 'test-icon';
      const TestIcon = () => <div data-testid={iconTestId}>Icon</div>;

      render(<Input {...defaultProps} icon={<TestIcon />} />);

      expect(screen.getByTestId(iconTestId)).toBeInTheDocument();
    });

    it('applies custom className', () => {
      render(<Input {...defaultProps} className="custom-class" />);

      const container = screen.getByLabelText('Test Input').closest('.custom-class');
      expect(container).toBeInTheDocument();
    });

    it('applies custom labelClassName', () => {
      render(<Input {...defaultProps} labelClassName="custom-label-class" />);

      const label = screen.getByText('Test Input');
      expect(label).toHaveClass('custom-label-class');
    });

    it('renders with data-testid when provided', () => {
      render(<Input {...defaultProps} dataTestId="custom-test-id" />);

      expect(screen.getByTestId('custom-test-id')).toBeInTheDocument();
    });
  });

  describe('Input States', () => {
    it('renders in disabled state', () => {
      render(<Input {...defaultProps} disabled />);

      const input = screen.getByTestId('test-input');
      expect(input).toBeDisabled();
      expect(input).toHaveClass('bg-gray-100', 'text-gray-500');
    });

    it('applies error styling when error is provided', () => {
      render(<Input {...defaultProps} error="Error message" />);

      const input = screen.getByTestId('test-input');
      expect(input).toHaveClass('border-red-500');
    });

    it('applies normal styling when no error', () => {
      render(<Input {...defaultProps} />);

      const input = screen.getByTestId('test-input');
      expect(input).toHaveClass('border-gray-300');
    });
  });

  describe('Password Visibility Toggle', () => {
    it('renders password field with eye icon when type is password', () => {
      render(<Input {...defaultProps} type="password" />);

      const input = screen.getByLabelText('Test Input');
      const toggleButton = screen.getByTestId('password-visibility-toggle-test-input');

      expect(input).toHaveAttribute('type', 'password');
      expect(toggleButton).toBeInTheDocument();
    });

    it('does not render password toggle button for non-password fields', () => {
      render(<Input {...defaultProps} type="text" />);

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('does not render password toggle when allowShowPassword is false', () => {
      render(<Input {...defaultProps} type="password" allowShowPassword={false} />);

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('shows Eye icon initially (password hidden)', () => {
      render(<Input {...defaultProps} type="password" />);

      expect(screen.getByTestId('password-visibility-toggle-test-input')).toBeInTheDocument();
      const eyeIcon = screen.getByTestId('eye-icon-test-input');
      expect(eyeIcon).toBeInTheDocument();
    });

    it('toggles password visibility when toggle button is clicked', async () => {
      const user = userEvent.setup();
      render(<Input {...defaultProps} type="password" />);

      const input = screen.getByLabelText('Test Input');
      const toggleButton = screen.getByTestId('password-visibility-toggle-test-input');

      // Initially password type
      expect(input).toHaveAttribute('type', 'password');
      expect(screen.getByTestId('eye-icon-test-input')).toBeInTheDocument();

      // Click to show password
      await user.click(toggleButton);
      expect(input).toHaveAttribute('type', 'text');
      expect(screen.getByTestId('eye-off-icon-test-input')).toBeInTheDocument();

      // Click to hide password again
      await user.click(toggleButton);
      expect(input).toHaveAttribute('type', 'password');
      expect(screen.getByTestId('eye-icon-test-input')).toBeInTheDocument();
    });

    it('toggle button does not interfere with tab navigation', () => {
      render(<Input {...defaultProps} type="password" />);

      const toggleButton = screen.getByTestId('password-visibility-toggle-test-input');
      expect(toggleButton).toHaveAttribute('tabIndex', '-1');
    });

    it('applies hover styles to toggle button', () => {
      render(<Input {...defaultProps} type="password" />);

      const toggleButton = screen.getByTestId('password-visibility-toggle-test-input');
      expect(toggleButton).toHaveClass('hover:text-gray-600', 'transition-colors');
    });

  });

  describe('Input Interactions', () => {
    it('calls onChange when input value changes', async () => {
      const mockOnChange = vi.fn();
      const user = userEvent.setup();

      render(<Input {...defaultProps} onChange={mockOnChange} />);

      const input = screen.getByTestId('test-input');
      await user.type(input, 'test');

      expect(mockOnChange).toHaveBeenCalled();
    });

    it('calls onKeyDown when key is pressed', async () => {
      const mockOnKeyDown = vi.fn();
      const user = userEvent.setup();

      render(<Input {...defaultProps} onKeyDown={mockOnKeyDown} />);

      const input = screen.getByTestId('test-input');
      await user.type(input, 'a');

      expect(mockOnKeyDown).toHaveBeenCalled();
    });

    it('calls onFocus when input is focused', async () => {
      const mockOnFocus = vi.fn();
      const user = userEvent.setup();

      render(<Input {...defaultProps} onFocus={mockOnFocus} />);

      const input = screen.getByTestId('test-input');
      await user.click(input);

      expect(mockOnFocus).toHaveBeenCalled();
    });

    it('displays placeholder text', () => {
      render(<Input {...defaultProps} placeholder="Enter text here" />);

      expect(screen.getByTestId('test-input')).toHaveAttribute('placeholder', 'Enter text here');
    });

    it('displays current value', () => {
      render(<Input {...defaultProps} value="current value" />);

      expect(screen.getByTestId('test-input')).toHaveValue('current value');
    });
  });

  describe('Password Field with Complex Interactions', () => {
    it('works correctly with controlled input pattern', async () => {
      const TestComponent = () => {
        const [value, setValue] = useState('');

        return (
          <Input
            id="controlled-password"
            label="Password"
            type="password"
            value={value}
            dataTestId="controlled-input"
            onChange={(e) => setValue(e.target.value)}
          />
        );
      };

      const user = userEvent.setup();
      render(<TestComponent />);

      const input = screen.getByTestId('controlled-input');
      const internalToggle = screen.getByTestId('password-visibility-toggle-controlled-input');

      // Type password
      await user.type(input, 'mysecret');
      expect(input).toHaveValue('mysecret');
      expect(input).toHaveAttribute('type', 'password');

      // Toggle using internal button
      await user.click(internalToggle);
      expect(input).toHaveAttribute('type', 'text');
      expect(input).toHaveValue('mysecret');
    });


    it('handles password with defaultValue', () => {
      render(
        <Input
          {...defaultProps}
          type="password"
          defaultValue="defaultpass"
          value="actualvalue"
        />
      );

      const input = screen.getByLabelText('Test Input');
      expect(input).toHaveValue('actualvalue'); // value prop takes precedence
      expect(input).toHaveAttribute('type', 'password');
    });
  });

  describe('Theme Integration', () => {
    it('applies theme color to focus styles', () => {
      render(<Input {...defaultProps} />);

      const input = screen.getByTestId('test-input');
      const style = input.style;

      expect(style.getPropertyValue('--tw-ring-color')).toBe(mockThemeColor);
      expect(style.getPropertyValue('--tw-ring-offset-color')).toBe(mockThemeColor);
    });

    it('maintains theme styling with password field', () => {
      render(<Input {...defaultProps} type="password" />);

      const input = screen.getByTestId('test-input');
      const style = input.style;

      expect(style.getPropertyValue('--tw-ring-color')).toBe(mockThemeColor);
      expect(style.getPropertyValue('--tw-ring-offset-color')).toBe(mockThemeColor);
    });
  });
});
