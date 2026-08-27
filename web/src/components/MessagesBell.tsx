import React, { useEffect, useState } from 'react';
import { Badge } from 'antd';
import { BellOutlined } from '@ant-design/icons';
import { unreadCount } from '@/services/api';
import { history } from '@umijs/max';

export default function MessagesBell() {
  const [count, setCount] = useState(0);
  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    const hasToken = () => !!(localStorage.getItem('token') || '');
    const poll = async () => {
      if (!hasToken()) return; // don't call API before login
      try {
        const r = await unreadCount();
        if (alive) setCount(Number(r.count || 0));
      } catch {}
    };
    // prime once
    poll();
    // periodic poll：未读数是弱实时信息，5 分钟一轮足够；
    // 页签不可见时跳过，避免后台页签空打请求。
    const loop = async () => {
      if (!document.hidden) {
        await poll();
      }
      timer = setTimeout(loop, 5 * 60 * 1000);
    };
    loop();
    return () => {
      alive = false;
      if (timer) clearTimeout(timer);
    };
  }, []);
  return (
    <span
      onClick={() => history.push('/admin/account/messages')}
      style={{ cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
    >
      <Badge count={count} size="small">
        <BellOutlined style={{ fontSize: 18 }} />
      </Badge>
    </span>
  );
}
