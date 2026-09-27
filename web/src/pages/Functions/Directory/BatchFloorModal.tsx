import React, { useEffect, useState } from 'react';
import { Alert, Modal, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import VersionFloorSelect from './VersionFloorSelect';

const { Text } = Typography;

type BatchFloorModalProps = {
  open: boolean;
  /** 已勾选函数数（弹窗文案与确认语义依赖它） */
  count: number;
  submitting: boolean;
  /** #26：scope 内全部历史版本并集（门槛只允许选历史出现过的版本） */
  versionOptions: string[];
  onSubmit: (minVersion: string) => void;
  onClose: () => void;
};

// 批量设置版本门槛弹窗：统一值语义——选一个 minVersion 应用到全部
// 勾选函数（#26：改服务端历史版本下拉，不再手输）。空值禁止提交
//（批量清除走独立按钮 + Popconfirm，不在弹窗里）。
// 独立成文件便于组件级测试（整页 index.tsx 渲染链过重）。
export default function BatchFloorModal({
  open,
  count,
  submitting,
  versionOptions,
  onSubmit,
  onClose,
}: BatchFloorModalProps) {
  const intl = useIntl();
  const [versionInput, setVersionInput] = useState('');

  // 每次打开重置选择（避免上次的值残留导致误提交）
  useEffect(() => {
    if (open) setVersionInput('');
  }, [open]);

  const handleOk = () => {
    const minVersion = versionInput.trim();
    if (!minVersion || submitting) return;
    onSubmit(minVersion);
  };

  return (
    <Modal
      title={intl.formatMessage({
        id: 'pages.functionsDirectory.batch.modalTitle',
        defaultMessage: '批量设置版本门槛',
      })}
      open={open}
      onOk={handleOk}
      onCancel={onClose}
      confirmLoading={submitting}
      okButtonProps={{ disabled: !versionInput.trim() }}
      destroyOnHidden
    >
      <Alert
        type="info"
        showIcon
        title={intl.formatMessage(
          {
            id: 'pages.functionsDirectory.batch.modalDescription',
            defaultMessage: '将把已选 {count} 个函数的最低可注册函数版本统一设为所选值。',
          },
          { count },
        )}
        style={{ marginBottom: 16 }}
      />
      <Text strong>
        {intl.formatMessage({
          id: 'pages.functionsDirectory.batch.versionLabel',
          defaultMessage: '最低函数版本',
        })}
      </Text>
      <div style={{ marginTop: 8 }}>
        <VersionFloorSelect
          value={versionInput}
          options={versionOptions}
          submitting={submitting}
          allowClear={false}
          placeholder={intl.formatMessage({
            id: 'pages.functionsDirectory.batch.versionPlaceholder',
            defaultMessage: '选择历史版本',
          })}
          style={{ minWidth: 200, width: '100%' }}
          onChange={(next) => setVersionInput(next ?? '')}
        />
      </div>
    </Modal>
  );
}
