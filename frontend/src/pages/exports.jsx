import {
  api,
  formatDayShort,
  saveBlob,
  stampLabel,
  useAsync,
  useLocationContext,
  useToast,
} from "../api/client.jsx";
import { AsyncPanel, Icon, Panel, Tag } from "../components/UI.jsx";

export default function ExportsPage() {
  const { location, date } = useLocationContext();
  const toast = useToast();
  const history = useAsync(() => api.history({ limit: 12 }), []);

  const run = async (label, fn, fallbackName) => {
    try {
      const res = await fn();
      const name = await saveBlob(res, fallbackName);
      toast(`${label} ready`, name, "⤓");
      history.reload();
    } catch (e) {
      toast(`${label} failed`, e.message, "!");
    }
  };

  const params = {
    lat: location.latitude,
    lon: location.longitude,
    name: location.name,
    timezone: location.timezone,
    date,
  };

  return (
    <section className="page active">
      <div className="page-title">Forecast exports</div>
      <p className="page-sub">
        Export {location.name} for {formatDayShort(date)} without leaving the workspace. Admins can export the
        full user directory from User management.
      </p>

      <div className="grid g3">
        <Panel title="CSV export" kicker="Machine-readable daily forecast with temperature, humidity and rainfall fields">
          <button
            className="primary-btn"
            style={{ marginTop: 14 }}
            onClick={() => run("CSV export", () => api.exportForecast({ ...params, format: "csv" }), "atmos-forecast.csv")}
          >
            <Icon name="download" size={14} /> Export CSV
          </button>
        </Panel>

        <Panel title="JSON export" kicker="Structured object ready for downstream processing or reporting">
          <button
            className="ghost-btn"
            style={{ marginTop: 14 }}
            onClick={() => run("JSON export", () => api.exportForecast({ ...params, format: "json" }), "atmos-forecast.json")}
          >
            <Icon name="download" size={14} /> Export JSON
          </button>
        </Panel>

        <Panel title="Report snapshot" kicker="Complete snapshot: forecast, current conditions, activity rating and metrics">
          <button
            className="ghost-btn"
            style={{ marginTop: 14 }}
            onClick={() => run("Report snapshot", () => api.exportReport(params), "atmos-report.json")}
          >
            <Icon name="download" size={14} /> Prepare snapshot
          </button>
        </Panel>
      </div>

      <Panel
        className="section-gap"
        title="Stored-history export"
        kicker="Every retrieval persisted for your account, as CSV or JSON"
        right={<Tag variant="success">RETENTION {history.data?.retention_days ?? "—"} DAYS</Tag>}
      >
        <div className="toolbar">
          <button
            className="ghost-btn"
            onClick={() => run("History CSV", () => api.exportHistory({ format: "csv" }), "atmos-history.csv")}
          >
            <Icon name="download" size={14} /> History CSV
          </button>
          <button
            className="ghost-btn"
            onClick={() => run("History JSON", () => api.exportHistory({ format: "json" }), "atmos-history.json")}
          >
            <Icon name="download" size={14} /> History JSON
          </button>
        </div>
        <p className="panel-kicker" style={{ marginTop: 10, lineHeight: 1.7 }}>
          The backend appends each retrieval to SQLite and to the structured CSV/JSON files under{" "}
          <strong>backend/data</strong>, so the same records can be exported at any time.
        </p>
      </Panel>

      <Panel className="section-gap" title="Export audit" kicker="Your most recent persisted retrievals">
        <AsyncPanel
          loading={history.loading}
          error={history.error}
          onRetry={history.reload}
          empty={!history.loading && !history.error && (history.data?.entries || []).length === 0}
          emptyLabel="No retrievals have been recorded for your account yet."
        >
          {(history.data?.entries || []).map((e) => (
            <div className="log" key={`${e.kind}-${e.id}`}>
              <div className="log-time">{stampLabel(e.ts)}</div>
              <div className="log-copy">
                <strong>
                  {e.kind === "query" ? "Location search recorded" : "Forecast retrieval persisted"}
                </strong>
                <p>
                  {(e.location || e.query || "—") + " · " + (e.source || "unknown source") + " · " + (e.result || "retrieved")}
                </p>
              </div>
            </div>
          ))}
        </AsyncPanel>
      </Panel>
    </section>
  );
}
