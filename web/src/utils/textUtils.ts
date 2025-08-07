/**
 * Creates a rehype plugin that truncates markdown text content to a specified maximum length.
 *
 * Features:
 * - Truncates text at the AST node level for proper markdown handling
 * - Preserves markdown structure while limiting text content
 * - Adds ellipsis ("...") when text is truncated
 * - Removes child nodes that exceed the limit to prevent partial content
 *
 * @param maxTextLength - Maximum number of characters allowed before truncation
 * @returns A rehype plugin function that can be used with ReactMarkdown
 *
 * @example
 * ```typescript
 * // Use with ReactMarkdown
 * <ReactMarkdown
 *   rehypePlugins={[[truncate, 100]]}
 * >
 *   {longText}
 * </ReactMarkdown>
 * ```
 */
export function truncate(maxTextLength: number): (tree: any) => void {
  const truncateNode = (node: any, textLength: number): number => {
    if (node.type === "text") {
      const newLength = textLength + node.value.length;
      if (newLength >= maxTextLength) {
        const excess = newLength - maxTextLength;
        node.value = node.value.slice(0, -excess) + '...';
        return maxTextLength;
      }
      return newLength;
    }

    if ((node.type === "root" || node.type === "element") && node.children) {
      const children = node.children;
      for (let i = 0; i < children.length; i++) {
        if (textLength >= maxTextLength) {
          children.length = i;
          break;
        }
        textLength = truncateNode(children[i], textLength);
      }
    }

    return textLength;
  };

  return (tree: any) => {
    truncateNode(tree, 0);
  };
}
