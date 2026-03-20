export async function fetchJson<TResponse>(
  input: string | URL,
  init: RequestInit
): Promise<TResponse> {
  const response = await fetch(input, init);
  const text = await response.text();
  const data = text.length > 0 ? safeJsonParse(text) : null;

  if (!response.ok) {
    const details = typeof data === 'object' && data ? JSON.stringify(data) : text;
    throw new Error(`HTTP ${response.status} ${response.statusText}: ${details}`);
  }

  return data as TResponse;
}

function safeJsonParse(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}
