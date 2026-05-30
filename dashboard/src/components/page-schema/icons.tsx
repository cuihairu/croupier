import React from 'react';
import {
  CheckCircleOutlined,
  CopyOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  ExperimentOutlined,
  HistoryOutlined,
  InfoCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
  SettingOutlined,
  WarningOutlined,
} from '@ant-design/icons';

const iconMap = {
  setting: <SettingOutlined />,
  check: <CheckCircleOutlined />,
  warning: <WarningOutlined />,
  experiment: <ExperimentOutlined />,
  history: <HistoryOutlined />,
  copy: <CopyOutlined />,
  plus: <PlusOutlined />,
  delete: <DeleteOutlined />,
  save: <SaveOutlined />,
  reload: <ReloadOutlined />,
  download: <DownloadOutlined />,
  edit: <EditOutlined />,
  info: <InfoCircleOutlined />,
  play: <PlayCircleOutlined />,
} as const;

export type SchemaIconName = keyof typeof iconMap;

export const resolveSchemaIcon = (icon: string) =>
  iconMap[icon as SchemaIconName] || <SettingOutlined />;
