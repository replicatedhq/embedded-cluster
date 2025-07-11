import React from 'react';

interface ConfigItemProps {
  id: string;
  label: string;
  dataTestId?: string;
  helpText?: string;
  children: React.ReactElement;
}

/**
 * A wrapper component that provides consistent styling and layout for configuration form elements
 *
 * Props:
 * @param {string} id - Unique identifier for the form element
 * @param {string} label - Label text to display above the input
 * @param {string} [dataTestId] - Optional test ID for e2e testing
 * @param {string} [helpText] - Optional help text displayed below the input
 * @param {React.ReactElement} children - The form input component to wrap
 *
 * The component clones the child element and injects common props (id, label, helpText)
 * to ensure consistent behavior across different input types.
 *
 * Example:
 * ```tsx
 * <ConfigItem
 *   id="hostname"
 *   label="Hostname"
 *   helpText="Enter the server hostname"
 * >
 *   <Input />
 * </ConfigItem>
 * ```
 */
const ConfigItem: React.FC<ConfigItemProps> = ({
  id,
  label,
  helpText,
  children,
}) => {
  // Clone the child element and inject the common props
  const enhancedChild = React.cloneElement(children, {
    id,
    label,
    helpText,
  } as React.HTMLAttributes<HTMLElement>);

  return (
    <div key={id} data-testid={`config-item-${id}`}>
      <div className="mb-4">
        {enhancedChild}
      </div>
    </div>
  );
};


export default ConfigItem;
