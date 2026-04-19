import { useEffect, useId, useRef, type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { AlertTriangle } from 'lucide-react';
import { Button } from './button';
import { Card, CardContent } from './card';

type ConfirmDialogTone = 'default' | 'danger';

type Props = {
  cancelLabel?: string;
  children?: ReactNode;
  confirmLabel?: string;
  description?: ReactNode;
  loading?: boolean;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
  open: boolean;
  tone?: ConfirmDialogTone;
  title: string;
};

function focusableElements(container: HTMLElement) {
  return Array.from(
    container.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  );
}

export function ConfirmDialog({
  cancelLabel = 'Cancel',
  children,
  confirmLabel = 'Confirm',
  description,
  loading = false,
  onConfirm,
  onOpenChange,
  open,
  tone = 'default',
  title,
}: Props) {
  const titleId = useId();
  const descriptionId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const cancelButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const focusTimer = window.setTimeout(() => cancelButtonRef.current?.focus(), 0);
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !loading) {
        onOpenChange(false);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener('keydown', onKeyDown);
      previousFocus?.focus();
    };
  }, [loading, onOpenChange, open]);

  if (!open || typeof document === 'undefined') {
    return null;
  }

  const onDialogKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'Tab' || !dialogRef.current) {
      return;
    }
    const elements = focusableElements(dialogRef.current);
    if (elements.length === 0) {
      event.preventDefault();
      return;
    }
    const first = elements[0];
    const last = elements[elements.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
      return;
    }
    if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  const dialog = (
    <div
      className="admin-modal confirm-dialog"
      onMouseDown={(event) => {
        if (!loading && event.target === event.currentTarget) {
          onOpenChange(false);
        }
      }}
    >
      <Card
        aria-describedby={description || children ? descriptionId : undefined}
        aria-labelledby={titleId}
        aria-modal="true"
        className="admin-modal__card confirm-dialog__card"
        onKeyDown={onDialogKeyDown}
        onMouseDown={(event) => event.stopPropagation()}
        ref={dialogRef}
        role="alertdialog"
      >
        <div className="confirm-dialog__header">
          <div className={`confirm-dialog__icon confirm-dialog__icon--${tone}`}>
            <AlertTriangle className="h-5 w-5" />
          </div>
          <div className="confirm-dialog__heading">
            <h2 className="confirm-dialog__title" id={titleId}>
              {title}
            </h2>
            {description ? (
              <p className="confirm-dialog__description" id={descriptionId}>
                {description}
              </p>
            ) : null}
          </div>
        </div>
        <CardContent className="confirm-dialog__content">
          {children ? (
            <div className="confirm-dialog__body" id={description ? undefined : descriptionId}>
              {children}
            </div>
          ) : null}
          <div className="confirm-dialog__actions">
            <Button
              type="button"
              variant="secondary"
              onClick={() => onOpenChange(false)}
              disabled={loading}
              ref={cancelButtonRef}
            >
              {cancelLabel}
            </Button>
            <Button type="button" variant={tone === 'danger' ? 'danger' : 'default'} onClick={onConfirm} disabled={loading}>
              {loading ? 'Working…' : confirmLabel}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );

  return createPortal(dialog, document.body);
}
