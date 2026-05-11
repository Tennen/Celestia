import { request } from './api';
import type { AgentApprovalRequest } from './agent';

export function runEvolutionOperation(payload: { action: string; goal_id?: string; commit_message?: string }) {
  return request<Record<string, unknown>>('/agent/evolution/ops/run', { method: 'POST', body: JSON.stringify(payload) });
}

export function createAgentApproval(payload: { kind?: string; action: string; goal_id?: string; title?: string; detail?: string; requested_by?: string }) {
  return request<AgentApprovalRequest>('/agent/approvals', { method: 'POST', body: JSON.stringify(payload) });
}

export function approveAgentApproval(id: string, payload: { actor?: string; note?: string } = {}) {
  return request<AgentApprovalRequest>(`/agent/approvals/${encodeURIComponent(id)}/approve`, { method: 'POST', body: JSON.stringify(payload) });
}

export function rejectAgentApproval(id: string, payload: { actor?: string; note?: string } = {}) {
  return request<AgentApprovalRequest>(`/agent/approvals/${encodeURIComponent(id)}/reject`, { method: 'POST', body: JSON.stringify(payload) });
}

export function runAgentScreenshot(payload: { url: string; width?: number; height?: number; full_page?: boolean; wait_ms?: number; output_dir?: string }) {
  return request<Record<string, unknown>>('/agent/screenshot', { method: 'POST', body: JSON.stringify(payload) });
}

export function runAgentServiceOperation(payload: { action: string; lines?: number }) {
  return request<Record<string, unknown>>('/agent/service/ops', { method: 'POST', body: JSON.stringify(payload) });
}
