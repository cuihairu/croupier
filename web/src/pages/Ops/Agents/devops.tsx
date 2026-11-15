import OpsAgentsPage from '.';
import DevopsSessionsPanel from './DevopsSessionsPanel';

/** devops 实例页：四类 Agent 公共页骨架 + 会话面板（AGENT-003）。
 * 会话面板作为类型职责区块挂在实例列表之后、同一 scope 头之下（§3 共用布局）。 */
export default function DevopsAgentsPage() {
  return (
    <OpsAgentsPage agentType="devops">
      <DevopsSessionsPanel />
    </OpsAgentsPage>
  );
}
