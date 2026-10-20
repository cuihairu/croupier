import OpsAgentsPage from '.';
import SidecarFunctions from './sidecarFunctions';

export default function SidecarAgentsPage() {
  return (
    <OpsAgentsPage agentType="sidecar">
      <SidecarFunctions />
    </OpsAgentsPage>
  );
}
