import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

/* ----------------------------------------------------------- icon sprite */

/** The 18 symbols used across the prototype, rendered once at the app root. */
export function IconSprite() {
  return (
    <svg width="0" height="0" style={{ position: "absolute" }} aria-hidden="true" focusable="false">
      <symbol id="i-cloud-sun" viewBox="0 0 24 24">
        <path d="M8 18h9a4 4 0 0 0 .6-7.95A6 6 0 0 0 6.2 8.7 4.7 4.7 0 0 0 8 18Z" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M6.5 6.5V4.7M3.9 7.6 2.6 6.3M10.1 7.6l1.3-1.3M5.2 10H3.4" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6" />
      </symbol>
      <symbol id="i-grid" viewBox="0 0 24 24">
        <rect x="3" y="3" width="7" height="7" rx="1.6" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <rect x="14" y="3" width="7" height="7" rx="1.6" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <rect x="3" y="14" width="7" height="7" rx="1.6" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <rect x="14" y="14" width="7" height="7" rx="1.6" fill="none" stroke="currentColor" strokeWidth="1.7" />
      </symbol>
      <symbol id="i-map" viewBox="0 0 24 24">
        <path d="M9 18 3.8 20V6L9 4l6 2 5.2-2v14L15 20l-6-2Z" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M9 4v14M15 6v14" fill="none" stroke="currentColor" strokeWidth="1.7" />
      </symbol>
      <symbol id="i-chart" viewBox="0 0 24 24">
        <path d="M4 19V5M4 19h16" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="m6 15 4-4 3 2 5-6" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" />
      </symbol>
      <symbol id="i-heart" viewBox="0 0 24 24">
        <path d="M20.8 8.8c0 5-8.8 10.2-8.8 10.2S3.2 13.8 3.2 8.8A4.7 4.7 0 0 1 8 4.1c1.5 0 3 .7 4 1.9a5.1 5.1 0 0 1 4-1.9 4.7 4.7 0 0 1 4.8 4.7Z" fill="none" stroke="currentColor" strokeWidth="1.7" />
      </symbol>
      <symbol id="i-history" viewBox="0 0 24 24">
        <path d="M4 12a8 8 0 1 0 2.3-5.7L4 8.6" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M4 4v4.6h4.6M12 7v5l3 2" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      </symbol>
      <symbol id="i-download" viewBox="0 0 24 24">
        <path d="M12 3v11" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
        <path d="m8 11 4 4 4-4M4 20h16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      </symbol>
      <symbol id="i-user" viewBox="0 0 24 24">
        <circle cx="12" cy="8" r="4" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M4.5 20a7.5 7.5 0 0 1 15 0" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      </symbol>
      <symbol id="i-settings" viewBox="0 0 24 24">
        <path d="m9.5 3 .8 2a7.8 7.8 0 0 0 2.6 0l.8-2 2.2.9-.4 2a8 8 0 0 1 2 2l2-.4.9 2.2-1.9.8a8 8 0 0 1 0 2.8l1.9.8-.9 2.2-2-.4a8 8 0 0 1-2 2l.4 2-2.2.9-.8-2a7.8 7.8 0 0 0-2.6 0l-.8 2-2.2-.9.4-2a8 8 0 0 1-2-2l-2 .4L3.1 14l1.9-.8a8 8 0 0 1 0-2.8l-1.9-.8L4 7.4l2 .4a8 8 0 0 1 2-2l-.4-2L9.5 3Z" fill="none" stroke="currentColor" strokeWidth="1.1" />
        <circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" strokeWidth="1.5" />
      </symbol>
      <symbol id="i-bell" viewBox="0 0 24 24">
        <path d="M5 18h14l-1.5-2V11a5.5 5.5 0 0 0-11 0v5L5 18Z" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M10 21h4" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
      </symbol>
      <symbol id="i-search" viewBox="0 0 24 24">
        <circle cx="10.8" cy="10.8" r="6.5" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="m16 16 5 5" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      </symbol>
      <symbol id="i-refresh" viewBox="0 0 24 24">
        <path d="M20 11a8 8 0 0 0-14-5l-2 2M4 7V3m0 4h4M4 13a8 8 0 0 0 14 5l2-2m0 0v4m0-4h-4" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
      </symbol>
      <symbol id="i-shield" viewBox="0 0 24 24">
        <path d="M12 3 20 6v5c0 5-3.3 8.2-8 10-4.7-1.8-8-5-8-10V6l8-3Z" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="m9 12 2 2 4-4" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
      </symbol>
      <symbol id="i-menu" viewBox="0 0 24 24">
        <path d="M4 6h16M4 12h16M4 18h16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      </symbol>
      <symbol id="i-moon" viewBox="0 0 24 24">
        <path d="M20 15.2A8.5 8.5 0 0 1 8.8 4 8.5 8.5 0 1 0 20 15.2Z" fill="none" stroke="currentColor" strokeWidth="1.7" />
      </symbol>
      <symbol id="i-sun" viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="4" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      </symbol>
      <symbol id="i-log-out" viewBox="0 0 24 24">
        <path d="M10 4H4v16h6M15 8l4 4-4 4M19 12H8" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
      </symbol>
      <symbol id="i-broadcast" viewBox="0 0 24 24">
        <path d="M8 9a4.8 4.8 0 0 0 0 6M4.8 6.2a9 9 0 0 0 0 11.6M16 9a4.8 4.8 0 0 1 0 6M19.2 6.2a9 9 0 0 1 0 11.6" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
        <circle cx="12" cy="12" r="2.3" fill="none" stroke="currentColor" strokeWidth="1.7" />
      </symbol>
      <symbol id="i-lock" viewBox="0 0 24 24">
        <rect x="5" y="10" width="14" height="11" rx="2" fill="none" stroke="currentColor" strokeWidth="1.7" />
        <path d="M8 10V7a4 4 0 0 1 8 0v3" fill="none" stroke="currentColor" strokeWidth="1.7" />
      </symbol>
    </svg>
  );
}

/** Renders one sprite symbol via the prototype's <use href="#id"> pattern. */
export function Icon({ name, size, className, style }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      style={{ width: size, height: size, ...style }}
      aria-hidden="true"
      focusable="false"
    >
      <use href={`#i-${name}`} />
    </svg>
  );
}

/* --------------------------------------------------------- toast context */

const ToastContext = createContext(null);
export const useToast = () => useContext(ToastContext) || (() => { });

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([]);
  const idRef = useRef(0);

  const push = useCallback((title, message, icon = "✓", kind = "default") => {
    const id = ++idRef.current;
    setToasts((t) => [...t, { id, title, message, icon, kind }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4400);
  }, []);

  const value = useMemo(() => push, [push]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-stack" role="status" aria-live="polite">
        {toasts.map((t) => (
          <div className="toast" key={t.id}>
            <div className="toast-icon">{t.icon}</div>
            <div>
              <strong>{t.title}</strong>
              {t.message ? <p>{t.message}</p> : null}
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

/* --------------------------------------------------------- modal context */

const ModalContext = createContext(null);
export const useModal = () => useContext(ModalContext) || { modal: null, openModal: () => { }, closeModal: () => { } };

export function ModalProvider({ children }) {
  const [modal, setModal] = useState(null); // { title, body, wide }

  const openModal = useCallback((title, body, opts = {}) => {
    setModal({ title, body, wide: !!opts.wide });
  }, []);
  const closeModal = useCallback(() => setModal(null), []);

  // Escape closes the modal, matching the prototype.
  useEffect(() => {
    if (!modal) return;
    const onKey = (e) => {
      if (e.key === "Escape") closeModal();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [modal, closeModal]);

  const value = useMemo(() => ({ modal, openModal, closeModal }), [modal, openModal, closeModal]);

  return (
    <ModalContext.Provider value={value}>
      {children}
      <div
        className={"modal-backdrop" + (modal ? " show" : "")}
        onClick={(e) => {
          if (e.target === e.currentTarget) closeModal();
        }}
      >
        <div className={"modal" + (modal?.wide ? " wide" : "")} role="dialog" aria-modal="true">
          <div className="modal-head">
            <h3>{modal?.title || "Modal"}</h3>
            <button className="icon-btn" onClick={closeModal} aria-label="Close dialog">
              ×
            </button>
          </div>
          <div>{modal?.body}</div>
        </div>
      </div>
    </ModalContext.Provider>
  );
}

/* ------------------------------------------------------------ UI states */

/** Centred spinner + label used while a panel is loading. */
export function Loading({ label = "Loading live weather…" }) {
  return (
    <div className="empty">
      <div className="loader" />
      <div>{label}</div>
    </div>
  );
}

/** Empty state with an optional action. */
export function Empty({ message, action }) {
  return (
    <div className="empty">
      <div>{message}</div>
      {action ? <div style={{ marginTop: 12 }}>{action}</div> : null}
    </div>
  );
}

/** Error state with a retry affordance. */
export function ErrorState({ error, onRetry, label = "Something went wrong" }) {
  const msg = error?.message || "Unknown error.";
  return (
    <div className="empty">
      <div style={{ color: "var(--danger)", marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 11 }}>{msg}</div>
      {onRetry ? (
        <button className="ghost-btn" style={{ marginTop: 12 }} onClick={onRetry}>
          Try again
        </button>
      ) : null}
    </div>
  );
}

/**
 * Renders the correct one of the three required states for a data panel, or
 * the panel's children when data is present.
 */
export function AsyncPanel({ loading, error, empty, onRetry, loadingLabel, emptyLabel, children }) {
  if (loading) return <Loading label={loadingLabel} />;
  if (error) return <ErrorState error={error} onRetry={onRetry} />;
  if (empty) return <Empty message={emptyLabel} />;
  return children;
}

/* ------------------------------------------------------------ form bits */

/** Accessible label + control field block used across the auth + admin forms. */
export function Field({ label, children, htmlFor }) {
  return (
    <div className="field">
      <label htmlFor={htmlFor}>{label}</label>
      {children}
    </div>
  );
}

/** The prototype's on/off switch bound to a boolean. */
export function Toggle({ checked, onChange, label, hint }) {
  return (
    <div className="toggle-row">
      <div>
        <strong style={{ fontSize: 11 }}>{label}</strong>
        {hint ? (
          <div style={{ fontSize: 9, color: "var(--muted)", marginTop: 3 }}>{hint}</div>
        ) : null}
      </div>
      <label className="toggle">
        <input type="checkbox" checked={!!checked} onChange={(e) => onChange(e.target.checked)} />
        <span className="slider" />
      </label>
    </div>
  );
}

/** Panel with an optional head (title, kicker, right-hand accessory). */
export function Panel({ title, kicker, right, children, className = "", style }) {
  return (
    <div className={"panel " + className} style={style}>
      {title || right ? (
        <div className="panel-head">
          <div>
            {title ? <div className="panel-title">{title}</div> : null}
            {kicker ? <div className="panel-kicker">{kicker}</div> : null}
          </div>
          {right}
        </div>
      ) : null}
      {children}
    </div>
  );
}

/** Metric card from the prototype's dashboard grids. */
export function MetricCard({ label, value, meta }) {
  return (
    <div className="metric-card">
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
      {meta ? <div className="metric-meta">{meta}</div> : null}
    </div>
  );
}

/** Page heading block. */
export function PageHead({ title, sub, right }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", gap: 14, alignItems: "flex-start", flexWrap: "wrap" }}>
      <div>
        <div className="page-title">{title}</div>
        <p className="page-sub">{sub}</p>
      </div>
      {right}
    </div>
  );
}

/** Small status tag with the prototype's success / warn / danger variants. */
export function Tag({ variant = "", children }) {
  return <span className={"tag" + (variant ? " " + variant : "")}>{children}</span>;
}

/** A horizontal labelled progress bar (used by accuracy + admin panels). */
export function HealthBar({ label, pct, value }) {
  const safe = Math.max(0, Math.min(100, Number(pct) || 0));
  return (
    <div className="health">
      <div>
        <label>{label}</label>
        <div className="bar">
          <i style={{ width: `${safe}%` }} />
        </div>
      </div>
      <b>{value != null ? value : `${Math.round(safe)}%`}</b>
    </div>
  );
}

/**
 * Maps a plain-language condition returned by the backend to the prototype's
 * weather glyphs. Presentation-only: the text always comes from the API.
 */
export function conditionIcon(condition) {
  const c = (condition || "").toLowerCase();
  if (c.includes("thunder") || c.includes("storm")) return "⛈️";
  if (c.includes("heavy rain")) return "🌧️";
  if (c.includes("rain") || c.includes("shower") || c.includes("drizzle")) return "🌦️";
  if (c.includes("fog") || c.includes("mist")) return "🌫️";
  if (c.includes("overcast")) return "☁️";
  if (c.includes("partly")) return "⛅";
  if (c.includes("cloud")) return "🌤️";
  if (c.includes("clear") || c.includes("sunny") || c.includes("sun")) return "☀️";
  return "🌤️";
}
