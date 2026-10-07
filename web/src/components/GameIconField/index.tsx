import { useCallback, useState } from 'react';
import { App, Button, Input, Progress, Space, Upload } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import GameIcon, { GAME_ICON_SIZE } from '@/components/GameIcon';
import { GAME_ICON_MAX_BYTES, uploadGameIcon } from '@/services/api/games';
import type { UploadProps } from 'antd/es/upload/interface';

/** 图标上传的 accept 口径；后端 sniffIconKind 按内容判定，此处仅预筛。 */
export const GAME_ICON_ACCEPT = '.png,.jpg,.jpeg,.webp,.svg';

const GAME_ICON_ALLOWED_MIME = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/svg+xml']);

/**
 * 图标文件的客户端预校验：类型白名单（MIME 或 .svg 扩展名）+ 大小上限。
 * 通过返回 null，否则返回可直接 message.error 的文案；最终仍以后端魔数
 * 嗅探为准，这里挡住明显误传、省一次往返。
 */
export function validateGameIconFile(
  file: File,
  formatMessage: (descriptor: { id: string; defaultMessage: string }) => string,
): string | null {
  const typeOk = GAME_ICON_ALLOWED_MIME.has(file.type) || /\.svg$/i.test(file.name);
  if (!typeOk) {
    return formatMessage({
      id: 'component.gameIconField.badType',
      defaultMessage: '仅支持 png/jpg/webp/svg 格式的图标',
    });
  }
  if (file.size > GAME_ICON_MAX_BYTES) {
    return formatMessage({
      id: 'component.gameIconField.oversize',
      defaultMessage: '图标文件超过 2MB 大小上限',
    });
  }
  return null;
}

/**
 * 游戏图标字段（受控表单组件）：URL 手填与本地文件上传双方式，上传成功
 * 自动把可访问 URL 回填进表单值，输入框前缀用 GameIcon 实时预览。
 *
 * 作为 Form.Item name="icon" 的直接子组件使用（value/onChange 协议），
 * 保存链路不需要感知上传的存在。
 */
export default function GameIconField({
  value,
  onChange,
  disabled,
}: {
  value?: string;
  onChange?: (value: string) => void;
  disabled?: boolean;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [uploading, setUploading] = useState(false);
  const [percent, setPercent] = useState(0);

  const formatMessage = useCallback(
    (descriptor: { id: string; defaultMessage: string }) => intl.formatMessage(descriptor),
    [intl],
  );

  const beforeUpload = useCallback(
    (file: File) => {
      const problem = validateGameIconFile(file, formatMessage);
      if (problem) {
        message.error(problem);
        return Upload.LIST_IGNORE;
      }
      return true;
    },
    [formatMessage, message],
  );

  const customRequest: UploadProps['customRequest'] = async ({ file, onSuccess, onError }) => {
    setUploading(true);
    setPercent(0);
    try {
      const result = await uploadGameIcon(file as File, setPercent);
      onChange?.(result.url);
      message.success(
        intl.formatMessage({
          id: 'component.gameIconField.uploaded',
          defaultMessage: '图标已上传并回填',
        }),
      );
      onSuccess?.(result);
    } catch (error) {
      message.error(error instanceof Error ? error.message : String(error));
      onError?.(error instanceof Error ? error : new Error(String(error)));
    } finally {
      setUploading(false);
    }
  };

  return (
    <Space orientation="vertical" style={{ width: '100%' }} size={8}>
      <Input
        value={value}
        onChange={(event) => onChange?.(event.target.value)}
        placeholder="https://..."
        allowClear
        disabled={disabled}
        data-testid="game-icon-input"
        prefix={<GameIcon icon={value} name={value || 'game'} size={GAME_ICON_SIZE.sm} />}
      />
      <Upload
        accept={GAME_ICON_ACCEPT}
        maxCount={1}
        showUploadList={false}
        beforeUpload={beforeUpload}
        customRequest={customRequest}
        disabled={disabled || uploading}
      >
        <Button
          icon={<UploadOutlined />}
          loading={uploading}
          disabled={disabled || uploading}
          data-testid="game-icon-upload"
        >
          <FormattedMessage id="component.gameIconField.upload" defaultMessage="上传图标" />
        </Button>
      </Upload>
      {uploading ? (
        <Progress percent={percent} size="small" data-testid="game-icon-progress" />
      ) : null}
    </Space>
  );
}
