import OpsAgentsPage from '.';
import CaptureGateStatus from './CaptureGateStatus';

export default function CaptureAgentsPage() {
  return (
    <OpsAgentsPage agentType="capture">
      <CaptureGateStatus />
    </OpsAgentsPage>
  );
}
