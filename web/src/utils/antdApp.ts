import type { MessageInstance } from 'antd/es/message/interface';
import type { NotificationInstance } from 'antd/es/notification/interface';
import type useApp from 'antd/es/app/useApp';

// Holds AntD App API instances (message/notification/modal) acquired via App.useApp().
// This allows non-React modules (e.g. requestErrorConfig) to use context-aware APIs
// instead of static message/notification/modal functions to avoid AntD 5 warnings
// and get theme/token-aware rendering.

/** 跟随 antd 版本的完整 App API 形状（message/notification/modal） */
export type AppApi = ReturnType<typeof useApp>;

/** modal 实例类型（confirm/info/success/error/warning，App 上下文版本） */
export type ModalHookAPI = AppApi['modal'];

let appApi: AppApi | null = null;

export function setAppApi(api: AppApi) {
  appApi = api;
}

export function getMessage(): MessageInstance | undefined {
  return appApi?.message;
}

export function getNotification(): NotificationInstance | undefined {
  return appApi?.notification;
}

export function getModal(): ModalHookAPI | undefined {
  return appApi?.modal;
}
