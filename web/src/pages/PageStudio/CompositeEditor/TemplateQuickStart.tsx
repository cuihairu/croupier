import React, { useEffect, useState } from 'react';
import { Card, Space, Spin, Tag, Typography } from 'antd';
import { AppstoreOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { request } from '@umijs/max';
import { instantiateTemplate, type ComponentTemplateDTO } from './ComponentLibrary';
import { schemaProperties } from './types';
import type { PageNode } from './model';

const { Text, Title } = Typography;

type QuickStartTemplate = ComponentTemplateDTO & { tree?: PageNode[] };

/**
 * 从模板开始（空白画布引导）：列出多节点组合模板，点击即实例化为
 * 初始页面（替代空白起步）。单节点模板不出现（组合页需 ≥2 区块）。
 */
export default function TemplateQuickStart({
  templates,
  onPick,
}: {
  /** 外部已加载的模板（测试/复用）；未提供则自行拉取。 */
  templates?: QuickStartTemplate[];
  onPick: (nodes: PageNode[], tpl: QuickStartTemplate) => void;
}) {
  const [fetched, setFetched] = useState<QuickStartTemplate[] | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (templates) return;
    setLoading(true);
    request('/api/v1/component-templates', { skipErrorHandler: true })
      .then((resp) => {
        const items = Array.isArray(resp)
          ? resp
          : ((resp as { items?: QuickStartTemplate[] })?.items ?? []);
        setFetched(items);
      })
      .catch(() => setFetched([]))
      .finally(() => setLoading(false));
  }, [templates]);

  const list = templates ?? fetched ?? [];
  const combos = list.filter((t) => (t.tree?.length ?? 0) >= 2);

  return (
    <Card size="small">
      <Space size={6} style={{ marginBottom: 8 }}>
        <ThunderboltOutlined style={{ color: '#1677ff' }} />
        <Title level={5} style={{ margin: 0 }}>
          从模板开始
        </Title>
        <Text type="secondary" style={{ fontSize: 12 }}>
          选择一个组合模板作为页面起点，之后可继续拖入积木微调
        </Text>
      </Space>
      {loading ? (
        <Spin size="small" />
      ) : combos.length === 0 ? (
        <Text type="secondary" style={{ fontSize: 12 }}>
          暂无组合模板——可先到「组件模板」页从契约重新生成，或直接从左侧面板拖入组件
        </Text>
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
            gap: 8,
          }}
        >
          {combos.map((tpl) => {
            const name = (tpl.name as Record<string, string>)?.['zh-CN'] ?? tpl.key;
            const desc = (tpl.description as Record<string, string>)?.['zh-CN'] ?? '';
            const fns = tpl.requiredFunctions ?? [];
            return (
              <Card
                key={tpl.key}
                size="small"
                hoverable
                style={{ borderColor: '#f0f0f0' }}
                onClick={() => onPick(nodesOf(tpl), tpl)}
              >
                <Space size={6}>
                  <AppstoreOutlined style={{ color: '#1677ff' }} />
                  <Text strong style={{ fontSize: 12 }}>
                    {name}
                  </Text>
                  {tpl.builtin && <Tag style={{ marginRight: 0, fontSize: 10 }}>内置</Tag>}
                </Space>
                <div>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {desc || `${tpl.tree?.length ?? 0} 个区块`}
                  </Text>
                </div>
                {fns.length > 0 && (
                  <div style={{ marginTop: 4 }}>
                    {fns.slice(0, 3).map((f) => (
                      <Tag key={f} style={{ fontSize: 10, marginRight: 2 }}>
                        {f}
                      </Tag>
                    ))}
                    {fns.length > 3 && (
                      <Text type="secondary" style={{ fontSize: 10 }}>
                        +{fns.length - 3}
                      </Text>
                    )}
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </Card>
  );
}

function nodesOf(tpl: QuickStartTemplate): PageNode[] {
  return (tpl.tree ?? []) as PageNode[];
}
