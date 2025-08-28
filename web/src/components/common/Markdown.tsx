import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PluggableList } from 'react-markdown/lib/react-markdown';

interface MarkdownProps {
  children: string;
  className?: string;
  rehypePlugins?: PluggableList;
}

const Markdown: React.FC<MarkdownProps> = ({ children, rehypePlugins = [] }) => {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={rehypePlugins}
      components={{
        a: ({ ...props }) => (
          <a
            {...props}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-600 hover:text-blue-800 underline"
          />
        ),
        // We use:
        // - pre selectors to style code blocks and inline code blocks differently. This is becasuse code blocks are rendered with a <pre><code>...</code></pre> structure
        // - before:content-none and after:content-none to remove the default backticks that are added by the Label component through the prose class
        code: ({ children }) => (
          <code className="font-mono text-xs bg-gray-100 px-1 py-0.5 rounded [pre_&]:text-sm [pre_&]:bg-transparent [pre_&]:px-0 [pre_&]:py-0 before:content-none after:content-none">
            {children}
          </code>
        ),
      }}
    >
      {children}
    </ReactMarkdown>
  );
};

export default Markdown;
