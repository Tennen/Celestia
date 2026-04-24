import { Activity, Bot, ChevronDown, ChevronLeft, ChevronRight, Home, Layers3, MessagesSquare, PlugZap, RefreshCw, Share2, Smartphone } from 'lucide-react';
import type { CapabilitySummary } from '../../lib/types';
import type { AppSection } from '../../lib/admin';
import { agentPanelItems, type AgentPanelId } from '../../lib/agent-admin';
import { capabilityDisplayName, summaryNumber } from '../../lib/capability';
import { workflowPageItems, type WorkflowPageId } from '../../lib/workflow-admin';
import { cn } from '../../lib/utils';
import { getApiBase } from '../../lib/api';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { ScrollArea } from '../ui/scroll-area';

type Props = {
  activeSection: AppSection;
  activeAgentPanel: AgentPanelId;
  activeWorkflowPage: WorkflowPageId;
  selectedCapabilityId: string;
  agentExpanded: boolean;
  workflowExpanded: boolean;
  capabilitiesExpanded: boolean;
  sidemenuCondensed: boolean;
  catalogCount: number;
  pluginCount: number;
  deviceCount: number;
  activityCount: number;
  capabilities: CapabilitySummary[];
  error: string | null;
  runtimeBadgeText: string;
  runtimeBadgeTone: 'accent' | 'bad' | 'good';
  onOpenSection: (sectionId: AppSection) => void;
  onOpenAgentPanel: (panelId: AgentPanelId) => void;
  onOpenWorkflowPage: (page: WorkflowPageId) => void;
  onOpenCapability: (capabilityId: string) => void;
  onToggleAgentExpanded: () => void;
  onToggleWorkflowExpanded: () => void;
  onToggleCapabilitiesExpanded: () => void;
  onToggleSidemenuCondensed: () => void;
  onRefresh: () => void;
  refreshing: boolean;
};

export function AppSidemenu(props: Props) {
  const {
    activeSection,
    activeAgentPanel,
    activeWorkflowPage,
    selectedCapabilityId,
    agentExpanded,
    workflowExpanded,
    capabilitiesExpanded,
    sidemenuCondensed,
    catalogCount,
    pluginCount,
    deviceCount,
    activityCount,
    capabilities,
    error,
    runtimeBadgeText,
    runtimeBadgeTone,
    onOpenSection,
    onOpenAgentPanel,
    onOpenWorkflowPage,
    onOpenCapability,
    onToggleAgentExpanded,
    onToggleWorkflowExpanded,
    onToggleCapabilitiesExpanded,
    onToggleSidemenuCondensed,
    onRefresh,
    refreshing,
  } = props;

  const capabilityItemCount = (capability: CapabilitySummary) =>
    capability.kind === 'automation' ? summaryNumber(capability, 'total') : summaryNumber(capability, 'rule_count');

  return (
    <aside className={cn('sidemenu', sidemenuCondensed && 'is-condensed')}>
      <div className="sidemenu__brand">
        <div className="sidemenu__brand-row">
          <div>
            <p className="eyebrow">Celestia Core Runtime</p>
            <h1>Admin Console</h1>
          </div>
          <Button variant="secondary" size="icon" className="sidemenu__compact-toggle" onClick={onToggleSidemenuCondensed}>
            {sidemenuCondensed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
          </Button>
        </div>
        <p className="topbar__sub">
          Real plugin orchestration, device control, and runtime inspection against <code>{getApiBase()}</code>
        </p>
        <div className="sidemenu__meta">
          <Badge tone="accent" size="xs">
            Gateway API
          </Badge>
          <Badge tone={runtimeBadgeTone} size="xs">
            {runtimeBadgeText}
          </Badge>
        </div>
      </div>

      <nav className="sidemenu__nav">
        <ScrollArea className="sidemenu__nav-scroll">
          <div className="sidemenu__nav-content">
            <NavButton icon={Home} label="Overview" count={pluginCount} active={activeSection === 'overview'} onClick={() => onOpenSection('overview')} />
            <NavButton icon={PlugZap} label="Plugins" count={catalogCount} active={activeSection === 'plugins'} onClick={() => onOpenSection('plugins')} />

            <div className={cn('sidemenu__group', activeSection === 'workflow' && 'is-active')}>
              <div className={cn('sidemenu__button', activeSection === 'workflow' && 'is-active', 'sidemenu__button--group')}>
                <button type="button" className="sidemenu__button-main" onClick={() => onOpenSection('workflow')}>
                  <span className="flex items-center gap-3">
                    <Share2 className="h-4 w-4" />
                    Workflow
                  </span>
                  <Badge tone={activeSection === 'workflow' ? 'accent' : 'neutral'} size="xs" className="min-w-6 tabular-nums">
                    {workflowPageItems.length}
                  </Badge>
                </button>
                <button
                  type="button"
                  className={cn('sidemenu__disclosure', workflowExpanded && 'is-open')}
                  aria-label={workflowExpanded ? 'Collapse workflow' : 'Expand workflow'}
                  aria-expanded={workflowExpanded}
                  onClick={onToggleWorkflowExpanded}
                >
                  <ChevronDown className="h-4 w-4" />
                </button>
              </div>

              {workflowExpanded ? (
                <div className="sidemenu__submenu">
                  {workflowPageItems.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      className={cn('sidemenu__subbutton', activeSection === 'workflow' && activeWorkflowPage === item.id && 'is-active')}
                      onClick={() => onOpenWorkflowPage(item.id)}
                    >
                      <span className="sidemenu__subbutton-label">{item.label}</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>

            <NavButton icon={MessagesSquare} label="Touchpoints" count={3} active={activeSection === 'touchpoints'} onClick={() => onOpenSection('touchpoints')} />

            <div className={cn('sidemenu__group', activeSection === 'agent' && 'is-active')}>
              <div className={cn('sidemenu__button', activeSection === 'agent' && 'is-active', 'sidemenu__button--group')}>
                <button type="button" className="sidemenu__button-main" onClick={() => onOpenSection('agent')}>
                  <span className="flex items-center gap-3">
                    <Bot className="h-4 w-4" />
                    Agent
                  </span>
                  <Badge tone={activeSection === 'agent' ? 'accent' : 'neutral'} size="xs" className="min-w-6 tabular-nums">
                    {agentPanelItems.length}
                  </Badge>
                </button>
                <button
                  type="button"
                  className={cn('sidemenu__disclosure', agentExpanded && 'is-open')}
                  aria-label={agentExpanded ? 'Collapse agent' : 'Expand agent'}
                  aria-expanded={agentExpanded}
                  onClick={onToggleAgentExpanded}
                >
                  <ChevronDown className="h-4 w-4" />
                </button>
              </div>

              {agentExpanded ? (
                <div className="sidemenu__submenu">
                  {agentPanelItems.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      className={cn('sidemenu__subbutton', activeSection === 'agent' && activeAgentPanel === item.id && 'is-active')}
                      onClick={() => onOpenAgentPanel(item.id)}
                    >
                      <span className="sidemenu__subbutton-label">{item.label}</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>

            <div className={cn('sidemenu__group', activeSection === 'capabilities' && 'is-active')}>
              <div className={cn('sidemenu__button', activeSection === 'capabilities' && 'is-active', 'sidemenu__button--group')}>
                <button type="button" className="sidemenu__button-main" onClick={() => onOpenSection('capabilities')}>
                  <span className="flex items-center gap-3">
                    <Layers3 className="h-4 w-4" />
                    Capabilities
                  </span>
                  <Badge tone={activeSection === 'capabilities' ? 'accent' : 'neutral'} size="xs" className="min-w-6 tabular-nums">
                    {capabilities.length}
                  </Badge>
                </button>
                <button
                  type="button"
                  className={cn('sidemenu__disclosure', capabilitiesExpanded && 'is-open')}
                  aria-label={capabilitiesExpanded ? 'Collapse capabilities' : 'Expand capabilities'}
                  aria-expanded={capabilitiesExpanded}
                  onClick={onToggleCapabilitiesExpanded}
                  disabled={capabilities.length === 0}
                >
                  <ChevronDown className="h-4 w-4" />
                </button>
              </div>

              {capabilitiesExpanded ? (
                <div className="sidemenu__submenu">
                  {capabilities.map((capability) => (
                    <button
                      key={capability.id}
                      type="button"
                      className={cn('sidemenu__subbutton', activeSection === 'capabilities' && selectedCapabilityId === capability.id && 'is-active')}
                      onClick={() => onOpenCapability(capability.id)}
                    >
                      <span className="sidemenu__subbutton-label">{capabilityDisplayName(capability)}</span>
                      <Badge tone={activeSection === 'capabilities' && selectedCapabilityId === capability.id ? 'accent' : 'neutral'} size="xs" className="min-w-6 tabular-nums">
                        {capabilityItemCount(capability)}
                      </Badge>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>

            <NavButton icon={Smartphone} label="Devices" count={deviceCount} active={activeSection === 'devices'} onClick={() => onOpenSection('devices')} />
            <NavButton icon={Activity} label="Activity" count={activityCount} active={activeSection === 'activity'} onClick={() => onOpenSection('activity')} />
          </div>
        </ScrollArea>
      </nav>

      <div className="sidemenu__footer">
        <Badge tone={error ? 'bad' : 'good'} size="xs">
          {error ? 'Degraded' : 'Connected'}
        </Badge>
        <Button
          variant="secondary"
          onClick={onRefresh}
          aria-busy={refreshing}
          className="min-w-[7.25rem] border-slate-700 bg-white/10 text-white hover:bg-white/15 hover:text-white"
        >
          <RefreshCw className={cn('mr-2 h-4 w-4', refreshing && 'animate-spin')} />
          Refresh
        </Button>
      </div>
    </aside>
  );
}

function NavButton(props: { icon: typeof Home; label: string; count: number; active: boolean; onClick: () => void }) {
  const { icon: Icon, label, count, active, onClick } = props;
  return (
    <button type="button" className={`sidemenu__button ${active ? 'is-active' : ''}`} onClick={onClick}>
      <span className="flex items-center gap-3">
        <Icon className="h-4 w-4" />
        {label}
      </span>
      <Badge tone={active ? 'accent' : 'neutral'} size="xs" className="min-w-6 tabular-nums">
        {count}
      </Badge>
    </button>
  );
}
