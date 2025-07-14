import React from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface LabelProps {
  content: string;
  className?: string;
  dataTestId?: string;
}

const Label: React.FC<LabelProps> = ({
  content,
  className = '',
  dataTestId,
}) => {
  return (
    <div className={`mb-4 ${className}`} data-testid={dataTestId}>
      <div className="prose prose-sm prose-gray max-w-none">
        <Markdown
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
          }}
        >
          {content}
        </Markdown>
      </div>
    </div>
  );
};

export default Label;
