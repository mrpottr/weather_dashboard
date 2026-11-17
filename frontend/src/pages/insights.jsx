import { useMemo } from "react";
import {
  api,
  formatDayShort,
  formatHourLabel,
  useAsync,
  useClock,
  useLocationContext,
} from "../api/client.jsx";
import { AsyncPanel, HealthBar, MetricCard, Panel, Tag } from "../components/UI.jsx";
import LineChart from "../components/Charts.jsx";

export default function InsightsPage() {
  const { location, date } = useLocationContext();
  const { format: clockFormat } = useClock();

  const forecast = useAsync(
    () =>
      api.forecast({
        lat: location.latitude,
        lon: location.longitude,
        name: location.name,
        timezone: location.timezone,
        date,
      }),
    [location.latitude, location.longitude, date]
  );
  const stats = useAsync(() => api.historyStats(), []);
  const accuracy = useAsync(
    () => api.accuracy({ lat: location.latitude, lon: location.longitude, name: location.name }),
    [location.latitude, location.longitude]
  );
  const history = useAsync(() => api.history({}), []);

  const daily = forecast.data?.daily || [];
  const fc = forecast.data;

  const hourly = useMemo(() => {
    const all = fc?.hourly || [];
    const forDate = all.filter((h) => (h.time || "").startsWith(date));
    return (forDate.length >= 2 ? forDate : all.slice(0, 24)).slice(0, 24);
  }, [fc, date]);

  const metrics = accuracy.data?.metrics || {};
  const metricEntries = Object.entries(metrics);

  const hottest = daily.reduce((a, b) => (!a || b.temp_max_c > a.temp_max_c ? b : a), null);
  const coolest = daily.reduce((a, b) => (!a || b.temp_min_c < a.temp_min_c ? b : a), null);
  const wettest = daily.reduce((a, b) => (!a || b.precip_probability_max > a.precip_probability_max ? b : a), null);

  const rainTrend = stats.data?.rain_trend_pct;

  return (
    <section className="page active">
      <div className="page-title">Weather insights</div>
      <p className="page-sub">
        Historical patterns, trends and forecast-accuracy comparisons for {location.name}.
      </p>

      <div className="grid g3">
        <MetricCard
          label="Mean temperature"
          value={stats.data?.mean_temp_c != null ? `${stats.data.mean_temp_c.toFixed(1)}°` : "—"}
          meta={
            stats.data?.mean_temp_c != null
              ? "stored forecasts (daily min/max midpoints)"
              : "no stored forecast history yet"
          }
        />
        <MetricCard
          label="Rainfall trend"
          value={rainTrend != null ? `${rainTrend > 0 ? "+" : ""}${rainTrend.toFixed(1)}%` : "—"}
          meta={rainTrend != null ? "last 7 stored days vs previous 7" : "needs at least two stored periods"}
        />
        <MetricCard
          label="Forecast accuracy"
          value={
            accuracy.data?.samples
              ? `${accuracy.data.overall.toFixed(1)}%`
              : stats.data?.accuracy != null
                ? `${stats.data.accuracy.toFixed(1)}%`
                : "—"
          }
          meta={
            accuracy.data?.samples
              ? `forecast vs latest observation · ${accuracy.data.samples} samples`
              : "no forecast-vs-observation samples yet"
          }
        />
      </div>

      <div className="grid g2 section-gap">
        <Panel
          title="Temperature history"
          kicker="Daily minimum and maximum across the forecast period"
          right={<Tag>{daily.length} days</Tag>}
        >
          <AsyncPanel
            loading={forecast.loading}
            error={forecast.error}
            onRetry={forecast.reload}
            empty={!forecast.loading && !forecast.error && daily.length < 2}
            emptyLabel="Not enough daily points to draw this chart."
          >
            <div className="chart-wrap">
              <LineChart
                values={daily.map((d) => d.temp_max_c)}
                labels={daily.map((d) => (d.date || "").slice(5))}
                ariaLabel="Daily maximum temperature"
              />
            </div>
            <div className="legend" style={{ marginTop: 8 }}>
              <span>
                <i /> Daily maximum temperature (°C)
              </span>
            </div>
            <div className="chart-wrap">
              <LineChart
                values={daily.map((d) => d.temp_min_c)}
                labels={daily.map((d) => (d.date || "").slice(5))}
                ariaLabel="Daily minimum temperature"
              />
            </div>
            <div className="legend" style={{ marginTop: 8 }}>
              <span>
                <i /> Daily minimum temperature (°C)
              </span>
            </div>
          </AsyncPanel>
        </Panel>

        <Panel
          title="Accuracy monitor"
          kicker="Forecast versus observation, aggregated per metric"
          right={<Tag variant={accuracy.data?.overall >= 90 ? "success" : accuracy.data?.overall >= 75 ? "warn" : "danger"}>
            {accuracy.data?.samples ? `${accuracy.data.samples} samples` : "NO SAMPLES"}
          </Tag>}
        >
          <AsyncPanel
            loading={accuracy.loading}
            error={accuracy.error}
            onRetry={accuracy.reload}
            empty={!accuracy.loading && !accuracy.error && metricEntries.length === 0}
            emptyLabel="Retrieve a forecast for the same location twice on the same day and the backend will start comparing its earlier prediction with the newer observation."
          >
            {metricEntries.map(([metric, pct]) => (
              <HealthBar key={metric} label={`${metric} forecasts`} pct={pct} value={`${pct.toFixed(1)}%`} />
            ))}
            <p className="panel-kicker" style={{ marginTop: 12, lineHeight: 1.7 }}>
              Each sample compares the previously stored forecast hour for this location with the newest
              observation of the same metric, then stores the percentage agreement.
            </p>
            {(accuracy.data?.recent || []).length ? (
              <div className="table-wrap" style={{ marginTop: 12 }}>
                <table>
                  <thead>
                    <tr>
                      <th>Date</th>
                      <th>Metric</th>
                      <th>Forecast</th>
                      <th>Observed</th>
                      <th>Match</th>
                    </tr>
                  </thead>
                  <tbody>
                    {accuracy.data.recent.slice(0, 8).map((s) => (
                      <tr key={s.id}>
                        <td>{s.date}</td>
                        <td>{s.metric}</td>
                        <td>{s.forecast.toFixed(1)}</td>
                        <td>{s.observed.toFixed(1)}</td>
                        <td>
                          <Tag variant={s.percent >= 90 ? "success" : s.percent >= 75 ? "warn" : "danger"}>
                            {s.percent.toFixed(1)}%
                          </Tag>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
          </AsyncPanel>
        </Panel>
      </div>

      <div className="grid g2 section-gap">
        <Panel title="Humidity profile" kicker={`Hourly relative humidity · ${formatDayShort(date)}`} right={<Tag>{hourly.length} hours</Tag>}>
          <AsyncPanel
            loading={forecast.loading}
            error={forecast.error}
            onRetry={forecast.reload}
            empty={!forecast.loading && !forecast.error && hourly.length < 2}
            emptyLabel="Not enough hourly points to draw this chart."
          >
            <div className="chart-wrap">
              <LineChart
                values={hourly.map((h) => h.humidity)}
                labels={hourly.map((h) => formatHourLabel(h.time, clockFormat).slice(0, 5))}
                ariaLabel="Hourly relative humidity"
              />
            </div>
            <div className="legend" style={{ marginTop: 8 }}>
              <span>
                <i /> Relative humidity (%)
              </span>
            </div>
          </AsyncPanel>
        </Panel>

        <Panel title="Rainfall outlook" kicker={`Hourly precipitation probability · ${formatDayShort(date)}`} right={<Tag>{hourly.length} hours</Tag>}>
          <AsyncPanel
            loading={forecast.loading}
            error={forecast.error}
            onRetry={forecast.reload}
            empty={!forecast.loading && !forecast.error && hourly.length < 2}
            emptyLabel="Not enough hourly points to draw this chart."
          >
            <div className="chart-wrap">
              <LineChart
                values={hourly.map((h) => h.precip_probability)}
                labels={hourly.map((h) => formatHourLabel(h.time, clockFormat).slice(0, 5))}
                ariaLabel="Hourly precipitation probability"
              />
            </div>
            <div className="legend" style={{ marginTop: 8 }}>
              <span>
                <i /> Precipitation probability (%)
              </span>
            </div>
          </AsyncPanel>
        </Panel>
      </div>

      <Panel className="section-gap" title="Insight summary" kicker="Human-readable interpretation of the data above">
        <div className="alert-banner info" style={{ margin: 0 }}>
          <div style={{ display: "flex", gap: 12 }}>
            <span className="alert-dot" />
            <div>
              <strong>Pattern detected</strong>
              <p>
                {daily.length ? (
                  <>
                    Across the {daily.length}-day outlook for {location.name}, the warmest day is{" "}
                    {formatDayShort(hottest.date)} at {Math.round(hottest.temp_max_c)}°C and the coolest
                    night is {formatDayShort(coolest.date)} at {Math.round(coolest.temp_min_c)}°C. The
                    highest rain probability ({wettest.precip_probability_max}%) falls on{" "}
                    {formatDayShort(wettest.date)}.
                  </>
                ) : (
                  "Load a forecast to generate the interpretation."
                )}{" "}
                {rainTrend != null
                  ? `Stored rainfall probability is ${rainTrend >= 0 ? "up" : "down"} ${Math.abs(
                    rainTrend
                  ).toFixed(1)}% versus the previous period.`
                  : "Rainfall trend appears once two stored periods exist."}{" "}
                {accuracy.data?.samples
                  ? `Forecast accuracy is currently ${accuracy.data.overall.toFixed(1)}% over ${accuracy.data.samples} samples.`
                  : "Accuracy appears after a second forecast for the same day."}{" "}
                {(history.data?.entries || []).length
                  ? `${history.data.entries.length} queries are stored in your history (retention ${history.data.retention_days} days).`
                  : "No query history has been recorded yet."}
              </p>
            </div>
          </div>
        </div>
      </Panel>
    </section>
  );
}
