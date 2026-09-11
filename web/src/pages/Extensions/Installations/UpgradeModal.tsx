import { useEffect, useState } from 'react';
import { App, Modal, Select } from 'antd';
import { useIntl } from '@umijs/max';
import {
  listExtensionCatalogReleases,
  upgradeExtension,
  type ExtensionInstallationItem,
} from '@/services/api/extensions';
import { adaptCatalogReleaseListResponse } from '@/services/adapters/extensions';
import { EXTENSION_ERROR_CODES } from '@/services/errors/codes';
import { mapExtensionError } from '@/services/errors/mapper';

/** 升级扩展弹窗：打开时拉取目录版本列表（补齐当前版本），提交后回调刷新。 */
export default function UpgradeModal({
  open,
  row,
  onClose,
  onUpgraded,
}: {
  open: boolean;
  row: ExtensionInstallationItem | null;
  onClose: () => void;
  onUpgraded: () => Promise<void>;
}) {
  const { message: msg } = App.useApp();
  const intl = useIntl();
  const [upgrading, setUpgrading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [version, setVersion] = useState('');
  const [options, setOptions] = useState<{ label: string; value: string }[]>([]);

  useEffect(() => {
    if (!open || !row) return;
    setVersion('');
    setOptions([]);
    setLoading(true);
    listExtensionCatalogReleases(row.extensionId)
      .then((resp) => {
        const releaseVM = adaptCatalogReleaseListResponse(resp);
        const opts = releaseVM.releases.map((r) => ({
          label: r.version,
          value: r.version,
        }));
        setOptions(opts);
        const hasCurrent = opts.some((o) => o.value === row.releaseVersion);
        if (!hasCurrent && row.releaseVersion) {
          setOptions([{ label: row.releaseVersion, value: row.releaseVersion }, ...opts]);
        }
      })
      .finally(() => setLoading(false));
  }, [open, row]);

  const handleOk = async () => {
    if (!row) return;
    if (!version.trim()) {
      msg.warning(
        intl.formatMessage({
          id: 'pages.extensionsInstallations.upgrade.missingVersionWarning',
          defaultMessage: '请输入目标版本',
        }),
      );
      return;
    }
    setUpgrading(true);
    try {
      await upgradeExtension(row.id, version.trim());
      msg.success(
        intl.formatMessage({
          id: 'pages.extensionsInstallations.upgrade.submitted',
          defaultMessage: '升级请求已提交',
        }),
      );
      onClose();
      await onUpgraded();
    } catch (err) {
      const uiErr = mapExtensionError(err as Error);
      const details = uiErr.details || {};
      if (uiErr.code === EXTENSION_ERROR_CODES.MISSING_DEPENDENCY) {
        msg.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsInstallations.upgrade.missingDependency',
              defaultMessage: `升级失败，缺少依赖扩展：${details.dependency || 'unknown'}`,
            },
            { dependency: String(details.dependency ?? 'unknown') },
          ),
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.VERSION_MISMATCH) {
        msg.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsInstallations.upgrade.versionMismatch',
              defaultMessage: `升级失败，依赖版本不匹配：${details.dependency || 'unknown'}，要求 ${
                details.requiredVersion || '-'
              }，当前 ${details.currentVersion || '-'}`,
            },
            {
              dependency: String(details.dependency ?? 'unknown'),
              requiredVersion: String(details.requiredVersion ?? '-'),
              currentVersion: String(details.currentVersion ?? '-'),
            },
          ),
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE) {
        msg.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsInstallations.upgrade.dependencyCycle',
              defaultMessage: `升级失败，检测到循环依赖：${details.dependency || 'unknown'}`,
            },
            { dependency: String(details.dependency ?? 'unknown') },
          ),
        );
        return;
      }
      msg.error(uiErr.message);
    } finally {
      setUpgrading(false);
    }
  };

  return (
    <Modal
      open={open}
      title={intl.formatMessage({
        id: 'pages.extensionsInstallations.upgrade.title',
        defaultMessage: '升级扩展',
      })}
      onCancel={onClose}
      onOk={() => void handleOk()}
      okButtonProps={{ loading: upgrading }}
    >
      <Select
        showSearch
        loading={loading}
        options={options}
        value={version}
        onChange={(value) => setVersion(value)}
        placeholder={intl.formatMessage({
          id: 'pages.extensionsInstallations.upgrade.versionPlaceholder',
          defaultMessage: '选择目标版本',
        })}
        style={{ width: '100%' }}
      />
    </Modal>
  );
}
