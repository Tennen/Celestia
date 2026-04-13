export type VisionDecisionView = {
  confidenceBreakdown?: Record<string, unknown>;
  confidenceScore?: number;
  raw: Record<string, unknown>;
  semanticDetails?: Record<string, unknown>;
  semanticVerdict?: string;
  source?: string;
};

function readString(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function readNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function readRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

export function visionDecisionFromMetadata(metadata: unknown): VisionDecisionView | null {
  const metadataRecord = readRecord(metadata);
  const decision = readRecord(metadataRecord?.decision);
  if (!decision) {
    return null;
  }

  const semanticDetails =
    readRecord(decision.semantic_checker) ??
    readRecord(decision.semantic_check) ??
    readRecord(decision.semantic_fallback) ??
    undefined;
  const semanticVerdict =
    readString(decision.semantic_verdict) ||
    readString(semanticDetails?.verdict) ||
    readString(semanticDetails?.status) ||
    undefined;

  return {
    confidenceBreakdown: readRecord(decision.confidence_breakdown) ?? undefined,
    confidenceScore: readNumber(decision.confidence_score),
    raw: decision,
    semanticDetails,
    semanticVerdict,
    source: readString(decision.source) || undefined,
  };
}
