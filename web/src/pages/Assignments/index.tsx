import React from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, Space } from 'antd';
import { useIntl } from '@umijs/max';
import { StandardListSection, SummaryOverview } from '@/components';
import HistoryModal from './HistoryModal';
import CloneModal from './CloneModal';
import PageRenderer from './PageRenderer';
import useAssignmentsPage from './useAssignmentsPage';

export default function AssignmentsPage() {
  const {
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
    cloneModalVisible,
    setCloneModalVisible,
    onCloneToEnv,
  } = useAssignmentsPage();

  const intl = useIntl();

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.assignments.page.title',
        defaultMessage: '函数开放范围',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.assignments.page.subTitle',
        defaultMessage: '选择当前游戏环境开放哪些函数，保存后生效',
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
              color: 'var(--brand-2)',
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

        {/* 执行闸门语义说明（OPEN-ISSUES #36）：分配页此前从未讲清「分配即白名单」，
            用户误以为分配不生效；后端 EnsureFunctionAssigned 按此语义执行 */}
        <Alert
          type="info"
          showIcon
          title={intl.formatMessage({
            id: 'pages.assignments.gate.title',
            defaultMessage: '执行闸门',
          })}
          description={intl.formatMessage({
            id: 'pages.assignments.gate.description',
            defaultMessage:
              '作用域（游戏/环境）未保存过分配时默认开放所有函数；一旦保存分配列表，该作用域按白名单执行——未分配的函数调用会返回 403「函数未开放执行权限」。清空列表保存即恢复默认开放。',
          })}
        />

        {/* 互释文案：目录总开关（全局启停）vs 本页白名单（环境维度）两套独立语义，
            消除「函数目录状态与开放范围状态为什么不同步」的误解 */}
        <Alert
          type="info"
          showIcon
          title={intl.formatMessage({
            id: 'pages.assignments.directoryRelation.title',
            defaultMessage: '与函数目录总开关的关系',
          })}
          description={intl.formatMessage({
            id: 'pages.assignments.directoryRelation.description',
            defaultMessage:
              '本页维护的是各环境的调用白名单；函数目录中的「启用/禁用」是全局总开关——目录禁用的函数在所有环境都不可调用，本页会将其标注「已禁用」并禁止勾选。函数可被调用需同时满足：目录已启用 且 在该环境白名单内。',
          })}
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
