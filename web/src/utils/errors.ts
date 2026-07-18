type ErrorPayload = {
  message?: unknown;
  error?: unknown;
};

type RequestLikeError = {
  data?: ErrorPayload;
  info?: { data?: ErrorPayload };
  response?: { data?: ErrorPayload };
  message?: unknown;
};

function firstString(values: unknown[]): string | undefined {
  return values.find((item): item is string => typeof item === 'string' && item.trim().length > 0);
}

export function extractErrorMessage(error: unknown, fallback: string): string {
  if (!error || typeof error !== 'object') return fallback;
  const err = error as RequestLikeError;
  return (
    firstString([
      err.data?.message,
      err.info?.data?.message,
      err.response?.data?.message,
      err.response?.data?.error,
      err.message,
    ]) || fallback
  );
}
