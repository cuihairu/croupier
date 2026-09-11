import React, { useRef, useState } from 'react';
import { App, Card, Space, Button, Checkbox, Input, Select, Form } from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import {
  listFeedback,
  createFeedback,
  convertFeedbackToTicket,
  updateFeedback,
  deleteFeedback,
  type FeedbackPayload,
} from '@/services/api/support';
import { getMessage } from '@/utils/antdApp';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';
import { FormattedMessage, useAccess, useIntl } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

interface FeedbackItem {
  id: number;
  playerId?: string;
  contact?: string;
  category?: string;
  priority?: string;
  status?: string;
  gameId?: string;
  env?: string;
  content?: string;
  updatedAt?: string;
  [key: string]: JSONValue | undefined;
}

interface AccessState {
  canSupportManage?: boolean;
}

export default function SupportFeedbackPage() {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [q, setQ] = useState('');
  const [category, setCategory] = useState('');
  const [status, setStatus] = useState('');
  const [gameId, setGameId] = useState('');
  const [pendingOnly, setPendingOnly] = useState(true);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<FeedbackItem | null>(null);
  const { modal } = App.useApp();
  const access: AccessState = useAccess?.() || {};

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
  // 重新挂载，新增/编辑切换不会残留上一次的预填值
  const openAdd = () => {
    setEditing(null);
    setOpen(true);
  };
  const openEdit = (rec: FeedbackItem) => {
    setEditing(rec);
    setOpen(true);
  };
  const onFinish = async (v: FeedbackPayload) => {
    try {
      if (editing) {
        await updateFeedback(editing.id, v);
      } else {
        await createFeedback(v);
      }
      actionRef.current?.reload();
      return true;
    } catch {
      // 原实现无本地弹错（全局请求拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const onDelete = (rec: FeedbackItem) => {
    modal.confirm({
      title: intl.formatMessage({
        id: 'pages.supportFeedback.deleteConfirm.title',
        defaultMessage: '删除反馈',
      }),
      onOk: async () => {
        await deleteFeedback(rec.id);
        actionRef.current?.reload();
      },
    });
  };

  const columns: ProColumns<FeedbackItem>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.playerId',
        defaultMessage: '玩家ID',
      }),
      dataIndex: 'playerId',
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.contact',
        defaultMessage: '联系方式',
      }),
      dataIndex: 'contact',
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.category',
        defaultMessage: '分类',
      }),
      dataIndex: 'category',
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.priority',
        defaultMessage: '优先级',
      }),
      dataIndex: 'priority',
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.gameEnv',
        defaultMessage: '游戏/环境',
      }),
      render: (_: unknown, r: FeedbackItem) => `${r.gameId || ''}/${r.env || ''}`,
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.content',
        defaultMessage: '内容',
      }),
      dataIndex: 'content',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      render: (_, row) => formatDateTime(row.updatedAt ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.supportFeedback.field.actions',
        defaultMessage: '操作',
      }),
      render: (_: unknown, r: FeedbackItem) => (
        <Space>
          <Button
            size="small"
            disabled={r.status === 'triaged'}
            onClick={async () => {
              try {
                const player = r.playerId
                  ? r.playerId
                  : intl.formatMessage({
                      id: 'pages.supportFeedback.convert.unknownPlayer',
                      defaultMessage: '未知',
                    });
                const res = await convertFeedbackToTicket(r.id, {
                  note: intl.formatMessage(
                    {
                      id: 'pages.supportFeedback.convert.note',
                      defaultMessage: `来源反馈#${r.id} 玩家:${player}`,
                    },
                    { feedbackId: r.id, player },
                  ),
                });
                getMessage()?.success(
                  res.alreadyConverted
                    ? intl.formatMessage(
                        {
                          id: 'pages.supportFeedback.message.alreadyConverted',
                          defaultMessage: `该反馈已转过工单 #${res.ticketId}`,
                        },
                        { ticketId: res.ticketId },
                      )
                    : intl.formatMessage(
                        {
                          id: 'pages.supportFeedback.message.converted',
                          defaultMessage: `已转工单 #${res.ticketId}`,
                        },
                        { ticketId: res.ticketId },
                      ),
                );
                actionRef.current?.reload();
              } catch (e) {
                getMessage()?.error(
                  extractErrorMessage(
                    e,
                    intl.formatMessage({
                      id: 'pages.supportFeedback.message.convertFailed',
                      defaultMessage: '转工单失败',
                    }),
                  ),
                );
              }
            }}
          >
            {r.status === 'triaged' ? (
              <FormattedMessage
                id="pages.supportFeedback.action.converted"
                defaultMessage="已转工单"
              />
            ) : (
              <FormattedMessage id="pages.supportFeedback.action.convert" defaultMessage="转工单" />
            )}
          </Button>
          {access.canSupportManage && (
            <Button size="small" onClick={() => openEdit(r)}>
              <FormattedMessage id="pages.supportFeedback.action.edit" defaultMessage="编辑" />
            </Button>
          )}
          {access.canSupportManage && (
            <Button size="small" danger onClick={() => onDelete(r)}>
              <FormattedMessage id="pages.supportFeedback.action.delete" defaultMessage="删除" />
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.supportFeedback.card.title',
          defaultMessage: '玩家反馈',
        })}
        extra={
          <Space>
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.supportFeedback.search.keyword',
                defaultMessage: '关键词',
              })}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              style={{ width: 200 }}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.supportFeedback.search.category',
                defaultMessage: '分类',
              })}
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              style={{ width: 140 }}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.supportFeedback.search.status',
                defaultMessage: '状态',
              })}
              value={status}
              onChange={(v) => {
                setStatus(v);
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 140 }}
              options={[
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.status.new',
                    defaultMessage: '新建',
                  }),
                  value: 'new',
                },
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.status.triaged',
                    defaultMessage: '已分流',
                  }),
                  value: 'triaged',
                },
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.status.closed',
                    defaultMessage: '已关闭',
                  }),
                  value: 'closed',
                },
              ]}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.supportFeedback.search.gameId',
                defaultMessage: '游戏',
              })}
              value={gameId}
              onChange={(e) => setGameId(e.target.value)}
              style={{ width: 120 }}
            />
            <Checkbox checked={pendingOnly} onChange={(e) => setPendingOnly(e.target.checked)}>
              <FormattedMessage
                id="pages.supportFeedback.filter.hideConverted"
                defaultMessage="隐藏已转工单"
              />
            </Checkbox>
            <Button
              type="primary"
              onClick={() => {
                // 回第 1 页并重查：已在第 1 页时 setPageInfo 不触发请求，
                // 由 reload 兜底；非第 1 页时双触发经 debounce + abort 合并
                actionRef.current?.setPageInfo?.({ current: 1 });
                actionRef.current?.reload();
              }}
            >
              <FormattedMessage id="pages.supportFeedback.action.query" defaultMessage="查询" />
            </Button>
            {access.canSupportManage && (
              <Button onClick={openAdd}>
                <FormattedMessage
                  id="pages.supportFeedback.action.create"
                  defaultMessage="新建反馈"
                />
              </Button>
            )}
          </Space>
        }
      >
        <ProTable<FeedbackItem>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ q, category, status, gameId, pendingOnly }}
          request={async ({
            current = 1,
            pageSize = 20,
            q: qFilter,
            category: categoryFilter,
            status: statusFilter,
            gameId: gameIdFilter,
            pendingOnly: pendingOnlyFlag,
          }) => {
            try {
              const res = await listFeedback({
                q: qFilter ?? '',
                category: categoryFilter ?? '',
                status: statusFilter ?? '',
                gameId: gameIdFilter ?? '',
                page: current,
                size: pageSize,
                // 分诊队列定位：默认隐藏已转工单的反馈，避免与工单列表重复
                excludeStatus: pendingOnlyFlag && !statusFilter ? 'triaged' : undefined,
              });
              return {
                data: (res.feedback || []) as unknown as FeedbackItem[],
                total: res.total || 0,
                success: true,
              };
            } catch (error) {
              getMessage()?.error(
                extractErrorMessage(
                  error,
                  intl.formatMessage({
                    id: 'pages.supportFeedback.loadFailed',
                    defaultMessage: '加载反馈失败',
                  }),
                ),
              );
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />

        <ModalForm<FeedbackPayload>
          title={
            editing
              ? intl.formatMessage({
                  id: 'pages.supportFeedback.form.editTitle',
                  defaultMessage: '编辑反馈',
                })
              : intl.formatMessage({
                  id: 'pages.supportFeedback.form.createTitle',
                  defaultMessage: '新建反馈',
                })
          }
          open={open}
          onOpenChange={setOpen}
          modalProps={{ destroyOnHidden: true }}
          width={520}
          submitter={{
            searchConfig: {
              submitText: intl.formatMessage({
                id: 'pages.supportFeedback.form.submit',
                defaultMessage: '确定',
              }),
            },
          }}
          initialValues={editing ?? { priority: 'normal', status: 'new' }}
          onFinish={onFinish}
        >
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.playerId',
              defaultMessage: '玩家ID',
            })}
            name="playerId"
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.contact',
              defaultMessage: '联系方式',
            })}
            name="contact"
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.content',
              defaultMessage: '内容',
            })}
            name="content"
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.supportFeedback.form.contentRequired',
                  defaultMessage: '请输入内容',
                }),
              },
            ]}
          >
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.category',
              defaultMessage: '分类',
            })}
            name="category"
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.priority',
              defaultMessage: '优先级',
            })}
            name="priority"
          >
            <Select
              options={[
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.priority.low',
                    defaultMessage: '低',
                  }),
                  value: 'low',
                },
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.priority.normal',
                    defaultMessage: '普通',
                  }),
                  value: 'normal',
                },
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.priority.high',
                    defaultMessage: '高',
                  }),
                  value: 'high',
                },
              ]}
            />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.status',
              defaultMessage: '状态',
            })}
            name="status"
          >
            <Select
              options={[
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.status.new',
                    defaultMessage: '新建',
                  }),
                  value: 'new',
                },
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.status.triaged',
                    defaultMessage: '已分流',
                  }),
                  value: 'triaged',
                },
                {
                  label: intl.formatMessage({
                    id: 'pages.supportFeedback.status.closed',
                    defaultMessage: '已关闭',
                  }),
                  value: 'closed',
                },
              ]}
            />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.attach',
              defaultMessage: '附件(JSON)',
            })}
            name="attach"
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.gameId',
              defaultMessage: '游戏',
            })}
            name="gameId"
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.supportFeedback.field.env',
              defaultMessage: '环境',
            })}
            name="env"
          >
            <Input />
          </Form.Item>
        </ModalForm>
      </Card>
    </PageContainer>
  );
}
