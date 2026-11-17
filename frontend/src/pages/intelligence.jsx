import { useEffect, useMemo, useState } from "react";
import {
  api,
  formatDayLong,
  useAsync,
  useLocationContext,
} from "../api/client.jsx";
import { AsyncPanel, Icon, Panel, Tag } from "../components/UI.jsx";
import { ActivityDetail, ActivityGrid } from "../components/Activity.jsx";

export default function IntelligencePage() {
  const { location, date, setDate } = useLocationContext();
  const [selected, setSelected] = useState(null);

  const intelligence = useAsync(
    () =>
      api.intelligence({
        lat: location.latitude,
        lon: location.longitude,
        name: location.name,
        timezone: location.timezone,
        date,
      }),
    [location.latitude, location.longitude, date]
  );

  const activities = intelligence.data?.activities || [];

  // Rank the grid best-first, but keep every activity visible.
  const ranked = useMemo(
    () => [...activities].sort((a, b) => (b.score ?? -1) - (a.score ?? -1)),
    [activities]
  );

  useEffect(() => {
    if (selected && activities.some((a) => a.activity_key === selected)) return;
    setSelected(ranked[0]?.activity_key || null);
  }, [ranked, activities, selected]);

  const chosen = activities.find((a) => a.activity_key === selected) || ranked[0] || null;
  const best = ranked[0] || null;

  return (
    <section className="page active">
      <div className="page-title">Weather intelligence</div>
      <p className="page-sub">
        Deterministic outdoor suitability for {location.name} on {formatDayLong(date)}. Scores, factors and
        advice are computed by the Go rubric directly from the live forecast values.
      </p>

      <Panel>
        <div className="toolbar">
          <div>
            <div className="panel-title">Selected date</div>
            <input
              className="input"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              style={{ maxWidth: 220, marginTop: 8 }}
              aria-label="Activity assessment date"
            />
          </div>
          <div style={{ marginLeft: "auto", display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
            {best ? (
              <Tag variant={best.score >= 65 ? "success" : best.score >= 50 ? "warn" : "danger"}>
                BEST TODAY · {best.emoji} {best.activity.toUpperCase()} {best.score}
              </Tag>
            ) : null}
            <Tag>{activities.length} activities assessed</Tag>
          </div>
        </div>
      </Panel>

      <Panel className="section-gap" title="All activities" kicker="Click any activity for the full explanation">
        <AsyncPanel
          loading={intelligence.loading}
          error={intelligence.error}
          onRetry={intelligence.reload}
          empty={!intelligence.loading && !intelligence.error && activities.length === 0}
          emptyLabel="No activity scores were returned for this location and date."
        >
          <ActivityGrid activities={ranked} selected={selected} onSelect={setSelected} />
          <div className="chip-row" style={{ marginTop: 14 }}>
            <Tag variant="success">GOOD · 65+</Tag>
            <Tag variant="warn">FAIR · 50–64</Tag>
            <Tag variant="danger">POOR · below 50</Tag>
            <span style={{ fontSize: 10, color: "var(--muted)", display: "inline-flex", alignItems: "center", gap: 6 }}>
              <Icon name="shield" size={14} /> Thresholds follow documented WHO UV, Beaufort wind and heat-stress bands.
            </span>
          </div>
        </AsyncPanel>
      </Panel>

      {chosen ? (
        <div className="section-gap">
          <ActivityDetail activity={chosen} />
        </div>
      ) : null}

      <div className="footer-note">
        The scoring rubric lives in comments at the top of backend/intelligence.go so it can be audited against
        the forecast values shown on this page.
      </div>
    </section>
  );
}
