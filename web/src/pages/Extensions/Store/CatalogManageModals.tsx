import React, { useEffect } from 'react';
import { Form, Input, Modal, Select } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ExtensionCatalogItem } from '@/services/api/extensions';

/**
 * 扩展商店管理动作弹窗（OPEN-ISSUES #46 批次 3 写路径的 UI 接线）：
 * CatalogRegisterModal 登记扩展 / ReleasePublishModal 发布版本。
 * 两弹窗均为受控开关 + 父组件持提交逻辑；表单校验为前端预检，
 * 后端仍做闭集/形态校验（重复登记 409、坏 semver 400 透出 message）。
 */

export type CatalogRegisterValues = {
  extensionId: string;
  name: string;
  displayName: string;
  vendor: string;
  kind: string;
  summary: string;
  iconUrl: string;
  homepageUrl: string;
  status: string;
};

export type ReleasePublishValues = {
  version: string;
  releaseChannel: string;
  minCoreVersion: string;
  packageRef: string;
  checksum: string;
  changelog: string;
  manifestJson: string;
};

const CATALOG_KINDS = ['ui', 'integration', 'analytics', 'ops', 'community'];
const CATALOG_STATUSES = ['active', 'delisted'];
const RELEASE_CHANNELS = ['stable', 'beta', 'alpha'];

export function CatalogRegisterModal({
  open,
  confirmLoading,
  onClose,
  onSubmit,
}: {
  open: boolean;
  confirmLoading: boolean;
  onClose: () => void;
  onSubmit: (values: CatalogRegisterValues) => void;
}) {
  const intl = useIntl();
  const [form] = Form.useForm<CatalogRegisterValues>();

  useEffect(() => {
    if (open) {
      form.resetFields();
      form.setFieldsValue({ status: 'active', kind: 'community' });
    }
  }, [open, form]);

  return (
    <Modal
      open={open}
      title={
        <FormattedMessage
          id="pages.extensionsStore.manage.registerTitle"
          defaultMessage="登记扩展到目录"
        />
      }
      confirmLoading={confirmLoading}
      onCancel={onClose}
      onOk={() => {
        // 校验失败即 inline 报错，属正常 UX 流程，吞掉 rejection 防未处理告警。
        void form
          .validateFields()
          .then((v) => onSubmit(v))
          .catch(() => undefined);
      }}
      destroyOnHidden
    >
      <Form form={form} name="catalog-register" layout="vertical" size="small">
        <Form.Item
          name="extensionId"
          label="Extension ID"
          rules={[
            { required: true },
            {
              pattern: /^[a-z0-9][a-z0-9._-]*$/,
              // trim 后再校验（提交载荷也 trim，两侧一致）。
              transform: (v: string | undefined) => (v ?? '').trim(),
              message: intl.formatMessage({
                id: 'pages.extensionsStore.manage.extensionIdPattern',
                defaultMessage: '小写字母或数字开头，仅含小写字母、数字、点、下划线、连字符',
              }),
            },
          ]}
        >
          <Input placeholder="com.example.myextension" maxLength={128} />
        </Form.Item>
        <Form.Item
          name="displayName"
          label={
            <FormattedMessage
              id="pages.extensionsStore.manage.displayName"
              defaultMessage="显示名"
            />
          }
        >
          <Input placeholder="My Extension" />
        </Form.Item>
        <Form.Item name="kind" label="Kind">
          <Select
            options={CATALOG_KINDS.map((k) => ({ label: k, value: k }))}
            placeholder="community"
          />
        </Form.Item>
        <Form.Item name="vendor" label="Vendor">
          <Input placeholder="external" />
        </Form.Item>
        <Form.Item
          name="summary"
          label={
            <FormattedMessage id="pages.extensionsStore.manage.summary" defaultMessage="简介" />
          }
        >
          <Input.TextArea rows={2} />
        </Form.Item>
        <Form.Item name="iconUrl" label="Icon URL">
          <Input placeholder="https://example.com/icon.png" />
        </Form.Item>
        <Form.Item name="homepageUrl" label="Homepage URL">
          <Input placeholder="https://example.com" />
        </Form.Item>
        <Form.Item name="status" label="Status">
          <Select options={CATALOG_STATUSES.map((k) => ({ label: k, value: k }))} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export function ReleasePublishModal({
  item,
  open,
  confirmLoading,
  onClose,
  onSubmit,
}: {
  item: ExtensionCatalogItem | undefined;
  open: boolean;
  confirmLoading: boolean;
  onClose: () => void;
  onSubmit: (values: ReleasePublishValues) => void;
}) {
  const intl = useIntl();
  const [form] = Form.useForm<ReleasePublishValues>();

  useEffect(() => {
    if (open) {
      form.resetFields();
      form.setFieldsValue({ releaseChannel: 'stable' });
    }
  }, [open, form]);

  return (
    <Modal
      open={open}
      title={
        <>
          <FormattedMessage
            id="pages.extensionsStore.manage.publishTitle"
            defaultMessage="发布版本"
          />
          {item ? `: ${item.displayName || item.name}` : ''}
        </>
      }
      confirmLoading={confirmLoading}
      onCancel={onClose}
      onOk={() => {
        // 校验失败即 inline 报错，属正常 UX 流程，吞掉 rejection 防未处理告警。
        void form
          .validateFields()
          .then((v) => onSubmit(v))
          .catch(() => undefined);
      }}
      destroyOnHidden
    >
      <Form form={form} name="release-publish" layout="vertical" size="small">
        <Form.Item
          name="version"
          label={
            <FormattedMessage id="pages.extensionsStore.manage.version" defaultMessage="版本号" />
          }
          rules={[
            { required: true },
            {
              pattern: /^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/,
              message: intl.formatMessage({
                id: 'pages.extensionsStore.manage.versionPattern',
                defaultMessage: '须为 semver 形态，如 1.2.3 或 1.0.0-beta.1',
              }),
            },
          ]}
        >
          <Input placeholder="1.2.3" />
        </Form.Item>
        <Form.Item name="releaseChannel" label="Channel">
          <Select options={RELEASE_CHANNELS.map((k) => ({ label: k, value: k }))} />
        </Form.Item>
        <Form.Item
          name="minCoreVersion"
          label={
            <FormattedMessage
              id="pages.extensionsStore.manage.minCoreVersion"
              defaultMessage="最低核心版本"
            />
          }
        >
          <Input placeholder="0.0.1" />
        </Form.Item>
        <Form.Item
          name="packageRef"
          label={
            <FormattedMessage
              id="pages.extensionsStore.manage.packageRef"
              defaultMessage="包引用"
            />
          }
        >
          <Input placeholder="packs/com.example-1.2.3.tgz" />
        </Form.Item>
        <Form.Item
          name="checksum"
          label={
            <FormattedMessage id="pages.extensionsStore.manage.checksum" defaultMessage="校验和" />
          }
        >
          <Input placeholder="sha256:..." />
        </Form.Item>
        <Form.Item name="changelog" label="Changelog">
          <Input.TextArea rows={2} />
        </Form.Item>
        <Form.Item
          name="manifestJson"
          label="Manifest (JSON)"
          rules={[
            { required: true },
            {
              validator: (_rule: unknown, value: string) => {
                if (!value || !value.trim()) {
                  return Promise.reject(
                    new Error(
                      intl.formatMessage({
                        id: 'pages.extensionsStore.manage.manifestRequired',
                        defaultMessage: 'manifest 必填（发布版本须携带能力/页面清单）',
                      }),
                    ),
                  );
                }
                try {
                  const parsed: unknown = JSON.parse(value);
                  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                    return Promise.reject(
                      new Error(
                        intl.formatMessage({
                          id: 'pages.extensionsStore.manage.manifestObject',
                          defaultMessage: 'manifest 须为 JSON 对象',
                        }),
                      ),
                    );
                  }
                  return Promise.resolve();
                } catch {
                  return Promise.reject(
                    new Error(
                      intl.formatMessage({
                        id: 'pages.extensionsStore.manage.manifestJsonInvalid',
                        defaultMessage: 'Manifest JSON 格式不正确',
                      }),
                    ),
                  );
                }
              },
            },
          ]}
        >
          <Input.TextArea rows={6} placeholder='{"capabilities": []}' />
        </Form.Item>
      </Form>
    </Modal>
  );
}
