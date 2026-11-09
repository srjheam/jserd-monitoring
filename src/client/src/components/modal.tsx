"use client";

import { Dialog, Modal, ModalOverlay } from "react-aria-components";

// The shared shell for every modal in the app, built from the
// `ModalOverlay`/`Modal`/`Dialog` pattern `ModalDialog` used first.
// `fullScreen` swaps the centered, max-width mode for one spanning the
// whole viewport; every other prop passes straight through to `Dialog`.
export function AppModal({
  isOpen,
  onOpenChange,
  fullScreen = false,
  dialogProps,
  children,
}: {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  fullScreen?: boolean;
  dialogProps?: Omit<React.ComponentProps<typeof Dialog>, "children">;
  children: React.ComponentProps<typeof Dialog>["children"];
}) {
  return (
    <ModalOverlay
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      className={`modal-overlay${fullScreen ? " modal-overlay--full-screen" : ""}`}
    >
      <Modal className={`modal${fullScreen ? " modal--full-screen" : ""}`}>
        <Dialog {...dialogProps}>{children}</Dialog>
      </Modal>
    </ModalOverlay>
  );
}
