import React, { useEffect, useState } from 'react';
import { Card, Space, Tag, Button, Descriptions, Divider, List, Input, Upload, Modal, Select, Form } from 'antd';
import { useParams, history, useModel } from '@umijs/max';
import { uploadAsset } from '@/services/croupier/storage';
import { getMessage } from '@/utils/antdApp';
import { updateTicket, deleteTicket, getTicket, listTicketComments, addTicketComment, transitionTicket } from '@/services/croupier/support';

export default function TicketDetailPage() {
  const { id } = useParams() as any;
  const mid = String(id||'');
  const [loading, setLoading] = useState(false);
  const [ticket, setTicket] = useState<any>(null);
  const [comments, setComments] = useState<any[]>([]);
  const [cmt, setCmt] = useState<string>('');
  const [files, setFiles] = useState<any[]>([]);
  const [transOpen, setTransOpen] = useState(false);
  const [transStatus, setTransStatus] = useState<string>('');
  const [transComment, setTransComment] = useState<string>('');
  const [editOpen, setEditOpen] = useState(false);
  const [form] = Form.useForm();
  const { initialState } = useModel('@@initialState');

  const priTag = (v?: string) => {
    const map:any = { urgent: 'red', high: 'volcano', normal: 'blue', low: 'default' };
    const t:any = { urgent: '紧急', high: '高', normal: '普通', low: '低' };
    return v ? <Tag color={map[v]||'default'}>{t[v]||v}</Tag> : '-';
  };
  const stTag = (v?: string) => {
    const map:any = { open: 'gold', in_progress: 'blue', resolved: 'green', closed: 'default' };
    const t:any = { open: '打开', in_progress: '处理中', resolved: '已解决', closed: '已关闭' };
    return v ? <Tag color={map[v]||'default'}>{t[v]||v}</Tag> : '-';
  };

  const load = async () => {
    setLoading(true);
    try {
      const [t, cm] = await Promise.all([getTicket(mid), listTicketComments(mid)]);
      setTicket(t);
      setComments(cm.comments||[]);
    } finally { setLoading(false); }
  };
  useEffect(()=>{ if (mid) load(); }, [mid]);

  const submitComment = async () => {
    try {
      const attach = files.map((f:any)=> ({ name: f.name, url: f.url||f.response?.URL, key: f.response?.Key })).filter((x:any)=> x.url);
      await addTicketComment(mid, { content: cmt, attach: JSON.stringify(attach) });
      setCmt(''); setFiles([]); load();
    } catch (e:any) { getMessage()?.error(e?.message||'评论失败'); }
  };

  const doTransition = async () => {
    try {
      await transitionTicket(mid, { status: transStatus, comment: transComment ? transComment : undefined });
      setTransOpen(false); setTransStatus(''); setTransComment(''); load();
    } catch (e:any) { getMessage()?.error(e?.message||'流转失败'); }
  };

  const openEdit = () => {
    if (!ticket) return;
    form.setFieldsValue({
      title: ticket.title, content: ticket.content, category: ticket.category, priority: ticket.priority,
      status: ticket.status, assignee: ticket.assignee, tags: ticket.tags, player_id: ticket.player_id,
      contact: ticket.contact, game_id: ticket.game_id, env: ticket.env, source: ticket.source,
    });
    setEditOpen(true);
  };
  const submitEdit = async () => {
    const v = await form.validateFields();
    try {
      await updateTicket(Number(mid), v);
      setEditOpen(false);
      load();
    } catch (e:any) { getMessage()?.error(e?.message||'更新失败'); }
  };
  const doDelete = async () => {
    Modal.confirm({ title:'删除工单', content:'确定删除该工单？', onOk: async()=> { await deleteTicket(Number(mid)); history.push('/support/tickets'); } });
  };
  const assignToMe = async () => {
    try {
      const me = (initialState as any)?.currentUser?.name as string | undefined;
      if (!me) { getMessage()?.warning('未获取到当前用户'); return; }
      await updateTicket(Number(mid), { assignee: me });
      getMessage()?.success('已指派给我');
      load();
    } catch (e:any) { getMessage()?.error(e?.message||'指派失败'); }
  };

  if (!mid) return null;

  return (
    <>
    <Card loading={loading} title={<Space>
      <Button onClick={()=> history.back()}>&lt; 返回</Button>
      <span>工单详情 #{mid}</span>
    </Space>} extra={<Space>
      <Button onClick={assignToMe}>指派给我</Button>
      <Button onClick={openEdit}>编辑工单</Button>
      <Button danger onClick={doDelete}>删除工单</Button>
      <Button onClick={()=> setTransOpen(true)}>流转</Button>
    </Space>}>
      {ticket && (
        <>
          <Descriptions bordered column={2} size="small">
            <Descriptions.Item label="标题" span={2}>{ticket.title}</Descriptions.Item>
            <Descriptions.Item label="状态">{stTag(ticket.status)}</Descriptions.Item>
            <Descriptions.Item label="优先级">{priTag(ticket.priority)}</Descriptions.Item>
            <Descriptions.Item label="处理人">{ticket.assignee||'-'}</Descriptions.Item>
            <Descriptions.Item label="标签">{ticket.tags||'-'}</Descriptions.Item>
            <Descriptions.Item label="玩家ID">{ticket.player_id||'-'}</Descriptions.Item>
            <Descriptions.Item label="联系方式" span={2}>{ticket.contact||'-'}</Descriptions.Item>
            <Descriptions.Item label="游戏/环境">{(ticket.game_id||'')+'/'+(ticket.env||'')}</Descriptions.Item>
            <Descriptions.Item label="来源">{ticket.source||'-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{ticket.created_at ? new Date(ticket.created_at).toLocaleString(): '-'}</Descriptions.Item>
            <Descriptions.Item label="更新时间">{ticket.updated_at ? new Date(ticket.updated_at).toLocaleString(): '-'}</Descriptions.Item>
            <Descriptions.Item label="内容" span={2}><div style={{ whiteSpace:'pre-wrap' }}>{ticket.content||'-'}</div></Descriptions.Item>
          </Descriptions>

          <Divider>评论</Divider>
          <List
            dataSource={comments}
            renderItem={(it:any)=>{
              let attachments: any[] = [];
              try { if (it.attach) attachments = JSON.parse(it.attach); } catch {}
              return (
                <List.Item>
                  <List.Item.Meta
                    title={<Space>
                      <strong>{it.author||'-'}</strong>
                      <span>{it.created_at ? new Date(it.created_at).toLocaleString() : ''}</span>
                    </Space>}
                    description={<div style={{ whiteSpace:'pre-wrap' }}>{it.content||''}</div>}
                  />
                  <div>
                    {attachments && attachments.length>0 && (
                      <div>
                        {attachments.map((a:any, idx:number)=> {
                          const url = a.url as string;
                          const isImg = /\.(png|jpe?g|gif|webp|bmp|svg)(\?.*)?$/i.test(url);
                          return (
                            <div key={idx} style={{ marginBottom: 6 }}>
                              {isImg ? (
                                <img src={url} alt={a.name||''} style={{ maxWidth: 160, maxHeight: 120, cursor:'pointer', border:'1px solid #eee', padding:2 }} onClick={()=> window.open(url, '_blank')} />
                              ) : (
                                <a href={url} target="_blank" rel="noreferrer">{a.name||url}</a>
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
          <Divider>添加评论</Divider>
          <Space direction="vertical" style={{ width:'100%' }}>
            <Input.TextArea rows={4} value={cmt} onChange={(e)=> setCmt(e.target.value)} placeholder="输入评论内容" />
            <Upload
              fileList={files as any}
              listType="picture"
              customRequest={async (opts:any)=> {
                try {
                  const res = await uploadAsset(opts.file as File);
                  const next = { uid: String(Date.now()), name: opts.file.name, status:'done', url: res.URL, response: res };
                  setFiles(prev => [...prev, next]);
                  opts.onSuccess && opts.onSuccess(res, opts.file);
                } catch (e:any) {
                  getMessage()?.error(e?.message||'上传失败');
                  opts.onError && opts.onError(e);
                }
              }}
              onRemove={(file)=> { setFiles(prev => prev.filter((f:any)=> f.uid!==file.uid)); return true; }}
            >
              <Button>上传附件</Button>
            </Upload>
            <Space>
              <Button type="primary" onClick={submitComment}>提交评论</Button>
              <Button onClick={()=> { setCmt(''); setFiles([]); }}>清空</Button>
            </Space>
          </Space>
        </>
      )}

      <Modal title="工单流转" open={transOpen} onOk={doTransition} onCancel={()=> setTransOpen(false)}>
        <Space direction="vertical" style={{ width:'100%' }}>
          <Select placeholder="选择状态" value={transStatus} onChange={setTransStatus} style={{ width:'100%' }} options={[
            {label:'打开',value:'open'},{label:'处理中',value:'in_progress'},{label:'已解决',value:'resolved'},{label:'已关闭',value:'closed'}
          ]} />
          <Input.TextArea rows={3} value={transComment} onChange={(e)=> setTransComment(e.target.value)} placeholder="流转备注（可选）" />
        </Space>
      </Modal>
    </Card>
    <Modal title="编辑工单" open={editOpen} onOk={submitEdit} onCancel={()=> setEditOpen(false)} destroyOnHidden>
      <Form form={form} layout="vertical">
        <Form.Item label="标题" name="title" rules={[{ required:true, message:'请输入标题' }]}> <Input /> </Form.Item>
        <Form.Item label="内容" name="content"> <Input.TextArea rows={4} /> </Form.Item>
        <Form.Item label="分类" name="category"> <Input /> </Form.Item>
        <Form.Item label="优先级" name="priority"> <Select options={[{label:'低',value:'low'},{label:'普通',value:'normal'},{label:'高',value:'high'},{label:'紧急',value:'urgent'}]} /> </Form.Item>
        <Form.Item label="状态" name="status"> <Select options={[{label:'打开',value:'open'},{label:'处理中',value:'in_progress'},{label:'已解决',value:'resolved'},{label:'已关闭',value:'closed'}]} /> </Form.Item>
        <Form.Item label="处理人" name="assignee"> <Input /> </Form.Item>
        <Form.Item label="标签" name="tags"> <Input /> </Form.Item>
        <Form.Item label="玩家ID" name="player_id"> <Input /> </Form.Item>
        <Form.Item label="联系方式" name="contact"> <Input /> </Form.Item>
        <Form.Item label="游戏" name="game_id"> <Input /> </Form.Item>
        <Form.Item label="环境" name="env"> <Input /> </Form.Item>
        <Form.Item label="来源" name="source"> <Input /> </Form.Item>
      </Form>
    </Modal>
    </>
  );
}
