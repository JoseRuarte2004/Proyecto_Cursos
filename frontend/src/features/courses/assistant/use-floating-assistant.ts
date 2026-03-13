import { useEffect, useId, useRef, useState } from "react";

type CloseOptions = {
  restoreFocus?: boolean;
};

export function useFloatingAssistant() {
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const hasOpenedRef = useRef(false);
  const shouldRestoreFocusRef = useRef(true);

  function openPanel() {
    shouldRestoreFocusRef.current = false;
    hasOpenedRef.current = true;
    setOpen(true);
  }

  function closePanel(options?: CloseOptions) {
    shouldRestoreFocusRef.current = options?.restoreFocus ?? true;
    setOpen(false);
  }

  function togglePanel() {
    if (open) {
      closePanel({ restoreFocus: false });
      return;
    }

    openPanel();
  }

  useEffect(() => {
    if (!open) {
      if (hasOpenedRef.current && shouldRestoreFocusRef.current) {
        triggerRef.current?.focus();
      }
      shouldRestoreFocusRef.current = true;
      return;
    }

    const focusTimer = window.setTimeout(() => {
      inputRef.current?.focus();
    }, 120);

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        closePanel();
      }
    }

    function handlePointerDown(event: MouseEvent | TouchEvent) {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }

      if (panelRef.current?.contains(target) || triggerRef.current?.contains(target)) {
        return;
      }

      closePanel({ restoreFocus: false });
    }

    document.addEventListener("keydown", handleKeyDown);
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("touchstart", handlePointerDown);

    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleKeyDown);
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("touchstart", handlePointerDown);
    };
  }, [open]);

  return {
    open,
    panelId,
    panelRef,
    triggerRef,
    inputRef,
    openPanel,
    closePanel,
    togglePanel,
  };
}
