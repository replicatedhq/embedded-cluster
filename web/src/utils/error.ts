// Special cases for acronyms that need specific formatting
const specialCases: Record<string, string> = {
  cidr: "CIDR",
  https: "HTTPS",
  http: "HTTP",
};

// isCamelCase checks if a word follows camelCase pattern (starts lowercase, has uppercase)
function isCamelCase(word: string): boolean {
  if (!word || !word[0]?.match(/[a-z]/)) {
    return false;
  }
  return Array.from(word.slice(1)).some((char: string) => char.match(/[A-Z]/));
}

// splitCamelCase splits a camelCase word into parts
function splitCamelCase(word: string): string[] {
  const parts: string[] = [];
  let current = "";

  for (let i = 0; i < word.length; i++) {
    const char = word[i];
    if (i > 0 && char.match(/[A-Z]/)) {
      // If previous char was lowercase, start new part
      if (word[i - 1].match(/[a-z]/)) {
        parts.push(current);
        current = "";
      } else if (i + 1 < word.length && word[i + 1].match(/[a-z]/)) {
        // If next char is lowercase, start new part (end of acronym)
        if (current) {
          parts.push(current);
          current = "";
        }
      }
    }
    current += char;
  }

  if (current) {
    parts.push(current);
  }

  return parts;
}

// transformWord converts a camelCase word to Title Case
function transformWord(word: string): string {
  // Check for special cases first
  const lowerWord = word.toLowerCase();
  if (specialCases[lowerWord]) {
    return specialCases[lowerWord];
  }

  // If not camelCase, return as is
  if (!isCamelCase(word)) {
    return word;
  }

  // Split camelCase word into parts
  const parts = splitCamelCase(word);

  // Process each part
  return parts
    .map(part => {
      const lowerPart = part.toLowerCase();
      return specialCases[lowerPart] || part.charAt(0).toUpperCase() + part.slice(1).toLowerCase();
    })
    .join(" ");
}

/**
 * formatErrorMessage converts camelCase words in an error message to Title Case.
 * It preserves non-camelCase words and handles special acronyms.
 *
 * @example
 * formatErrorMessage("podCidr is required") // "Pod CIDR is required"
 * formatErrorMessage("httpProxy and httpsProxy") // "HTTP Proxy and HTTPS Proxy
 */
export function formatErrorMessage(message: string): string {
  if (!message) return "";

  return message
    .split(" ")
    .map(word => transformWord(word))
    .join(" ");
}