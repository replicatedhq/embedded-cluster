import React, { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { truncate } from '../../utils/textUtils';

interface HelpTextProps {
  dataTestId?: string;
  helpText?: string;
  defaultValue?: string;
  error?: string;
}

const HelpText: React.FC<HelpTextProps> = ({ dataTestId, helpText, defaultValue, error }) => {
  const [showFullText, setShowFullText] = useState(false);
  const maxTextLength = 80;

  // The truncation threshold prevents cutting off text that's only slightly over the max length as
  // it would be preferable to display the full text than show/hide a small number of characters.
  const truncationThreshold = 40;

  if ((!helpText && !defaultValue) || error) return null;

  // Build the combined text
  let combinedText = helpText || '';
  if (defaultValue) {
    const defaultText = `(Default: \`${defaultValue}\`)`;
    combinedText = helpText ? `${helpText} ${defaultText}` : defaultText;
  }

  const exceedsMaxLength = combinedText.length > maxTextLength;
  const withinThreshold = (combinedText.length - maxTextLength) <= truncationThreshold;
  const shouldTruncate = exceedsMaxLength && !withinThreshold;

  return (
    <div data-testid={dataTestId ? `help-text-${dataTestId}` : "help-text"} className="mt-1 text-sm text-gray-500 [&_p]:inline [&_p]:mb-0">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={shouldTruncate && !showFullText ? [[truncate, maxTextLength]] : []}
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
      {shouldTruncate && (
        <button
          onClick={() => setShowFullText(!showFullText)}
          className="ml-1 text-blue-600 hover:text-blue-800 text-xs cursor-pointer"
          type="button"
        >
          {showFullText ? 'Show less' : 'Show more'}
        </button>
      )}
    </div>
  );
};

export default HelpText;
