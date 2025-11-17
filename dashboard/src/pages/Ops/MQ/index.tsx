import React, { useEffect, useState } from 'react';
import { Card, Space, Button, Descriptions, Tag } from 'antd';
import { request } from '@umijs/max';

export default function OpsMQPage() {
  const [loading, setLoading] = useState(false);
  const [info, setInfo] = useState<any>({});
  const load = async () => {
    setLoading(true);
    try { const r = await request('/api/ops/mq'); setInfo(r||{}); } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const typ = String(info?.type||'');
  return (
    <div style={{ padding: 24 }}>
      <Card title="消息队列" extra={<Space><Button onClick={load} loading={loading}>刷新</Button></Space>}>
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="类型">{typ ? <Tag color={typ==='redis'?'green':typ==='kafka'?'blue':'default'}>{typ}</Tag> : '-'}</Descriptions.Item>
          {typ==='redis' && (
            <>
              <Descriptions.Item label="Redis URL">{info?.redis?.url || '-'}</Descriptions.Item>
              <Descriptions.Item label="Streams">
                events: <Tag>{info?.redis?.streams?.events||'-'}</Tag>
                payments: <Tag>{info?.redis?.streams?.payments||'-'}</Tag>
              </Descriptions.Item>
              {info?.lengths && (
                <Descriptions.Item label="长度">
                  {Object.entries(info.lengths).map(([k,v]:any)=> <span key={k}><Tag>{k}</Tag> {String(v)}&nbsp;&nbsp;</span>)}
                </Descriptions.Item>
              )}
              {Array.isArray(info?.groups) && info.groups.length>0 && (
                <Descriptions.Item label="消费者组">
                  <div>
                    {info.groups.map((g:any, idx:number)=> (
                      <div key={idx} style={{ marginBottom: 6 }}>
                        <Tag color="blue">{g.stream}</Tag> 组：<Tag>{g.name}</Tag> 消费者：{g.consumers} 待处理：{g.pending} Lag：{g.lag ?? '-'}
                      </div>
                    ))}
                  </div>
                </Descriptions.Item>
              )}
            </>
          )}
          {typ==='kafka' && (
            <>
              <Descriptions.Item label="Brokers">{info?.kafka?.brokers||'-'}</Descriptions.Item>
              <Descriptions.Item label="Topics">
                events: <Tag>{info?.kafka?.topics?.events||'-'}</Tag>
                payments: <Tag>{info?.kafka?.topics?.payments||'-'}</Tag>
              </Descriptions.Item>
            </>
          )}
        </Descriptions>
      </Card>
    </div>
  );
}
