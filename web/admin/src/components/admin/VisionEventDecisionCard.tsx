import { prettyJson } from '../../lib/utils';
import { visionDecisionFromMetadata } from '../../lib/visionDecision';
import { AggregatedInfoCard } from './shared/AggregatedInfoCard';

type Props = {
  metadata: unknown;
};

export function VisionEventDecisionCard({ metadata }: Props) {
  const decision = visionDecisionFromMetadata(metadata);
  if (!decision) {
    return null;
  }

  const items = [
    decision.source ? { label: 'Source', value: decision.source } : null,
    typeof decision.confidenceScore === 'number'
      ? { label: 'Confidence', value: decision.confidenceScore.toFixed(2), title: String(decision.confidenceScore) }
      : null,
    decision.semanticVerdict ? { label: 'Semantic Verdict', value: decision.semanticVerdict } : null,
  ].filter((item): item is NonNullable<typeof item> => item !== null);

  return (
    <div className="vision-history-detail__payload">
      <span className="muted">Decision Summary</span>
      <AggregatedInfoCard items={items} />
      {decision.confidenceBreakdown ? (
        <>
          <span className="muted">Confidence Breakdown</span>
          <pre className="vision-history-detail__json">{prettyJson(decision.confidenceBreakdown)}</pre>
        </>
      ) : null}
      {decision.semanticDetails ? (
        <>
          <span className="muted">Semantic Checker</span>
          <pre className="vision-history-detail__json">{prettyJson(decision.semanticDetails)}</pre>
        </>
      ) : null}
    </div>
  );
}
