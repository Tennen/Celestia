import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../../lib/utils';

const badgeVariants = cva(
  'inline-flex max-w-full items-center justify-center overflow-hidden rounded-full border font-medium leading-none text-ellipsis whitespace-nowrap',
  {
    variants: {
      tone: {
        neutral: 'border-border bg-secondary text-secondary-foreground',
        good: 'border-success/25 bg-success/10 text-success',
        warn: 'border-warning/25 bg-warning/10 text-amber-700',
        bad: 'border-destructive/25 bg-destructive/10 text-destructive',
        accent: 'border-primary/20 bg-primary/10 text-primary',
      },
      size: {
        default: 'px-2 py-0.5 text-[10px]',
        sm: 'px-1.5 py-0.5 text-[9px]',
        xs: 'px-1.5 py-px text-[8px]',
      },
    },
    defaultVariants: {
      tone: 'neutral',
      size: 'default',
    },
  },
);

type Props = React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>;

export function Badge({ className, tone, size, ...props }: Props) {
  return <span className={cn(badgeVariants({ tone, size }), className)} {...props} />;
}
