import type { ReactNode } from 'react';
import { CardDescription, CardTitle } from '../../ui/card';
import { cn } from '../../../lib/utils';

type Props = {
  title: ReactNode;
  description?: ReactNode;
  aside?: ReactNode;
  className?: string;
};

export function CardHeading({ title, description, aside, className }: Props) {
  return (
    <div className={cn('section-title', className)}>
      <div>
        <CardTitle>{title}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </div>
      {aside}
    </div>
  );
}
