import React from 'react';

interface ConfigItemProps {
  id: string;
  label: string;
  dataTestId?: string;
  helpText?: string;
  children: React.ReactElement;
}

const ConfigItem: React.FC<ConfigItemProps> = ({
  id,
  label,
  dataTestId,
  helpText,
  children,
}) => {
  // Clone the child element and inject the common props
  const enhancedChild = React.cloneElement(children, {
    id,
    label,
    dataTestId,
    helpText,
  } as any);

  return (
    <div className="mb-4">
      {enhancedChild}
    </div>
  );
};

// Helper function to create a ConfigItem-wrapped component
export const withConfigItem = <P extends object>(
  WrappedComponent: React.ComponentType<P>
) => {
  return React.forwardRef<any, P & Omit<ConfigItemProps, 'children'>>((props, ref) => {
    const { id, label, dataTestId, helpText, ...wrappedComponentProps } = props;
    return (
      <ConfigItem
        id={id}
        label={label}
        dataTestId={dataTestId}
        helpText={helpText}
      >
        <WrappedComponent {...(wrappedComponentProps as P)} ref={ref} />
      </ConfigItem>
    );
  });
};

export default ConfigItem;
