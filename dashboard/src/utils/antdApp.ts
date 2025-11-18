import type { MessageInstance } from 'antd/es/message/interface';
import type { NotificationInstance } from 'antd/es/notification/interface';

// Holds AntD App API instances (message/notification) acquired via App.useApp().
// This allows non-React modules (e.g. requestErrorConfig) to use context-aware APIs
// instead of static message/notification functions to avoid AntD 5 warnings.

export type AppApi = {
  message: MessageInstance;
  notification: NotificationInstance;
};

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

