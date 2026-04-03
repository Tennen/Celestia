import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { cn } from '../../../lib/utils';

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  title: ReactNode;
  description?: ReactNode;
  badges?: ReactNode;
  support?: ReactNode;
  selected?: boolean;
};

export function SelectableListItem({
  title,
  description,
  badges,
  support,
  selected = false,
  className,
  ...props
}: Props) {
  return (
    <button
      type="button"
      className={cn('table__row', selected && 'is-selected', className)}
      {...props}
    >
      <div className="list-item__row">
        <div className="list-item__meta">
          <strong>{title}</strong>
          {description ? <div className="text-xs text-muted-foreground">{description}</div> : null}
        </div>
        {badges ? <div className="list-item__badges">{badges}</div> : null}
      </div>
      {support ? <div className="list-item__support">{support}</div> : null}
    </button>
  );
}
