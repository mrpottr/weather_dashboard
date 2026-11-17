import { useEffect, useState } from "react";
import { useLocation as useRouteLocation, useOutletContext } from "react-router-dom";
import { api, relativeTime, useAsync, useToast } from "../api/client.jsx";
import { AsyncPanel, Icon, Panel, Tag } from "../components/UI.jsx";

const SEV_VARIANT = { extreme: "danger", high: "warn", medium: "accent" };

export default function AlertsPage() {
  const route = useRouteLocation();
  const { refreshNotifications } = useOutletContext() || {};
  const toast = useToast();

  const [tab, setTab] = useState(route.search.includes("tab=system") ? "system" : "weather");
  const [openAlert, setOpenAlert] = useState(null);

  useEffect(() => {
    setTab(route.search.includes("tab=system") ? "system" : "weather");
  }, [route.search]);

  const alerts = useAsync(() => api.alerts({}), []);
  const notices = useAsync(() => api.announcements(), []);

  const goTab = (next) => {
    setTab(next);
    window.history.replaceState({}, "", next === "system" ? "/alerts?tab=system" : "/alerts");
  };

  const dismissAlert = async (a) => {
    try {
      await api.dismissAlert(a.id);
      alerts.reload();
      if (refreshNotifications) refreshNotifications();
      toast("Alert dismissed", a.title, "✓");
    } catch (e) {
      toast("Could not dismiss alert", e.message, "!");
    }
  };

  const dismissNotice = async (n) => {
    try {
      await api.dismissAnnouncement(n.id);
      notices.reload();
      if (refreshNotifications) refreshNotifications();
      toast("Notice dismissed", n.title, "✓");
    } catch (e) {
      toast("Could not dismiss notice", e.message, "!");
    }
  };

  const weather = alerts.data?.alerts || [];
  const system = notices.data?.announcements || [];
  const activeCount = system.filter((n) => n.active && !n.dismissed).length;

  return (
    <section className="page active">
      <div className="page-title">Weather alerts</div>
      <p className="page-sub">
        Threshold alerts for your saved and current locations, plus system notices published by administration.
      </p>

      <Panel>
        <div className="toolbar">
          <div className="chip-row">
            <button className={tab === "weather" ? "active" : ""} onClick={() => goTab("weather")}>
              Weather alerts ({weather.length})
            </button>
            <button className={tab === "system" ? "active" : ""} onClick={() => goTab("system")}>
              System notices ({activeCount})
            </button>
          </div>
          <div style={{ marginLeft: "auto", display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
            {alerts.data?.source ? <Tag>source · {alerts.data.source}</Tag> : null}
            {alerts.data?.generated_at ? (
              <Tag>generated {relativeTime(alerts.data.generated_at)}</Tag>
            ) : null}
          </div>
        </div>
      </Panel>

      {tab === "weather" ? (
        <div className="section-gap">
          <AsyncPanel
            loading={alerts.loading}
            error={alerts.error}
            onRetry={alerts.reload}
            empty={!alerts.loading && !alerts.error && weather.length === 0}
            emptyLabel="No threshold alerts are active for your locations right now."
          >
            {weather.map((a) => (
              <div className={"alert-card sev-" + (a.severity || "medium")} key={a.id}>
                <div className="alert-card-head">
                  <div>
                    <h4>
                      <Icon name="bell" size={14} /> {a.title}
                    </h4>
                    <div className="a-sub">
                      {a.location} · {a.type} · {a.window}
                    </div>
                  </div>
                  <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                    <Tag variant={SEV_VARIANT[a.severity] || ""}>{(a.severity || "medium").toUpperCase()}</Tag>
                    <button className="tiny-btn" onClick={() => setOpenAlert(openAlert === a.id ? null : a.id)}>
                      {openAlert === a.id ? "Hide detail" : "View more"}
                    </button>
                    <button className="danger-btn" style={{ padding: "8px 10px", fontSize: 11 }} onClick={() => dismissAlert(a)}>
                      Dismiss
                    </button>
                  </div>
                </div>

                <p style={{ fontSize: 11, color: "var(--muted)", lineHeight: 1.65, margin: "10px 0 0" }}>
                  {a.message}
                </p>

                {openAlert === a.id ? (
                  <div className="alert-detail">
                    <div className="d-row">
                      <label>What triggered it</label>
                      <span>{a.trigger}</span>
                    </div>
                    <div className="d-row">
                      <label>Actual value</label>
                      <span>{a.actual_value}</span>
                    </div>
                    <div className="d-row">
                      <label>Threshold value</label>
                      <span>{a.threshold_value}</span>
                    </div>
                    <div className="d-row">
                      <label>Affected location</label>
                      <span>{a.location}</span>
                    </div>
                    <div className="d-row">
                      <label>Time window</label>
                      <span>{a.window}</span>
                    </div>
                    <div className="d-row">
                      <label>Severity</label>
                      <span>{(a.severity || "medium").toUpperCase()}</span>
                    </div>
                    <div className="d-row">
                      <label>Recommended precautions</label>
                      <span>{a.precautions}</span>
                    </div>
                    <div className="d-row">
                      <label>Raised at</label>
                      <span>{a.created_at ? new Date(a.created_at).toLocaleString("en-AU") : "—"}</span>
                    </div>
                  </div>
                ) : null}
              </div>
            ))}
          </AsyncPanel>
        </div>
      ) : (
        <div className="section-gap">
          <AsyncPanel
            loading={notices.loading}
            error={notices.error}
            onRetry={notices.reload}
            empty={!notices.loading && !notices.error && system.length === 0}
            emptyLabel="No system notices have been published."
          >
            {system.map((n) => (
              <div className="sys-notice" key={n.id}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
                  <h4>{n.title}</h4>
                  <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                    <Tag variant={n.active ? "success" : ""}>{(n.state || (n.active ? "active" : "expired")).toUpperCase()}</Tag>
                    <Tag>{n.dismissed ? "DISMISSED" : "UNREAD"}</Tag>
                    {n.active && !n.dismissed ? (
                      <button className="tiny-btn" onClick={() => dismissNotice(n)}>
                        Dismiss
                      </button>
                    ) : null}
                  </div>
                </div>
                <p>{n.message}</p>
                <div className="meta">
                  <span>
                    <Icon name="user" size={12} /> {n.author || "Administrator"}
                  </span>
                  <span>Published {n.created_at ? new Date(n.created_at).toLocaleString("en-AU") : "—"}</span>
                  <span>{n.expires_at ? `Expires ${new Date(n.expires_at).toLocaleString("en-AU")}` : "No expiry set"}</span>
                </div>
              </div>
            ))}
          </AsyncPanel>
        </div>
      )}
    </section>
  );
}
