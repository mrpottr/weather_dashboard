import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  api,
  relativeTime,
  saveBlob,
  todayISO,
  useAsync,
  useAuth,
  useLocationContext,
  useToast,
} from "../api/client.jsx";
import {
  AsyncPanel,
  Field,
  HealthBar,
  Icon,
  MetricCard,
  Panel,
  Tag,
  Toggle,
  useModal,
} from "../components/UI.jsx";

const WIND_LEVELS = ["strong", "gale", "storm"];

export default function AdminPage() {
  const { loginAs } = useAuth();
  const { location, date } = useLocationContext();
  const toast = useToast();
  const navigate = useNavigate();
  const { openModal, closeModal } = useModal();

  const metrics = useAsync(() => api.adminMetrics(), []);
  const source = useAsync(() => api.adminSourceCheck(), []);
  const thresholds = useAsync(() => api.adminThresholds(), []);
  const logs = useAsync(() => api.adminLogs({ limit: 8 }), []);
  const announcements = useAsync(() => api.adminAnnouncements(), []);

  const [draft, setDraft] = useState(null);
  const [savingThresholds, setSavingThresholds] = useState(false);
  const [refreshResult, setRefreshResult] = useState(null);
  const [refreshing, setRefreshing] = useState(false);

  const current = draft || thresholds.data || null;
  const setField = (k, v) => setDraft((d) => ({ ...(d || thresholds.data), [k]: v }));

  /* ---------------------------------------------------------- actions */

  const broadcast = () => {
    openModal("Broadcast a system notice", <BroadcastForm onDone={() => {
      closeModal();
      announcements.reload();
      logs.reload();
      toast("Announcement published", "Every signed-in user can now see it in System notices.", "📣");
    }} onCancel={closeModal} />);
  };

  const verifySource = async () => {
    source.reload();
    openModal(
      "Source verification",
      <div>
        {source.loading ? <div className="loader" /> : null}
        {source.error ? (
          <div className="empty">Verification failed: {source.error.message}</div>
        ) : source.data ? (
          <div className="alert-detail" style={{ borderTop: 0, paddingTop: 0 }}>
            <div className="d-row">
              <label>Endpoint</label>
              <span style={{ wordBreak: "break-all" }}>{source.data.endpoint}</span>
            </div>
            <div className="d-row">
              <label>Primary BOM feed</label>
              <span>
                {source.data.primary_ok ? "responded with usable values" : "no usable values"} —{" "}
                {source.data.primary_note}
              </span>
            </div>
            <div className="d-row">
              <label>Fallback endpoint</label>
              <span>
                {source.data.fallback_ok ? "responded with usable values" : "no usable values"} —{" "}
                {source.data.fallback_note}
              </span>
            </div>
            <div className="d-row">
              <label>Currently serving</label>
              <span>
                {source.data.serving || "unknown"} — {source.data.source_note}
              </span>
            </div>
            <div className="d-row">
              <label>Measured latency</label>
              <span>{source.data.latency_ms} ms</span>
            </div>
            <div className="d-row">
              <label>Checked at</label>
              <span>{new Date(source.data.checked_at).toLocaleString("en-AU")}</span>
            </div>
            <div className="d-row">
              <label>Summary</label>
              <span>{source.data.message}</span>
            </div>
          </div>
        ) : null}
      </div>
    );
  };

  const livePreview = async () => {
    openModal(
      "Live data preview",
      <LivePreview location={location} date={date} />,
      { wide: true }
    );
  };

  const viewLogs = (full) => {
    openModal(
      "System logs",
      <LogViewer limit={full ? 200 : 40} />,
      { wide: true }
    );
  };

  const manualRefresh = async () => {
    setRefreshing(true);
    setRefreshResult(null);
    try {
      const res = await api.adminRefresh();
      setRefreshResult(res);
      metrics.reload();
      logs.reload();
      toast(
        "BOM refresh complete",
        `${res.concurrent_ok}/${res.locations.length} locations retrieved concurrently (${res.concurrent_ms} ms) versus ${res.sequential_ms} ms sequentially.`,
        "↻"
      );
    } catch (e) {
      toast("Manual refresh failed", e.message, "!");
    } finally {
      setRefreshing(false);
    }
  };

  const impersonate = async () => {
    try {
      const user = await loginAs();
      toast("Standard-user session opened", `${user.first_name} ${user.last_name} · ${user.email}`, "⇄");
      navigate("/overview");
    } catch (e) {
      toast("Could not open standard-user session", e.message, "!");
    }
  };

  const saveThresholds = async () => {
    if (!current) return;
    setSavingThresholds(true);
    try {
      const saved = await api.adminPutThresholds({
        high_temp_c: Number(current.high_temp_c),
        low_temp_c: Number(current.low_temp_c),
        rain_pct: Number(current.rain_pct),
        wind_level: current.wind_level,
        uv_index: Number(current.uv_index),
        push_enabled: !!current.push_enabled,
        retention_days: Number(current.retention_days),
        keep_cache: !!current.keep_cache,
        retain_history: !!current.retain_history,
      });
      thresholds.setData(saved);
      setDraft(null);
      toast("Thresholds saved", "The alerts engine uses these values immediately.", "✓");
      logs.reload();
    } catch (e) {
      toast("Could not save thresholds", e.message, "!");
    } finally {
      setSavingThresholds(false);
    }
  };

  const exportUsers = async () => {
    try {
      const res = await api.adminUsersExport();
      const name = await saveBlob(res, "atmos-users.csv");
      toast("User export ready", name, "⤓");
    } catch (e) {
      toast("User export failed", e.message, "!");
    }
  };

  const m = metrics.data;
  const src = source.data;

  return (
    <section className="page active">
      <div className="admin-hero">
        <div className="preview-banner">Administrator control layer</div>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 15, alignItems: "flex-end", flexWrap: "wrap" }}>
          <div>
            <div className="page-title" style={{ margin: 0 }}>
              Control room
            </div>
            <p className="page-sub" style={{ margin: "6px 0 0" }}>
              Operate the source connection, announcements, thresholds, users, cache and observability layer.
            </p>
          </div>
          <div className="toolbar">
            <button className="ghost-btn" onClick={impersonate}>
              Log in as standard user
            </button>
            <button className="primary-btn" style={{ width: "auto" }} onClick={manualRefresh} disabled={refreshing}>
              <Icon name="refresh" size={14} />
              {refreshing ? "Refreshing…" : "Refresh from BOM"}
            </button>
          </div>
        </div>
        {refreshing ? (
          <p className="panel-kicker" style={{ marginTop: 12 }}>
            Running one concurrent pass and one sequential pass across the major cities to measure the real
            goroutine benefit. This can take up to about 30 seconds.
          </p>
        ) : null}
      </div>

      <div className="grid g4 section-gap">
        <MetricCard
          label="Active users"
          value={m ? m.active_users : "—"}
          meta={m ? `${m.total_users} accounts in total` : "loading…"}
        />
        <MetricCard
          label="Average BOM latency"
          value={m ? `${(m.avg_latency_ms / 1000).toFixed(1)}s` : "—"}
          meta={m ? `across ${m.locations_fetched || 0} measured locations` : "loading…"}
        />
        <MetricCard
          label="Cache coverage"
          value={m ? `${m.cache_coverage_pct.toFixed(1)}%` : "—"}
          meta={m ? `${m.cache_entries} cached payloads stored` : "loading…"}
        />
        <MetricCard
          label="System alerts"
          value={m ? String(m.alerts_active).padStart(2, "0") : "—"}
          meta={m ? `${m.history_rows} history rows persisted` : "loading…"}
        />
      </div>

      {refreshResult ? (
        <Panel
          className="section-gap"
          title="Concurrent versus sequential retrieval"
          kicker="Report novelty: the same locations fetched in parallel and in series"
          right={<Tag variant="success">×{refreshResult.speedup_factor} faster</Tag>}
        >
          <div className="latency-compare">
            <MetricCard
              label="Concurrent (goroutines)"
              value={`${refreshResult.concurrent_ms} ms`}
              meta={`${refreshResult.concurrent_ok}/${refreshResult.locations.length} locations succeeded`}
            />
            <MetricCard
              label="Sequential (one by one)"
              value={`${refreshResult.sequential_ms} ms`}
              meta={`${refreshResult.sequential_ok}/${refreshResult.locations.length} locations succeeded`}
            />
          </div>
          <div className="table-wrap" style={{ marginTop: 12 }}>
            <table>
              <thead>
                <tr>
                  <th>Location</th>
                  <th>Latency</th>
                  <th>Result</th>
                  <th>Temperature</th>
                  <th>Condition</th>
                </tr>
              </thead>
              <tbody>
                {refreshResult.locations.map((l) => (
                  <tr key={l.name}>
                    <td>{l.name}</td>
                    <td>{l.latency_ms} ms</td>
                    <td>
                      <Tag variant={l.ok ? "success" : "danger"}>{l.ok ? "OK" : "FAILED"}</Tag>
                    </td>
                    <td>{l.temp_c != null ? `${Math.round(l.temp_c)}°C` : "—"}</td>
                    <td>{l.condition || l.error || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="panel-kicker" style={{ marginTop: 10 }}>
            Measured {relativeTime(refreshResult.checked_at)} with the cache bypassed so the comparison is honest.
          </p>
        </Panel>
      ) : null}

      <div className="admin-grid section-gap">
        <Panel title="Administrative actions" kicker="Mapped directly to the admin functional requirements">
          <div className="admin-actions">
            <button className="action-card" onClick={broadcast}>
              <div className="action-icon">
                <Icon name="broadcast" />
              </div>
              <strong>Broadcast notice</strong>
              <span>Publish a system-wide announcement.</span>
            </button>
            <button className="action-card" onClick={verifySource}>
              <div className="action-icon">
                <Icon name="shield" />
              </div>
              <strong>Verify source</strong>
              <span>Check BOM / endpoint connectivity.</span>
            </button>
            <button className="action-card" onClick={livePreview}>
              <div className="action-icon">
                <Icon name="cloud-sun" />
              </div>
              <strong>Live preview</strong>
              <span>Inspect current weather as admin.</span>
            </button>
            <button className="action-card" onClick={() => viewLogs(false)}>
              <div className="action-icon">
                <Icon name="history" />
              </div>
              <strong>System logs</strong>
              <span>Review recent runtime activity.</span>
            </button>
            <button className="action-card" onClick={() => navigate("/admin/users")}>
              <div className="action-icon">
                <Icon name="user" />
              </div>
              <strong>Manage users</strong>
              <span>Search accounts and change access.</span>
            </button>
            <button className="action-card" onClick={exportUsers}>
              <div className="action-icon">
                <Icon name="download" />
              </div>
              <strong>Export users</strong>
              <span>Download the account directory as CSV.</span>
            </button>
          </div>
        </Panel>

        <Panel title="Source health" kicker="Live connection status (verified against the real endpoints)">
          <AsyncPanel loading={source.loading} error={source.error} onRetry={source.reload}>
            <div className="source-health">
              <div className="signal">
                <span className={"signal-led" + (src?.healthy ? "" : " down")} />
                <div>
                  <strong style={{ fontSize: 12 }}>{src?.healthy ? "Healthy" : "Degraded"}</strong>
                  <div style={{ fontSize: 9, color: "var(--muted)", marginTop: 3 }}>
                    {src?.source_note || "checking the upstream feeds…"}
                  </div>
                </div>
              </div>
              <Tag variant={src?.healthy ? "success" : "danger"}>{src ? `${src.latency_ms} ms` : "—"}</Tag>
            </div>

            <div style={{ marginTop: 14, display: "grid", gap: 6 }}>
              <div className="chip-row">
                <Tag variant={src?.primary_ok ? "success" : "danger"}>
                  BOM ACCESS-G feed · {src?.primary_ok ? "DATA" : "NO DATA"}
                </Tag>
                <Tag variant={src?.fallback_ok ? "success" : "danger"}>
                  Open-Meteo fallback · {src?.fallback_ok ? "DATA" : "NO DATA"}
                </Tag>
              </div>
              <p className="panel-kicker" style={{ lineHeight: 1.7 }}>
                Currently serving: <strong>{src?.serving || "unknown"}</strong>. {src?.primary_note} {src?.fallback_note}
              </p>
            </div>

            <HealthBar label="API success rate" pct={m?.api_success_pct ?? 0} value={m ? `${m.api_success_pct.toFixed(0)}%` : "—"} />
            <HealthBar label="Cache coverage" pct={m?.cache_coverage_pct ?? 0} value={m ? `${m.cache_coverage_pct.toFixed(0)}%` : "—"} />
            <HealthBar
              label="Concurrent speed-up"
              pct={m && m.speedup_factor ? Math.min(100, m.speedup_factor * 25) : 0}
              value={m && m.speedup_factor ? `×${m.speedup_factor.toFixed(1)}` : "—"}
            />
          </AsyncPanel>
        </Panel>
      </div>

      <div className="grid g2 section-gap">
        <Panel
          title="Alert thresholds"
          kicker="Temperature and weather condition policies used by the alerts engine"
          right={
            <button className="tiny-btn" onClick={saveThresholds} disabled={savingThresholds || !current}>
              {savingThresholds ? "Saving…" : "Save"}
            </button>
          }
        >
          <AsyncPanel loading={thresholds.loading} error={thresholds.error} onRetry={thresholds.reload}>
            {current ? (
              <>
                <div className="threshold-grid">
                  <div className="threshold">
                    <label htmlFor="thHigh">High temperature °C</label>
                    <input
                      id="thHigh"
                      type="number"
                      value={current.high_temp_c}
                      onChange={(e) => setField("high_temp_c", e.target.value)}
                    />
                  </div>
                  <div className="threshold">
                    <label htmlFor="thLow">Low temperature °C</label>
                    <input
                      id="thLow"
                      type="number"
                      value={current.low_temp_c}
                      onChange={(e) => setField("low_temp_c", e.target.value)}
                    />
                  </div>
                  <div className="threshold">
                    <label htmlFor="thRain">Rain threshold %</label>
                    <input
                      id="thRain"
                      type="number"
                      value={current.rain_pct}
                      onChange={(e) => setField("rain_pct", e.target.value)}
                    />
                  </div>
                  <div className="threshold">
                    <label htmlFor="thWind">Wind condition</label>
                    <select id="thWind" value={current.wind_level} onChange={(e) => setField("wind_level", e.target.value)}>
                      {WIND_LEVELS.map((w) => (
                        <option key={w} value={w}>
                          {w === "strong" ? "Strong wind (≥40 km/h)" : w === "gale" ? "Gale (≥62 km/h)" : "Storm (≥89 km/h)"}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="threshold">
                    <label htmlFor="thUv">UV index trigger</label>
                    <input
                      id="thUv"
                      type="number"
                      step="0.5"
                      value={current.uv_index}
                      onChange={(e) => setField("uv_index", e.target.value)}
                    />
                  </div>
                  <div className="threshold">
                    <label htmlFor="thRetention">Retention (days)</label>
                    <input
                      id="thRetention"
                      type="number"
                      value={current.retention_days}
                      onChange={(e) => setField("retention_days", e.target.value)}
                    />
                  </div>
                </div>
                <Toggle
                  label="Push threshold alerts"
                  hint="Include the threshold alerts in the alert centre and bell badge."
                  checked={current.push_enabled}
                  onChange={(v) => setField("push_enabled", v)}
                />
              </>
            ) : (
              <div className="empty">Thresholds could not be loaded.</div>
            )}
          </AsyncPanel>
        </Panel>

        <Panel title="Retention & resilience" kicker="Operational settings for historical data and outages" right={<Tag>NFR READY</Tag>}>
          <AsyncPanel loading={thresholds.loading} error={thresholds.error} onRetry={thresholds.reload}>
            {current ? (
              <>
                <Toggle
                  label="Keep cached forecast"
                  hint="Continue showing the latest stored data, flagged as cached, when the upstream source is unavailable."
                  checked={current.keep_cache}
                  onChange={(v) => setField("keep_cache", v)}
                />
                <Toggle
                  label="Retain query history"
                  hint="Persist every retrieval in SQLite and the CSV/JSON history files."
                  checked={current.retain_history}
                  onChange={(v) => setField("retain_history", v)}
                />
                <div className="latency-compare">
                  <MetricCard label="Concurrent" value={m ? `${m.concurrent_ms} ms` : "—"} meta="goroutines + channels" />
                  <MetricCard label="Sequential" value={m ? `${m.sequential_ms} ms` : "—"} meta="one location at a time" />
                </div>
                <div style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 15, flexWrap: "wrap" }}>
                  <Tag variant="success">RETENTION {current.retention_days} DAYS</Tag>
                  <span style={{ fontSize: 9, color: "var(--muted)" }}>
                    Pruning runs on every retrieval, removing rows older than the retention window.
                  </span>
                </div>
                <button className="ghost-btn" style={{ marginTop: 14 }} onClick={saveThresholds} disabled={savingThresholds}>
                  {savingThresholds ? "Saving…" : "Save retention settings"}
                </button>
              </>
            ) : null}
          </AsyncPanel>
        </Panel>
      </div>

      <Panel
        className="section-gap"
        title="Published announcements"
        kicker="Everything broadcast to users, with expiry state"
        right={
          <button className="tiny-btn" onClick={broadcast}>
            Broadcast notice
          </button>
        }
      >
        <AsyncPanel
          loading={announcements.loading}
          error={announcements.error}
          onRetry={announcements.reload}
          empty={!announcements.loading && !announcements.error && (announcements.data?.announcements || []).length === 0}
          emptyLabel="No announcements have been published yet."
        >
          {(announcements.data?.announcements || []).slice(0, 6).map((a) => (
            <div className="sys-notice" key={a.id}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
                <h4>{a.title}</h4>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <Tag variant={a.active ? "success" : ""}>{(a.state || (a.active ? "active" : "expired")).toUpperCase()}</Tag>
                  <Tag>{a.author || "Administrator"}</Tag>
                </div>
              </div>
              <p>{a.message}</p>
              <div className="meta">
                <span>Published {a.created_at ? new Date(a.created_at).toLocaleString("en-AU") : "—"}</span>
                <span>{a.expires_at ? `Expires ${new Date(a.expires_at).toLocaleString("en-AU")}` : "No expiry set"}</span>
              </div>
              <div style={{ display: "flex", gap: 8, marginTop: 10 }}>
                <button
                  className="tiny-btn"
                  onClick={async () => {
                    try {
                      await api.adminUpdateAnnouncement(a.id, { active: !a.active });
                      announcements.reload();
                      toast("Announcement updated", a.title, "✓");
                    } catch (e) {
                      toast("Could not update announcement", e.message, "!");
                    }
                  }}
                >
                  {a.active ? "Deactivate" : "Activate"}
                </button>
                <button
                  className="danger-btn"
                  style={{ padding: "8px 10px", fontSize: 11 }}
                  onClick={async () => {
                    try {
                      await api.adminUpdateAnnouncement(a.id, { delete: true });
                      announcements.reload();
                      toast("Announcement removed", a.title, "✓");
                    } catch (e) {
                      toast("Could not remove announcement", e.message, "!");
                    }
                  }}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </AsyncPanel>
      </Panel>

      <Panel
        className="section-gap"
        title="Runtime logs"
        kicker="Latest admin-observable events"
        right={
          <button className="tiny-btn" onClick={() => viewLogs(true)}>
            View all
          </button>
        }
      >
        <AsyncPanel
          loading={logs.loading}
          error={logs.error}
          onRetry={logs.reload}
          empty={!logs.loading && !logs.error && (logs.data?.logs || []).length === 0}
          emptyLabel="No runtime events recorded yet."
        >
          {(logs.data?.logs || []).map((l) => (
            <div className="log" key={l.id}>
              <div className="log-time">{new Date(l.ts).toLocaleTimeString("en-AU", { hour: "2-digit", minute: "2-digit" })}</div>
              <div className="log-copy">
                <strong>
                  {l.action} · {l.actor} ({l.role})
                </strong>
                <p>{l.detail}</p>
              </div>
            </div>
          ))}
        </AsyncPanel>
      </Panel>
    </section>
  );
}

/* --------------------------------------------------------------- helpers */

/** Broadcast-notice form (title, message, optional expiry). */
function BroadcastForm({ onDone, onCancel }) {
  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [expires, setExpires] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api.adminCreateAnnouncement({
        title: title.trim(),
        message: message.trim(),
        expires_at: expires ? new Date(expires).toISOString() : undefined,
      });
      onDone();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit}>
      <Field label="Headline" htmlFor="bcTitle">
        <input id="bcTitle" className="input" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Scheduled maintenance" required />
      </Field>
      <Field label="Message" htmlFor="bcMessage">
        <textarea
          id="bcMessage"
          className="input"
          rows={4}
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Explain what users need to know."
          required
        />
      </Field>
      <Field label="Expires at (optional)" htmlFor="bcExpires">
        <input id="bcExpires" className="input" type="date" min={todayISO()} value={expires} onChange={(e) => setExpires(e.target.value)} />
      </Field>
      <div className="error-note" role="alert">
        {error}
      </div>
      <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 10 }}>
        <button className="ghost-btn" type="button" onClick={onCancel}>
          Cancel
        </button>
        <button className="primary-btn" style={{ width: "auto" }} type="submit" disabled={busy}>
          {busy ? "Publishing…" : "Publish notice"}
        </button>
      </div>
    </form>
  );
}

/** Admin live preview: the exact payload a standard user receives. */
function LivePreview({ location, date }) {
  const preview = useAsync(
    () =>
      api.adminPreview({
        lat: location.latitude,
        lon: location.longitude,
        name: location.name,
        timezone: location.timezone,
        date,
      }),
    [location.latitude, location.longitude, date]
  );

  return (
    <AsyncPanel loading={preview.loading} error={preview.error} onRetry={preview.reload}>
      {preview.data ? (
        <div>
          <div className="preview-banner">{preview.data.banner}</div>
          <div className="grid g2">
            <MetricCard
              label="Temperature"
              value={preview.data.forecast?.current ? `${Math.round(preview.data.forecast.current.temp_c)}°` : "—"}
              meta={preview.data.forecast?.current?.condition || "no current block"}
            />
            <MetricCard
              label="Source"
              value={preview.data.forecast?.from_cache ? "Cached" : "Live"}
              meta={preview.data.forecast?.source || ""}
            />
            <MetricCard label="Latency" value={`${preview.data.forecast?.latency_ms ?? "—"} ms`} meta="this retrieval" />
            <MetricCard
              label="Forecast days"
              value={preview.data.forecast?.daily?.length ?? "—"}
              meta={`timezone ${preview.data.forecast?.timezone || "—"}`}
            />
          </div>
          <div className="chip-row" style={{ marginTop: 12 }}>
            <Tag>active users {preview.data.metrics?.active_users ?? "—"}</Tag>
            <Tag>cache coverage {preview.data.metrics?.cache_coverage_pct ?? "—"}%</Tag>
            <Tag>average latency {preview.data.metrics?.avg_latency_ms ?? "—"} ms</Tag>
          </div>
          <p className="panel-kicker" style={{ marginTop: 10 }}>
            Previewed {preview.data.previewed_at ? new Date(preview.data.previewed_at).toLocaleString("en-AU") : "just now"}.
          </p>
        </div>
      ) : null}
    </AsyncPanel>
  );
}

/** Searchable runtime-log viewer used by the "System logs" action. */
function LogViewer({ limit }) {
  const [query, setQuery] = useState("");
  const [applied, setApplied] = useState("");
  const logs = useAsync(() => api.adminLogs({ q: applied || undefined, limit }), [applied, limit]);

  return (
    <div>
      <form
        className="toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          setApplied(query.trim());
        }}
      >
        <div className="searchbox">
          <Icon name="search" />
          <input
            className="input"
            placeholder="Filter by action, actor or detail"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            aria-label="Filter system logs"
          />
        </div>
        <button className="ghost-btn" type="submit">
          Filter
        </button>
      </form>
      <div style={{ marginTop: 12 }}>
        <AsyncPanel
          loading={logs.loading}
          error={logs.error}
          onRetry={logs.reload}
          empty={!logs.loading && !logs.error && (logs.data?.logs || []).length === 0}
          emptyLabel="No log entries match that filter."
        >
          {(logs.data?.logs || []).map((l) => (
            <div className="log" key={l.id}>
              <div className="log-time">{new Date(l.ts).toLocaleTimeString("en-AU", { hour: "2-digit", minute: "2-digit" })}</div>
              <div className="log-copy">
                <strong>
                  {l.action} · {l.actor} ({l.role})
                </strong>
                <p>{l.detail}</p>
              </div>
            </div>
          ))}
        </AsyncPanel>
      </div>
    </div>
  );
}
