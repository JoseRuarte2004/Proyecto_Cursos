import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ChevronDown,
  Check,
  Clock3,
  Download,
  FileText,
  LoaderCircle,
  MessagesSquare,
  Paperclip,
  Send,
  X,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { buildAssetURL, buildWebSocketURL } from "@/api/client";
import { chatApi } from "@/api/endpoints";
import type {
  ChatHello,
  ChatMessage,
  ChatPrivateContact,
  ChatRealtimeError,
  ChatRealtimeMessage,
  MediaAttachment,
  Role,
} from "@/api/types";
import { cn } from "@/app/utils";
import { useSession } from "@/auth/session";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

type ConnectionStatus = "connecting" | "live" | "offline";
type ChatMode = "course" | "private";
type DeliveryStatus = "sending" | "sent";
type ChatMessageView = ChatMessage & {
  deliveryStatus?: DeliveryStatus;
  localId?: string;
};

type SendFallbackInput = {
  mode: ChatMode;
  content: string;
  attachments?: File[];
  otherUserId?: string;
  localId: string;
  originalDraft: string;
  originalAttachments: File[];
};

export function CourseChatPanel({ courseId }: { courseId: string }) {
  const { token, user } = useSession();
  const [mode, setMode] = useState<ChatMode>("course");
  const [selectedContactId, setSelectedContactId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessageView[]>([]);
  const [draft, setDraft] = useState("");
  const [pendingAttachments, setPendingAttachments] = useState<File[]>([]);
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const [showScrollToBottom, setShowScrollToBottom] = useState(false);
  const socketRef = useRef<WebSocket | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const shouldStickToBottomRef = useRef(true);

  const contactsQuery = useQuery({
    queryKey: ["chat-private-contacts", courseId],
    queryFn: () => chatApi.listPrivateContacts(courseId),
    enabled: Boolean(courseId) && Boolean(token),
  });

  const contacts = contactsQuery.data ?? [];
  const selectedContact = useMemo(
    () => contacts.find((item) => item.userId === selectedContactId) ?? null,
    [contacts, selectedContactId],
  );

  useEffect(() => {
    if (mode !== "private") {
      return;
    }
    if (!contacts.length) {
      setSelectedContactId(null);
      return;
    }
    setSelectedContactId((current) =>
      current && contacts.some((item) => item.userId === current)
        ? current
        : contacts[0]!.userId,
    );
  }, [mode, contacts]);

  const conversationReady = mode === "course" || Boolean(selectedContactId);

  const historyQuery = useQuery({
    queryKey: [
      "chat-history",
      courseId,
      mode,
      mode === "private" ? selectedContactId : "course",
    ],
    queryFn: () => {
      if (mode === "private") {
        return chatApi.listPrivateMessages(courseId, selectedContactId ?? "", 50);
      }
      return chatApi.listMessages(courseId, 50);
    },
    enabled: Boolean(courseId) && conversationReady,
    refetchInterval: conversationReady ? 2000 : false,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  });

  const fallbackMutation = useMutation({
    mutationFn: ({
      mode: targetMode,
      content,
      attachments,
      otherUserId,
    }: SendFallbackInput) => {
      if (targetMode === "private") {
        if (!otherUserId) {
          throw new Error("Selecciona un contacto para iniciar el chat privado.");
        }
        return chatApi.sendPrivateMessage(courseId, otherUserId, {
          content,
          attachments,
        });
      }
      return chatApi.sendMessage(courseId, {
        content,
        attachments,
      });
    },
    onSuccess: (message, variables) => {
      setMessages((current) =>
        replaceOptimisticMessage(current, variables.localId, message, user?.id),
      );
    },
    onError: (error: Error, variables) => {
      setMessages((current) =>
        current.filter((message) => message.localId !== variables.localId && message.id !== variables.localId),
      );
      setDraft((current) => (current.length ? current : variables.originalDraft));
      setPendingAttachments((current) =>
        current.length ? current : variables.originalAttachments,
      );
      toast.error(error.message);
    },
  });

  useEffect(() => {
    setMessages([]);
    setPendingAttachments([]);
  }, [courseId, mode, selectedContactId]);

  useEffect(() => {
    if (historyQuery.data) {
      setMessages((current) => mergeMessages(current, historyQuery.data, user?.id));
    }
  }, [historyQuery.data, user?.id]);

  useEffect(() => {
    if (!courseId || !token || !conversationReady) {
      setStatus("offline");
      return;
    }

    const path =
      mode === "private"
        ? `/chat/ws/courses/${courseId}/private/${selectedContactId}`
        : `/chat/ws/courses/${courseId}`;
    const socket = new WebSocket(buildWebSocketURL(path, token));
    socketRef.current = socket;
    setStatus("connecting");

    socket.onopen = () => {
      setStatus("live");
    };

    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as
          | ChatHello
          | ChatRealtimeMessage
          | ChatRealtimeError;

        if (payload.type === "message") {
          setMessages((current) => mergeMessages(current, [payload], user?.id));
          return;
        }

        if (payload.type === "error") {
          toast.error(payload.message);
        }
      } catch {
        toast.error("No pudimos interpretar un mensaje del chat.");
      }
    };

    socket.onerror = () => {
      setStatus("offline");
    };

    socket.onclose = () => {
      setStatus("offline");
    };

    return () => {
      socket.close();
      socketRef.current = null;
    };
  }, [courseId, token, mode, selectedContactId, conversationReady, user?.id]);

  useEffect(() => {
    if (!listRef.current) {
      return;
    }
    if (shouldStickToBottomRef.current) {
      scrollToBottom(listRef.current, "auto");
    }
  }, [messages]);

  const handleListScroll = () => {
    if (!listRef.current) {
      return;
    }

    const distanceFromBottom =
      listRef.current.scrollHeight -
      listRef.current.scrollTop -
      listRef.current.clientHeight;
    const isNearBottom = distanceFromBottom <= 96;

    shouldStickToBottomRef.current = isNearBottom;
    setShowScrollToBottom(!isNearBottom);
  };

  const submitMessage = () => {
    const content = draft.trim();
    if (!content) {
      return;
    }

    if (mode === "private" && !selectedContactId) {
      toast.error("Selecciona un contacto para iniciar el chat privado.");
      return;
    }

    const attachments = pendingAttachments.slice();
    const optimisticMessage = buildOptimisticMessage({
      courseId,
      userId: user?.id,
      userName: user?.name,
      userRole: user?.role,
      content,
    });
    setMessages((current) => mergeMessages(current, [optimisticMessage], user?.id));
    setDraft("");
    setPendingAttachments([]);

    fallbackMutation.mutate({
      mode,
      content,
      attachments,
      otherUserId: selectedContactId ?? undefined,
      localId: optimisticMessage.localId ?? optimisticMessage.id,
      originalDraft: content,
      originalAttachments: attachments,
    });
  };

  return (
    <Card className="relative flex h-[min(72vh,42rem)] min-h-[30rem] flex-col overflow-hidden">
      <div className="shrink-0 border-b border-slate-200 px-4 py-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.28em] text-brand">
              Chat del curso
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="font-heading text-xl font-semibold text-slate-950">
                Conversacion
              </h3>
              <Badge
                tone={
                  status === "live"
                    ? "success"
                    : status === "connecting"
                      ? "brand"
                      : "warning"
                }
              >
                {status === "live"
                  ? "En vivo"
                  : status === "connecting"
                    ? "Conectando"
                    : "Offline"}
              </Badge>
            </div>
          </div>
        </div>

        <div className="mt-2 flex flex-wrap gap-2">
          <Button
            size="sm"
            variant={mode === "course" ? "primary" : "secondary"}
            onClick={() => setMode("course")}
          >
            Curso completo
          </Button>
          <Button
            size="sm"
            variant={mode === "private" ? "primary" : "secondary"}
            onClick={() => setMode("private")}
          >
            Privado
          </Button>
        </div>

        {mode === "private" ? (
          <div className="mt-2 space-y-2">
            <p className="text-sm text-slate-600">
              {user?.role === "teacher"
                ? "Habla en privado con tus alumnos de este curso."
                : "Habla en privado con el profesor del curso."}
            </p>
            {contactsQuery.isLoading ? (
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Cargando contactos...
              </div>
            ) : contactsQuery.isError ? (
              <div className="rounded-2xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {contactsQuery.error.message}
              </div>
            ) : contacts.length ? (
              <Select
                value={selectedContactId ?? ""}
                onChange={(event) => setSelectedContactId(event.target.value)}
              >
                {contacts.map((contact) => (
                  <option key={contact.userId} value={contact.userId}>
                    {contactLabel(contact)}
                  </option>
                ))}
              </Select>
            ) : (
              <div className="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
                No hay contactos disponibles para chat privado en este curso.
              </div>
            )}
          </div>
        ) : null}
      </div>

      <div className="relative min-h-0 flex-1 bg-slate-50/70">
        <div
          ref={listRef}
          onScroll={handleListScroll}
          className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto px-3 py-3 overscroll-contain sm:px-4"
        >
          {historyQuery.isLoading ? (
            <div className="flex h-full items-center justify-center text-slate-500">
              <LoaderCircle className="h-5 w-5 animate-spin" />
            </div>
          ) : historyQuery.isError ? (
            <div className="rounded-[24px] border border-rose-100 bg-white p-4 text-sm text-rose-700">
              {historyQuery.error.message}
            </div>
          ) : mode === "private" && !selectedContactId ? (
            <div className="flex h-full flex-col items-center justify-center gap-3 rounded-[28px] border border-dashed border-slate-200 bg-white/80 p-8 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-500">
                <MessagesSquare className="h-7 w-7" />
              </div>
              <div>
                <p className="font-heading text-xl font-semibold text-slate-950">
                  Selecciona un contacto
                </p>
                <p className="mt-2 text-sm text-slate-600">
                  Elige profesor o alumno para iniciar una conversacion privada.
                </p>
              </div>
            </div>
          ) : messages.length ? (
            messages.map((message) => {
              const own = message.senderId === user?.id;
              const senderName = own
                ? user?.name?.trim() || "Vos"
                : message.senderName?.trim() || roleLabel(message.senderRole);
              const showSender = !own;

              return (
                <div
                  key={message.id}
                  className={cn("flex", own ? "justify-end" : "justify-start")}
                >
                  <div
                    className={cn(
                      "max-w-[82%] rounded-[18px] px-3 py-2 shadow-sm",
                      own
                        ? "rounded-br-md bg-brand text-white"
                        : "rounded-bl-md border border-slate-200 bg-white text-slate-900",
                    )}
                  >
                    {showSender ? (
                      <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
                        {senderName} · {roleLabel(message.senderRole)}
                      </p>
                    ) : null}
                    <p
                      className={cn(
                        "text-sm leading-5",
                        own ? "text-white" : "text-slate-700",
                      )}
                    >
                      {message.content}
                    </p>
                    {message.attachments.length ? (
                      <div className="mt-3 space-y-3">
                        {message.attachments.map((attachment) => (
                          <MessageAttachmentCard
                            key={attachment.id}
                            attachment={attachment}
                            token={token}
                            own={own}
                          />
                        ))}
                      </div>
                    ) : null}
                    <div
                      className={cn(
                        "mt-2 flex items-center justify-end gap-1 text-[11px]",
                        own ? "text-white/80" : "text-slate-400",
                      )}
                    >
                      <span>{formatMessageTime(message.createdAt)}</span>
                      {own ? (
                        <MessageDeliveryStatus
                          status={message.deliveryStatus ?? "sent"}
                          className={own ? "text-white/85" : "text-slate-400"}
                        />
                      ) : null}
                    </div>
                  </div>
                </div>
              );
            })
          ) : (
            <div className="flex h-full flex-col items-center justify-center gap-3 rounded-[28px] border border-dashed border-slate-200 bg-white/80 p-8 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-500">
                <MessagesSquare className="h-7 w-7" />
              </div>
              <div>
                <p className="font-heading text-xl font-semibold text-slate-950">
                  Todavia no hay mensajes
                </p>
                <p className="mt-2 text-sm text-slate-600">
                  {mode === "private"
                    ? "Inicia la charla privada con tu primer mensaje."
                    : "Rompe el hielo con la primera pregunta o comparti una idea con el curso."}
                </p>
              </div>
            </div>
          )}
        </div>

        {showScrollToBottom && messages.length ? (
          <button
            type="button"
            onClick={() => {
              if (!listRef.current) {
                return;
              }
              shouldStickToBottomRef.current = true;
              setShowScrollToBottom(false);
              scrollToBottom(listRef.current, "smooth");
            }}
            className="absolute bottom-4 right-4 inline-flex h-9 w-9 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-600 shadow-card transition hover:border-brand/30 hover:text-brand"
            aria-label="Bajar al ultimo mensaje"
          >
            <ChevronDown className="h-4 w-4" />
          </button>
        ) : null}
      </div>

      <div className="shrink-0 border-t border-slate-200 bg-white px-3 py-2 sm:px-4">
        <div className="rounded-[24px] border border-slate-200 bg-white p-2.5 shadow-sm">
          {pendingAttachments.length ? (
            <div className="mb-2 flex flex-wrap gap-2">
                {pendingAttachments.map((file, index) => (
                  <div
                    key={`${file.name}-${file.size}-${index}`}
                    className="inline-flex max-w-full items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs text-slate-700"
                  >
                    <Paperclip className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                    <span className="max-w-[180px] truncate font-medium">{file.name}</span>
                    <button
                      type="button"
                      onClick={() =>
                        setPendingAttachments((current) =>
                          current.filter((_, currentIndex) => currentIndex !== index),
                        )
                      }
                      className="inline-flex h-5 w-5 items-center justify-center rounded-full text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
                      aria-label={`Quitar ${file.name}`}
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ))}
            </div>
          ) : null}

          <Textarea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder={
              mode === "private"
                ? selectedContact
                  ? `Escribi un mensaje para ${selectedContact.name || "tu contacto"}...`
                  : "Selecciona un contacto para chatear..."
                : "Escribi un mensaje para el curso..."
            }
            maxLength={2000}
            rows={1}
            className="min-h-[38px] max-h-20 resize-none border-0 bg-transparent px-1 py-1 text-sm leading-5 focus:border-transparent focus:ring-0"
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                submitMessage();
              }
            }}
            disabled={mode === "private" && !selectedContactId}
          />

          <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
            <p className="hidden text-[11px] text-slate-400 lg:block">
              Enter envia. Shift + Enter agrega una nueva linea.
            </p>

            <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
              <label className="inline-flex cursor-pointer items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:border-brand/40 hover:text-brand">
                <Paperclip className="h-4 w-4" />
                Adjuntar
                <input
                  type="file"
                  multiple
                  className="hidden"
                  disabled={mode === "private" && !selectedContactId}
                  onChange={(event) => {
                    const files = Array.from(event.target.files ?? []);
                    if (files.length) {
                      setPendingAttachments((current) => [...current, ...files]);
                    }
                    event.target.value = "";
                  }}
                />
              </label>

              <Button
                size="sm"
                className="min-w-[112px]"
                onClick={submitMessage}
                loading={fallbackMutation.isPending}
                disabled={!draft.trim() || (mode === "private" && !selectedContactId)}
              >
                <Send className="h-4 w-4" />
                Enviar
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Card>
  );
}

function MessageAttachmentCard({
  attachment,
  token,
  own,
}: {
  attachment: MediaAttachment;
  token?: string | null;
  own: boolean;
}) {
  const assetURL = buildAssetURL(attachment.url, token);

  if (attachment.kind === "image") {
    return (
      <a href={assetURL} target="_blank" rel="noreferrer" className="block overflow-hidden rounded-2xl">
        <img
          src={assetURL}
          alt={attachment.fileName}
          className="max-h-72 w-full rounded-2xl object-cover"
        />
      </a>
    );
  }

  if (attachment.kind === "video") {
    return <video className="max-h-72 w-full rounded-2xl bg-slate-950" src={assetURL} controls />;
  }

  return (
    <a
      href={assetURL}
      target="_blank"
      rel="noreferrer"
      className={cn(
        "flex items-center justify-between gap-3 rounded-2xl px-3 py-3 text-sm",
        own ? "bg-white/15 text-white" : "border border-slate-200 bg-slate-50 text-slate-700",
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <div
          className={cn(
            "flex h-10 w-10 items-center justify-center rounded-2xl",
            own ? "bg-white/15 text-white" : "bg-white text-brand shadow-card",
          )}
        >
          <FileText className="h-4 w-4" />
        </div>
        <div className="min-w-0">
          <p className="truncate font-medium">{attachment.fileName}</p>
          <p className={cn("text-xs", own ? "text-white/70" : "text-slate-500")}>
            Archivo adjunto
          </p>
        </div>
      </div>
      <Download className={cn("h-4 w-4", own ? "text-white/80" : "text-slate-400")} />
    </a>
  );
}

function contactLabel(contact: ChatPrivateContact) {
  const role = roleLabel(contact.role);
  const name = contact.name?.trim() || contact.userId;
  return `${name} (${role})`;
}

function mergeMessages(
  current: ChatMessageView[],
  incoming: ChatMessage[],
  currentUserID?: string,
) {
  const merged = current.map(normalizeMessage);

  for (const rawMessage of incoming) {
    const message = normalizeMessage(rawMessage);
    const optimisticIndex = findMatchingOptimisticMessageIndex(
      merged,
      message,
      currentUserID,
    );
    if (optimisticIndex >= 0) {
      merged.splice(optimisticIndex, 1, {
        ...message,
        deliveryStatus: "sent",
      });
      continue;
    }

    const existingIndex = merged.findIndex((candidate) => candidate.id === message.id);
    if (existingIndex >= 0) {
      merged.splice(existingIndex, 1, {
        ...message,
        deliveryStatus: "sent",
      });
      continue;
    }

    merged.push({
      ...message,
      deliveryStatus: message.deliveryStatus ?? "sent",
    });
  }

  return merged.sort(compareMessagesByCreatedAt);
}

function normalizeMessage(message: ChatMessage | ChatMessageView): ChatMessageView {
  return {
    ...message,
    attachments: message.attachments ?? [],
  };
}

function formatMessageTime(value: string) {
  const date = parseChatDate(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("es-AR", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone: "America/Argentina/Buenos_Aires",
  }).format(date);
}

function compareMessagesByCreatedAt(left: ChatMessageView, right: ChatMessageView) {
  const leftStamp = parseChatTimestamp(left.createdAt);
  const rightStamp = parseChatTimestamp(right.createdAt);

  if (leftStamp.epochMilliseconds !== rightStamp.epochMilliseconds) {
    return leftStamp.epochMilliseconds - rightStamp.epochMilliseconds;
  }

  if (leftStamp.extraNanoseconds !== rightStamp.extraNanoseconds) {
    return leftStamp.extraNanoseconds - rightStamp.extraNanoseconds;
  }

  if (left.deliveryStatus !== right.deliveryStatus) {
    return left.deliveryStatus === "sending" ? 1 : -1;
  }

  return 0;
}

function parseChatDate(value: string) {
  const timestamp = parseChatTimestamp(value);
  if (!Number.isFinite(timestamp.epochMilliseconds)) {
    return new Date(Number.NaN);
  }
  return new Date(timestamp.epochMilliseconds);
}

function parseChatTimestamp(value: string) {
  const trimmedValue = value.trim();
  const match = trimmedValue.match(
    /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})$/,
  );

  if (!match) {
    const parsed = new Date(trimmedValue).getTime();
    return {
      epochMilliseconds: Number.isNaN(parsed) ? Number.NEGATIVE_INFINITY : parsed,
      extraNanoseconds: 0,
    };
  }

  const [, base, fraction = "", offset] = match;
  const milliseconds = fraction ? fraction.slice(0, 3).padEnd(3, "0") : "000";
  const extraNanoseconds = fraction ? Number(fraction.slice(3, 9).padEnd(6, "0")) : 0;
  const parsed = new Date(`${base}.${milliseconds}${offset}`).getTime();

  return {
    epochMilliseconds: Number.isNaN(parsed) ? Number.NEGATIVE_INFINITY : parsed,
    extraNanoseconds,
  };
}

function buildOptimisticMessage({
  courseId,
  userId,
  userName,
  userRole,
  content,
}: {
  courseId: string;
  userId?: string;
  userName?: string;
  userRole?: Role;
  content: string;
}): ChatMessageView {
  const localId = `local-${crypto.randomUUID()}`;

  return {
    id: localId,
    localId,
    courseId,
    senderId: userId ?? "",
    senderName: userName ?? "Vos",
    senderRole: userRole ?? "student",
    content,
    attachments: [],
    createdAt: new Date().toISOString(),
    deliveryStatus: "sending",
  };
}

function replaceOptimisticMessage(
  messages: ChatMessageView[],
  localId: string,
  confirmedMessage: ChatMessage,
  currentUserID?: string,
) {
  const filtered = messages.filter(
    (message) => message.localId !== localId && message.id !== localId,
  );
  return mergeMessages(filtered, [confirmedMessage], currentUserID);
}

function findMatchingOptimisticMessageIndex(
  messages: ChatMessageView[],
  incoming: ChatMessageView,
  currentUserID?: string,
) {
  if (!currentUserID || incoming.senderId !== currentUserID) {
    return -1;
  }

  const incomingStamp = parseChatTimestamp(incoming.createdAt);
  return messages.findIndex((message) => {
    if (message.deliveryStatus !== "sending") {
      return false;
    }
    if (message.senderId !== incoming.senderId) {
      return false;
    }
    if (message.content !== incoming.content) {
      return false;
    }
    if ((message.attachments?.length ?? 0) !== (incoming.attachments?.length ?? 0)) {
      return false;
    }

    const messageStamp = parseChatTimestamp(message.createdAt);
    return (
      Math.abs(messageStamp.epochMilliseconds - incomingStamp.epochMilliseconds) <=
      30_000
    );
  });
}

function MessageDeliveryStatus({
  status,
  className,
}: {
  status: DeliveryStatus;
  className?: string;
}) {
  if (status === "sending") {
    return <Clock3 className={cn("h-3.5 w-3.5", className)} aria-label="Enviando" />;
  }

  return <Check className={cn("h-3.5 w-3.5", className)} aria-label="Enviado" />;
}

function roleLabel(role: Role) {
  switch (role) {
    case "admin":
      return "Admin";
    case "teacher":
      return "Teacher";
    default:
      return "Student";
  }
}

function scrollToBottom(element: HTMLDivElement, behavior: ScrollBehavior) {
  element.scrollTo({
    top: element.scrollHeight,
    behavior,
  });
}
