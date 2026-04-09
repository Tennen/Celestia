import { Card, CardContent } from '../ui/card';
import { useAdminStore } from '../../stores/adminStore';
import { AutomationWorkspace } from './AutomationWorkspace';
import { VisionCapabilityPanel } from './VisionCapabilityPanel';

type Props = {
  selectedCapabilityId: string;
};

export function CapabilityWorkspace({ selectedCapabilityId }: Props) {
  const { capabilities, devices, reportError } = useAdminStore();
  const selectedCapability =
    capabilities.find((capability) => capability.id === selectedCapabilityId) ?? capabilities[0] ?? null;

  if (!selectedCapability) {
    return (
      <Card>
        <CardContent className="pt-6">
          <p className="muted">No capabilities reported by the gateway yet.</p>
        </CardContent>
      </Card>
    );
  }

  if (selectedCapability.id === 'automation') {
    return <AutomationWorkspace />;
  }

  if (selectedCapability.id === 'vision_entity_stay_zone') {
    return <VisionCapabilityPanel summary={selectedCapability} devices={devices} onError={reportError} />;
  }

  return (
    <Card>
      <CardContent className="pt-6">
        <p className="muted">{selectedCapability.name} does not have an admin workspace yet.</p>
      </CardContent>
    </Card>
  );
}
