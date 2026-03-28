import type { SVGProps } from 'react';
import { cn } from '../../lib/utils';

const ICON_SIZES = {
  sm: 14,
  md: 16,
  lg: 18,
  xl: 20,
} as const;

type IconSize = keyof typeof ICON_SIZES | number;

type Props = Omit<SVGProps<SVGSVGElement>, 'width' | 'height'> & {
  size?: IconSize;
};

export function Icon({ className, size = 'md', style, ...props }: Props) {
  const resolvedSize = typeof size === 'number' ? size : ICON_SIZES[size];

  return (
    <svg
      className={cn('icon', className)}
      width={resolvedSize}
      height={resolvedSize}
      style={{ ...style, flex: 'none' }}
      {...props}
    />
  );
}
