import type { ReactNode } from 'react';
import { cn } from '../../../lib/utils';

export type AggregatedInfoCardItem = {
  label: string;
  title?: string;
  value: ReactNode;
};

type Props = {
  className?: string;
  items: AggregatedInfoCardItem[];
};

export function AggregatedInfoCard({ className, items }: Props) {
  if (items.length === 0) {
    return null;
  }

  return (
    <div className={cn('aggregated-info-card', className)}>
      {items.map((item, index) => {
        const title = item.title ?? (typeof item.value === 'string' ? item.value : undefined);
        return (
          <div key={`${item.label}-${index}`} className="aggregated-info-card__item">
            <span className="aggregated-info-card__label">{item.label}</span>
            <strong className="aggregated-info-card__value" title={title}>
              {item.value}
            </strong>
          </div>
        );
      })}
    </div>
  );
}
