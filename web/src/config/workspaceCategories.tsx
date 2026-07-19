import {
  UserOutlined,
  GiftOutlined,
  ShoppingOutlined,
  DollarOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons';
import type { WorkspaceCategory } from '@/types/workspace';

export interface WorkspaceCategoryConfig {
  name: string;
  icon: React.ReactNode;
  order: number;
}

export const WORKSPACE_CATEGORIES: Record<WorkspaceCategory, WorkspaceCategoryConfig> = {
  player: {
    name: '玩家管理',
    icon: <UserOutlined />,
    order: 1,
  },
  inventory: {
    name: '物品管理',
    icon: <GiftOutlined />,
    order: 2,
  },
  order: {
    name: '订单管理',
    icon: <ShoppingOutlined />,
    order: 3,
  },
  economy: {
    name: '经济系统',
    icon: <DollarOutlined />,
    order: 4,
  },
  social: {
    name: '社交系统',
    icon: <TeamOutlined />,
    order: 5,
  },
  other: {
    name: '其他工具',
    icon: <ToolOutlined />,
    order: 99,
  },
};

export function getCategoryConfig(category?: WorkspaceCategory) {
  return WORKSPACE_CATEGORIES[category || 'other'] || WORKSPACE_CATEGORIES.other;
}
