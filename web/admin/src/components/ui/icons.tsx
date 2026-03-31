import { Icon } from './icon';

export function EditIcon() {
  return (
    <Icon size="lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 20h9" />
      <path d="m16.5 3.5 4 4L8 20l-5 1 1-5Z" />
    </Icon>
  );
}

export function CheckIcon() {
  return (
    <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="m5 12 5 5L20 7" />
    </Icon>
  );
}

export function ResetIcon() {
  return (
    <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 12a9 9 0 1 0 3-6.7" />
      <path d="M3 4v5h5" />
    </Icon>
  );
}

export function PlayIcon() {
  return (
    <Icon size="lg" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="m8 6 10 6-10 6Z" />
    </Icon>
  );
}

export function VisibilityIcon({ visible }: { visible: boolean }) {
  if (visible) {
    return (
      <Icon size="lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M2 12s3.6-6 10-6 10 6 10 6-3.6 6-10 6-10-6-10-6Z" />
        <circle cx="12" cy="12" r="3" />
      </Icon>
    );
  }
  return (
    <Icon size="lg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M10.7 5.1A11.4 11.4 0 0 1 12 5c6.4 0 10 7 10 7a17.2 17.2 0 0 1-2.4 3.2" />
      <path d="M6.6 6.7A16.8 16.8 0 0 0 2 12s3.6 7 10 7a10.8 10.8 0 0 0 4.1-.8" />
      <path d="m3 3 18 18" />
      <path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" />
    </Icon>
  );
}
