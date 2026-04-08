import { useEffect, useState } from 'react';
import { Badge } from '../ui/badge';
import { Card, CardContent } from '../ui/card';
import { ScrollArea } from '../ui/scroll-area';
import { Section } from '../ui/section';
import { summaryNumber, summaryString } from '../../lib/capability';
import { useAdminStore } from '../../stores/adminStore';
import { AutomationWorkspace } from './AutomationWorkspace';
import { SelectableListItem } from './shared/SelectableListItem';
import { VisionCapabilityPanel } from './VisionCapabilityPanel';

export function CapabilityWorkspace() {
  const { capabilities, devices, reportError } = useAdminStore();
  const [selectedCapabilityId, setSelectedCapabilityId] = useState('');

  useEffect(() => {
    if (!capabilities.length) {
      setSelectedCapabilityId('');
      return;
    }
    setSelectedCapabilityId((current) =>
      capabilities.some((capability) => capability.id === current) ? current : capabilities[0].id,
    );
  }, [capabilities]);

  const selectedCapability = capabilities.find((capability) => capability.id === selectedCapabilityId) ?? null;

  return (
    <Section stack={false} className="capability-workspace">
      <Card className="plugin-explorer explorer-card">
        <CardContent className="explorer-card__content pt-6">
          <ScrollArea className="explorer-scroll">
            <div className="list-stack">
              {capabilities.map((capability) => (
                <SelectableListItem
                  key={capability.id}
                  selected={capability.id === selectedCapabilityId}
                  onClick={() => setSelectedCapabilityId(capability.id)}
                  title={capability.name}
                  description={capability.description}
                  badges={
                    <>
                      <Badge tone={capability.enabled ? 'good' : 'neutral'} size="xs">
                        {capability.enabled ? 'enabled' : 'disabled'}
                      </Badge>
                      <Badge tone={capability.status === 'healthy' ? 'good' : capability.status === 'stopped' ? 'neutral' : 'warn'} size="xs">
                        {capability.status}
                      </Badge>
                    </>
                  }
                  support={
                    capability.kind === 'automation' ? (
                      <span className="muted">{summaryNumber(capability, 'total')} automations</span>
                    ) : (
                      <span className="muted">
                        {summaryNumber(capability, 'enabled_rule_count')}/{summaryNumber(capability, 'rule_count')} rules ·{' '}
                        {summaryString(capability, 'service_url') || 'service unset'}
                      </span>
                    )
                  }
                />
              ))}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      <div className="capability-detail">
        {selectedCapability?.id === 'automation' ? <AutomationWorkspace /> : null}
        {selectedCapability?.id === 'vision_entity_stay_zone' ? (
          <VisionCapabilityPanel summary={selectedCapability} devices={devices} onError={reportError} />
        ) : null}
      </div>
    </Section>
  );
}
