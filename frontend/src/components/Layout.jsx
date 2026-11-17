
import React, { useEffect, useRef, useState } from "react";
import { NavLink, Navigate, Outlet, useLocation as useRouteLocation } from "react-router-dom";
import {
  api,
  useAuth,
  useClock,
  useLocationContext,
  useTheme,
  useAsync,
} from "../api/client.jsx";
import { Icon, ToastProvider, ModalProvider, useToast, useModal } from "./UI.jsx";

/* ------------------------------------------------------------ nav model */

const WORKSPACE_NAV = [
  { to: "/overview", label: "Overview", icon: "grid" },
  { to: "/forecast", label: "Forecast", icon: "cloud-sun" },
  { to: "/map", label: "Weather map", icon: "map" },
  { to: "/intelligence", label: "Intelligence", icon: "chart" },
  { to: "/insights", label: "Insights", icon: "chart" },
  { to: "/locations", label: "Saved locations", icon: "heart" },
  { to: "/history", label: "History", icon: "history" },
  { to: "/exports", label: "Exports", icon: "download" },
  { to: "/alerts", label: "Alerts", icon: "bell" },
  { to: "/account", label: "Account", icon: "user" },
];

const ADMIN_NAV = [
  { to: "/admin", label: "Admin console", icon: "settings" },
  { to: "/admin/users", label: "User management", icon: "user" },
];

const CRUMBS = {
  "/overview": "Workspace / Overview",
  "/forecast": "Workspace / Forecast",
  "/map": "Workspace / Weather map",
  "/intelligence": "Workspace / Intelligence",
  "/insights": "Workspace / Insights",
  "/locations": "Workspace / Saved locations",
  "/history": "Workspace / History",
  "/exports": "Workspace / Exports",
  "/alerts": "Workspace / Alerts",
  "/account": "Account / Centre",
  "/admin": "Administration / Control room",
  "/admin/users": "Administration / User management",
};

/* ------------------------------------------------------------- guard ---- */

/** Route guard: bounces signed-out visitors to the login screen. */
export function RequireAuth() {
  const { user, booting } = useAuth();
  if (booting) {
    return (
      <div className="empty" style={{ minHeight: "60vh", display: "grid", placeItems: "center" }}>
        <div className="loader" />
        <div style={{ marginTop: 12 }}>Restoring your session…</div>
      </div>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  return <Outlet />;
}

/** Keeps non-administrators out of the administration routes. */
export function RequireAdmin() {
  const { isAdmin } = useAuth();
  if (!isAdmin) {
    return (
      <div className="panel" style={{ marginTop: 20 }}>
        <div className="empty">
          <div style={{ color: "var(--danger)", marginBottom: 6 }}>Access denied</div>
          <div>This function is reserved for administrator accounts.</div>
        </div>
      </div>
    );
  }
  return <Outlet />;
}

/* ------------------------------------------------------------ clock ---- */

/** Live clock block under the navigation (2.4). */
function ClockBlock() {
  const { time, dateLong, zone, format, setClockFormat } = useClock();
  return (
    <div className="sidebar-clock">
      <div className="clock-label">Local time</div>
      <div className="clock-time">{time}</div>
      <div className="clock-date">{dateLong}</div>
      <div className="clock-zone">{zone}</div>
      <div className="clock-toggle">
        <button
          className={format === "12h" ? "active" : ""}
          onClick={() => setClockFormat("12h")}
        >
          12h
        </button>
        <button
          className={format === "24h" ? "active" : ""}
          onClick={() => setClockFormat("24h")}
        >
          24h
        </button>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------- bell ---- */

/** Alert-centre bell with the unread badge and a small dropdown menu. */
function BellMenu({ notifications, onNavigate }) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  const unread = notifications?.unread || 0;

  return (
    <div className="bell-wrap" ref={wrapRef}>
      <button
        className="icon-btn"
        title="Alert centre"
        aria-label={`Alert centre, ${unread} unread`}
        onClick={() => setOpen((o) => !o)}
      >
        <Icon name="bell" />
        {unread > 0 ? <span className="bell-badge">{unread > 99 ? "99+" : unread}</span> : null}
      </button>
      {open ? (
        <div className="bell-menu">
          <div className="bell-head">Alert centre</div>
          <button
            className="bell-item"
            onClick={() => {
              setOpen(false);
              onNavigate("/alerts");
            }}
          >
            <span className="bi-icon">
              <Icon name="bell" size={16} />
            </span>
            <span>
              <strong>Weather alerts</strong>
              <span>Threshold alerts for your locations</span>
            </span>
            <span className="count">{notifications?.weather_count ?? 0}</span>
          </button>
          <button
            className="bell-item"
            onClick={() => {
              setOpen(false);
              onNavigate("/alerts?tab=system");
            }}
          >
            <span className="bi-icon">
              <Icon name="broadcast" size={16} />
            </span>
            <span>
              <strong>System notices</strong>
              <span>Announcements from administration</span>
            </span>
            <span className="count">{notifications?.system_count ?? 0}</span>
          </button>
        </div>
      ) : null}
    </div>
  );
}

/* --------------------------------------------------------- the shell ---- */

function Shell() {
  const { user, isAdmin, logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const { location } = useLocationContext();
  const toast = useToast();
  const { openModal } = useModal();
  const navigate = useRouteLocation();

  const notifications = useAsync(() => api.notifications(), [], { immediate: !!user });
  // Poll the badge every 60 seconds so the count stays honest.
  useEffect(() => {
    if (!user) return;
    const t = setInterval(() => notifications.reload(), 60000);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user]);

  const crumb = CRUMBS[navigate.pathname] || "Workspace";
  const firstName = user?.first_name || "there";
  const initials =
    ((user?.first_name || "?")[0] || "") + ((user?.last_name || "")[0] || "");
  const displayName = user ? `${user.first_name} ${user.last_name}`.trim() : "Signed out";
  const roleLabel = isAdmin ? "Administrator" : "Standard user";

  const go = (path) => {
    window.history.pushState({}, "", path);
    window.dispatchEvent(new PopStateEvent("popstate"));
  };

  const onSignOut = () => {
    logout();
    toast("Signed out", "Your weather workspace has been closed.", "↗");
  };

  const openAccount = () => {
    openModal(
      "Account centre",
      <div>
        <div className="notice-card">
          <div className="notice-copy">
            <div className="notice-icon">
              <Icon name="user" size={18} />
            </div>
            <div>
              <strong>{displayName}</strong>
              <p>
                {roleLabel} · Default location {location?.name || "—"}
              </p>
            </div>
          </div>
        </div>
        <p style={{ fontSize: 11, color: "var(--muted)", lineHeight: 1.7, marginTop: 12 }}>
          Open the Account centre to change your name, email, password, default
          location and theme preferences, or to delete your account.
        </p>
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 12 }}>
          <button className="ghost-btn" onClick={() => go("/account")}>
            Open account centre
          </button>
        </div>
      </div>
    );
  };

  return (
    <div className="app">
      <div className="shell">
        <aside className="sidebar">
          <div className="side-brand">
            <span className="square">
              <Icon name="cloud-sun" />
            </span>
            ATMOS
          </div>

          <div className="side-section">Workspace</div>
          {WORKSPACE_NAV.map((n) => (
            <NavLink key={n.to} to={n.to} className="nav-btn">
              <Icon name={n.icon} />
              {n.label}
            </NavLink>
          ))}

          {isAdmin ? (
            <>
              <div className="side-section admin-nav-label">Administration</div>
              {ADMIN_NAV.map((n) => (
                <NavLink key={n.to} to={n.to} className="nav-btn">
                  <Icon name={n.icon} />
                  {n.label}
                </NavLink>
              ))}
            </>
          ) : null}

          <div className="nav-spacer" />
          <ClockBlock />

          <button
            className="nav-btn"
            onClick={(e) => toggleTheme(e.currentTarget)}
            title={theme === "light" ? "Switch to dark / aurora theme" : "Switch to light theme"}
          >
            <Icon name={theme === "light" ? "sun" : "moon"} />
            Switch theme
          </button>
          <button className="nav-btn" onClick={onSignOut}>
            <Icon name="log-out" />
            Sign out
          </button>

          <div
            className="profile-mini"
            onClick={openAccount}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => e.key === "Enter" && openAccount()}
          >
            <div className="avatar">{initials.toUpperCase() || "?"}</div>
            <div>
              <strong>{firstName}</strong>
              <span>{roleLabel}</span>
            </div>
          </div>
        </aside>

        <main className="main">
          <header className="topbar">
            <div className="top-left">
              <div className="crumb">{crumb}</div>
            </div>
            <div className="top-right">
              <div
                className={
                  "status-pill" +
                  (notifications.data && notifications.data.unread > 0 ? " cached" : "")
                }
                title={
                  notifications.error
                    ? "Source status unavailable"
                    : "Live connection to the BOM forecast source"
                }
              >
                <span
                  className={
                    "status-dot" + (notifications.error ? " down" : notifications.data && notifications.data.unread > 0 ? " warn" : "")
                  }
                />
                {notifications.error
                  ? "Source status unavailable"
                  : notifications.data && notifications.data.unread > 0
                    ? `${notifications.data.unread} unread notices`
                    : "BOM source healthy"}
              </div>
              <BellMenu
                notifications={notifications.data}
                onNavigate={(p) => go(p)}
              />
              <button
                className="icon-btn"
                title={theme === "light" ? "Switch to dark / aurora theme" : "Switch to light theme"}
                onClick={(e) => toggleTheme(e.currentTarget)}
              >
                <Icon name={theme === "light" ? "sun" : "moon"} />
              </button>
            </div>
          </header>

          <Outlet context={{ notifications, refreshNotifications: notifications.reload }} />
        </main>
      </div>

      <nav className="mobile-bottom" aria-label="Primary">
        <NavLink to="/overview">
          <Icon name="grid" />
          Overview
        </NavLink>
        <NavLink to="/forecast">
          <Icon name="cloud-sun" />
          Forecast
        </NavLink>
        <NavLink to="/intelligence">
          <Icon name="chart" />
          Insights
        </NavLink>
        <NavLink to="/locations">
          <Icon name="heart" />
          Saved
        </NavLink>
        <NavLink to="/alerts">
          <Icon name="bell" />
          Alerts
        </NavLink>
        {isAdmin ? (
          <NavLink to="/admin">
            <Icon name="settings" />
            Admin
          </NavLink>
        ) : null}
      </nav>
    </div>
  );
}

/** Splash screen shown while the very first paint boots. */
export function Splash() {
  return (
    <section className="splash" aria-label="Application splash screen">
      <div className="splash-grid" />
      <div className="splash-glow" />
      <div className="splash-content">
        <div className="brand-mark">
          <Icon name="cloud-sun" />
        </div>
        <div className="splash-kicker">Australian Weather Intelligence</div>
        <div className="splash-title">
          ATMOS<span style={{ color: "#72f2da" }}>//</span>BOM
        </div>
        <div className="splash-sub">
          Live forecast intelligence · historical insights · responsive by design
        </div>
        <div className="boot-line">
          <span />
        </div>
        <div className="boot-status">Synchronising interface / weather layers</div>
      </div>
    </section>
  );
}

/** The routed, authenticated shell (providers included). */
export default function Layout() {
  return (
    <ToastProvider>
      <ModalProvider>
        <Shell />
      </ModalProvider>
    </ToastProvider>
  );
}
