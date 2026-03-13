import { AssistantPanel } from "@/features/courses/assistant/assistant-panel";
import { AssistantTrigger } from "@/features/courses/assistant/assistant-trigger";
import { useCourseAdvisor } from "@/features/courses/assistant/use-course-advisor";
import { useFloatingAssistant } from "@/features/courses/assistant/use-floating-assistant";

export function FloatingAssistant() {
  const controller = useCourseAdvisor();
  const assistant = useFloatingAssistant();

  return (
    <>
      <AssistantPanel
        open={assistant.open}
        panelId={assistant.panelId}
        panelRef={assistant.panelRef}
        inputRef={assistant.inputRef}
        controller={controller}
        onClose={() => assistant.closePanel()}
      />
      <AssistantTrigger
        open={assistant.open}
        panelId={assistant.panelId}
        buttonRef={assistant.triggerRef}
        onClick={assistant.togglePanel}
      />
    </>
  );
}
