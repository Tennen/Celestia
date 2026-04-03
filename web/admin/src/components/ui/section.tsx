import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/utils';

type Props = HTMLAttributes<HTMLElement> & {
  stack?: boolean;
};

export function Section({ className, stack = true, ...props }: Props) {
  return <section className={cn(stack && 'space-y-6', className)} {...props} />;
}
