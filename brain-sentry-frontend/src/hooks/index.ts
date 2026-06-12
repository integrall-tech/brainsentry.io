import { useState, useEffect, useCallback, useRef } from "react";

import { getStoredTenantId } from "@/lib/api/client";

// Hook for fetching data with loading and error states
interface UseFetchResult<T> {
  data: T | null;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
}

export function useFetch<T>(
  url: string,
  options?: RequestInit & { skip?: boolean }
): UseFetchResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async () => {
    if (options?.skip) return;

    setIsLoading(true);
    setError(null);

    try {
      const token = localStorage.getItem("brain_sentry_token");
      const tenantId = getStoredTenantId();
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...(tenantId ? { "X-Tenant-ID": tenantId } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(options?.headers as Record<string, string> || {}),
      };
      const response = await fetch(url, { ...options, headers });
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const json = await response.json();
      setData(json);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [url, options]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, isLoading, error, refetch: fetchData };
}

// Hook for debouncing values
export function useDebounce<T>(value: T, delay: number = 500): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value);

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => {
      clearTimeout(handler);
    };
  }, [value, delay]);

  return debouncedValue;
}

// Hook for local storage with state sync
export function useLocalStorage<T>(
  key: string,
  initialValue: T
): [T, (value: T | ((val: T) => T)) => void] {
  const [storedValue, setStoredValue] = useState<T>(() => {
    try {
      const item = window.localStorage.getItem(key);
      return item ? JSON.parse(item) : initialValue;
    } catch (error) {
      console.warn(`Error reading localStorage key "${key}":`, error);
      return initialValue;
    }
  });

  const setValue = (value: T | ((val: T) => T)) => {
    try {
      const valueToStore = value instanceof Function ? value(storedValue) : value;
      setStoredValue(valueToStore);
      window.localStorage.setItem(key, JSON.stringify(valueToStore));
    } catch (error) {
      console.warn(`Error setting localStorage key "${key}":`, error);
    }
  };

  return [storedValue, setValue];
}

// Hook for async operations with loading state
export function useAsync<T>() {
  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const execute = useCallback(async (asyncFn: () => Promise<T>) => {
    setIsLoading(true);
    setError(null);

    try {
      const result = await asyncFn();
      setData(result);
      return result;
    } catch (err) {
      const error = err as Error;
      setError(error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const reset = useCallback(() => {
    setData(null);
    setError(null);
    setIsLoading(false);
  }, []);

  return { data, isLoading, error, execute, reset };
}

// Hook for toggle state
export function useToggle(initialValue: boolean = false) {
  const [value, setValue] = useState(initialValue);

  const toggle = useCallback(() => setValue((v) => !v), []);

  return [value, toggle, setValue] as const;
}

// Hook for previous value
export function usePrevious<T>(value: T): T | undefined {
  const ref = useRef<T | undefined>(undefined);

  useEffect(() => {
    ref.current = value;
  }, [value]);

  return ref.current;
}

// Hook for interval
export function useInterval(callback: () => void, delay: number | null) {
  const savedCallback = useRef(callback);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    if (delay === null) return;

    const tick = () => savedCallback.current();
    const id = setInterval(tick, delay);

    return () => clearInterval(id);
  }, [delay]);
}

// Hook for media query
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window !== "undefined") {
      return window.matchMedia(query).matches;
    }
    return false;
  });

  useEffect(() => {
    const mediaQuery = window.matchMedia(query);
    const handler = (event: MediaQueryListEvent) => setMatches(event.matches);

    mediaQuery.addEventListener("change", handler);
    return () => mediaQuery.removeEventListener("change", handler);
  }, [query]);

  return matches;
}

// Hook for detecting click outside
export function useClickOutside<T extends HTMLElement>(
  ref: React.RefObject<T>,
  handler: (event: MouseEvent | TouchEvent) => void
) {
  useEffect(() => {
    const listener = (event: MouseEvent | TouchEvent) => {
      if (!ref.current || ref.current.contains(event.target as Node)) {
        return;
      }
      handler(event);
    };

    document.addEventListener("mousedown", listener);
    document.addEventListener("touchstart", listener);

    return () => {
      document.removeEventListener("mousedown", listener);
      document.removeEventListener("touchstart", listener);
    };
  }, [ref, handler]);
}

// Hook for copy to clipboard
export function useClipboard(text: string, timeout = 1500) {
  const [hasCopied, setHasCopied] = useState(false);

  const onCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
      setHasCopied(true);

      setTimeout(() => {
        setHasCopied(false);
      }, timeout);
    } catch (error) {
      console.error("Failed to copy:", error);
    }
  }, [text, timeout]);

  return { hasCopied, onCopy };
}

// Hook for form state management
interface UseFormReturn<T> {
  values: T;
  handleChange: (field: keyof T) => (value: T[keyof T]) => void;
  setValues: (values: T) => void;
  reset: (defaults?: T) => void;
}

export function useForm<T extends Record<string, any>>(
  initialValues: T
): UseFormReturn<T> {
  const [values, setValues] = useState<T>(initialValues);

  const handleChange =
    (field: keyof T) => (value: T[keyof T]) => {
      setValues((prev) => ({ ...prev, [field]: value }));
    };

  const reset = (defaults?: T) => {
    setValues(defaults || initialValues);
  };

  return {
    values,
    handleChange,
    setValues,
    reset,
  };
}

// Hook for infinite scroll
interface UseInfiniteScrollProps {
  callback: () => void;
  hasMore: boolean;
  rootMargin?: string;
}

export function useInfiniteScroll({
  callback,
  hasMore,
  rootMargin = "100px",
}: UseInfiniteScrollProps) {
  const observerTarget = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!hasMore) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          callback();
        }
      },
      { rootMargin }
    );

    const current = observerTarget.current;
    if (current) {
      observer.observe(current);
    }

    return () => {
      if (current) {
        observer.unobserve(current);
      }
    };
  }, [callback, hasMore, rootMargin]);

  return { observerTarget };
}

// Hook for keyboard shortcuts
export function useKeyboard(
  keys: string[],
  callback: (event: KeyboardEvent) => void,
  deps: React.DependencyList = []
) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (keys.includes(event.key)) {
        callback(event);
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [keys, callback, deps]);
}

export { useWebSocket } from "./useWebSocket";
export type { WebSocketStatus, WebSocketMessage } from "./useWebSocket";
