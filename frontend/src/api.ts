export type Health = {
  status: 'ok' | 'degraded'
  database: 'ok' | 'down'
  version: string
  time: string
}

// 503 仍带 JSON（数据库不可达），需要解析出来展示「数据库异常」；
// 只有网络失败或非健康检查响应才抛错，由调用方降级为「离线」。
export async function getHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch('/api/health', {
    headers: { Accept: 'application/json' },
    signal,
  })
  if (!response.ok && response.status !== 503) {
    throw new Error(`health check failed with status ${response.status}`)
  }

  const payload = (await response.json()) as Partial<Health>
  if (typeof payload.database !== 'string' || typeof payload.status !== 'string') {
    throw new Error('health check returned an unexpected payload')
  }
  return payload as Health
}
