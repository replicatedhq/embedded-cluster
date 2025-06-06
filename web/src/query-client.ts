// @ts-strict
import { QueryClient, type QueryClientConfig } from "@tanstack/react-query";

const shouldRetryAfterError = (error: Error) => {
  const message = error?.message;
  if (!message) {
    return false;
  }
  return !["404", "403", "401", "400", "forbidden", "not found"].some(
    (element) => message.toLowerCase().includes(element),
  );
};

// Export the client factory so we can reuse
// the same config in tests, which create their own client
export const createQueryClient = (defaultOptions?: QueryClientConfig) => {
  return new QueryClient(
    defaultOptions || {
      defaultOptions: {
        queries: {
          retry: (failureCount, error) => {
            return shouldRetryAfterError(error) ? failureCount < 3 : false;
          },
          refetchOnWindowFocus: false,
          refetchOnReconnect: true,
        },
      },
    },
  );
};

// react-query client as a singleton, primarily to clear the cache from redux
// logout action. This is the client we use throughout the entire app.
const client = createQueryClient();

export const getQueryClient = () => client;
