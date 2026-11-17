import React, { useEffect, useMemo, useRef, useState } from "react";
import { Icon, Loading } from "./UI.jsx";
import { api, formatDayLong, formatHourLabel, useAsync, useClock } from "../api/client.jsx";

/* ------------------------------------------------------- scene animation */

/**
 * WeatherScene draws a lightweight, purely decorative animation (sun rays,
 * drifting clouds, rain streaks) that reflects the real forecast condition.
 * It is decoration only — every number shown on the hero comes from the API.
 */
function WeatherScene({ condition, tempC }) {
  const canvasRef = useRef(null);
  const stateRef = useRef({ clouds: [], drops: [] });

  const kind = useMemo(() => {
    const c = (condition || "").toLowerCase();
    if (c.includes("thunder") || c.includes("storm")) return "storm";
    if (c.includes("rain") || c.includes("shower")) return "rain";
    if (c.includes("fog") || c.includes("mist")) return "fog";
    if (c.includes("cloud") || c.includes("overcast") || c.includes("partly")) return "cloud";
    if (c.includes("clear") || c.includes("sun")) return "sun";
    return tempC >= 29 ? "sun" : "cloud";
  }, [condition, tempC]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    let raf = 0;
    let t = 0;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    const seed = () => {
      const rect = canvas.getBoundingClientRect();
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = Math.max(1, Math.floor(rect.width * dpr));
      canvas.height = Math.max(1, Math.floor(rect.height * dpr));
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      const s = stateRef.current;
      s.clouds = [0, 1, 2].map((i) => ({
        x: rect.width * (0.12 + i * 0.3),
        y: rect.height * (0.16 + i * 0.14),
        w: rect.width * (0.24 - i * 0.03),
        speed: 0.12 + i * 0.05,
      }));
      s.drops = Array.from({ length: 42 }, (_, i) => ({
        x: (i * 37) % Math.max(1, rect.width),
        y: (i * 53) % Math.max(1, rect.height),
        len: 8 + (i % 4) * 3,
        speed: 2.2 + (i % 5) * 0.5,
      }));
      return rect;
    };

    let rect = seed();
    const onResize = () => {
      rect = seed();
    };
    window.addEventListener("resize", onResize);

    const drawCloud = (c, alpha) => {
      ctx.globalAlpha = alpha;
      ctx.fillStyle = "rgba(226,238,246,.85)";
      ctx.beginPath();
      ctx.ellipse(c.x, c.y, c.w * 0.5, c.w * 0.26, 0, 0, Math.PI * 2);
      ctx.ellipse(c.x - c.w * 0.32, c.y + c.w * 0.07, c.w * 0.32, c.w * 0.19, 0, 0, Math.PI * 2);
      ctx.ellipse(c.x + c.w * 0.3, c.y + c.w * 0.08, c.w * 0.34, c.w * 0.2, 0, 0, Math.PI * 2);
      ctx.fill();
      ctx.globalAlpha = 1;
    };

    const frame = () => {
      t += 1;
      ctx.clearRect(0, 0, rect.width, rect.height);
      const s = stateRef.current;

      if (kind === "sun") {
        const cx = rect.width * 0.62;
        const cy = rect.height * 0.34;
        const pulse = 1 + Math.sin(t / 45) * 0.05;
        const grad = ctx.createRadialGradient(cx, cy, 4, cx, cy, rect.width * 0.22 * pulse);
        grad.addColorStop(0, "rgba(255,214,140,.55)");
        grad.addColorStop(1, "rgba(255,214,140,0)");
        ctx.fillStyle = grad;
        ctx.beginPath();
        ctx.arc(cx, cy, rect.width * 0.22 * pulse, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = "rgba(255,236,190,.5)";
        ctx.lineWidth = 1.4;
        for (let i = 0; i < 12; i++) {
          const a = (i / 12) * Math.PI * 2 + t / 240;
          const r0 = rect.width * 0.1 * pulse;
          const r1 = rect.width * 0.14 * pulse;
          ctx.beginPath();
          ctx.moveTo(cx + Math.cos(a) * r0, cy + Math.sin(a) * r0);
          ctx.lineTo(cx + Math.cos(a) * r1, cy + Math.sin(a) * r1);
          ctx.stroke();
        }
        ctx.fillStyle = "rgba(255,226,160,.85)";
        ctx.beginPath();
        ctx.arc(cx, cy, rect.width * 0.075, 0, Math.PI * 2);
        ctx.fill();
      }

      if (kind === "cloud" || kind === "rain" || kind === "storm" || kind === "fog") {
        s.clouds.forEach((c, i) => {
          c.x += c.speed * (reduced ? 0 : 1);
          if (c.x - c.w > rect.width) c.x = -c.w;
          drawCloud(c, kind === "fog" ? 0.5 : 0.34 + i * 0.06);
        });
      }

      if (kind === "rain" || kind === "storm") {
        ctx.strokeStyle = kind === "storm" ? "rgba(150,190,255,.55)" : "rgba(180,214,240,.5)";
        ctx.lineWidth = 1.2;
        s.drops.forEach((d) => {
          if (!reduced) {
            d.y += d.speed;
            d.x += d.speed * 0.25;
            if (d.y > rect.height) {
              d.y = -d.len;
              d.x = (d.x + 61) % rect.width;
            }
          }
          ctx.beginPath();
          ctx.moveTo(d.x, d.y);
          ctx.lineTo(d.x - 2, d.y + d.len);
          ctx.stroke();
        });
        if (kind === "storm" && !reduced && t % 190 < 4) {
          ctx.fillStyle = "rgba(210,226,255,.16)";
          ctx.fillRect(0, 0, rect.width, rect.height);
        }
      }

      if (kind === "fog") {
        for (let i = 0; i < 4; i++) {
          const y = rect.height * (0.4 + i * 0.13);
          ctx.strokeStyle = `rgba(214,228,238,${0.16 - i * 0.02})`;
          ctx.lineWidth = 8;
          ctx.beginPath();
          ctx.moveTo(-20, y + Math.sin(t / 90 + i) * 4);
          ctx.lineTo(rect.width + 20, y + Math.cos(t / 70 + i) * 4);
          ctx.stroke();
        }
      }

      if (!reduced) raf = requestAnimationFrame(frame);
    };

    frame();
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", onResize);
    };
  }, [kind]);

  return (
    <div className="hero-anim" aria-hidden="true">
      <canvas ref={canvasRef} />
    </div>
  );
}

/* ------------------------------------------------------------- the hero */

/**
 * WeatherHero renders the full hero for one location + forecast date.
 *
 * forecast: ForecastResponse | null
 * day:      DailyForecast | null   (the selected date's daily row)
 * loading / error: the fetch state for honest loading + error rendering
 */
export default function WeatherHero({ location, date, forecast, day, loading, error, onRetry, onDateChange, onSearchLocation, refreshing }) {
  const [photoIdx, setPhotoIdx] = useState(0);
  const { format: clockFormat } = useClock();
  const town = (location?.name || "").split(",")[0].trim();

  const images = useAsync(() => (town ? api.townImages(town) : Promise.resolve(null)), [town], {
    enabled: !!town,
  });
  const photoList = images.data?.images || [];

  // Rotate through the available town photos like a live visual layer.
  useEffect(() => {
    setPhotoIdx(0);
    if (photoList.length < 2) return;
    const t = setInterval(() => setPhotoIdx((i) => (i + 1) % photoList.length), 12000);
    return () => clearInterval(t);
  }, [photoList.length, town]);

  const current = forecast?.current;
  const daily = forecast?.daily || [];
  const shown = day || daily[0] || null;

  const temp = shown ? Math.round((shown.temp_min_c + shown.temp_max_c) / 2) : current ? Math.round(current.temp_c) : null;

  const heroStyle = photoList.length
    ? { backgroundImage: `url("${photoList[photoIdx]}")` }
    : undefined;

  return (
    <section className="hero" style={heroStyle}>
      {photoList.length ? (
        <div
          className="hero-photo"
          style={{ backgroundImage: `url("${photoList[photoIdx]}")` }}
        />
      ) : null}
      <WeatherScene condition={current?.condition || shown?.condition} tempC={current?.temp_c ?? temp} />

      <div className="hero-top">
        <div>
          <div className="eyebrow">Live atmospheric snapshot</div>
          <h1>{location?.name || "Select a location"}</h1>
          <div className="hero-sub">
            {formatDayLong(date)} · selected forecast date
          </div>
        </div>
        <div className="hero-controls">
          <input
            className="input"
            list="atmos-location-options"
            placeholder="Search city / postcode / coordinates"
            aria-label="Search city, postcode or coordinates"
            onChange={(e) => {
              const v = e.target.value.trim();
              if (v.length >= 3 && onSearchLocation) onSearchLocation(v);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" && onSearchLocation) onSearchLocation(e.target.value.trim());
            }}
          />
          <datalist id="atmos-location-options">
            <option value="Melbourne, VIC" />
            <option value="Sydney, NSW" />
            <option value="Brisbane, QLD" />
            <option value="Perth, WA" />
            <option value="Hobart, TAS" />
            <option value="Adelaide, SA" />
            <option value="Darwin, NT" />
            <option value="Canberra, ACT" />
          </datalist>
          <input
            className="input"
            type="date"
            value={date || ""}
            aria-label="Forecast date"
            onChange={(e) => onDateChange && onDateChange(e.target.value)}
          />
        </div>
      </div>

      {loading ? (
        <div className="current-row">
          <div className="condition-meta">
            <Loading label="Fetching the live BOM forecast…" />
          </div>
        </div>
      ) : error ? (
        <div className="current-row">
          <div className="condition-meta">
            <div style={{ color: "#ffd6de", fontWeight: 800 }}>Forecast unavailable</div>
            <p style={{ color: "#d2dee5", fontSize: 12, maxWidth: 420, lineHeight: 1.6 }}>
              {error.message}
            </p>
            <button className="ghost-btn" style={{ marginTop: 12 }} onClick={onRetry}>
              Retry
            </button>
          </div>
        </div>
      ) : (
        <>
          <div className="current-row">
            <div className="temp">{temp != null ? `${temp}°` : "—"}</div>
            <div className="condition-meta">
              <div className="condition-title">{current?.condition || shown?.condition || "No data"}</div>
              <div className="mini-metrics">
                {current ? (
                  <>
                    <span>
                      Feels like <b>{Math.round(current.feels_like_c)}°</b>
                    </span>
                    <span>
                      Wind <b>{Math.round(current.wind_speed_kmh)} km/h {current.wind_dir}</b>
                    </span>
                    <span>
                      Humidity <b>{current.humidity}%</b>
                    </span>
                  </>
                ) : (
                  <span>No current observations available</span>
                )}
              </div>
            </div>
          </div>

          <div className="hero-bottom">
            <div className="source-tag">
              <span className="source-badge">B</span>
              <span>
                {forecast?.source || "BOM forecast source"}
                {forecast?.source_degraded ? " · fallback endpoint in use" : ""}
                {forecast?.from_cache ? " · cached copy" : ""}
              </span>
            </div>
            <div className="quick-stat-row">
              <div className="quick-stat">
                <span>Min</span>
                <strong>{shown ? `${Math.round(shown.temp_min_c)}°` : "—"}</strong>
                <em>°C</em>
              </div>
              <div className="quick-stat">
                <span>Max</span>
                <strong>{shown ? `${Math.round(shown.temp_max_c)}°` : "—"}</strong>
                <em>°C</em>
              </div>
              <div className="quick-stat">
                <span>Rain</span>
                <strong>{shown ? `${shown.precip_probability_max}%` : "—"}</strong>
                <em>{shown ? `${shown.precip_sum_mm.toFixed(1)} mm` : ""}</em>
              </div>
              <div className="quick-stat">
                <span>Pressure</span>
                <strong>{shown ? Math.round(shown.pressure_avg_hpa) : "—"}</strong>
                <em>hPa</em>
              </div>
              <div className="quick-stat">
                <span>Wind gust</span>
                <strong>{shown ? Math.round(shown.wind_gust_max_kmh) : "—"}</strong>
                <em>km/h</em>
              </div>
              <div className="quick-stat">
                <span>UV index</span>
                <strong>{shown ? shown.uv_index_max.toFixed(0) : "—"}</strong>
                <em>max</em>
              </div>
              <div className="quick-stat">
                <span>Visibility</span>
                <strong>{shown ? (shown.visibility_km ?? 0).toFixed(0) : "—"}</strong>
                <em>km</em>
              </div>
              <div className="quick-stat">
                <span>Cloud</span>
                <strong>{shown ? `${Math.round(shown.cloud_cover_avg)}%` : "—"}</strong>
                <em>cover</em>
              </div>
              <div className="quick-stat">
                <span>Dew pt</span>
                <strong>{current ? `${Math.round(current.dew_point_c)}°` : "—"}</strong>
                <em>°C</em>
              </div>
              <div className="quick-stat">
                <span>Sunrise</span>
                <strong>{shown ? formatHourLabel(shown.sunrise, clockFormat) : "—"}</strong>
                <em>local</em>
              </div>
              <div className="quick-stat">
                <span>Sunset</span>
                <strong>{shown ? formatHourLabel(shown.sunset, clockFormat) : "—"}</strong>
                <em>local</em>
              </div>
              <div className="quick-stat">
                <span>Sync</span>
                <strong>{forecast ? `${Math.round((forecast.latency_ms || 0) / 100) / 10}s` : "—"}</strong>
                <em>{refreshing ? "refreshing" : forecast?.from_cache ? "cached" : "live"}</em>
              </div>
            </div>
          </div>
        </>
      )}
    </section>
  );
}

/** Small helper used by the top bar source pill. */
export function SourceIcon() {
  return <Icon name="cloud-sun" size={14} />;
}
