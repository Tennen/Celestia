import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/utils';

type Props = HTMLAttributes<HTMLSpanElement> & {
  tone?: 'neutral' | 'good' | 'warn' | 'bad' | 'accent';
};

export function Badge({ className, tone = 'neutral', ...props }: Props) {
  return <span className={cn('badge', `badge--${tone}`, className)} {...props} />;
}

