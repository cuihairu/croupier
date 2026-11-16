/**
 * LandingRedirect - 根路径按角色能力落地（GUX-002/GUX-003）
 *
 * 访问系统时不再落到产品建设说明页（原 Welcome），而是按现有权限
 * 重定向到第一个可进入的工作区：运营 → 运行控制台；开发 → 函数目录；
 * 平台 → 系统管理；全部不可见 → 个人中心（可改密/看消息）。
 * 权限在 bootstrap 时由 profile 派生，此处只读，不发请求。
 */

import { useEffect, useMemo } from 'react';
import { history, useAccess } from '@umijs/max';
import { Spin } from 'antd';

type LandingAccess = {
  canConsoleRead?: boolean;
  canFunctionsRead?: boolean;
  canSystemConfigRead?: boolean;
};

export default function LandingRedirect() {
  const access = useAccess() as LandingAccess;

  const target = useMemo(() => {
    if (access.canConsoleRead) return '/console/home';
    if (access.canFunctionsRead) return '/functions/catalog';
    if (access.canSystemConfigRead) return '/system';
    return '/account';
  }, [access.canConsoleRead, access.canFunctionsRead, access.canSystemConfigRead]);

  useEffect(() => {
    history.replace(target);
  }, [target]);

  return (
    <div style={{ display: 'flex', justifyContent: 'center', padding: '120px 0' }}>
      <Spin size="large" />
    </div>
  );
}
