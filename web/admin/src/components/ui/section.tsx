import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/utils';

export function Section({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return <section className={cn('space-y-6', className)} {...props} />;
}
