import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { cn } from '../../../lib/utils';

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  title: ReactNode;
  description?: ReactNode;
  badges?: ReactNode;
  layout?: 'default' | 'stacked_badges';
  support?: ReactNode;
  selected?: boolean;
};

export function SelectableListItem({
  title,
  description,
  badges,
  layout = 'default',
  support,
  selected = false,
  className,
  ...props
}: Props) {
  const stackedBadges = layout === 'stacked_badges';

  return (
    <button
      type="button"
      className={cn('table__row space-y-3', selected && 'is-selected', className)}
      {...props}
    >
      <div className={cn('list-item__row', stackedBadges && 'list-item__row--stacked')}>
        <div className="list-item__meta">
          <strong className="list-item__title">{title}</strong>
          {!stackedBadges && description ? <div className="list-item__description">{description}</div> : null}
        </div>
        {!stackedBadges && badges ? <div className="list-item__badges">{badges}</div> : null}
      </div>
      {stackedBadges && (description || badges) ? (
        <div className="list-item__subrow">
          <div className="list-item__description">{description}</div>
          {badges ? <div className="list-item__badges list-item__badges--compact">{badges}</div> : null}
        </div>
      ) : null}
      {support ? <div className="list-item__support">{support}</div> : null}
    </button>
  );
}
