// isFetchError checks if the error is a fetch error with a status property
function isFetchError(error: unknown): error is { status: number } {
  return (
    typeof error === "object" &&
    error !== null &&
    "status" in error &&
    typeof error.status === "number"
  );
}

// isAPIBodyError checks if the error is a response from our API with a statusCode property
function isAPIBodyError(error: unknown): error is { statusCode: number } {
  return (
    typeof error === "object" &&
    error !== null &&
    "statusCode" in error &&
    typeof error.statusCode === "number"
  );
}

/**
 * Handle unauthorized errors (HTTP 401).
 * Clears auth state and reloads the page to reset the application state.
 * @param error The error object to check.
 * @returns true if the error was handled, false otherwise.
 */

export const handleUnauthorized = (error: unknown) => {
  // Check if it's a fetch error with a response
  let status = 200;
  if (isAPIBodyError(error)) {
    status = error.statusCode;
  } else if (isFetchError(error)) {
    status = error.status;
  }

  if (status === 401) {
    // Get auth context from localStorage since we can't use hooks in regular functions
    localStorage.removeItem("auth");
    // Clear session storage to reset installation progress
    sessionStorage.clear();
    // Force reload the page to reset all auth state
    window.location.reload();
    return true;
  }
  return false;
};

