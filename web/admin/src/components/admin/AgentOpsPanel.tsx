import { useState } from 'react';
import { Camera, Check, FileText, Power, RefreshCw, X } from 'lucide-react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { runAgentScreenshot, approveAgentApproval, rejectAgentApproval, runAgentServiceOperation } from '../../lib/agent-ops';
import { Field, ToggleField, parseOptionalNumber } from './AgentFormFields';
import type { AgentSnapshot } from '../../lib/agent';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  onRun: AgentRunner;
};

export function AgentOpsPanel({ snapshot, onRun }: Props) {
  const [url, setUrl] = useState('http://localhost:3000');
  const [width, setWidth] = useState('1440');
  const [height, setHeight] = useState('1000');
  const [fullPage, setFullPage] = useState(true);
  const pending = snapshot.approvals.requests.filter((item) => item.status === 'pending');

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Screenshot</CardTitle>
          <CardDescription>Capture loopback web pages with the bundled Playwright runtime</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Field label="URL" value={url} onChange={setUrl} />
          <div className="grid grid-cols-2 gap-3">
            <Field label="Width" value={width} onChange={setWidth} />
            <Field label="Height" value={height} onChange={setHeight} />
          </div>
          <ToggleField label="Full page" checked={fullPage} onChange={setFullPage} />
          <Button
            onClick={() =>
              onRun('screenshot', () =>
                runAgentScreenshot({
                  url,
                  width: parseOptionalNumber(width),
                  height: parseOptionalNumber(height),
                  full_page: fullPage,
                }),
              )
            }
            disabled={!url.trim()}
          >
            <Camera className="mr-2 h-4 w-4" />
            Capture
          </Button>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Approvals</CardTitle>
          <CardDescription>{pending.length} pending remote operation approvals</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          {pending.map((item) => (
            <div key={item.id} className="rounded-md border border-border-light p-3 text-sm">
              <div className="button-row">
                <Badge tone="warn" size="xxs">{item.action}</Badge>
                <span className="text-muted-foreground">{item.id}</span>
              </div>
              <p className="font-medium">{item.title}</p>
              {item.detail ? <p className="text-muted-foreground">{item.detail}</p> : null}
              <div className="button-row">
                <Button variant="secondary" onClick={() => onRun('approval-approve', () => approveAgentApproval(item.id, { actor: 'admin' }))}>
                  <Check className="mr-2 h-4 w-4" />
                  Approve
                </Button>
                <Button variant="danger" onClick={() => onRun('approval-reject', () => rejectAgentApproval(item.id, { actor: 'admin' }))}>
                  <X className="mr-2 h-4 w-4" />
                  Reject
                </Button>
              </div>
            </div>
          ))}
          {pending.length === 0 ? <div className="detail">No pending approvals.</div> : null}
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Gateway Service</CardTitle>
          <CardDescription>Background gateway process managed by tool/celestia-service.sh</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <div className="button-row">
            <Button variant="secondary" onClick={() => onRun('service-status', () => runAgentServiceOperation({ action: 'status' }))}>
              <Power className="mr-2 h-4 w-4" />
              Status
            </Button>
            <Button variant="secondary" onClick={() => onRun('service-logs', () => runAgentServiceOperation({ action: 'logs', lines: 160 }))}>
              <FileText className="mr-2 h-4 w-4" />
              Logs
            </Button>
            <Button variant="secondary" onClick={() => onRun('service-restart', () => runAgentServiceOperation({ action: 'restart' }))}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Restart
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
