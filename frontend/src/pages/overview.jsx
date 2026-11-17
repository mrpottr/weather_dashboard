import { useMemo, useState } from "react";
import { useNavigate, useOutletContext } from "react-router-dom";
import {
  api,
  formatWeekday,
  pickLocationFromInput,
  useAsync,
  useAuth,
  useLocationContext,
  useToast,
} from "../api/client.jsx";
import { AsyncPanel, conditionIcon, Icon, MetricCard, Panel, Tag, useModal } from "../components/UI.jsx";
import WeatherHero from "../components/WeatherHero.jsx";
import LineChart from "../components/Charts.jsx";
import { ActivityGrid, SuitabilitySummary } from "../components/Activity.jsx";

export default function OverviewPage() {
  const { user } = useAuth();
  const { location, setLocation, date, setDate } = useLocationContext();
  const { refreshNotifications } = useOutletContext() || {};
  const navigate = useNavigate();
  const toast = useToast();
  const { openModal, closeModal } = useModal();

  const [activityKey, setActivityKey] = useState("cycling");
  const [refreshing, setRefreshing] = useState(false);

  const forecast = useAsync(
    () =>
      api.forecast({
        lat: location.latitude,
        lon: location.longitude,
        name: location.name,
        timezone: location.timezone,
        date,
      }),
    [location.latitude, location.longitude, date],
    { enabled: !!location }
  );

  const accuracy = useAsync(
    () => api.accuracy({ lat: location.latitude, lon: location.longitude, name: location.name }),
    [location.latitude, location.longitude],
    { enabled: !!location }
  );

  const favourites = useAsync(() => api.listLocations(), [], { enabled: !!user });
  const announcements = useAsync(() => api.announcements(), [], { enabled: !!user });
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

  const fc = forecast.data;
  const daily = fc?.daily || [];
  const day = useMemo(() => daily.find((d) => d.date === date) || daily[0] || null, [daily, date]);
  const current = fc?.current;

  // Hours for the selected date, falling back to the first day available.
  const chartPoints = useMemo(() => {
    const hourly = fc?.hourly || [];
    const forDate = hourly.filter((h) => (h.time || "").startsWith(date));
    return (forDate.length >= 2 ? forDate : hourly.slice(0, 24)).slice(0, 24);
  }, [fc, date]);

  const activeNotice = (announcements.data?.announcements || []).find(
    (a) => a.active && !a.dismissed
  );

  const onRefresh = async () => {
    if (!location) return;
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
      accuracy.reload();
      toast("Forecast refreshed", `${location.name} · ${fresh.from_cache ? "served from cache" : "live retrieval"}`, "↻");
    } catch (e) {
      toast("Refresh failed", e.message, "!");
    } finally {
      setRefreshing(false);
    }
  };

  const onSearchLocation = async (text) => {
    try {
      const picked = await pickLocationFromInput(text);
      if (!picked) {
        toast("No Australian match", `Nothing found for “${text}”.`, "!");
        return;
      }
      setLocation(picked);
      toast("Location updated", picked.name, "◎");
    } catch (e) {
      toast("Location search failed", e.message, "!");
    }
  };

  const chooseActivity = () => {
    openModal(
      "Choose an outdoor activity",
      <div>
        <p style={{ fontSize: 11, color: "var(--muted)", marginTop: 0 }}>
          The suitability rating is recalculated from the live forecast for {location?.name} on {date}.
        </p>
        <ActivityGrid
          activities={scored}
          selected={activityKey}
          onSelect={(key) => {
            setActivityKey(key);
            closeModal();
          }}
        />
      </div>
    );
  };

  const dismissNotice = async (id) => {
    try {
      await api.dismissAnnouncement(id);
      announcements.reload();
      if (refreshNotifications) refreshNotifications();
      toast("Announcement dismissed", "It will stay hidden for your account.", "✓");
    } catch (e) {
      toast("Could not dismiss", e.message, "!");
    }
  };

  const scored = intelligence.data?.activities || [];
  const activity =
    scored.find((a) => a.activity_key === activityKey) || scored[0] || null;

  return (
    <section className="page active">
      {activeNotice ? (
        <div className="alert-banner">
          <div style={{ display: "flex", gap: 12 }}>
            <span className="alert-dot" />
            <div>
              <strong>System announcement · {activeNotice.title}</strong>
              <p>{activeNotice.message}</p>
            </div>
          </div>
          <button className="tiny-btn" onClick={() => dismissNotice(activeNotice.id)}>
            Dismiss
          </button>
        </div>
      ) : null}

      {fc?.from_cache ? (
        <div className="cache-banner">
          <div>
            <strong>Cached forecast in use</strong>
            <p>
              The upstream BOM/Open-Meteo request did not complete, so the most recent stored
              forecast is being shown. Last successful retrieval:{" "}
              {fc.fetched_at ? new Date(fc.fetched_at).toLocaleString("en-AU") : "unknown"}.
            </p>
          </div>
          <button className="tiny-btn" onClick={onRefresh}>
            Retry live
          </button>
        </div>
      ) : null}

      <WeatherHero
        location={location}
        date={date}
        forecast={fc}
        day={day}
        loading={forecast.loading}
        error={forecast.error}
        refreshing={refreshing}
        onRetry={forecast.reload}
        onDateChange={setDate}
        onSearchLocation={onSearchLocation}
      />

      <div className="grid g4 section-gap">
        <MetricCard
          label="Temperature range"
          value={day ? `${Math.round(day.temp_min_c)}–${Math.round(day.temp_max_c)}°` : "—"}
          meta="Daily minimum / maximum"
        />
        <MetricCard
          label="Humidity"
          value={current ? `${current.humidity}%` : day ? `${day.humidity_avg}%` : "—"}
          meta={current ? "Current relative humidity" : "Daily average humidity"}
        />
        <MetricCard
          label="Rain probability"
          value={day ? `${day.precip_probability_max}%` : "—"}
          meta={day ? `${day.precip_sum_mm.toFixed(1)} mm expected` : "Selected date"}
        />
        <MetricCard
          label="Forecast accuracy"
          value={
            accuracy.data && accuracy.data.samples > 0 && accuracy.data.overall != null
              ? `${accuracy.data.overall.toFixed(1)}%`
              : accuracy.data && accuracy.data.overall != null
                ? `${accuracy.data.overall.toFixed(1)}%`
                : "—"
          }
          meta={
            accuracy.data
              ? accuracy.data.samples > 0
                ? `${accuracy.data.samples} forecast-vs-observation sample${accuracy.data.samples === 1 ? "" : "s"}`
                : "No accuracy samples recorded yet"
              : "Loading accuracy…"
          }
        />
      </div>

      <div className="grid g2 section-gap">
        <Panel
          title="Multi-day outlook"
          kicker="Auto-scrolling · click a card to change the selected date"
          right={
            <Tag>{daily.length} day outlook</Tag>
          }
        >
          <AsyncPanel
            loading={forecast.loading}
            error={forecast.error}
            onRetry={forecast.reload}
            empty={!forecast.loading && !forecast.error && daily.length === 0}
            emptyLabel="No daily outlook is available for this location."
          >
            <div className="outlook-marquee" tabIndex={0} aria-label="Multi-day outlook">
              <div className="marquee-viewport">
                <div className="marquee-track">
                  {[...daily, ...daily].map((d, i) => (
                    <button
                      key={`${d.date}-${i}`}
                      className={"forecast-day" + (d.date === date ? " active" : "")}
                      onClick={() => setDate(d.date)}
                      aria-label={`${formatWeekday(d.date)} ${Math.round(d.temp_min_c)} to ${Math.round(d.temp_max_c)} degrees, ${d.condition}`}
                    >
                      <div className="day">{formatWeekday(d.date)}</div>
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
                            width: `${Math.max(
                              8,
                              Math.min(100, ((d.temp_max_c - d.temp_min_c) / 20) * 100)
                            )}%`,
                          }}
                        />
                      </div>
                      <div style={{ fontSize: 10, color: "var(--muted)" }}>
                        💧 {d.precip_probability_max}%
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </AsyncPanel>
        </Panel>

        <Panel
          title="Weather intelligence"
          kicker="Practical recommendation from the current forecast"
          right={
            activity ? (
              <Tag variant={activity.score >= 65 ? "success" : activity.score >= 50 ? "warn" : "danger"}>
                {activity.tag || activity.rating}
              </Tag>
            ) : null
          }
        >
          <AsyncPanel
            loading={intelligence.loading}
            error={intelligence.error}
            onRetry={intelligence.reload}
            empty={!intelligence.loading && !intelligence.error && !activity}
            emptyLabel="Weather intelligence appears once a forecast is loaded."
          >
            <SuitabilitySummary activity={activity} onManage={chooseActivity} />
            <div style={{ display: "flex", gap: 8, marginTop: 14, flexWrap: "wrap" }}>
              <button className="tiny-btn" onClick={chooseActivity}>
                Change activity
              </button>
              <button
                className="tiny-btn"
                onClick={() => {
                  navigate("/intelligence");
                }}
              >
                View all activities
              </button>
            </div>
          </AsyncPanel>
        </Panel>
      </div>

      <div className="grid g2 section-gap">
        <Panel
          title="24-hour temperature trend"
          kicker="Hourly temperature for the selected date"
          right={
            <div className="legend">
              <span>
                <i /> Temperature
              </span>
            </div>
          }
        >
          <AsyncPanel
            loading={forecast.loading}
            error={forecast.error}
            onRetry={forecast.reload}
            empty={!forecast.loading && !forecast.error && chartPoints.length < 2}
            emptyLabel="Not enough hourly points to draw the trend."
          >
            <div className="chart-wrap">
              <LineChart
                values={chartPoints.map((h) => h.temp_c)}
                labels={chartPoints.map((h) => (h.time || "").slice(11, 16))}
                ariaLabel="Hourly temperature trend"
              />
            </div>
          </AsyncPanel>
        </Panel>

        <Panel
          title="Saved locations"
          kicker="Quick weather views from your database"
          right={
            <button
              className="tiny-btn"
              onClick={() => {
                navigate("/locations");
              }}
            >
              Manage
            </button>
          }
        >
          <AsyncPanel
            loading={favourites.loading}
            error={favourites.error}
            onRetry={favourites.reload}
            empty={!favourites.loading && !favourites.error && (favourites.data?.locations || []).length === 0}
            emptyLabel="No saved locations yet — add one from the Saved locations page."
          >
            <div className="location-list">
              {(favourites.data?.locations || []).slice(0, 4).map((l) => (
                <button
                  className="location-card"
                  key={l.id}
                  onClick={() => setLocation({ name: l.name, latitude: l.latitude, longitude: l.longitude, timezone: l.timezone, admin_area: l.admin_area })}
                  style={{ cursor: "pointer", textAlign: "left", width: "100%" }}
                >
                  <div className="left">
                    <span className="loc-icon">
                      <Icon name="heart" size={16} />
                    </span>
                    <div>
                      <strong>{l.name}</strong>
                      <small>
                        {l.admin_area || "Australia"} · {l.latitude.toFixed(2)}, {l.longitude.toFixed(2)}
                      </small>
                    </div>
                  </div>
                  <Icon name="map" size={16} />
                </button>
              ))}
            </div>
          </AsyncPanel>
        </Panel>
      </div>

      <div className="footer-note">
        {fc
          ? `${fc.source} · ${fc.licence || "BOM data via Open-Meteo"}. Weather data is used for non-commercial purposes only.`
          : "Weather data is retrieved by the Go backend from the BOM-based Open-Meteo endpoint."}
        {fc?.source_url ? (
          <>
            {" "}
            <a href={fc.source_url} target="_blank" rel="noreferrer noopener" style={{ color: "inherit" }}>
              Upstream source
            </a>
            .
          </>
        ) : null}
      </div>
    </section>
  );
}
