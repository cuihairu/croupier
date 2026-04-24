import React from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Space } from 'antd';
import { StandardListSection, SummaryOverview } from '@/components';
import HistoryModal from './HistoryModal';
import CanaryModal from './CanaryModal';
import CloneModal from './CloneModal';
import PageRenderer from './PageRenderer';
import useAssignmentsPage from './useAssignmentsPage';

export default function AssignmentsPage() {
  const {
    message,
    pageCtx,
    headerActions,
    historyVisible,
    setHistoryVisible,
    history,
    historyLoading,
    historyPage,
    historyPageSize,
    historyTotal,
    historyActionFilter,
    setHistoryActionFilter,
    loadHistory,
    canaryModalVisible,
    setCanaryModalVisible,
    editingAssignment,
    cloneModalVisible,
    setCloneModalVisible,
    onCloneToEnv,
  } = useAssignmentsPage();

  return (
    <PageContainer
      title="函数分配管理"
      subTitle="管理不同游戏环境中可用的函数列表"
      extra={headerActions}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="分配概览"
          description="这里优先处理“当前环境该开放哪些函数”，先看分配总览，再进入列表、分类或路由视图细化调整。"
          items={[
            { color: '#1677ff', text: `总函数 ${pageCtx.stats.total}` },
            { color: '#52c41a', text: `已分配 ${pageCtx.stats.active}` },
            { color: '#d9d9d9', text: `未分配 ${pageCtx.stats.inactive}` },
            { color: '#722ed1', text: `分类 ${pageCtx.stats.categories}` },
            {
              color: pageCtx.hasScope ? '#13c2c2' : '#faad14',
              text: pageCtx.hasScope ? '已选择作用域' : '尚未选择作用域',
            },
          ]}
          hint={
            pageCtx.hasScope
              ? '推荐先在列表视图完成批量选择，再到分类或路由视图做补充调整。'
              : '当前还没有选择游戏或环境，部分操作会被禁用。'
          }
          hintType={pageCtx.hasScope ? 'info' : 'warning'}
        />

        <StandardListSection title="分配列表">
          <PageRenderer {...pageCtx} />
        </StandardListSection>
      </Space>

      <HistoryModal
        visible={historyVisible}
        history={history}
        loading={historyLoading}
        page={historyPage}
        pageSize={historyPageSize}
        total={historyTotal}
        actionFilter={historyActionFilter}
        onClose={() => setHistoryVisible(false)}
        onActionFilterChange={(next) => {
          setHistoryActionFilter(next);
          loadHistory(1, historyPageSize, next);
        }}
        onReload={() => loadHistory(historyPage, historyPageSize, historyActionFilter)}
        onPageChange={(page, pageSize) => loadHistory(page, pageSize, historyActionFilter)}
      />

      <CanaryModal
        visible={canaryModalVisible}
        assignment={editingAssignment}
        onClose={() => setCanaryModalVisible(false)}
        onSave={(values) => {
          message.success(`灰度配置已保存 (${values.functionId || editingAssignment?.id || '-'})`);
          setCanaryModalVisible(false);
        }}
      />

      <CloneModal
        visible={cloneModalVisible}
        onClose={() => setCloneModalVisible(false)}
        onSave={async (targetEnv) => {
          const ok = await onCloneToEnv(targetEnv);
          if (ok) setCloneModalVisible(false);
        }}
      />
    </PageContainer>
  );
}
