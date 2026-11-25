"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AssistantMessageContent } from "@/components/chat/AssistantMessageContent/AssistantMessageContent";
import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { ChatScrollToBottom } from "@/components/chat/ChatScrollToBottom/ChatScrollToBottom";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import composerStyles from "@/features/conversations/ConversationComposer/ConversationComposer.module.css";
import historyStyles from "@/features/conversations/ConversationHistory/ConversationHistory.module.css";
import { useOptionalWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { useWorkspaceModelSelection } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";
import Link from "@/i18n/Link";
import { useDictionary } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";
import { presentationChatCopy as copy } from "@/i18n/presentation-chat";
import { startGenerationPolling } from "@/lib/generation-polling";
import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import type { ConversationMessage } from "@/lib/web-api/contracts";
import { DeckRequestError, failedPresentationStatuses } from "./deck-generation";
import {
  createPresentationChat, fetchPresentationChatQuote, maxPresentationChatMessageBytes, readPresentationChatHistory, readPresentationChatTurn,
  recoverPresentationChatAttempt, resumePresentationChatTurn, sendPresentationChatTurn, startPresentationFromChat,
  type PresentationChatQuote, type PresentationChatTurn, type PresentationDeckPrice,
} from "./presentation-chat-api";
import { presentationChatModelId, presentationChatPath, presentationWorkspacePath } from "./route";
import styles from "./PresentationChat.module.css";

type Props = { conversationId?: string; initialMessages?: ConversationMessage[] };
type Attempt = Awaited<ReturnType<typeof recoverPresentationChatAttempt>>;
type LaunchFailure = { job: string; error: Error; approvedQuote?: PresentationDeckPrice };

const completeBrief = (turn: PresentationChatTurn) => turn.status === "succeeded" && turn.missing?.length === 0;
const finished = (status: string) => status === "succeeded" || failedPresentationStatuses.has(status);

export function PresentationChat({ conversationId, initialMessages }: Props) {
  const { profile } = useWorkspaceAccountSnapshot();
  return <PresentationChatContent key={profile.account_id} accountId={profile.account_id} conversationId={conversationId} initialMessages={initialMessages} />;
}

function PresentationChatContent({ accountId, conversationId, initialMessages }: Props & { accountId: string }) {
  const t = useDictionary();
  const router = useRouter();
  const list = useOptionalWorkspaceConversationList();
  const announceModel = useWorkspaceModelSelection()?.setConversationModel;
  const [prompt, setPrompt] = useState("");
  const [messages, setMessages] = useState(initialMessages ?? []);
  const [pendingPrompt, setPendingPrompt] = useState("");
  const [quote, setQuote] = useState<PresentationChatQuote>();
  const [quoteRevision, setQuoteRevision] = useState(0);
  const [quoteFailed, setQuoteFailed] = useState(false);
  const [turn, setTurn] = useState<PresentationChatTurn | null>(null);
  const [trackedJob, setTrackedJob] = useState<string>();
  const [loaded, setLoaded] = useState(!conversationId);
  const [readError, setReadError] = useState("");
  const [pollRevision, setPollRevision] = useState(0);
  const [sendError, setSendError] = useState("");
  const [launchFailure, setLaunchFailure] = useState<LaunchFailure>();
  const [writing, setWriting] = useState<"send" | "launch">();
  const [scrollRegion, setScrollRegion] = useState<HTMLElement | null>(null);
  const [scrollRequest, setScrollRequest] = useState(0);
  const operation = useRef<AbortController | null>(null);
  const launchAttempt = useRef<string | undefined>(undefined);
  const attempt = useRef<{ prompt: string; quote: PresentationChatQuote; keys: Attempt } | null>(null);
  const createdConversation = useRef<string | undefined>(undefined);
  const acceptedDeck = useRef<{ job: string; presentation: NonNullable<PresentationChatTurn["presentation"]> } | null>(null);
  const generated = turn?.presentation;
  const canStart = Boolean(turn && completeBrief(turn));
  const launchError = launchFailure?.job === turn?.job_id ? launchFailure?.error : undefined;
  const requote = launchError instanceof DeckRequestError && launchError.status === 409;
  const waiting = Boolean(trackedJob && (turn?.job_id !== trackedJob || !finished(turn.status)) || turn && !finished(turn.status));
  const deckWaiting = Boolean(generated && !generated.unavailable && !finished(generated.status));
  const needsTurnPayment = turn?.status === "awaiting_payment";
  const needsDeckPayment = generated?.status === "awaiting_payment" || (launchError instanceof DeckRequestError && launchError.status === 402);
  const showComposer = !generated && !canStart && !needsTurnPayment;
  const needsQuote = showComposer || requote;
  const refreshQuote = useCallback(() => {
    setQuoteFailed(false);
    setQuote(undefined);
    setQuoteRevision(current => current + 1);
  }, []);

  useEffect(() => {
    if (!needsQuote) return;
    const controller = new AbortController();
    fetchPresentationChatQuote(controller.signal).then(value => {
      if (!controller.signal.aborted) setQuote(value);
    }).catch(() => { if (!controller.signal.aborted) setQuoteFailed(true); });
    return () => controller.abort();
  }, [needsQuote, quoteRevision]);

  useEffect(() => {
    if (!conversationId) return;
    announceModel?.(conversationId, presentationChatModelId);
    return () => announceModel?.(conversationId, null);
  }, [announceModel, conversationId]);

  useEffect(() => () => operation.current?.abort(), []);

  const attachScrollRegion = useCallback((node: HTMLElement | null) => {
    setScrollRegion(node?.closest<HTMLElement>('main[data-testid="workspace-scroll-region"]') ?? null);
  }, []);

  useEffect(() => {
    if (!conversationId) return;
    let historySynced = false;
    const tracker = startGenerationPolling({
      delayMs: attemptNo => Math.min(7500, 1000 + attemptNo * 500),
      timeoutMs: 180_000,
      pauseWhenHidden: true,
      poll: async signal => {
        try {
          const value = await readPresentationChatTurn(conversationId, trackedJob, signal);
          if (signal.aborted) return false;
          setLoaded(true);
          setReadError("");
          if (!value) {
            setTurn(null);
            return false;
          }
          const presentation = value.presentation ?? (acceptedDeck.current?.job === value.job_id ? acceptedDeck.current.presentation : undefined);
          setTurn({ ...value, presentation });
          const turnFinished = finished(value.status);
          if (!historySynced && turnFinished) {
            try {
              const history = await readPresentationChatHistory(conversationId, signal);
              if (signal.aborted) return false;
              setMessages(history);
              setPendingPrompt("");
              historySynced = true;
            } catch {
              if (!signal.aborted) setReadError(copy.historyUnavailable);
            }
          }
          return !turnFinished || Boolean(presentation && !presentation.unavailable && !finished(presentation.status));
        } catch {
          if (!signal.aborted) setReadError(copy.readUnavailable);
          return true;
        }
      },
      onExhausted: () => setReadError(copy.readUnavailable),
    });
    return () => tracker.stop();
  }, [conversationId, trackedJob, pollRevision]);

  const launch = useCallback(async (readyTurn: PresentationChatTurn, approvedQuote?: PresentationDeckPrice) => {
    if (!conversationId || operation.current) return;
    launchAttempt.current = readyTurn.job_id;
    const controller = new AbortController();
    operation.current = controller;
    const signal = AbortSignal.any([controller.signal, browserSessionSnapshot().signal]);
    setLaunchFailure(undefined);
    setWriting("launch");
    try {
      const presentation = await startPresentationFromChat(conversationId, readyTurn, signal, approvedQuote);
      if (signal.aborted) return;
      acceptedDeck.current = { job: readyTurn.job_id, presentation };
      setTurn(current => current?.job_id === readyTurn.job_id ? { ...current, presentation } : current);
      setPollRevision(current => current + 1);
      setScrollRequest(current => current + 1);
    } catch (error) {
      if (signal.aborted) return;
      setLaunchFailure({ job: readyTurn.job_id, error: error instanceof DeckRequestError ? error : new Error(copy.launchUnavailable), approvedQuote });
      if (error instanceof DeckRequestError && error.status === 409) {
        refreshQuote();
        setTrackedJob(undefined);
        setPollRevision(current => current + 1);
      }
    } finally {
      operation.current = null;
      if (!signal.aborted) setWriting(undefined);
    }
  }, [conversationId, refreshQuote]);

  useEffect(() => {
    if (!turn || !canStart || turn.presentation || launchAttempt.current === turn.job_id) return;
    let active = true;
    queueMicrotask(() => { if (active) void launch(turn); });
    return () => { active = false; };
  }, [canStart, launch, turn]);

  const trimmedPrompt = prompt.trim();
  const overLimit = new TextEncoder().encode(trimmedPrompt).length > maxPresentationChatMessageBytes;
  const canSubmit = Boolean(quote && loaded && trimmedPrompt && !overLimit && !waiting && !writing && showComposer);

  async function send() {
    if (!canSubmit || !quote || operation.current) return;
    const controller = new AbortController();
    operation.current = controller;
    const signal = AbortSignal.any([controller.signal, browserSessionSnapshot().signal]);
    setWriting("send");
    setSendError("");
    let navigate = false;
    try {
      if (attempt.current?.prompt !== trimmedPrompt || attempt.current.quote !== quote) {
        const keys = await recoverPresentationChatAttempt(accountId, conversationId, trimmedPrompt, quote);
        attempt.current = { prompt: trimmedPrompt, quote, keys };
      }
      const { keys } = attempt.current;
      signal.throwIfAborted();
      let target = conversationId ?? createdConversation.current;
      if (!target) {
        const conversation = await createPresentationChat(keys.conversationKey, signal);
        signal.throwIfAborted();
        target = createdConversation.current = conversation.id;
        list?.upsertConversation(conversation);
      }
      const job = await sendPresentationChatTurn(target, keys.messageKey, trimmedPrompt, quote, signal);
      if (signal.aborted) return;
      keys.clear();
      attempt.current = null;
      setPrompt("");
      setPendingPrompt(trimmedPrompt);
      setLaunchFailure(undefined);
      setReadError("");
      setTrackedJob(job);
      setScrollRequest(current => current + 1);
      if (!conversationId) {
        router.replace(`/app/chat/${target}`);
        navigate = true;
      }
    } catch (error) {
      if (signal.aborted) return;
      if (error instanceof DeckRequestError && error.status === 409) {
        attempt.current?.keys.clear();
        attempt.current = null;
        refreshQuote();
        setSendError(copy.changedQuote);
      } else {
        setSendError(error instanceof DeckRequestError && error.status < 500 ? error.message : copy.sendUnavailable);
        if (conversationId && error instanceof DeckRequestError && error.status === 402) {
          setTrackedJob(undefined);
          setPollRevision(current => current + 1);
        }
      }
    } finally {
      operation.current = null;
      if (!signal.aborted && !navigate) setWriting(undefined);
    }
  }

  async function resumeTurn() {
    if (!conversationId || !turn || !needsTurnPayment || operation.current) return;
    const controller = new AbortController();
    operation.current = controller;
    const signal = AbortSignal.any([controller.signal, browserSessionSnapshot().signal]);
    setWriting("send");
    setSendError("");
    try {
      await resumePresentationChatTurn(conversationId, turn.job_id, signal);
      if (signal.aborted) return;
      attempt.current?.keys.clear();
      attempt.current = null;
      setPendingPrompt(trimmedPrompt);
      setPrompt("");
      setTurn(current => current?.job_id === turn.job_id ? { ...current, status: "queued" } : current);
      setTrackedJob(turn.job_id);
      setPollRevision(current => current + 1);
      setScrollRequest(current => current + 1);
    } catch (error) {
      if (!signal.aborted) setSendError(error instanceof DeckRequestError ? error.message : copy.sendUnavailable);
    } finally {
      operation.current = null;
      if (!signal.aborted) setWriting(undefined);
    }
  }

  const lastMessage = messages.at(-1);
  const reply = lastMessage?.role === "assistant" && lastMessage.text === turn?.reply ? undefined : turn?.reply;
  const currentReply = waiting ? undefined : reply;
  const statusRetry = { label: copy.retryStatus, onClick: () => setPollRevision(current => current + 1) };
  const childPrice = turn && generated?.model_id && generated.presentation_credits
    && (generated.model_id !== turn.model_id || generated.presentation_credits !== turn.presentation_credits)
    ? { model_id: generated.model_id, credits: generated.presentation_credits } : undefined;
  const launchRetry = turn && !requote && !generated?.unavailable
    ? { label: copy.retryGeneration, onClick: () => void launch(turn, launchFailure?.approvedQuote ?? childPrice) } : undefined;
  const resultHref = generated && !generated.unavailable && `${presentationWorkspacePath}?run=${generated.conversation_id}&job=${generated.job_id}`;

  return <section aria-label={copy.title} className={historyStyles.content} ref={attachScrollRegion}>
    <div className={`${historyStyles.history} ${styles.history}`}>
      <h1 className={styles.title}>{copy.title}</h1>
      <ol className={historyStyles.messages}>
        {!messages.length && !pendingPrompt && !currentReply ? <li className={historyStyles.assistantMessage}><AssistantMessageContent markdown={copy.welcome} /></li> : null}
        {messages.map(message => <li className={message.role === "user" ? historyStyles.userMessage : historyStyles.assistantMessage} key={message.id}>
          {message.role === "user" ? <p className={styles.userText}>{message.text}</p> : <AssistantMessageContent markdown={message.text} />}
        </li>)}
        {pendingPrompt && !(lastMessage?.role === "user" && lastMessage.text === pendingPrompt)
          ? <li className={historyStyles.userMessage}><p className={styles.userText}>{pendingPrompt}</p></li> : null}
        {currentReply ? <li className={historyStyles.assistantMessage} aria-live="polite"><AssistantMessageContent markdown={currentReply} /></li> : null}
      </ol>
      {readError ? <StateNotice inline kind="error" action={statusRetry}>{readError}</StateNotice>
        : !loaded || waiting ? <StateNotice inline kind="loading">{loaded ? copy.waiting : copy.reading}</StateNotice> : null}
      {needsTurnPayment ? <StateNotice inline kind={writing ? "loading" : "error"}
        action={writing ? undefined : { label: copy.retryTurn, onClick: () => void resumeTurn() }}>
        {writing ? copy.submitting : sendError || new DeckRequestError(402).message}
      </StateNotice> : turn && !waiting && failedPresentationStatuses.has(turn.status) && !sendError
        ? <StateNotice inline kind="error">{copy.turnFailed}</StateNotice> : null}
      {canStart && !generated && !launchError ? <StateNotice inline kind="loading">{copy.launch}</StateNotice> : null}
      {requote && turn ? <div className={styles.result}>
        <StateNotice inline kind="info">{copy.changedDeckQuote}</StateNotice>
        {quote ? <div className={styles.links}>
          <CreditAmount prefix={copy.newDeckPrice} value={quote.credits} />
          <Button onClick={() => void launch(turn, quote)}>{copy.approveDeckPrice}</Button>
        </div> : quoteFailed ? <StateNotice inline kind="error" action={{ label: copy.refreshQuote, onClick: refreshQuote }}>{copy.pricingUnavailable}</StateNotice>
          : <StateNotice inline kind="loading">{copy.pricingPending}</StateNotice>}
      </div> : launchError && (!generated || !needsDeckPayment) ? <StateNotice inline kind="error" action={launchRetry}>{launchError.message}</StateNotice> : null}
      {generated ? <div className={styles.result}>
        <StateNotice inline kind={deckWaiting || writing === "launch" ? "loading" : generated.status === "succeeded" && !generated.unavailable ? "success" : "error"}
          action={needsDeckPayment && !writing ? launchRetry : undefined}>
          {generated.unavailable ? copy.unavailableDeck : writing === "launch" ? copy.launch : needsDeckPayment ? launchError?.message || new DeckRequestError(402).message
            : generated.status === "succeeded" ? copy.ready : deckWaiting ? copy.generating : copy.generationFailed}
        </StateNotice>
        <div className={styles.links}>
          {resultHref ? <Link className={styles.link} href={resultHref}>{generated.status === "succeeded" ? copy.open : deckWaiting ? copy.openProgress : copy.openDetails}</Link> : null}
          {generated.unavailable || (!deckWaiting && !needsDeckPayment) ? <Link className={styles.link} href={presentationChatPath}>{copy.newBrief}</Link> : null}
        </div>
      </div> : null}
    </div>
    {showComposer ? <div className={composerStyles.dock}>
      {sendError ? <StateNotice inline kind="error">{sendError}</StateNotice> : null}
      {overLimit ? <StateNotice inline kind="error">{copy.limit}</StateNotice> : null}
      {writing === "send" ? <StateNotice inline kind="loading">{copy.submitting}</StateNotice> : null}
      {quoteFailed ? <StateNotice inline kind="error" action={{ label: copy.refreshQuote, onClick: refreshQuote }}>{copy.pricingUnavailable}</StateNotice>
        : !quote ? <StateNotice inline kind="loading">{copy.pricingPending}</StateNotice> : null}
      <ChatComposer
        additionalControls={quote ? <div className={styles.prices}>
          <CreditAmount prefix={copy.turnPrice} value={quote.turn_credits} />
          <CreditAmount prefix={copy.presentationPrice} value={quote.credits} />
        </div> : undefined}
        canSubmit={canSubmit}
        disabled={Boolean(writing || waiting)}
        label={copy.composerLabel}
        mediaLabel={t.conversations.composerMediaUpload}
        note={copy.pricingNote}
        onChange={event => setPrompt(event.target.value)}
        onSend={() => void send()}
        placeholder={copy.placeholder}
        submitLabel={t.conversations.composerSubmit}
        value={prompt}
        variant="conversation"
      />
    </div> : null}
    <ChatScrollToBottom contentVersion={`${lastMessage?.id ?? ""}:${pendingPrompt}:${currentReply ?? ""}:${generated?.status ?? ""}`}
      forceScrollRequest={scrollRequest} scrollContainer={scrollRegion} />
  </section>;
}
