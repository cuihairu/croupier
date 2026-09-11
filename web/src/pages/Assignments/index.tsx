import React from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Space } from 'antd';
import { useIntl } from '@umijs/max';
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

  const intl = useIntl();

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.assignments.page.title',
        defaultMessage: '函数分配管理',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.assignments.page.subTitle',
        defaultMessage: '管理不同游戏环境中可用的函数列表',
      })}
      extra={headerActions}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.assignments.summary.title',
            defaultMessage: '分配概览',
          })}
          description={intl.formatMessage({
            id: 'pages.assignments.summary.description',
            defaultMessage:
              '这里优先处理“当前环境该开放哪些函数”，函数按 resource 分组；菜单和页面归属不在这里配置。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                {
                  id: 'pages.assignments.summary.stat.total',
                  defaultMessage: `总函数 ${pageCtx.stats.total}`,
                },
                { total: pageCtx.stats.total },
              ),
            },
            {
              color: '#52c41a',
              text: intl.formatMessage(
                {
                  id: 'pages.assignments.summary.stat.assigned',
                  defaultMessage: `已分配 ${pageCtx.stats.active}`,
                },
                { active: pageCtx.stats.active },
              ),
            },
            {
              color: '#d9d9d9',
              text: intl.formatMessage(
                {
                  id: 'pages.assignments.summary.stat.unassigned',
                  defaultMessage: `未分配 ${pageCtx.stats.inactive}`,
                },
                { inactive: pageCtx.stats.inactive },
              ),
            },
            {
              color: '#722ed1',
              text: intl.formatMessage(
                {
                  id: 'pages.assignments.summary.stat.resources',
                  defaultMessage: `资源 ${pageCtx.stats.resources}`,
                },
                { resources: pageCtx.stats.resources },
              ),
            },
            {
              color: pageCtx.hasScope ? '#13c2c2' : '#faad14',
              text: pageCtx.hasScope
                ? intl.formatMessage({
                    id: 'pages.assignments.summary.stat.scopeSelected',
                    defaultMessage: '已选择作用域',
                  })
                : intl.formatMessage({
                    id: 'pages.assignments.summary.stat.scopeMissing',
                    defaultMessage: '尚未选择作用域',
                  }),
            },
          ]}
          hint={
            pageCtx.hasScope
              ? intl.formatMessage({
                  id: 'pages.assignments.summary.hint.scoped',
                  defaultMessage:
                    '推荐先在列表视图完成批量选择，再到资源分组或能力归属视图做补充确认。',
                })
              : intl.formatMessage({
                  id: 'pages.assignments.summary.hint.unscoped',
                  defaultMessage: '当前还没有选择游戏或环境，部分操作会被禁用。',
                })
          }
          hintType={pageCtx.hasScope ? 'info' : 'warning'}
        />

        <StandardListSection
          title={intl.formatMessage({
            id: 'pages.assignments.list.title',
            defaultMessage: '分配列表',
          })}
        >
          <PageRenderer {...pageCtx} />
        </StandardListSection>
      </Space>

      <HistoryModal
        open={historyVisible}
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
        open={canaryModalVisible}
        assignment={editingAssignment}
        onClose={() => setCanaryModalVisible(false)}
        onSave={(values) => {
          const functionId = String(values.functionId || editingAssignment?.id || '-');
          message.success(
            intl.formatMessage(
              {
                id: 'pages.assignments.canary.saved',
                defaultMessage: `灰度配置已保存 (${functionId})`,
              },
              { functionId },
            ),
          );
          setCanaryModalVisible(false);
        }}
      />

      <CloneModal
        open={cloneModalVisible}
        onClose={() => setCloneModalVisible(false)}
        onSave={async (targetEnv) => {
          const ok = await onCloneToEnv(targetEnv);
          if (ok) setCloneModalVisible(false);
        }}
      />
    </PageContainer>
  );
}
