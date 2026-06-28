import { useEffect, useRef, useState, useCallback } from "react";

export type WebSocketStatus = "connecting" | "connected" | "disconnected" | "error";

export interface WebSocketMessage {
  type: string;
  data: any;
  timestamp: string;
}

interface UseWebSocketOptions {
  url: string;
  onMessage?: (msg: WebSocketMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (err: Event) => void;
  reconnect?: boolean;
  reconnectInterval?: number;
  maxRetries?: number;
  /** Quando false, nunca conecta (status fica "disconnected"). */
  enabled?: boolean;
}

export function useWebSocket({
  url,
  onMessage,
  onOpen,
  onClose,
  onError,
  reconnect = true,
  reconnectInterval = 3000,
  maxRetries = 10,
  enabled = true,
}: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const retriesRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [status, setStatus] = useState<WebSocketStatus>("disconnected");
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    try {
      setStatus("connecting");
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        setStatus("connected");
        retriesRef.current = 0;
        onOpen?.();
      };

      ws.onmessage = (event) => {
        try {
          const msg: WebSocketMessage = JSON.parse(event.data);
          setLastMessage(msg);
          onMessage?.(msg);
        } catch {
          // Non-JSON message
          const msg: WebSocketMessage = {
            type: "raw",
            data: event.data,
            timestamp: new Date().toISOString(),
          };
          setLastMessage(msg);
          onMessage?.(msg);
        }
      };

      ws.onclose = () => {
        setStatus("disconnected");
        wsRef.current = null;
        onClose?.();

        if (reconnect && retriesRef.current < maxRetries) {
          const delay = reconnectInterval * Math.pow(1.5, retriesRef.current);
          retriesRef.current++;
          reconnectTimerRef.current = setTimeout(connect, Math.min(delay, 30000));
        }
      };

      ws.onerror = (err) => {
        setStatus("error");
        onError?.(err);
      };
    } catch {
      setStatus("error");
    }
  }, [url, onMessage, onOpen, onClose, onError, reconnect, reconnectInterval, maxRetries]);

  const disconnect = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    retriesRef.current = maxRetries; // prevent reconnect
    wsRef.current?.close();
    wsRef.current = null;
    setStatus("disconnected");
  }, [maxRetries]);

  const send = useCallback((data: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(typeof data === "string" ? data : JSON.stringify(data));
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;
    connect();
    return () => {
      disconnect();
    };
  }, [enabled, connect, disconnect]);

  return { status, lastMessage, send, connect, disconnect };
}
