import { useState, useEffect, useRef, useCallback } from 'react';

/**
 * useWebSocket - connects to the backend WebSocket and returns the latest
 * leaderboard data, auto-reconnecting on disconnect.
 */
export default function useWebSocket() {
  const [leaderboard, setLeaderboard] = useState([]);
  const [connected, setConnected] = useState(false);
  const [updateTrigger, setUpdateTrigger] = useState(0);
  const wsRef = useRef(null);
  const reconnectTimeout = useRef(null);
  const isUnmounted = useRef(false);

  const connect = useCallback(function connectFn() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/ws`;

    const socket = new WebSocket(url);
    wsRef.current = socket;

    socket.onopen = () => {
      setConnected(true);
    };

    socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'leaderboard_update' && Array.isArray(data.payload)) {
          setLeaderboard(data.payload);
          setUpdateTrigger(prev => prev + 1);
        }
      } catch {
        // Ignore malformed messages
      }
    };

    socket.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      // Only reconnect if the component hasn't unmounted
      if (!isUnmounted.current) {
        reconnectTimeout.current = setTimeout(connectFn, 3000);
      }
    };

    socket.onerror = () => {
      socket.close();
    };
  }, []);

  useEffect(() => {
    isUnmounted.current = false;
    connect();
    return () => {
      isUnmounted.current = true;
      if (reconnectTimeout.current) clearTimeout(reconnectTimeout.current);
      if (wsRef.current) wsRef.current.close();
    };
  }, [connect]);

  return { leaderboard, connected, updateTrigger };
}
