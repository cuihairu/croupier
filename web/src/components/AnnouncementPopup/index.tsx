import React, { useCallback, useEffect, useState } from 'react';
import { Modal, Tag, Typography } from 'antd';
import { request, useIntl } from '@umijs/max';

/**
 * 登录后公告弹窗（OPEN-ISSUES #17）。
 *
 * 后端 /announcements/active 一直存在（announcement.Active），管理端也能勾选
 * 「登录弹出」，但前端此前**没有任何消费方**——popup 公告从未真正弹出。
 *
 * 行为：
 * - 登录态挂载（app.tsx childrenRender 里 isAuthed 时渲染）后拉取 active 列表；
 * - shouldPopup=true 的公告逐条 Modal 展示；
 * - 确认后 POST /announcements/:id/dismiss（服务端按用户幂等落 announcement_reads，
 *   下次登录不再弹）；dismiss 失败时 sessionStorage 兜底，本会话不再重复弹。
 */
type ActiveAnnouncement = {
  id: number;
  title: string;
  contentMd: string;
  audience: string;
  shouldPopup: boolean;
};

// 服务端 dismiss 是权威去重；sessionStorage 只在 dismiss 请求失败时兜底，
// 避免同一次会话里反复弹同一条。
const SESSION_SHOWN_KEY = 'announcement_popup_shown';

function readShownIds(): number[] {
  try {
    const parsed: unknown = JSON.parse(sessionStorage.getItem(SESSION_SHOWN_KEY) || '[]');
    return Array.isArray(parsed) ? parsed.filter((v): v is number => typeof v === 'number') : [];
  } catch {
    return [];
  }
}

function markShown(id: number): void {
  try {
    sessionStorage.setItem(SESSION_SHOWN_KEY, JSON.stringify([...readShownIds(), id]));
  } catch {
    // 存储不可用时跳过兜底（最坏情况：下次进页面再弹一次）
  }
}

export default function AnnouncementPopup() {
  const intl = useIntl();
  const [queue, setQueue] = useState<ActiveAnnouncement[]>([]);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const resp = await request<{ items?: ActiveAnnouncement[] }>(
          '/api/v1/announcements/active',
          { method: 'GET' },
        );
        if (cancelled) return;
        const shown = readShownIds();
        setQueue(
          (resp?.items || []).filter((item) => item.shouldPopup && !shown.includes(item.id)),
        );
      } catch {
        // 公告是增强信息：拉取失败（未授权/网络）保持静默，不阻塞页面
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const current = queue[0];

  const handleOk = useCallback(async () => {
    const item = queue[0];
    if (!item) return;
    try {
      await request(`/api/v1/announcements/${item.id}/dismiss`, { method: 'POST' });
    } catch {
      // 服务端确认失败：本会话不再弹（sessionStorage），下次登录服务端仍会下发
    }
    markShown(item.id);
    setQueue((prev) => prev.slice(1));
  }, [queue]);

  // Modal 的 onCancel 与 onOk 同义：公告只能「知道了」，不能拒绝后不再出现
  return (
    <Modal
      open={Boolean(current)}
      title={
        <span data-testid="announcement-popup-title">
          {current?.title ||
            intl.formatMessage({ id: 'app.announcement.popup.title', defaultMessage: '系统公告' })}
        </span>
      }
      onOk={() => void handleOk()}
      onCancel={() => void handleOk()}
      okText={intl.formatMessage({ id: 'app.announcement.popup.ack', defaultMessage: '知道了' })}
      cancelButtonProps={{ style: { display: 'none' } }}
      okButtonProps={{ 'data-testid': 'announcement-popup-ack' } as never}
      closable={false}
      keyboard={false}
      mask={{ closable: false }}
      width={560}
    >
      <Typography.Paragraph
        type="secondary"
        style={{ whiteSpace: 'pre-wrap', maxHeight: '50vh', overflow: 'auto', marginBottom: 8 }}
      >
        {current?.contentMd}
      </Typography.Paragraph>
      {current?.audience === 'role' && (
        <Tag color="purple">
          {intl.formatMessage({
            id: 'app.announcement.popup.roleOnly',
            defaultMessage: '面向指定角色',
          })}
        </Tag>
      )}
    </Modal>
  );
}
