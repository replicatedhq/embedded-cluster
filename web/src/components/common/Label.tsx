import React from 'react';
import Markdown from './Markdown';

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
        <Markdown>
          {content}
        </Markdown>
      </div>
    </div>
  );
};

export default Label;
