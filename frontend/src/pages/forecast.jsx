import { useEffect, useRef, useState } from "react";
import {
  api,
  formatDayShort,
  formatHourLabel,
  pickLocationFromInput,
  saveBlob,
  useAsync,
  useClock,
  useLocationContext,
  useToast,
} from "../api/client.jsx";
import { AsyncPanel, conditionIcon, Field, Icon, MetricCard, Panel, Tag } from "../components/UI.jsx";

export default function ForecastPage() {
  const { location, setLocation, date, setDate } = useLocationContext();
  const { format: clockFormat } = useClock();
  const toast = useToast();

  const [view, setView] = useState("Daily");
  const [query, setQuery] = useState("");
  const [suggestions, setSuggestions] = useState([]);
  const [searching, setSearching] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const debounce = useRef(0);

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

  // Australian geocoding suggestions while typing (3.2.2.1).
  useEffect(() => {
    const q = query.trim();
    window.clearTimeout(debounce.current);
    if (q.length < 3) {
      setSuggestions([]);
      return undefined;
    }
    debounce.current = window.setTimeout(async () => {
      setSearching(true);
      try {
        const res = await api.geocode(q);
        setSuggestions(res?.results || []);
      } catch {
        setSuggestions([]);
      } finally {
        setSearching(false);
      }
    }, 350);
    return () => window.clearTimeout(debounce.current);
  }, [query]);

  const choose = (place) => {
    setLocation({
      name: place.admin_area ? `${place.name}, ${place.admin_area}` : place.name,
      latitude: place.latitude,
      longitude: place.longitude,
      timezone: place.timezone,
      admin_area: place.admin_area,
    });
    setQuery("");
    setSuggestions([]);
    toast("Location updated", place.name, "◎");
  };

  const onSearchSubmit = async (e) => {
    e.preventDefault();
    const q = query.trim();
    if (!q) return;
    try {
      const picked = await pickLocationFromInput(q);
      if (!picked) {
        toast("No Australian match", `Nothing found for “${q}”.`, "!");
        return;
      }
      setLocation(picked);
      setQuery("");
      setSuggestions([]);
      toast("Location updated", picked.name, "◎");
    } catch (err) {
      toast("Location search failed", err.message, "!");
    }
  };

  const onRefresh = async () => {
    setRefreshing(true);
    try {
      const fresh = await api.forecast({
        lat: location.latitude,
        lon: location.longitude,
        name: location.name,
        timezone: location.timezone,
        date,
        refresh: 1,
      });
      forecast.setData(fresh);
      toast("Forecast refreshed", fresh.from_cache ? "Upstream unavailable — cached copy served." : "Live retrieval complete.", "↻");
    } catch (err) {
      toast("Refresh failed", err.message, "!");
    } finally {
      setRefreshing(false);
    }
  };

  const onExport = async (format) => {
    try {
      const res = await api.exportForecast({
        lat: location.latitude,
        lon: location.longitude,
        name: location.name,
        timezone: location.timezone,
        date,
        format,
      });
      const name = await saveBlob(res, `atmos-forecast.${format === "json" ? "json" : "csv"}`);
      toast("Export ready", name, "⤓");
    } catch (err) {
      toast("Export failed", err.message, "!");
    }
  };

  const fc = forecast.data;
  const daily = fc?.daily || [];
  const hourly = (fc?.hourly || []).filter((h) => (h.time || "").startsWith(date));
  const day = daily.find((d) => d.date === date) || daily[0] || null;
  const current = fc?.current;

  return (
    <section className="page active">
      <div className="page-title">Forecast matrix</div>
      <p className="page-sub">Daily and hourly layers for the selected Australian location.</p>

      {fc?.from_cache ? (
        <div className="cache-banner">
          <div>
            <strong>Cached forecast in use</strong>
            <p>Upstream retrieval failed, so the latest stored forecast is shown. Retry to attempt a live pull.</p>
          </div>
          <button className="tiny-btn" onClick={onRefresh}>
            Retry live
          </button>
        </div>
      ) : null}

      <div className="panel">
        <form className="toolbar" onSubmit={onSearchSubmit}>
          <div className="searchbox">
            <Icon name="search" />
            <input
              className="input"
              placeholder="Search another city, postcode or coordinates"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Search another Australian location"
            />
            {suggestions.length > 0 ? (
              <div className="suggest">
                {suggestions.slice(0, 6).map((p, i) => (
                  <button type="button" key={`${p.latitude}-${p.longitude}-${i}`} onClick={() => choose(p)}>
                    {p.name}
                    <span>
                      {p.admin_area || p.country} · {p.latitude.toFixed(2)}, {p.longitude.toFixed(2)}
                    </span>
                  </button>
                ))}
              </div>
            ) : null}
          </div>
          <select
            className="select"
            style={{ width: "auto" }}
            value={view}
            onChange={(e) => setView(e.target.value)}
            aria-label="Forecast view"
          >
            <option>Daily</option>
            <option>Hourly</option>
          </select>
          <button className="ghost-btn" type="button" onClick={onRefresh} disabled={refreshing}>
            <Icon name="refresh" size={14} />
            {refreshing ? "Refreshing…" : "Refresh"}
          </button>
          <button className="ghost-btn" type="button" onClick={() => onExport("csv")}>
            <Icon name="download" size={14} />
            Export CSV
          </button>
          <button className="ghost-btn" type="button" onClick={() => onExport("json")}>
            Export JSON
          </button>
          {searching ? <Tag>Searching…</Tag> : null}
        </form>
        <p className="panel-kicker" style={{ marginTop: 10 }}>
          Showing {location.name} · {formatDayShort(date)} · source {fc?.source || "loading"}
          {fc?.source_degraded ? " (fallback endpoint)" : ""}
        </p>
      </div>

      <div className="grid g4 section-gap">
        <AsyncPanel
          loading={forecast.loading}
          error={forecast.error}
          onRetry={forecast.reload}
          empty={!forecast.loading && !forecast.error && daily.length === 0}
          emptyLabel="No daily forecast is available for this location."
        >
          {daily.map((d) => (
            <button
              key={d.date}
              className={"forecast-day" + (d.date === date ? " active" : "")}
              onClick={() => setDate(d.date)}
            >
              <div className="day">{formatDayShort(d.date)}</div>
              <div className="weather-symbol" aria-hidden="true">
                {conditionIcon(d.condition)}
              </div>
              <div className="forecast-temp">
                <strong>{Math.round(d.temp_max_c)}°</strong>
                <span>{Math.round(d.temp_min_c)}°</span>
              </div>
              <div className="range">
                <i
                  style={{
                    width: `${Math.max(10, Math.min(100, ((d.temp_max_c - d.temp_min_c) / 20) * 100))}%`,
                  }}
                />
              </div>
              <div style={{ fontSize: 10, color: "var(--muted)" }}>
                💧 {d.precip_probability_max}% · {d.condition}
              </div>
            </button>
          ))}
        </AsyncPanel>
      </div>

      <Panel
        className="section-gap"
        title="Forecast detail"
        kicker={view === "Hourly" ? "Every available hour for the selected date" : "Daily summary cards for the selected date"}
        right={<Tag>{view.toUpperCase()}</Tag>}
      >
        <AsyncPanel
          loading={forecast.loading}
          error={forecast.error}
          onRetry={forecast.reload}
          empty={!forecast.loading && !forecast.error && !day}
          emptyLabel="Select a date to inspect conditions."
        >
          {view === "Hourly" ? (
            hourly.length ? (
              <div className="forecast-strip">
                {hourly.map((h) => (
                  <div className="forecast-day" key={h.time} style={{ cursor: "default" }}>
                    <div className="day">{formatHourLabel(h.time, clockFormat)}</div>
                    <div className="weather-symbol" aria-hidden="true">
                      {conditionIcon(h.condition)}
                    </div>
                    <div className="forecast-temp">
                      <strong>{Math.round(h.temp_c)}°</strong>
                      <span>{Math.round(h.feels_like_c)}°</span>
                    </div>
                    <div style={{ fontSize: 10, color: "var(--muted)" }}>
                      💧 {h.precip_probability}% · {Math.round(h.wind_speed_kmh)} km/h {h.wind_dir}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="empty">
                No hourly rows were returned for this date. Try another date from the outlook strip.
              </div>
            )
          ) : (
            <div className="grid g2">
              <div>
                <div className="condition-title" style={{ fontSize: 18 }}>
                  {conditionIcon(day?.condition)} {day?.condition}
                </div>
                <p className="panel-kicker" style={{ lineHeight: 1.7 }}>
                  Daylight window {formatHourLabel(day?.sunrise, clockFormat)} –{" "}
                  {formatHourLabel(day?.sunset, clockFormat)}. Maximum wind{" "}
                  {Math.round(day?.wind_max_kmh || 0)} km/h {day?.wind_dir} with gusts to{" "}
                  {Math.round(day?.wind_gust_max_kmh || 0)} km/h.
                </p>
                <div className="chip-row" style={{ marginTop: 10 }}>
                  <Tag variant={day && day.precip_probability_max >= 50 ? "warn" : ""}>
                    Rain chance {day?.precip_probability_max}%
                  </Tag>
                  <Tag variant={day && day.uv_index_max >= 8 ? "danger" : day && day.uv_index_max >= 3 ? "warn" : ""}>
                    UV max {day?.uv_index_max?.toFixed(0)}
                  </Tag>
                  <Tag>Humidity {day?.humidity_avg}%</Tag>
                  <Tag>Cloud {day?.cloud_cover_avg}%</Tag>
                </div>
              </div>
              <div className="grid g2">
                <MetricCard label="Apparent min" value={day ? `${Math.round(day.apparent_min_c)}°` : "—"} meta="feels-like °C" />
                <MetricCard label="Apparent max" value={day ? `${Math.round(day.apparent_max_c)}°` : "—"} meta="feels-like °C" />
                <MetricCard label="Pressure" value={day ? Math.round(day.pressure_avg_hpa) : "—"} meta="hPa average" />
                <MetricCard label="Visibility" value={day ? `${(day.visibility_km ?? 0).toFixed(0)}` : "—"} meta="kilometres" />
              </div>
            </div>
          )}
        </AsyncPanel>
      </Panel>

      <Panel className="section-gap" title="Selected date conditions" kicker="Clear display of the BOM-derived daily values" right={<Tag>{formatDayShort(date)}</Tag>}>
        <div className="grid g4">
          <MetricCard label="Minimum temperature" value={day ? `${Math.round(day.temp_min_c)}°` : "—"} meta="°C" />
          <MetricCard label="Maximum temperature" value={day ? `${Math.round(day.temp_max_c)}°` : "—"} meta="°C" />
          <MetricCard label="Humidity" value={day ? `${day.humidity_avg}%` : "—"} meta="relative" />
          <MetricCard label="Rainfall probability" value={day ? `${day.precip_probability_max}%` : "—"} meta={`${(day?.precip_sum_mm ?? 0).toFixed(1)} mm forecast`} />
          <MetricCard label="Wind" value={day ? `${Math.round(day.wind_max_kmh)}` : "—"} meta={`km/h ${day?.wind_dir || ""}`} />
          <MetricCard label="Wind gust" value={day ? `${Math.round(day.wind_gust_max_kmh)}` : "—"} meta="km/h maximum" />
          <MetricCard label="Dew point" value={day ? `${Math.round(day.dew_point_avg_c)}°` : "—"} meta="°C average" />
          <MetricCard label="Current temperature" value={current ? `${Math.round(current.temp_c)}°` : "—"} meta={current ? `feels like ${Math.round(current.feels_like_c)}°` : "no live observation"} />
        </div>
        <Field label="Selected date">
          <input
            className="input"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            style={{ maxWidth: 220 }}
            aria-label="Selected forecast date"
          />
        </Field>
      </Panel>
    </section>
  );
}
