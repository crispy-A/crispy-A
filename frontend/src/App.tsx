import { useEffect, useState } from 'react'
import { getHealth } from './api'

type ApiStatus = 'checking' | 'online' | 'degraded' | 'offline'

const statusText: Record<ApiStatus, string> = {
  checking: '正在连接 API',
  online: 'API 在线',
  degraded: '数据库异常',
  offline: 'API 离线',
}

export default function App() {
  const [status, setStatus] = useState<ApiStatus>('checking')

  useEffect(() => {
    const controller = new AbortController()
    const timeout = window.setTimeout(() => controller.abort(), 4000)

    getHealth(controller.signal)
      .then((health) => setStatus(health.database === 'ok' ? 'online' : 'degraded'))
      .catch(() => setStatus('offline'))
      .finally(() => window.clearTimeout(timeout))

    return () => {
      window.clearTimeout(timeout)
      controller.abort()
    }
  }, [])

  return (
    <main className="stage">
      <div className="grain" aria-hidden="true" />

      <header className="topbar">
        <span className="wordmark">CRISPY</span>
        <span className="meta">GO · REACT · POSTGRES</span>
      </header>

      <section className="hero" aria-labelledby="hero-title">
        <p className="kicker">Personal site / est. 2026</p>
        <h1 id="hero-title" className="monogram">
          <span className="monogram-ghost" aria-hidden="true">
            A
          </span>
          <span className="monogram-face">A</span>
        </h1>
        <p className="tagline">一个留白待写的个人网站脚手架</p>
      </section>

      <footer className="footer">
        <p className={`status status-${status}`}>
          <span className="status-dot" aria-hidden="true" />
          {statusText[status]}
        </p>
        <p className="version">v0.1.0</p>
      </footer>
    </main>
  )
}
