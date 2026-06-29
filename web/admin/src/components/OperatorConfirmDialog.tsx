import type { ReactNode } from "react";

type OperatorConfirmDialogProps = {
  busy?: boolean;
  cancelLabel?: string;
  children?: ReactNode;
  confirmLabel: string;
  danger?: boolean;
  description: string;
  disabled?: boolean;
  eyebrow?: string;
  onCancel: () => void;
  onConfirm: () => void;
  title: string;
};

export function OperatorConfirmDialog({
  busy = false,
  cancelLabel = "Cancel",
  children,
  confirmLabel,
  danger = false,
  description,
  disabled = false,
  eyebrow = "Operator confirmation",
  onCancel,
  onConfirm,
  title,
}: OperatorConfirmDialogProps) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section
        aria-label={`${title} confirmation`}
        aria-modal="true"
        className={danger ? "surface action-modal action-modal--danger" : "surface action-modal"}
        role="dialog"
      >
        <div className="section-heading">
          <div>
            <p className="eyebrow">{eyebrow}</p>
            <h3>{title}</h3>
          </div>
          <span>{danger ? "guarded action" : "safe action"}</span>
        </div>
        <p className="muted">{description}</p>
        {children ? <div className="confirmation-body">{children}</div> : null}
        <div className="modal-actions">
          <button className="button-secondary" disabled={busy} onClick={onCancel} type="button">
            {cancelLabel}
          </button>
          <button className={danger ? "button-danger" : undefined} disabled={busy || disabled} onClick={onConfirm} type="button">
            {busy ? "Working" : confirmLabel}
          </button>
        </div>
      </section>
    </div>
  );
}
