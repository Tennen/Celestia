import { describe, expect, it } from 'vitest';
import { visionDecisionFromMetadata } from './visionDecision';

describe('visionDecisionFromMetadata', () => {
  it('extracts readable decision fields from event metadata', () => {
    const decision = visionDecisionFromMetadata({
      decision: {
        source: 'roi_vlm_fallback',
        confidence_score: 0.91,
        confidence_breakdown: {
          detector: 0.52,
          semantic: 0.96,
        },
        semantic_checker: {
          verdict: 'pass',
          prompt: 'Is the cat eating?',
        },
      },
    });

    expect(decision).toEqual({
      confidenceBreakdown: {
        detector: 0.52,
        semantic: 0.96,
      },
      confidenceScore: 0.91,
      raw: {
        source: 'roi_vlm_fallback',
        confidence_score: 0.91,
        confidence_breakdown: {
          detector: 0.52,
          semantic: 0.96,
        },
        semantic_checker: {
          verdict: 'pass',
          prompt: 'Is the cat eating?',
        },
      },
      semanticDetails: {
        verdict: 'pass',
        prompt: 'Is the cat eating?',
      },
      semanticVerdict: 'pass',
      source: 'roi_vlm_fallback',
    });
  });

  it('returns null when metadata has no decision block', () => {
    expect(visionDecisionFromMetadata({ source: 'vision-service' })).toBeNull();
  });
});
