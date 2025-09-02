import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PluggableList } from 'react-markdown/lib/react-markdown';

interface MarkdownProps {
  children: string;
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
        // We need to use not-prose here to prevent Tailwind Typography from styling the code block differently within a prose parent (e.g. Label)
        pre: ({ children }) => (
          <pre className="not-prose bg-gray-100 p-4 rounded overflow-x-auto">
            {children}
          </pre>
        ),
        // We need to use not-prose here to prevent Tailwind Typography from styling the code block differently within a prose parent (e.g. Label)
        // Specific selectors used for `pre` to make sure inline code styling does not conflict with code block styling since code blocks are rendered
        // within a pre element
        code: ({ children }) => (
          <code className="not-prose font-mono font-semibold text-xs text-gray-700 bg-gray-100 px-1 py-0.5 rounded [pre_&]:bg-transparent [pre_&]:px-0 [pre_&]:py-0">
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
