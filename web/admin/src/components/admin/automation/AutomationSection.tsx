import type { ReactNode } from 'react';
import { cn } from '../../../lib/utils';

type Props = {
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
  children: ReactNode;
};

export function AutomationSection({ title, description, action, className, children }: Props) {
  return (
    <section className={cn('automation-section', className)}>
      <div className="automation-section__header">
        <div className="automation-section__heading">
          <h3 className="automation-section__title">{title}</h3>
          {description ? <p className="muted">{description}</p> : null}
        </div>
        {action}
      </div>
      <div className="automation-section__body">{children}</div>
    </section>
  );
}
