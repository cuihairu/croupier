import React, { useEffect, useState } from 'react';
import { Badge } from 'antd';
import { BellOutlined } from '@ant-design/icons';
import { openMessagesStream, unreadCount } from '@/services/croupier';
import { history } from '@umijs/max';

export default function MessagesBell() {
  const [count, setCount] = useState(0);
  useEffect(() => {
    let alive = true; let timer: any; let es: EventSource | null = null;
    const hasToken = () => !!(localStorage.getItem('token') || '');
    const poll = async () => {
      if (!hasToken()) return; // don't call API before login
      try { const r = await unreadCount(); if (alive) setCount(Number(r.count || 0)); } catch {}
    };
    const startSSE = () => {
      try {
        const token = localStorage.getItem('token') || '';
        if (!token) return; // wait until token available
        es = openMessagesStream();
        const onUnread = (ev: MessageEvent) => {
          try { const data = JSON.parse(ev.data || '{}'); if (typeof data.count === 'number') setCount(data.count); } catch {}
        };
        es.addEventListener('unread', onUnread as any);
        es.onerror = () => { es && es.close(); es = null; /* fallback to polling; will retry later */ };
      } catch {
        es = null;
      }
    };
    // prime once
    poll();
    startSSE();
    // periodic fallback poll
    const loop = async () => {
      await poll();
      if (!es) { startSSE(); }
      timer = setTimeout(loop, 30000);
    };
    loop();
    return () => { alive = false; if (timer) clearTimeout(timer); if (es) es.close(); };
  }, []);
  return (
    <span onClick={() => history.push('/admin/account/messages')} style={{ cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
      <Badge count={count} size="small">
        <BellOutlined style={{ fontSize: 18 }} />
      </Badge>
    </span>
  );
}
