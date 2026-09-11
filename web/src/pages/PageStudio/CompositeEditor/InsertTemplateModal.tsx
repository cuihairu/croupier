import { Form, Input, InputNumber, Modal, Switch } from 'antd';
import { useIntl } from '@umijs/max';
import type { ComponentTemplateDTO } from './ComponentLibrary';
import { localizedText } from '@/utils/localizedText';

/** 带参数模板拖入的快速配置弹窗（U6）：参数确认后再按拖拽落点插入
 * （不丢失 drop 位置）。插入决策（planTemplateDrop）在编辑器主页。 */
export default function InsertTemplateModal({
  tplState,
  onClose,
  onConfirm,
}: {
  tplState: { tpl: ComponentTemplateDTO; overId: string } | null;
  onClose: () => void;
  onConfirm: (tpl: ComponentTemplateDTO, values: Record<string, unknown>, overId: string) => void;
}) {
  const [form] = Form.useForm<Record<string, unknown>>();
  const intl = useIntl();

  const close = () => {
    onClose();
    form.resetFields();
  };

  return (
    <Modal
      title={intl.formatMessage(
        {
          id: 'pages.pageStudio.editor.insertTpl.title',
          defaultMessage: '配置组件参数：{name}',
        },
        {
          name: localizedText(tplState?.tpl.name, 'zh-CN', tplState?.tpl.key ?? ''),
        },
      )}
      open={tplState !== null}
      onCancel={close}
      onOk={() => {
        if (!tplState) return;
        const values = form.getFieldsValue() as Record<string, unknown>;
        // 参数确认后按拖拽落点插入（容器/弹窗/链式，planTemplateDrop 决策）
        onConfirm(tplState.tpl, values, tplState.overId);
        close();
      }}
      okText={intl.formatMessage({
        id: 'pages.pageStudio.editor.insertTpl.ok',
        defaultMessage: '插入',
      })}
      cancelText={intl.formatMessage({
        id: 'pages.pageStudio.editor.constantImport.cancel',
        defaultMessage: '取消',
      })}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false}>
        {tplState?.tpl.params?.map((p) => (
          <Form.Item
            key={p.key}
            name={p.key}
            label={localizedText(p.label, 'zh-CN', p.key)}
            initialValue={p.default}
            valuePropName={p.prop === 'autoRun' ? 'checked' : 'value'}
          >
            {p.prop === 'autoRun' ? (
              <Switch />
            ) : p.prop === 'span' ? (
              <InputNumber min={1} max={24} style={{ width: '100%' }} />
            ) : (
              <Input maxLength={60} />
            )}
          </Form.Item>
        ))}
      </Form>
    </Modal>
  );
}
