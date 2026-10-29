/**
 * 表单容器组件复用契约测试（D1 组件复用基座 · 回归防护）。
 *
 * 「界面生成链收口」要求优先复用 Ant Design/ProComponents 的表单容器
 * （ModalForm / DrawerForm / StepsForm），自研仅聚焦业务语义、函数绑定和
 * 执行治理。本测试为这三个 ProComponents 复用目标建立契约基线：
 * - 渲染契约：标题、步骤、字段、提交/取消按钮与禁用态；
 * - 数据契约：onFinish 收到表单值（含跨步骤合并与 click 快捷提交）；
 * - 校验契约：必填未填阻断提交；提交后成功/失败的开合行为。
 *
 * 这些容器是后续生成表单迁移（RJSF SchemaFormRenderer → ProComponents
 * 容器）的对接面。一旦升级 @ant-design/pro-components 或替换为自研容器，
 * 快照与断言会提示契约漂移，避免静默破坏现有生成表单。
 *
 * @module components/FormContract/formContainers
 */
import React from 'react';
import { act, render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import {
  DrawerForm,
  ModalForm,
  StepsForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';

function renderWithApp(ui: React.ReactNode) {
  return render(<App>{ui}</App>);
}

/** 快照目标：只取容器根节点，避免把 portal 残留与测试全局文件混入。 */
function snapshot(element: Element | null) {
  element?.querySelectorAll('[class*="css-var-_r_"]').forEach((node) => {
    node.className = node.className.replace(/css-var-_r_[0-9a-zA-Z_]+/g, 'css-var');
  });
  expect(element).toMatchSnapshot();
}

describe('ProComponents 表单容器契约（D1 组件复用基座）', () => {
  describe('ModalForm', () => {
    it('渲染契约：标题/字段/提交按钮齐备，并锁定快照', async () => {
      renderWithApp(
        <ModalForm
          title="新建补偿"
          open
          width={560}
          onFinish={async () => true}

          modalProps={{ destroyOnHidden: true }}
        >
          <ProFormText name="title" label="标题" rules={[{ required: true }]} />
          <ProFormText name="playerId" label="玩家 ID" rules={[{ required: true }]} />
          <ProFormTextArea name="remark" label="备注" />
        </ModalForm>,
      );

      const dialog = await screen.findByRole('dialog');
      await act(async () => {});
      snapshot(dialog);
      expect(within(dialog).getByText('新建补偿')).toBeInTheDocument();
      expect(within(dialog).getByText('标题')).toBeInTheDocument();
      expect(within(dialog).getByText('玩家 ID')).toBeInTheDocument();
      expect(within(dialog).getByText('备注')).toBeInTheDocument();
      expect(within(dialog).getByRole('button', { name: /确\s*认/ })).toBeInTheDocument();
      expect(within(dialog).getByRole('button', { name: /取\s*消/ })).toBeInTheDocument();
    });

    it('校验契约：必填未填时阻断提交、不触发 onFinish', async () => {
      const onFinish = jest.fn().mockResolvedValue(true);
      renderWithApp(
        <ModalForm title="必填校验" open onFinish={onFinish}>
          <ProFormText name="title" label="标题" rules={[{ required: true }]} />
        </ModalForm>,
      );
      const dialog = await screen.findByRole('dialog');
      await act(async () => {});
      fireEvent.click(within(dialog).getByRole('button', { name: /确\s*认/ }));
      await waitFor(() => expect(within(dialog).getByText('请输入标题')).toBeInTheDocument());
      expect(onFinish).not.toHaveBeenCalled();
    });

    it('数据契约：提交携带表单值，成功后触发 onOpenChange(false)', async () => {
      const onFinish = jest.fn().mockResolvedValue(true);
      const onOpenChange = jest.fn();
      renderWithApp(
        <ModalForm title="提交数据" open onFinish={onFinish} onOpenChange={onOpenChange}>
          <ProFormText name="title" label="标题" rules={[{ required: true }]} />
        </ModalForm>,
      );
      const dialog = await screen.findByRole('dialog');
      await act(async () => {});
      fireEvent.change(within(dialog).getByLabelText('标题'), {
        target: { value: '十月补偿' },
      });
      fireEvent.click(within(dialog).getByRole('button', { name: /确\s*认/ }));
      await waitFor(() =>
        expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ title: '十月补偿' })),
      );
      await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    });
  });

  describe('DrawerForm', () => {
    it('渲染契约：标题/字段/提交按钮齐备，并锁定快照', async () => {
      renderWithApp(
        <DrawerForm title="编辑补偿" open width={560} onFinish={async () => true}>
          <ProFormText name="title" label="标题" rules={[{ required: true }]} />
          <ProFormTextArea name="remark" label="备注" />
        </DrawerForm>,
      );
      const dialog = await screen.findByRole('dialog');
      await act(async () => {});
      snapshot(dialog);
      expect(within(dialog).getByText('编辑补偿')).toBeInTheDocument();
      expect(within(dialog).getByRole('button', { name: /确\s*认/ })).toBeInTheDocument();
      expect(within(dialog).getByRole('button', { name: /取\s*消/ })).toBeInTheDocument();
    });

    it('数据契约：提交携带表单值', async () => {
      const onFinish = jest.fn().mockResolvedValue(true);
      renderWithApp(
        <DrawerForm title="提交数据" open onFinish={onFinish}>
          <ProFormText name="title" label="标题" rules={[{ required: true }]} />
        </DrawerForm>,
      );
      const dialog = await screen.findByRole('dialog');
      await act(async () => {});
      fireEvent.change(within(dialog).getByLabelText('标题'), {
        target: { value: '十月补偿' },
      });
      fireEvent.click(within(dialog).getByRole('button', { name: /确\s*认/ }));
      await waitFor(() =>
        expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ title: '十月补偿' })),
      );
    });
  });

  describe('StepsForm', () => {
    it('渲染契约：两步步骤指示/字段/提交按钮齐备，并锁定快照', async () => {
      const onFinish = jest.fn().mockResolvedValue(true);
      const { container } = renderWithApp(
        <StepsForm
          onFinish={onFinish}
          stepsProps={{ items: [{ title: '基础信息' }, { title: '补充说明' }] }}
        >
          <StepsForm.StepForm step={0} stepProps={{ title: '基础信息' }}>
            <ProFormText name="title" label="标题" rules={[{ required: true }]} />
          </StepsForm.StepForm>
          <StepsForm.StepForm step={1} stepProps={{ title: '补充说明' }}>
            <ProFormTextArea name="remark" label="备注" />
          </StepsForm.StepForm>
        </StepsForm>,
      );
      expect(within(container).getByText('基础信息')).toBeInTheDocument();
      expect(within(container).getByText('补充说明')).toBeInTheDocument();
      snapshot(container);
    });

    it('数据契约：跨步骤上一步值保留，最后一步提交合并值', async () => {
      const onFinish = jest.fn().mockResolvedValue(true);
      renderWithApp(
        <StepsForm
          onFinish={onFinish}
          stepsProps={{ items: [{ title: '基础信息' }, { title: '补充说明' }] }}
        >
          <StepsForm.StepForm step={0} stepProps={{ title: '基础信息' }}>
            <ProFormText name="title" label="标题" rules={[{ required: true }]} />
          </StepsForm.StepForm>
          <StepsForm.StepForm step={1} stepProps={{ title: '补充说明' }}>
            <ProFormTextArea name="remark" label="备注" />
          </StepsForm.StepForm>
        </StepsForm>,
      );
      fireEvent.change(await screen.findByLabelText('标题'), {
        target: { value: '十月补偿' },
      });
      fireEvent.click(await screen.findByRole('button', { name: /下\s*一\s*步/ }));
      fireEvent.change(await screen.findByLabelText('备注'), {
        target: { value: '面向全体玩家' },
      });
      fireEvent.click(await screen.findByRole('button', { name: /提\s*交/ }));
      await waitFor(() =>
        expect(onFinish).toHaveBeenCalledWith(
          expect.objectContaining({ title: '十月补偿', remark: '面向全体玩家' }),
        ),
      );
    });
  });
});
