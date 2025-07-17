import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface HelpTextProps {
  helpText?: string;
  defaultValue?: string;
  error?: string;
}

const HelpText: React.FC<HelpTextProps> = ({ helpText, defaultValue, error }) => {
  if ((!helpText && !defaultValue) || error) return null;

  // Build the combined text with markdown formatting
  let combinedText = helpText || '';
  if (defaultValue) {
    const defaultText = `(Default: \`${defaultValue}\`)`;
    combinedText = helpText ? `${helpText} ${defaultText}` : defaultText;
  }

  return (
    <div className="mt-1 text-sm text-gray-500 [&_p]:inline [&_p]:mb-0">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ ...props }) => (
            <a
              {...props}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:text-blue-800 underline"
            />
          ),
          code: ({ children }) => (
            <code className="font-mono text-xs bg-gray-100 px-1 py-0.5 rounded">
              {children}
            </code>
          ),
        }}
      >
        {combinedText}
      </ReactMarkdown>
    </div>
  );
};

export default HelpText;
