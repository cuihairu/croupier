import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Card,
  Space,
  Tag,
  Button,
  Descriptions,
  Divider,
  List,
  Input,
  Upload,
  Modal,
  Select,
  Form,
  Rate,
} from 'antd';
import type { UploadFile as AntUploadFile } from 'antd/es/upload/interface';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useParams, history, useModel } from '@umijs/max';
import { uploadAsset } from '@/services/api/storage';
import { getMessage } from '@/utils/antdApp';
import { formatDateTime } from '@/utils/format';
import {
  updateTicket,
  deleteTicket,
  getTicket,
  listTicketComments,
  addTicketComment,
  convertTicketToBug,
  rateTicket,
  transitionTicket,
  type Ticket,
  type TicketComment,
  type TicketPayload,
} from '@/services/api/support';

interface ExtendedTicket extends Ticket {
  contact?: string;
  env?: string;
  source?: string;
}

interface ExtendedComment extends TicketComment {
  attach?: string;
}

interface Attachment {
  name?: string;
  url?: string;
  key?: string;
}

interface InitialState {
  currentUser?: {
    name?: string;
  };
}

type IntlMessage = { id: string; defaultMessage: string };

const priColorMap: Record<string, string> = {
  urgent: 'red',
  high: 'volcano',
  normal: 'blue',
  low: 'default',
};
const priTextMap: Record<string, IntlMessage> = {
  urgent: { id: 'pages.ticketsDetail.priority.urgent', defaultMessage: '紧急' },
  high: { id: 'pages.ticketsDetail.priority.high', defaultMessage: '高' },
  normal: { id: 'pages.ticketsDetail.priority.normal', defaultMessage: '普通' },
  low: { id: 'pages.ticketsDetail.priority.low', defaultMessage: '低' },
};
const stColorMap: Record<string, string> = {
  open: 'gold',
  inProgress: 'blue',
  resolved: 'green',
  closed: 'default',
};
const stTextMap: Record<string, IntlMessage> = {
  open: { id: 'pages.ticketsDetail.status.open', defaultMessage: '打开' },
  inProgress: { id: 'pages.ticketsDetail.status.inProgress', defaultMessage: '处理中' },
  resolved: { id: 'pages.ticketsDetail.status.resolved', defaultMessage: '已解决' },
  closed: { id: 'pages.ticketsDetail.status.closed', defaultMessage: '已关闭' },
};

export default function TicketDetailPage() {
  const { modal } = App.useApp();
  const intl = useIntl();
  const { id } = useParams();
  const mid = String(id || '');
  const [loading, setLoading] = useState(false);
  const [ticket, setTicket] = useState<ExtendedTicket | null>(null);
  const [comments, setComments] = useState<ExtendedComment[]>([]);
  const [cmt, setCmt] = useState<string>('');
  const [files, setFiles] = useState<AntUploadFile[]>([]);
  const [submittingCmt, setSubmittingCmt] = useState(false);
  const [transOpen, setTransOpen] = useState(false);
  const [transStatus, setTransStatus] = useState<string>('');
  const [transComment, setTransComment] = useState<string>('');
  const [editOpen, setEditOpen] = useState(false);
  const { initialState } = useModel('@@initialState');

  // 处理文件上传
  const handleUpload = async (file: File): Promise<void> => {
    try {
      const res = await uploadAsset(file);
      const next: AntUploadFile = {
        uid: String(Date.now()),
        name: file.name,
        status: 'done',
        url: res.URL,
      };
      setFiles((prev) => [...prev, next]);
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.ticketsDetail.operationFailed',
              defaultMessage: '操作失败',
            });
      getMessage()?.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.ticketsDetail.uploadFailed',
            defaultMessage: '上传失败',
          }),
      );
    }
  };

  const priTag = (v?: string) => {
    return v ? (
      <Tag color={priColorMap[v] || 'default'}>
        {priTextMap[v] ? intl.formatMessage(priTextMap[v]) : v}
      </Tag>
    ) : (
      '-'
    );
  };
  const stTag = (v?: string) => {
    return v ? (
      <Tag color={stColorMap[v] || 'default'}>
        {stTextMap[v] ? intl.formatMessage(stTextMap[v]) : v}
      </Tag>
    ) : (
      '-'
    );
  };

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [t, cm] = await Promise.all([getTicket(mid), listTicketComments(mid)]);
      setTicket(t);
      setComments(cm.comments || []);
    } finally {
      setLoading(false);
    }
  }, [mid]);
  useEffect(() => {
    if (mid) load();
  }, [mid, load]);

  const submitComment = async () => {
    if (!cmt.trim()) {
      getMessage()?.warning(
        intl.formatMessage({
          id: 'pages.ticketsDetail.comment.emptyWarning',
          defaultMessage: '评论内容不能为空',
        }),
      );
      return;
    }
    setSubmittingCmt(true);
    try {
      const attach = files
        .map((f) => ({
          name: f.name,
          url: f.url || (f.response as { URL?: string })?.URL,
          key: (f.response as { Key?: string })?.Key,
        }))
        .filter((x) => x.url);
      await addTicketComment(mid, { content: cmt, attach: JSON.stringify(attach) });
      setCmt('');
      setFiles([]);
      load();
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.ticketsDetail.operationFailed',
              defaultMessage: '操作失败',
            });
      getMessage()?.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.ticketsDetail.comment.submitFailed',
            defaultMessage: '评论失败',
          }),
      );
    } finally {
      setSubmittingCmt(false);
    }
  };

  const doTransition = async () => {
    try {
      await transitionTicket(mid, {
        status: transStatus,
        comment: transComment ? transComment : undefined,
      });
      setTransOpen(false);
      setTransStatus('');
      setTransComment('');
      load();
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.ticketsDetail.operationFailed',
              defaultMessage: '操作失败',
            });
      getMessage()?.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.ticketsDetail.transition.failed',
            defaultMessage: '流转失败',
          }),
      );
    }
  };

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 ticket 记录
  // 重新挂载（openEdit 的守卫保证打开时 ticket 已加载）
  const openEdit = () => {
    if (!ticket) return;
    setEditOpen(true);
  };
  const onFinish = async (v: TicketPayload) => {
    try {
      await updateTicket(Number(mid), v);
      load();
      return true;
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.ticketsDetail.operationFailed',
              defaultMessage: '操作失败',
            });
      getMessage()?.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.ticketsDetail.updateFailed',
            defaultMessage: '更新失败',
          }),
      );
      return false;
    }
  };
  const doDelete = async () => {
    modal.confirm({
      title: intl.formatMessage({
        id: 'pages.ticketsDetail.deleteConfirm.title',
        defaultMessage: '删除工单',
      }),
      content: intl.formatMessage({
        id: 'pages.ticketsDetail.deleteConfirm.content',
        defaultMessage: '确定删除该工单？',
      }),
      onOk: async () => {
        await deleteTicket(Number(mid));
        history.push('/support/tickets');
      },
    });
  };
  const assignToMe = async () => {
    try {
      const state = initialState as InitialState | undefined;
      const me = state?.currentUser?.name;
      if (!me) {
        getMessage()?.warning(
          intl.formatMessage({
            id: 'pages.ticketsDetail.assign.noCurrentUser',
            defaultMessage: '未获取到当前用户',
          }),
        );
        return;
      }
      await updateTicket(Number(mid), { assignee: me });
      getMessage()?.success(
        intl.formatMessage({
          id: 'pages.ticketsDetail.assign.success',
          defaultMessage: '已指派给我',
        }),
      );
      load();
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.ticketsDetail.operationFailed',
              defaultMessage: '操作失败',
            });
      getMessage()?.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.ticketsDetail.assign.failed',
            defaultMessage: '指派失败',
          }),
      );
    }
  };

  if (!mid) return null;

  return (
    <>
      <Card
        loading={loading}
        title={
          <Space>
            <Button onClick={() => history.back()}>
              <FormattedMessage id="pages.ticketsDetail.back" defaultMessage="< 返回" />
            </Button>
            <span>
              {intl.formatMessage(
                { id: 'pages.ticketsDetail.title', defaultMessage: '工单详情 #{id}' },
                { id: mid },
              )}
            </span>
          </Space>
        }
        extra={
          <Space>
            <Button onClick={assignToMe}>
              <FormattedMessage id="pages.ticketsDetail.assign.toMe" defaultMessage="指派给我" />
            </Button>
            <Button onClick={openEdit}>
              <FormattedMessage id="pages.ticketsDetail.edit.button" defaultMessage="编辑工单" />
            </Button>
            <Button danger onClick={doDelete}>
              <FormattedMessage id="pages.ticketsDetail.delete.button" defaultMessage="删除工单" />
            </Button>
            <Button onClick={() => setTransOpen(true)}>
              <FormattedMessage id="pages.ticketsDetail.transition.open" defaultMessage="流转" />
            </Button>
            <Button
              onClick={async () => {
                try {
                  const res = await convertTicketToBug(Number(mid), {
                    steps: ticket?.content || undefined,
                  });
                  getMessage()?.success(
                    intl.formatMessage(
                      {
                        id: 'pages.ticketsDetail.convertToBug.success',
                        defaultMessage: '已升级为缺陷 #{bugId}（研发 → 缺陷追踪）',
                      },
                      { bugId: res.bugId },
                    ),
                  );
                  load();
                } catch (e) {
                  const errMsg =
                    e instanceof Error
                      ? e.message
                      : intl.formatMessage({
                          id: 'pages.ticketsDetail.convertToBug.failed',
                          defaultMessage: '升级失败',
                        });
                  getMessage()?.error(errMsg);
                }
              }}
            >
              <FormattedMessage
                id="pages.ticketsDetail.convertToBug.button"
                defaultMessage="升级为缺陷"
              />
            </Button>
          </Space>
        }
      >
        {ticket && (
          <>
            <Descriptions bordered column={2} size="small">
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.title',
                  defaultMessage: '标题',
                })}
                span={2}
              >
                {ticket.title}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.status',
                  defaultMessage: '状态',
                })}
              >
                {stTag(ticket.status)}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.priority',
                  defaultMessage: '优先级',
                })}
              >
                {priTag(ticket.priority)}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.assignee',
                  defaultMessage: '处理人',
                })}
              >
                {ticket.assignee || '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.tags',
                  defaultMessage: '标签',
                })}
              >
                {ticket.tags || '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.playerId',
                  defaultMessage: '玩家ID',
                })}
              >
                {ticket.playerId || '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.contact',
                  defaultMessage: '联系方式',
                })}
                span={2}
              >
                {ticket.contact || '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.gameEnv',
                  defaultMessage: '游戏/环境',
                })}
              >
                {(ticket.gameId || '') + '/' + (ticket.env || '')}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.source',
                  defaultMessage: '来源',
                })}
              >
                {ticket.source || '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.createdAt',
                  defaultMessage: '创建时间',
                })}
              >
                {ticket.createdAt ? formatDateTime(ticket.createdAt) : '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.updatedAt',
                  defaultMessage: '更新时间',
                })}
              >
                {ticket.updatedAt ? formatDateTime(ticket.updatedAt) : '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.content',
                  defaultMessage: '内容',
                })}
                span={2}
              >
                <div style={{ whiteSpace: 'pre-wrap' }}>{ticket.content || '-'}</div>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.ticketsDetail.field.rating',
                  defaultMessage: '满意度',
                })}
                span={2}
              >
                {['resolved', 'closed'].includes(ticket.status) ? (
                  <Rate
                    value={ticket.rating || 0}
                    onChange={async (v) => {
                      try {
                        await rateTicket(Number(mid), v);
                        getMessage()?.success(
                          intl.formatMessage({
                            id: 'pages.ticketsDetail.rating.success',
                            defaultMessage: '已记录评价',
                          }),
                        );
                        load();
                      } catch (e) {
                        const errMsg =
                          e instanceof Error
                            ? e.message
                            : intl.formatMessage({
                                id: 'pages.ticketsDetail.rating.failed',
                                defaultMessage: '评价失败',
                              });
                        getMessage()?.error(errMsg);
                      }
                    }}
                  />
                ) : (
                  <span style={{ color: '#999' }}>
                    <FormattedMessage
                      id="pages.ticketsDetail.rating.hint"
                      defaultMessage="工单解决/关闭后可评价"
                    />
                  </span>
                )}
              </Descriptions.Item>
            </Descriptions>

            <Divider>
              <FormattedMessage
                id="pages.ticketsDetail.comment.sectionTitle"
                defaultMessage="评论"
              />
            </Divider>
            <List<ExtendedComment>
              dataSource={comments}
              renderItem={(it: ExtendedComment) => {
                let attachments: Attachment[] = [];
                try {
                  if (it.attach) attachments = JSON.parse(it.attach);
                } catch {}
                return (
                  <List.Item>
                    <List.Item.Meta
                      title={
                        <Space>
                          <strong>{it.author || '-'}</strong>
                          <span>{it.createdAt ? formatDateTime(it.createdAt) : ''}</span>
                        </Space>
                      }
                      description={<div style={{ whiteSpace: 'pre-wrap' }}>{it.content || ''}</div>}
                    />
                    <div>
                      {attachments && attachments.length > 0 && (
                        <div>
                          {attachments.map((a: Attachment, idx: number) => {
                            const url = a.url as string;
                            const isImg = /\.(png|jpe?g|gif|webp|bmp|svg)(\?.*)?$/i.test(url);
                            return (
                              <div key={idx} style={{ marginBottom: 6 }}>
                                {isImg ? (
                                  <img
                                    src={url}
                                    alt={a.name || ''}
                                    style={{
                                      maxWidth: 160,
                                      maxHeight: 120,
                                      cursor: 'pointer',
                                      border: '1px solid #eee',
                                      padding: 2,
                                    }}
                                    onClick={() => window.open(url, '_blank')}
                                  />
                                ) : (
                                  <a href={url} target="_blank" rel="noreferrer">
                                    {a.name || url}
                                  </a>
                                )}
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  </List.Item>
                );
              }}
            />
            <Divider>
              <FormattedMessage
                id="pages.ticketsDetail.comment.addTitle"
                defaultMessage="添加评论"
              />
            </Divider>
            <Space orientation="vertical" style={{ width: '100%' }}>
              <Input.TextArea
                rows={4}
                value={cmt}
                onChange={(e) => setCmt(e.target.value)}
                placeholder={intl.formatMessage({
                  id: 'pages.ticketsDetail.comment.placeholder',
                  defaultMessage: '输入评论内容',
                })}
              />
              <Upload<AntUploadFile>
                fileList={files}
                listType="picture"
                customRequest={async (opts) => {
                  await handleUpload(opts.file as File);
                }}
                onRemove={(file) => {
                  setFiles((prev) => prev.filter((f) => f.uid !== file.uid));
                  return true;
                }}
              >
                <Button>
                  <FormattedMessage
                    id="pages.ticketsDetail.comment.uploadAttachment"
                    defaultMessage="上传附件"
                  />
                </Button>
              </Upload>
              <Space>
                <Button
                  type="primary"
                  loading={submittingCmt}
                  disabled={!cmt.trim()}
                  onClick={() => void submitComment()}
                >
                  <FormattedMessage
                    id="pages.ticketsDetail.comment.submit"
                    defaultMessage="提交评论"
                  />
                </Button>
                <Button
                  onClick={() => {
                    setCmt('');
                    setFiles([]);
                  }}
                >
                  <FormattedMessage id="pages.ticketsDetail.comment.clear" defaultMessage="清空" />
                </Button>
              </Space>
            </Space>
          </>
        )}

        <Modal
          title={intl.formatMessage({
            id: 'pages.ticketsDetail.transition.modalTitle',
            defaultMessage: '工单流转',
          })}
          open={transOpen}
          onOk={doTransition}
          onCancel={() => setTransOpen(false)}
        >
          <Space orientation="vertical" style={{ width: '100%' }}>
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.ticketsDetail.transition.statusPlaceholder',
                defaultMessage: '选择状态',
              })}
              value={transStatus}
              onChange={setTransStatus}
              style={{ width: '100%' }}
              options={[
                { label: intl.formatMessage(stTextMap.open), value: 'open' },
                { label: intl.formatMessage(stTextMap.inProgress), value: 'in_progress' },
                { label: intl.formatMessage(stTextMap.resolved), value: 'resolved' },
                { label: intl.formatMessage(stTextMap.closed), value: 'closed' },
              ]}
            />
            <Input.TextArea
              rows={3}
              value={transComment}
              onChange={(e) => setTransComment(e.target.value)}
              placeholder={intl.formatMessage({
                id: 'pages.ticketsDetail.transition.notePlaceholder',
                defaultMessage: '流转备注（可选）',
              })}
            />
          </Space>
        </Modal>
      </Card>
      <ModalForm<TicketPayload>
        title={intl.formatMessage({
          id: 'pages.ticketsDetail.edit.modalTitle',
          defaultMessage: '编辑工单',
        })}
        open={editOpen}
        onOpenChange={setEditOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        layout="vertical"
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.ticketsDetail.edit.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        // 预填收敛为打开时同步确定的 initialValues：表单只消费已注册字段，
        // ticket 携带的 id/createdAt 等多余键不会进入提交值
        initialValues={ticket ?? undefined}
        onFinish={onFinish}
      >
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.title',
            defaultMessage: '标题',
          })}
          name="title"
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.ticketsDetail.form.titleRequired',
                defaultMessage: '请输入标题',
              }),
            },
          ]}
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.content',
            defaultMessage: '内容',
          })}
          name="content"
        >
          {' '}
          <Input.TextArea rows={4} />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.category',
            defaultMessage: '分类',
          })}
          name="category"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.priority',
            defaultMessage: '优先级',
          })}
          name="priority"
        >
          {' '}
          <Select
            options={[
              { label: intl.formatMessage(priTextMap.low), value: 'low' },
              { label: intl.formatMessage(priTextMap.normal), value: 'normal' },
              { label: intl.formatMessage(priTextMap.high), value: 'high' },
              { label: intl.formatMessage(priTextMap.urgent), value: 'urgent' },
            ]}
          />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.status',
            defaultMessage: '状态',
          })}
          name="status"
        >
          {' '}
          <Select
            options={[
              { label: intl.formatMessage(stTextMap.open), value: 'open' },
              { label: intl.formatMessage(stTextMap.inProgress), value: 'in_progress' },
              { label: intl.formatMessage(stTextMap.resolved), value: 'resolved' },
              { label: intl.formatMessage(stTextMap.closed), value: 'closed' },
            ]}
          />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.assignee',
            defaultMessage: '处理人',
          })}
          name="assignee"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.tags',
            defaultMessage: '标签',
          })}
          name="tags"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.playerId',
            defaultMessage: '玩家ID',
          })}
          name="playerId"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.contact',
            defaultMessage: '联系方式',
          })}
          name="contact"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.gameId',
            defaultMessage: '游戏',
          })}
          name="gameId"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.env',
            defaultMessage: '环境',
          })}
          name="env"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.ticketsDetail.field.source',
            defaultMessage: '来源',
          })}
          name="source"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
      </ModalForm>
    </>
  );
}
