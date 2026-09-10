import { useCallback, useRef, useState } from 'react';
import { App, Avatar, Form, Input, Upload } from 'antd';
import type { FormInstance } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import { InboxOutlined, UserOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { updateMyProfile } from '@/services/api/me';
import { buildAvatarObjectKey, uploadAsset } from '@/services/api/storage';
import type { UploadProps } from 'antd/es/upload/interface';

type AvatarFormValues = { avatar: string };

/** 头像弹窗：拖拽上传（自动持久化）或手填 URL；open 时回填当前头像。 */
export default function AvatarModal({
  open,
  avatar,
  onClose,
  onPersisted,
}: {
  open: boolean;
  avatar?: string;
  onClose: () => void;
  onPersisted: () => Promise<void>;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  // 上传成功后需要命令式把 URL 回填进 avatar 字段；formRef 指向当前挂载的表单
  // 实例（destroyOnHidden 下每次打开都是新实例，不会持有陈旧 store）
  const formRef = useRef<FormInstance<AvatarFormValues> | undefined>(undefined);
  const [uploading, setUploading] = useState(false);

  const persistAvatar = async (avatarUrl: string, closeModal = true) => {
    await updateMyProfile({ avatar: avatarUrl });
    message.success(formatMessage('profile.avatar.success'));
    if (closeModal) {
      onClose();
    }
    await onPersisted();
  };

  const handleUpload: UploadProps['customRequest'] = async (options) => {
    const { file, onSuccess, onError } = options;

    setUploading(true);
    try {
      const uploaded = await uploadAsset(file as File, {
        path: buildAvatarObjectKey(file as File),
      });
      const avatarUrl = uploaded?.URL || '';
      if (!avatarUrl) {
        throw new Error('missing avatar url');
      }
      formRef.current?.setFieldsValue({ avatar: avatarUrl });
      await persistAvatar(avatarUrl);
      onSuccess?.(uploaded);
    } catch (error) {
      message.error(formatMessage('profile.update.error'));
      onError?.(error instanceof Error ? error : new Error(String(error)));
    } finally {
      setUploading(false);
    }
  };

  return (
    <ModalForm<AvatarFormValues>
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      formRef={formRef}
      title={formatMessage('profile.avatar.modal.title')}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{
        searchConfig: { submitText: formatMessage('profile.avatar.modal.submit') },
        // 对齐原 confirmLoading={uploading}：仅上传期间提交按钮 loading
        // （原实现表单提交期间按钮无 loading，此处保持一致）
        submitButtonProps: { loading: uploading },
      }}
      // open 时同步预填当前头像，替代原 useEffect setFieldsValue
      initialValues={{ avatar: avatar || '' }}
      onFinish={async (values) => {
        try {
          await persistAvatar(values.avatar);
          return true;
        } catch {
          message.error(formatMessage('profile.update.error'));
          return false;
        }
      }}
    >
      <Form.Item noStyle dependencies={['avatar']}>
        {({ getFieldValue }) => {
          const avatarValue = getFieldValue('avatar') || avatar;
          return (
            <div className="avatar-upload-preview">
              <Avatar
                size={72}
                src={avatarValue}
                icon={!avatarValue ? <UserOutlined /> : undefined}
              />
              <div>
                <div className="avatar-upload-preview__title">
                  {formatMessage('profile.avatar.modal.title')}
                </div>
                <div className="avatar-upload-preview__hint">
                  支持拖拽图片、点击上传，也支持直接输入图片 URL。
                </div>
              </div>
            </div>
          );
        }}
      </Form.Item>
      <Form.Item label="上传头像">
        <Upload.Dragger
          accept="image/*"
          maxCount={1}
          showUploadList={false}
          customRequest={handleUpload}
          disabled={uploading}
        >
          <p className="ant-upload-drag-icon">
            <InboxOutlined />
          </p>
          <p className="ant-upload-text">拖拽图片到这里，或点击上传</p>
          <p className="ant-upload-hint">上传后会自动回填到头像地址</p>
        </Upload.Dragger>
      </Form.Item>
      <Form.Item
        name="avatar"
        label={formatMessage('profile.avatar.modal.label')}
        rules={[
          { required: true, message: formatMessage('profile.avatar.modal.required') },
          {
            validator(_, value) {
              if (!value) {
                return Promise.reject(new Error(formatMessage('profile.avatar.modal.required')));
              }
              try {
                new URL(value);
                return Promise.resolve();
              } catch {
                return Promise.reject(new Error(formatMessage('profile.avatar.modal.invalid')));
              }
            },
          },
        ]}
      >
        <Input placeholder={formatMessage('profile.avatar.modal.placeholder')} />
      </Form.Item>
    </ModalForm>
  );
}
