/**
 * Runtime configuration from Vite env.
 *
 * VITE_API_MOCKS — "true" serves /api/v1 from local fixtures (default in dev
 * until the BFF exists), "false" calls the real gateway.
 */
const mocksFlag = import.meta.env.VITE_API_MOCKS as string | undefined;

export const env = {
  apiBaseUrl: (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1',
  useMocks: mocksFlag ? mocksFlag === 'true' : import.meta.env.DEV || import.meta.env.MODE === 'test',
} as const;
