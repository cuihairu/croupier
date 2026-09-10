import React from 'react';
import { Alert, Button, Input, Modal, Space, Upload } from 'antd';
import type { UploadFile } from 'antd/es/upload/interface';
import { CloudUploadOutlined } from '@ant-design/icons';
import type { SourceModalMode } from './shared';

/** 上传/更新 OpenAPI Source 弹窗：文件上传 + JSON 粘贴，
 * 表单值由页面持有（受控），提交/取消经回调上抛。 */
export default function SourceModal({
  open,
  mode,
  name,
  onNameChange,
  specText,
  onSpecChange,
  file,
  onFileChange,
  onCancel,
  onOk,
}: {
  open: boolean;
  mode: SourceModalMode;
  name: string;
  onNameChange: (value: string) => void;
  specText: string;
  onSpecChange: (value: string) => void;
  file: UploadFile | null;
  onFileChange: (file: UploadFile | null) => void;
  onCancel: () => void;
  onOk: () => void;
}) {
  const isUpdatingSource = mode === 'update';
  return (
    <Modal
      title={isUpdatingSource ? '更新 OpenAPI Source' : '上传 OpenAPI Source'}
      open={open}
      onCancel={onCancel}
      onOk={onOk}
      okText={isUpdatingSource ? '更新 revision' : '创建'}
      width={760}
    >
      <Space orientation="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type={isUpdatingSource ? 'info' : 'warning'}
          showIcon
          message={isUpdatingSource ? '更新只产生新的 Source revision' : '不要在 OpenAPI 中写 UI'}
          description={
            isUpdatingSource
              ? '更新会刷新 Source 的 operations 和 diagnostics，保留现有 Provider binding；OpenAPI 不能写 UI，只允许 x-resource/x-operation/x-capability/x-execution/x-risk/x-enabled/x-permission。'
              : 'OpenAPI 不能写 UI，只允许 x-resource/x-operation/x-capability/x-execution/x-risk/x-enabled/x-permission。'
          }
        />
        <Input
          addonBefore="name"
          placeholder="可选，默认使用 info.title"
          value={name}
          onChange={(event) => onNameChange(event.target.value)}
        />
        {isUpdatingSource ? null : (
          <Upload
            beforeUpload={(uploading) => {
              onFileChange(uploading);
              return false;
            }}
            maxCount={1}
            fileList={file ? [file] : []}
            onRemove={() => {
              onFileChange(null);
              return true;
            }}
          >
            <Button icon={<CloudUploadOutlined />}>选择 JSON/YAML 文件</Button>
          </Upload>
        )}
        <Input.TextArea
          rows={12}
          placeholder={
            isUpdatingSource
              ? '粘贴新的 OpenAPI JSON。YAML 更新请走 API raw PUT。'
              : '或粘贴 OpenAPI JSON。YAML 请使用文件上传。'
          }
          value={specText}
          onChange={(event) => onSpecChange(event.target.value)}
        />
      </Space>
    </Modal>
  );
}
