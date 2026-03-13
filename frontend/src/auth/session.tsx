import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

import { SESSION_STORAGE_KEY } from "@/app/constants";
import { usersApi } from "@/api/endpoints";
import { setUnauthorizedHandler } from "@/api/client";
import type { AuthUser } from "@/api/types";

type SessionState = {
  token: string | null;
  user: AuthUser | null;
};

type SessionContextValue = SessionState & {
  isBootstrapping: boolean;
  login: (token: string, user: AuthUser) => void;
  logout: () => void;
  setUser: (user: AuthUser | null) => void;
};

const SessionContext = createContext<SessionContextValue | null>(null);

function readInitialSession(): SessionState {
  try {
    const raw = localStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) {
      return { token: null, user: null };
    }
    const parsed = JSON.parse(raw) as SessionState;
    return {
      token: parsed.token ?? null,
      user: parsed.user ?? null,
    };
  } catch {
    return { token: null, user: null };
  }
}

export function SessionProvider({ children }: PropsWithChildren) {
  const [session, setSession] = useState<SessionState>(() => readInitialSession());
  const [isBootstrapping, setIsBootstrapping] = useState(Boolean(session.token));

  const persist = useCallback((next: SessionState) => {
    setSession(next);
    if (!next.token) {
      localStorage.removeItem(SESSION_STORAGE_KEY);
      return;
    }
    localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(next));
  }, []);

  const logout = useCallback(() => {
    persist({ token: null, user: null });
  }, [persist]);

  const login = useCallback(
    (token: string, user: AuthUser) => {
      persist({ token, user });
    },
    [persist],
  );

  const setUser = useCallback(
    (user: AuthUser | null) => {
      persist({
        token: session.token,
        user,
      });
    },
    [persist, session.token],
  );

  useEffect(() => {
    setUnauthorizedHandler(logout);
    return () => setUnauthorizedHandler(null);
  }, [logout]);

  useEffect(() => {
    if (!session.token) {
      setIsBootstrapping(false);
      return;
    }

    let active = true;
    setIsBootstrapping(true);
    usersApi
      .me()
      .then((user) => {
        if (!active) {
          return;
        }
        persist({ token: session.token, user });
      })
      .catch(() => {
        if (!active) {
          return;
        }
        logout();
      })
      .finally(() => {
        if (active) {
          setIsBootstrapping(false);
        }
      });

    return () => {
      active = false;
    };
  }, [logout, persist, session.token]);

  const value = useMemo<SessionContextValue>(
    () => ({
      ...session,
      isBootstrapping,
      login,
      logout,
      setUser,
    }),
    [session, isBootstrapping, login, logout, setUser],
  );

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession() {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("useSession must be used within SessionProvider");
  }
  return context;
}
