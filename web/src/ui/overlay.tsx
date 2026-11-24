import {FieldBoundary} from "./field";
import {useRef, type ReactNode, type RefObject} from "react";
import {AlertDialog} from "@base-ui/react/alert-dialog";
import {Dialog} from "@base-ui/react/dialog";
import {X} from "lucide-react";
import {Button} from "./button";
import {ErrorLine} from "./feedback";
import {FormActions, FormBoundary} from "./form";
import {HoldButton} from "./hold-button";

export function Overlay({open, onClose, title, children, busy, drawer = false, finalFocus, footer}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  busy?: boolean;
  drawer?: boolean;
  finalFocus?: RefObject<HTMLElement | null>;
  footer?: ReactNode;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  const popup = useRef<HTMLDivElement>(null);
  return <Dialog.Root open={open} onOpenChange={next => { if (!next && !busy) onClose(); }}>
    <Dialog.Portal>
      <Dialog.Backdrop className="overlay-backdrop" />
      <Dialog.Popup ref={popup} className="overlay" data-drawer={drawer || undefined} finalFocus={finalFocus}
        initialFocus={interaction => drawer ? heading.current : interaction === "touch" ? true : popup.current?.querySelector("input") ?? true}>
        <header className="overlay-heading">
          <Dialog.Title ref={heading} tabIndex={-1}>{title}</Dialog.Title>
          <Dialog.Close render={<Button variant="ghost" size="icon" title="关闭" aria-label="关闭" disabled={busy}><X aria-hidden /></Button>} />
        </header>
        <div className="overlay-body"><FormBoundary><FieldBoundary>{children}</FieldBoundary></FormBoundary></div>
        {footer && <footer className="overlay-footer">
          <fieldset className="overlay-footer-fields" disabled={busy}>
            <FormBoundary><FieldBoundary><FormActions>{footer}</FormActions></FieldBoundary></FormBoundary>
          </fieldset>
        </footer>}
      </Dialog.Popup>
    </Dialog.Portal>
  </Dialog.Root>;
}

export function ConfirmDialog({open, title, description, confirm, critical = false, children, onClose, onConfirm, busy, error, finalFocus}: {
  open: boolean;
  title: string;
  description: string;
  confirm: string;
  critical?: boolean;
  children?: ReactNode;
  onClose: () => void;
  onConfirm: () => void;
  busy: boolean;
  error: string;
  finalFocus?: RefObject<HTMLElement | null> | (() => HTMLElement | null);
}) {
  const cancel = useRef<HTMLButtonElement>(null);
  return <AlertDialog.Root open={open} onOpenChange={next => { if (!next && !busy) onClose(); }}>
    <AlertDialog.Portal>
      <AlertDialog.Backdrop className="overlay-backdrop" />
      <AlertDialog.Popup className="overlay" initialFocus={cancel} finalFocus={finalFocus}>
        <header className="overlay-heading"><AlertDialog.Title>{title}</AlertDialog.Title></header>
        <div className="overlay-body">
          <AlertDialog.Description>{description}</AlertDialog.Description>
          {children}
          <ErrorLine value={error} />
        </div>
        <footer className="overlay-footer">
          <FormActions>
            <AlertDialog.Close render={<Button ref={cancel} variant="ghost" disabled={busy}>取消</Button>} />
            {critical
              ? <HoldButton busy={busy} onConfirm={onConfirm}>{confirm}</HoldButton>
              : <Button variant="danger-outline" busy={busy} onClick={onConfirm}>{confirm}</Button>}
          </FormActions>
        </footer>
      </AlertDialog.Popup>
    </AlertDialog.Portal>
  </AlertDialog.Root>;
}
