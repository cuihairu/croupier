import React from 'react';
import { Modal, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ExtensionCatalogItem, ExtensionReleaseItem } from '@/services/api/extensions';

const { Text } = Typography;

/** 扩展目录详情弹窗：ID/描述/能力/可用版本。数据由页面拉取注入。 */
export default function CatalogDetailModal({
  open,
  loading,
  item,
  capabilities,
  releases,
  onClose,
}: {
  open: boolean;
  loading: boolean;
  item: ExtensionCatalogItem | undefined;
  capabilities: string[];
  releases: ExtensionReleaseItem[];
  onClose: () => void;
}) {
  const intl = useIntl();
  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      title={
        item?.displayName ||
        item?.name ||
        intl.formatMessage({
          id: 'pages.extensionsStore.detail.titleFallback',
          defaultMessage: '扩展详情',
        })
      }
      width={840}
    >
      <Space orientation="vertical" style={{ width: '100%' }}>
        {loading && (
          <Text type="secondary">
            <FormattedMessage
              id="pages.extensionsStore.detail.loading"
              defaultMessage="加载中..."
            />
          </Text>
        )}
        <div>
          <Text strong>ID: </Text>
          <Text>{item?.id || '-'}</Text>
        </div>
        <div>
          <Text strong>
            <FormattedMessage
              id="pages.extensionsStore.detail.descriptionLabel"
              defaultMessage="描述: "
            />
          </Text>
          <Text>{item?.summary || '-'}</Text>
        </div>
        <div>
          <Text strong>
            <FormattedMessage
              id="pages.extensionsStore.detail.capabilitiesLabel"
              defaultMessage="能力: "
            />
          </Text>
          <Space wrap>
            {(capabilities || []).map((cap) => (
              <Tag key={cap} color="blue">
                {cap}
              </Tag>
            ))}
            {!capabilities?.length && (
              <Text type="secondary">
                <FormattedMessage id="pages.extensionsStore.detail.none" defaultMessage="无" />
              </Text>
            )}
          </Space>
        </div>
        <div>
          <Text strong>
            <FormattedMessage
              id="pages.extensionsStore.detail.releasesLabel"
              defaultMessage="可用版本:"
            />
          </Text>
          <div style={{ marginTop: 8 }}>
            <Space wrap>
              {(releases || []).map((release) => (
                <Tag key={release.version} color="processing">
                  {release.version}
                </Tag>
              ))}
              {!releases?.length && (
                <Text type="secondary">
                  <FormattedMessage id="pages.extensionsStore.detail.none" defaultMessage="无" />
                </Text>
              )}
            </Space>
          </div>
        </div>
      </Space>
    </Modal>
  );
}
