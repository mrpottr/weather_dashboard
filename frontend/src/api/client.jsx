/* ==========================================================================
   client.jsx — the single API layer plus the shared React contexts that every
   page and component consumes: authentication (JWT), theme, live clock and the
   active Australian location.

   Design rules enforced here:
     * every network call goes through the Go backend (no third-party weather
       calls are ever made from the browser);
     * every failure surfaces as an ApiError with the backend's {error, code}
       shape so the UI can render honest loading / empty / error states;
     * token storage is centralised so a 401 always clears the session.
   ========================================================================== */

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

// The toast + modal contexts live with the shared UI primitives in UI.jsx.
// Pages that only talk to the API layer import useToast from here, so it is
// re-exported to keep a single import site for data + feedback helpers.
export { useToast, useModal } from "../components/UI.jsx";

/* ------------------------------------------------------------------ keys */
const TOKEN_KEY = "atmosToken";
const THEME_KEY = "atmosTheme";
const CLOCK_KEY = "atmosClock";
const LOCATION_KEY = "atmosLocation";
const DATE_KEY = "atmosDate";

/* --------------------------------------------------------------- helpers */

/** Serialises a params object, skipping empty values. */
export function qs(params) {
  const sp = new URLSearchParams();
  Object.entries(params || {}).forEach(([k, v]) => {
    if (v === undefined || v === null || v === "") return;
    sp.set(k, String(v));
  });
  const s = sp.toString();
  return s ? `?${s}` : "";
}

/** The single error type used by every call site. */
export class ApiError extends Error {
  constructor(message, code = "error", status = 0) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

export function getToken() {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

function storeToken(t) {
  try {
    if (t) localStorage.setItem(TOKEN_KEY, t);
    else localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* storage disabled — session simply will not persist */
  }
}

/** Core fetch wrapper: attaches the bearer token, unwraps JSON, maps errors. */
async function request(path, opts = {}) {
  const { method = "GET", body, raw = false, headers = {} } = opts;
  const h = { Accept: "application/json", ...headers };
  const tok = getToken();
  if (tok) h.Authorization = `Bearer ${tok}`;

  let res;
  try {
    res = await fetch("/api" + path, {
      method,
      headers: h,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(
      "Could not reach the ATMOS server. Make sure the Go backend is running on port 8080.",
      "network_error",
      0
    );
  }

  const isJson = (res.headers.get("content-type") || "").includes("application/json");

  if (!res.ok) {
    let payload = null;
    try {
      payload = isJson ? await res.json() : await res.text();
    } catch {
      /* ignore body parse failures */
    }
    const message =
      (payload && payload.error) ||
      (typeof payload === "string" && payload) ||
      `Request failed with status ${res.status}.`;
    const code = (payload && payload.code) || `http_${res.status}`;
    if (res.status === 401) storeToken(null);
    throw new ApiError(message, code, res.status);
  }

  if (raw) return res;
  if (res.status === 204) return null;
  if (!isJson) return await res.text();
  return await res.json();
}

/**
 * Triggers a browser download for a streamed export response. The filename is
 * read from Content-Disposition when the backend supplies one.
 */
export async function saveBlob(res, fallbackName) {
  const blob = await res.blob();
  const cd = res.headers.get("content-disposition") || "";
  const m = /filename="?([^";]+)"?/.exec(cd);
  const name = (m && m[1]) || fallbackName;
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
  return name;
}

/* ------------------------------------------------------------ REST surface */

export const api = {
  /* service */
  health: () => request("/health"),

  /* authentication */
  login: (email, password) => request("/auth/login", { method: "POST", body: { email, password } }),
  register: (payload) => request("/auth/register", { method: "POST", body: payload }),
  me: () => request("/auth/me"),
  updateMe: (payload) => request("/auth/me", { method: "PUT", body: payload }),
  deleteMe: () => request("/auth/me", { method: "DELETE" }),
  getPreferences: () => request("/preferences"),
  putPreferences: (payload) => request("/preferences", { method: "PUT", body: payload }),

  /* weather (BOM via Open-Meteo, backend-only) */
  forecast: (params) => request("/weather/forecast" + qs(params)),
  geocode: (q) => request("/weather/geocode" + qs({ q })),
  markers: () => request("/weather/markers"),
  metaLocations: () => request("/meta/locations"),

  /* saved locations (both roles) */
  listLocations: (q) => request("/locations" + qs({ q })),
  createLocation: (payload) => request("/locations", { method: "POST", body: payload }),
  deleteLocation: (id) => request(`/locations/${id}`, { method: "DELETE" }),

  /* intelligence */
  intelligence: (params) => request("/intelligence" + qs(params)),
  activities: () => request("/intelligence/activities"),

  /* alerts, notifications, announcements */
  alerts: (params) => request("/alerts" + qs(params)),
  dismissAlert: (id) => request(`/alerts/${id}/dismiss`, { method: "POST" }),
  notifications: () => request("/notifications"),
  announcements: () => request("/announcements"),
  dismissAnnouncement: (id) => request(`/announcements/${id}/dismiss`, { method: "POST" }),

  /* history, accuracy */
  history: (params) => request("/history" + qs(params)),
  historyStats: () => request("/history/stats"),
  accuracy: (params) => request("/accuracy" + qs(params)),

  /* exports (streamed attachments) */
  exportForecast: (params) => request("/exports/forecast" + qs(params), { raw: true }),
  exportReport: (params) => request("/exports/report" + qs(params), { raw: true }),
  exportHistory: (params) => request("/exports/history" + qs(params), { raw: true }),

  /* hero photos */
  townImages: (name) => request("/images/town" + qs({ name })),

  /* administrator layer */
  adminUsers: (params) => request("/admin/users" + qs(params)),
  adminUsersExport: () => request("/admin/users/export", { raw: true }),
  adminUpdateUserStatus: (id, status) =>
    request(`/admin/users/${id}/status`, { method: "PUT", body: { status } }),
  adminAnnouncements: () => request("/admin/announcements"),
  adminCreateAnnouncement: (payload) =>
    request("/admin/announcements", { method: "POST", body: payload }),
  adminUpdateAnnouncement: (id, payload) =>
    request(`/admin/announcements/${id}`, { method: "PUT", body: payload }),
  adminThresholds: () => request("/admin/thresholds"),
  adminPutThresholds: (payload) => request("/admin/thresholds", { method: "PUT", body: payload }),
  adminRefresh: () => request("/admin/refresh", { method: "POST" }),
  adminSourceCheck: () => request("/admin/source/check"),
  adminPreview: (params) => request("/admin/preview" + qs(params)),
  adminLogs: (params) => request("/admin/logs" + qs(params)),
  adminMetrics: () => request("/admin/metrics"),
  adminLoginAs: (userId) =>
    request("/admin/login-as", { method: "POST", body: userId ? { user_id: userId } : {} }),
};

/* ------------------------------------------------------------- data hook */

/**
 * useAsync runs an async loader with first-class loading / error / reload
 * state, so every panel in the UI can render the three required states.
 *
 *   const { data, loading, error, reload } = useAsync(() => api.forecast(loc), [loc], {enabled});
 */
export function useAsync(fn, deps = [], { immediate = true, enabled = true } = {}) {
  const [state, setState] = useState({ data: null, loading: false, error: null });
  const fnRef = useRef(fn);
  fnRef.current = fn;
  const [tick, setTick] = useState(0);

  const run = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }));
    try {
      const data = await fnRef.current();
      setState({ data, loading: false, error: null });
      return data;
    } catch (e) {
      setState({ data: null, loading: false, error: e });
      return null;
    }
  }, []);

  useEffect(() => {
    if (!immediate || !enabled) return;
    run();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick, immediate, enabled]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  // setData accepts a value or an updater, mirroring React's setState, so pages
  // can patch loaded data in place (e.g. after a row-level admin action).
  const setData = useCallback(
    (d) => setState((s) => ({ ...s, data: typeof d === "function" ? d(s.data) : d })),
    []
  );
  return { ...state, reload, run, setData };
}

/* ------------------------------------------------------------ time helpers */

/** Formats a Date as 12h or 24h, seconds included. */
export function formatTime(date, format = "24h") {
  const pad = (n) => String(n).padStart(2, "0");
  const h = date.getHours();
  const m = date.getMinutes();
  const s = date.getSeconds();
  if (format === "12h") {
    const ap = h >= 12 ? "PM" : "AM";
    let hh = h % 12;
    if (hh === 0) hh = 12;
    return `${hh}:${pad(m)}:${pad(s)} ${ap}`;
  }
  return `${pad(h)}:${pad(m)}:${pad(s)}`;
}

/** Formats an ISO "YYYY-MM-DD" as a friendly Australian date. */
export function formatDayLong(iso) {
  if (!iso) return "";
  const d = new Date(iso + "T00:00:00");
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("en-AU", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

/** Formats an ISO "YYYY-MM-DD" as "08 OCT 2026". */
export function formatDayShort(iso) {
  if (!iso) return "";
  const d = new Date(iso + "T00:00:00");
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("en-AU", { day: "2-digit", month: "short", year: "numeric" }).toUpperCase();
}

/** Formats an ISO "YYYY-MM-DD" as "Thu 08". */
export function formatWeekday(iso) {
  if (!iso) return "";
  const d = new Date(iso + "T00:00:00");
  if (Number.isNaN(d.getTime())) return iso.slice(0, 10);
  return d.toLocaleDateString("en-AU", { weekday: "short", day: "2-digit" });
}

/** Converts a "HH:mm" or "YYYY-MM-DDTHH:mm" string into 12h/24h clock text. */
export function formatHourLabel(iso, format = "24h") {
  const raw = (iso || "").slice(11, 16);
  if (!raw) return "";
  if (format !== "12h") return raw;
  const [h, m] = raw.split(":").map(Number);
  const ap = h >= 12 ? "PM" : "AM";
  let hh = h % 12;
  if (hh === 0) hh = 12;
  return `${hh}:${String(m).padStart(2, "0")} ${ap}`;
}

/** "2 min ago" style relative label for ISO timestamps. */
export function relativeTime(iso) {
  if (!iso) return "never";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "unknown";
  const diff = Date.now() - t;
  const mins = Math.round(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins} min ago`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return `${hours} h ago`;
  return `${Math.round(hours / 24)} days ago`;
}

/** "21:11" clock label for a log/timeline timestamp. */
export function clockLabel(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** "08 Oct · 21:11" label for the history timeline. */
export function stampLabel(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return (
    d.toLocaleDateString("en-AU", { day: "2-digit", month: "short" }) +
    " · " +
    clockLabel(iso)
  );
}

export const todayISO = () => {
  const d = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

/* =========================================================================
   Contexts
   ========================================================================= */

const AuthContext = createContext(null);
const ThemeContext = createContext(null);
const ClockContext = createContext(null);
const LocationContext = createContext(null);

export const useAuth = () => useContext(AuthContext) || {};
export const useTheme = () => useContext(ThemeContext) || {};
export const useClock = () => useContext(ClockContext) || {};
export const useLocationContext = () => useContext(LocationContext) || {};

/* --------------------------------------------------------------- Auth ---- */

function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [booting, setBooting] = useState(true);

  const adopt = useCallback((auth) => {
    storeToken(auth.token);
    setUser(auth.user);
    return auth.user;
  }, []);

  const logout = useCallback(() => {
    storeToken(null);
    setUser(null);
  }, []);

  const refreshMe = useCallback(async () => {
    try {
      const me = await api.me();
      setUser(me);
      return me;
    } catch {
      storeToken(null);
      setUser(null);
      return null;
    }
  }, []);

  // Restore an existing session on first paint.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      if (!getToken()) {
        if (!cancelled) setBooting(false);
        return;
      }
      try {
        const me = await api.me();
        if (!cancelled) setUser(me);
      } catch {
        storeToken(null);
      } finally {
        if (!cancelled) setBooting(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(
    async (email, password) => adopt(await api.login(email, password)),
    [adopt]
  );

  const register = useCallback(
    async (payload) => adopt(await api.register(payload)),
    [adopt]
  );

  const loginAs = useCallback(
    async (userId) => adopt(await api.adminLoginAs(userId)),
    [adopt]
  );

  const updateMe = useCallback(async (payload) => {
    const me = await api.updateMe(payload);
    setUser(me);
    return me;
  }, []);

  const deleteMe = useCallback(async () => {
    await api.deleteMe();
    storeToken(null);
    setUser(null);
  }, []);

  const updatePreferences = useCallback(async (patch) => {
    try {
      const prefs = await api.putPreferences(patch);
      setUser((u) => (u ? { ...u, preferences: prefs } : u));
      return prefs;
    } catch {
      return null;
    }
  }, []);

  const value = useMemo(
    () => ({
      user,
      booting,
      isAdmin: user?.role === "admin",
      login,
      register,
      logout,
      loginAs,
      updateMe,
      deleteMe,
      refreshMe,
      updatePreferences,
    }),
    [user, booting, login, register, logout, loginAs, updateMe, deleteMe, refreshMe, updatePreferences]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/* -------------------------------------------------------------- Theme ---- */

function ThemeProvider({ children }) {
  const { user, updatePreferences } = useAuth();
  const [theme, setThemeState] = useState(() => {
    try {
      return localStorage.getItem(THEME_KEY) || "dark";
    } catch {
      return "dark";
    }
  });

  // Paint the body class and remember the choice.
  useEffect(() => {
    document.body.classList.toggle("theme-light", theme === "light");
    document.body.classList.toggle("theme-dark", theme !== "light");
    try {
      localStorage.setItem(THEME_KEY, theme);
    } catch {
      /* ignore */
    }
  }, [theme]);

  // Adopt the signed-in account's stored preference.
  useEffect(() => {
    const t = user?.preferences?.theme;
    if (t === "dark" || t === "light") setThemeState(t);
  }, [user?.preferences?.theme]);

  const setTheme = useCallback(
    (next, origin) => {
      const doc = document;
      const apply = () => setThemeState(next);
      if (origin && typeof doc.startViewTransition === "function") {
        const x = origin.clientX ?? window.innerWidth / 2;
        const y = origin.clientY ?? window.innerHeight / 2;
        const r = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y));
        doc.documentElement.style.setProperty("--vt-x", `${x}px`);
        doc.documentElement.style.setProperty("--vt-y", `${y}px`);
        doc.documentElement.style.setProperty("--vt-r", `${Math.ceil(r)}px`);
        doc.startViewTransition(apply);
      } else {
        apply();
      }
      if (user) updatePreferences({ theme: next });
    },
    [user, updatePreferences]
  );

  const toggleTheme = useCallback((origin) => setTheme(theme === "light" ? "dark" : "light", origin), [
    theme,
    setTheme,
  ]);

  const value = useMemo(() => ({ theme, setTheme, toggleTheme }), [theme, setTheme, toggleTheme]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

/* -------------------------------------------------------------- Clock ---- */

function ClockProvider({ children }) {
  const { user, updatePreferences } = useAuth();
  const [format, setFormat] = useState(() => {
    try {
      return localStorage.getItem(CLOCK_KEY) || "24h";
    } catch {
      return "24h";
    }
  });
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    const f = user?.preferences?.clock_format;
    if (f === "12h" || f === "24h") setFormat(f);
  }, [user?.preferences?.clock_format]);

  const setClockFormat = useCallback(
    (f) => {
      setFormat(f);
      try {
        localStorage.setItem(CLOCK_KEY, f);
      } catch {
        /* ignore */
      }
      if (user) updatePreferences({ clock_format: f });
    },
    [user, updatePreferences]
  );

  const value = useMemo(
    () => ({
      now,
      format,
      setClockFormat,
      time: formatTime(now, format),
      dateLong: now.toLocaleDateString("en-AU", {
        weekday: "long",
        day: "numeric",
        month: "long",
        year: "numeric",
      }),
      zone: Intl.DateTimeFormat().resolvedOptions().timeZone || "Australia/Melbourne",
    }),
    [now, format, setClockFormat]
  );

  return <ClockContext.Provider value={value}>{children}</ClockContext.Provider>;
}

/* ----------------------------------------------------------- Location ---- */

/** The eight capital cities, mirrored from the backend meta endpoint. */
export const FALLBACK_LOCATIONS = [
  { name: "Melbourne, VIC", latitude: -37.8136, longitude: 144.9631, timezone: "Australia/Melbourne", admin_area: "VIC" },
  { name: "Sydney, NSW", latitude: -33.8688, longitude: 151.2093, timezone: "Australia/Sydney", admin_area: "NSW" },
  { name: "Brisbane, QLD", latitude: -27.4698, longitude: 153.0251, timezone: "Australia/Brisbane", admin_area: "QLD" },
  { name: "Perth, WA", latitude: -31.9523, longitude: 115.8613, timezone: "Australia/Perth", admin_area: "WA" },
  { name: "Hobart, TAS", latitude: -42.8821, longitude: 147.3272, timezone: "Australia/Hobart", admin_area: "TAS" },
  { name: "Adelaide, SA", latitude: -34.9285, longitude: 138.6007, timezone: "Australia/Adelaide", admin_area: "SA" },
  { name: "Darwin, NT", latitude: -12.4634, longitude: 130.8456, timezone: "Australia/Darwin", admin_area: "NT" },
  { name: "Canberra, ACT", latitude: -35.2809, longitude: 149.13, timezone: "Australia/Sydney", admin_area: "ACT" },
];

function normaliseLocation(l) {
  if (!l) return null;
  if (typeof l.latitude !== "number" || typeof l.longitude !== "number") return null;
  return {
    name: l.name || `${l.latitude.toFixed(2)}, ${l.longitude.toFixed(2)}`,
    latitude: l.latitude,
    longitude: l.longitude,
    timezone: l.timezone || "Australia/Melbourne",
    admin_area: l.admin_area || "",
  };
}

/**
 * Turns free text into a location the app can fetch for:
 *   "-37.81, 144.96" -> a coordinate reference (timezone resolved by the server)
 *   "Geelong"        -> the best Australian geocoding match
 * Returns null when nothing usable is found.
 */
export async function pickLocationFromInput(text) {
  const t = (text || "").trim();
  if (!t) return null;
  const m = t.match(/^(-?\d{1,2}(?:\.\d+)?)\s*,\s*(-?\d{1,3}(?:\.\d+)?)$/);
  if (m) {
    const latitude = parseFloat(m[1]);
    const longitude = parseFloat(m[2]);
    if (Number.isNaN(latitude) || Number.isNaN(longitude)) return null;
    return normaliseLocation({
      name: `${latitude.toFixed(2)}, ${longitude.toFixed(2)}`,
      latitude,
      longitude,
      timezone: "",
    });
  }
  const res = await api.geocode(t);
  const hit = res?.results?.[0];
  if (!hit) return null;
  return normaliseLocation({
    name: hit.admin_area ? `${hit.name}, ${hit.admin_area}` : hit.name,
    latitude: hit.latitude,
    longitude: hit.longitude,
    timezone: hit.timezone,
    admin_area: hit.admin_area,
  });
}

function LocationProvider({ children }) {
  const { user } = useAuth();
  const [meta, setMeta] = useState(FALLBACK_LOCATIONS);
  const [location, setLocationState] = useState(() => {
    try {
      const raw = localStorage.getItem(LOCATION_KEY);
      return raw ? normaliseLocation(JSON.parse(raw)) : null;
    } catch {
      return null;
    }
  });
  const [date, setDateState] = useState(() => {
    try {
      return localStorage.getItem(DATE_KEY) || todayISO();
    } catch {
      return todayISO();
    }
  });

  const setDate = useCallback((d) => {
    if (!d) return;
    setDateState(d);
    try {
      localStorage.setItem(DATE_KEY, d);
    } catch {
      /* ignore */
    }
  }, []);

  // Pull the authoritative capital-city list once per session.
  useEffect(() => {
    let cancelled = false;
    api
      .metaLocations()
      .then((res) => {
        if (cancelled || !res?.locations?.length) return;
        setMeta(res.locations.map((l) => normaliseLocation(l)).filter(Boolean));
      })
      .catch(() => {
        /* keep the mirrored fallback list */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const setLocation = useCallback((l) => {
    const n = normaliseLocation(l);
    if (!n) return;
    setLocationState(n);
    try {
      localStorage.setItem(LOCATION_KEY, JSON.stringify(n));
    } catch {
      /* ignore */
    }
  }, []);

  // When a user signs in (and nothing is stored), adopt their default location.
  useEffect(() => {
    if (!user || location) return;
    const wanted = (user.default_location || "").toLowerCase();
    const match =
      meta.find((m) => m.name.toLowerCase() === wanted) ||
      meta.find((m) => wanted.startsWith(m.name.split(",")[0].toLowerCase())) ||
      meta[0];
    if (match) setLocation(match);
  }, [user, location, meta, setLocation]);

  const value = useMemo(
    () => ({
      location: location || meta[0],
      setLocation,
      meta,
      date,
      setDate,
      isDefault: !!location === false,
    }),
    [location, setLocation, meta, date, setDate]
  );

  return <LocationContext.Provider value={value}>{children}</LocationContext.Provider>;
}

/* -------------------------------------------------------- root provider --- */

/** Composes every shared context in the required order. */
export function AppProviders({ children }) {
  return (
    <AuthProvider>
      <ThemeProvider>
        <ClockProvider>
          <LocationProvider>{children}</LocationProvider>
        </ClockProvider>
      </ThemeProvider>
    </AuthProvider>
  );
}
