import { apiClient, ApiClient } from './client';

/**
 * useApi hook provides the initialized ApiClient with native authentication session.
 */
export function useApi(): ApiClient {
  return apiClient;
}
