// @ts-strict
import {  QueryClient, type QueryClientConfig } from "@tanstack/react-query";
import { handleUnauthorized } from "./utils/auth";


// Export the client factory so we can reuse
// the same config in tests, which create their own client
export const createQueryClient = (defaultOptions?: QueryClientConfig) => {
  return new QueryClient(defaultOptions || {
    defaultOptions: {
      queries: {
        retry: (failureCount, error) => {
          // Don't retry on 401 unauthorized
          if (handleUnauthorized(error)) {
            return false;
          }
          // Otherwise retry 3 times
          return failureCount < 3;
        },
        refetchOnWindowFocus: false,
        refetchOnReconnect: true,
       
      },
      mutations: {
        retry: (failureCount, error) => {
          // Don't retry on 401 unauthorized
          if (handleUnauthorized(error)) {
            return false;
          }
          // Otherwise retry once
          return failureCount < 1;
        },
      },
    },
  });
};

// react-query client as a singleton, primarily to clear the cache from redux
// logout action. This is the client we use throughout the entire app.
const client = createQueryClient();

export const getQueryClient = () => client;
