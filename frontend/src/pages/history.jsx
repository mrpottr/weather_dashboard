import { useEffect, useRef, useState } from "react";
import {
  api,
  formatDayShort,
  saveBlob,
  stampLabel,
  useAsync,
  useToast,
} from "../api/client.jsx";
import { AsyncPanel, Icon, Panel, Tag } from "../components/UI.jsx";

export default function HistoryPage() {
  const toast = useToast();
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [limit, setLimit] = useState(50);
  const timer = useRef(0);

  useEffect(() => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setDebounced(query.trim()), 300);
    return () => window.clearTimeout(timer.current);
  }, [query]);

  const history = useAsync(() => api.history({ q: debounced || undefined, limit }), [debounced, limit]);

  const entries = history.data?.entries || [];

  const onExport = async (format) => {
    try {
      const res = await api.exportHistory({ format });
      const name = await saveBlob(res, `atmos-history.${format === "json" ? "json" : "csv"}`);
      toast("History export ready", name, "⤓");
    } catch (e) {
      toast("Export failed", e.message, "!");
    }
  };

  return (
    <section className="page active">
      <div className="page-title">Forecast history</div>
      <p className="page-sub">
        Previously retrieved forecasts and location queries, persisted in SQLite and mirrored to CSV/JSON files.
      </p>

      <div className="panel">
        <div className="toolbar">
          <div className="searchbox">
            <Icon name="search" />
            <input
              className="input"
              placeholder="Filter history by location, query or result"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Filter history"
            />
          </div>
          <select
            className="select"
            style={{ width: "auto" }}
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            aria-label="Rows to show"
          >
            <option value={25}>25 rows</option>
            <option value={50}>50 rows</option>
            <option value={100}>100 rows</option>
            <option value={250}>250 rows</option>
          </select>
          <button className="ghost-btn" onClick={() => onExport("csv")}>
            <Icon name="download" size={14} />
            Export CSV
          </button>
          <button className="ghost-btn" onClick={() => onExport("json")}>
            Export JSON
          </button>
        </div>
      </div>

      <Panel
        className="section-gap"
        title="Query timeline"
        kicker="Structured record of every retrieval and search"
        right={
          <Tag variant="success">
            RETENTION {history.data?.retention_days != null ? `${history.data.retention_days} DAYS` : "…"}
          </Tag>
        }
      >
        <AsyncPanel
          loading={history.loading}
          error={history.error}
          onRetry={history.reload}
          empty={!history.loading && !history.error && entries.length === 0}
          emptyLabel={
            debounced
              ? `No history records match “${debounced}”.`
              : "No history yet — retrieve a forecast or search a location and it will appear here."
          }
        >
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Type</th>
                  <th>Location / query</th>
                  <th>Date</th>
                  <th>Result</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => {
                  const cached = (e.result || "").toLowerCase() === "cached";
                  return (
                    <tr key={`${e.kind}-${e.id}`}>
                      <td>{stampLabel(e.ts)}</td>
                      <td>
                        <Tag variant={e.kind === "query" ? "accent" : ""}>
                          {e.kind === "query" ? "SEARCH" : "FORECAST"}
                        </Tag>
                      </td>
                      <td>{e.location || e.query || "—"}</td>
                      <td>{e.date ? formatDayShort(e.date) : "—"}</td>
                      <td>
                        <Tag variant={cached ? "warn" : "success"}>
                          {(e.result || "retrieved").toUpperCase()}
                        </Tag>
                      </td>
                      <td>{e.source || "—"}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="panel-kicker" style={{ marginTop: 12, lineHeight: 1.7 }}>
            Showing {entries.length} record{entries.length === 1 ? "" : "s"}. Records older than the configured
            retention window are pruned automatically by the backend.
          </p>
        </AsyncPanel>
      </Panel>
    </section>
  );
}
