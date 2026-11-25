export type Health = {
  status: 'ok' | 'degraded'
  database: 'ok' | 'down'
  version: string
  time: string
}

export async function getHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch('/api/health', {
    headers: { Accept: 'application/json' },
    signal,
  })
  if (!response.ok) {
    throw new Error(`health check failed with status ${response.status}`)
  }
  return (await response.json()) as Health
}
