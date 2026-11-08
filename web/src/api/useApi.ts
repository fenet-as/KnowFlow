import { useAuth } from '@clerk/clerk-react';
import { useMemo } from 'react';
import { ApiClient } from './client';

/**
 * useApi hook provides a memoized instance of ApiClient with Clerk JWT injection.
 */
export function useApi(): ApiClient {
  const { getToken } = useAuth();

  return useMemo(() => {
    return new ApiClient({
      getToken: async () => {
        try {
          return await getToken();
        } catch (err) {
          console.warn('Failed to retrieve Clerk JWT', err);
          return null;
        }
      },
    });
  }, [getToken]);
}

