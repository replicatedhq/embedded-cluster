
export const handleUnauthorized = (error: unknown) => {
  // Check if it's a fetch error with a response
  if (error instanceof Error && 'status' in error) {
    const status = (error as { status: number }).status;
    if (status === 401) {
      // Get auth context from localStorage since we can't use hooks in regular functions
      localStorage.removeItem("auth");
      // Force reload the page to reset all auth state
      window.location.reload();
      return true;
    }
  }
  return false;
}; 