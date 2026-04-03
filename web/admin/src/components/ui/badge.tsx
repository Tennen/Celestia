import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../../lib/utils';

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.18em]',
  {
    variants: {
      tone: {
        neutral: 'border-border bg-secondary text-secondary-foreground',
        good: 'border-success/25 bg-success/10 text-success',
        warn: 'border-warning/25 bg-warning/10 text-amber-700',
        bad: 'border-destructive/25 bg-destructive/10 text-destructive',
        accent: 'border-primary/20 bg-primary/10 text-primary',
      },
    },
    defaultVariants: {
      tone: 'neutral',
    },
  },
);

type Props = React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>;

export function Badge({ className, tone, ...props }: Props) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
