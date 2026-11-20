import {useRef, type ReactNode, type RefObject} from "react";
import {AlertDialog} from "@base-ui/react/alert-dialog";
import {Dialog} from "@base-ui/react/dialog";
import {X} from "lucide-react";
import {Button} from "./button";
import {ErrorLine} from "./feedback";
import {FormBoundary} from "./form";

export function Overlay({open, onClose, title, children, busy, drawer = false}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  busy?: boolean;
  drawer?: boolean;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  const popup = useRef<HTMLDivElement>(null);
  return <Dialog.Root open={open} onOpenChange={next => { if (!next && !busy) onClose(); }}>
    <Dialog.Portal>
      <Dialog.Backdrop className="overlay-backdrop" />
      <Dialog.Popup ref={popup} className="overlay" data-drawer={drawer || undefined}
        initialFocus={interaction => drawer ? heading.current : interaction === "touch" ? true : popup.current?.querySelector("input") ?? true}>
        <header className="overlay-heading">
          <Dialog.Title ref={heading} tabIndex={-1}>{title}</Dialog.Title>
          <Dialog.Close render={<Button variant="ghost" size="icon" title="关闭" aria-label="关闭" disabled={busy}><X aria-hidden /></Button>} />
        </header>
        <div className="overlay-body"><FormBoundary>{children}</FormBoundary></div>
      </Dialog.Popup>
    </Dialog.Portal>
  </Dialog.Root>;
}

export function ConfirmDialog({open, title, description, confirm, onClose, onConfirm, busy, error, finalFocus}: {
  open: boolean;
  title: string;
  description: string;
  confirm: string;
  onClose: () => void;
  onConfirm: () => void;
  busy: boolean;
  error: string;
  finalFocus?: RefObject<HTMLElement | null>;
}) {
  const cancel = useRef<HTMLButtonElement>(null);
  return <AlertDialog.Root open={open} onOpenChange={next => { if (!next && !busy) onClose(); }}>
    <AlertDialog.Portal>
      <AlertDialog.Backdrop className="overlay-backdrop" />
      <AlertDialog.Popup className="overlay" initialFocus={cancel} finalFocus={finalFocus}>
        <header className="overlay-heading"><AlertDialog.Title>{title}</AlertDialog.Title></header>
        <div className="overlay-body">
          <AlertDialog.Description>{description}</AlertDialog.Description>
          <ErrorLine value={error} />
          <div className="form-actions">
            <AlertDialog.Close render={<Button ref={cancel} variant="outline" disabled={busy}>取消</Button>} />
            <Button variant="destructive" busy={busy} onClick={onConfirm}>{confirm}</Button>
          </div>
        </div>
      </AlertDialog.Popup>
    </AlertDialog.Portal>
  </AlertDialog.Root>;
}
