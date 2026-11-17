import { useEffect, useRef } from "react";
import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { api, useAsync, useLocationContext, useToast } from "../api/client.jsx";
import { AsyncPanel, Icon, Panel, Tag } from "../components/UI.jsx";

const AU_BOUNDS = [
  [-44.5, 112.0],
  [-9.0, 154.5],
];

export default function MapPage() {
  const { setLocation } = useLocationContext();
  const toast = useToast();
  const frameRef = useRef(null);
  const mapRef = useRef(null);
  const layerRef = useRef(null);

  const markers = useAsync(() => api.markers(), []);
  const locations = markers.data?.locations || [];

  const openLocation = (lat, lon, name, timezone, adminArea) => {
    setLocation({ name, latitude: Number(lat), longitude: Number(lon), timezone, admin_area: adminArea });
    toast("Workspace location updated", name, "◎");
    window.history.pushState({}, "", "/overview");
    window.dispatchEvent(new PopStateEvent("popstate"));
  };

  // Initialise the map once.
  useEffect(() => {
    if (!frameRef.current || mapRef.current) return undefined;
    const map = L.map(frameRef.current, {
      center: [-25.6, 134.4],
      zoom: 4,
      minZoom: 4,
      maxZoom: 12,
      maxBounds: AU_BOUNDS,
      maxBoundsViscosity: 1.0,
      worldCopyJump: false,
      attributionControl: true,
    });
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
      crossOrigin: true,
    }).addTo(map);
    layerRef.current = L.layerGroup().addTo(map);

    // The popup "Open in workspace" buttons are handled by delegation so the
    // marker layer can be rebuilt without re-binding listeners.
    const onClick = (ev) => {
      const btn = ev.target.closest ? ev.target.closest("[data-open-location]") : null;
      if (!btn) return;
      openLocation(
        btn.getAttribute("data-lat"),
        btn.getAttribute("data-lon"),
        btn.getAttribute("data-name"),
        btn.getAttribute("data-tz"),
        btn.getAttribute("data-area")
      );
    };
    frameRef.current.addEventListener("click", onClick);
    mapRef.current = map;

    return () => {
      if (frameRef.current) frameRef.current.removeEventListener("click", onClick);
      map.remove();
      mapRef.current = null;
      layerRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // (Re)draw the markers whenever the payload changes.
  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    layer.clearLayers();
    locations.forEach((m) => {
      const temp = m.temp_c != null ? `${Math.round(m.temp_c)}°` : "–";
      const icon = L.divIcon({
        className: "",
        html: `<div class="map-pin${m.saved ? " saved" : ""}">${m.icon || temp}</div>`,
        iconSize: [30, 30],
        iconAnchor: [15, 15],
      });
      const popup = `
        <div>
          <strong>${escapeHtml(m.name)}</strong>
          <div style="font-size:10px;color:var(--muted);margin-top:4px">
            ${m.condition ? escapeHtml(m.condition) : "No live values"} ·
            ${temp} · ${escapeHtml(m.admin_area || "AU")}
          </div>
          <div style="font-size:9px;color:var(--dim);margin-top:4px">
            ${m.from_cache ? "served from cache" : "live"} · ${escapeHtml(m.timezone || "")}
          </div>
          <button class="tiny-btn" style="margin-top:8px"
            data-open-location="1"
            data-lat="${m.latitude}" data-lon="${m.longitude}"
            data-name="${escapeHtml(m.name)}" data-tz="${escapeHtml(m.timezone || "")}"
            data-area="${escapeHtml(m.admin_area || "")}">Open in workspace</button>
        </div>`;
      L.marker([m.latitude, m.longitude], { icon, title: m.name, alt: m.name })
        .bindPopup(popup)
        .addTo(layer);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [locations.length, markers.data]);

  return (
    <section className="page active">
      <div className="page-title">Weather map</div>
      <p className="page-sub">
        Live conditions for the major Australian cities and your saved locations, plotted concurrently by the
        backend.
      </p>

      {markers.data ? (
        <Panel className="section-gap">
          <div className="toolbar">
            <Tag variant="success">
              {markers.data.count} locations plotted
            </Tag>
            <Tag>concurrent {markers.data.concurrent_ms} ms</Tag>
            <Tag>sequential {markers.data.sequential_ms} ms</Tag>
            <Tag variant="accent">
              speed-up ×
              {markers.data.concurrent_ms > 0
                ? (markers.data.sequential_ms / markers.data.concurrent_ms).toFixed(1)
                : "—"}
            </Tag>
          </div>
        </Panel>
      ) : null}

      <div className="map-frame section-gap">
        <div ref={frameRef} style={{ width: "100%", height: "100%" }} />
        <div className="map-legend">
          <span>
            <span className="map-pin" style={{ width: 16, height: 16 }} /> city
          </span>
          <span>
            <span className="map-pin saved" style={{ width: 16, height: 16 }} /> saved
          </span>
          <span>Tap a marker for live values</span>
        </div>
      </div>

      <Panel className="section-gap" title="Plotted locations" kicker="Live temperature and condition for each marker">
        <AsyncPanel
          loading={markers.loading}
          error={markers.error}
          onRetry={markers.reload}
          empty={!markers.loading && !markers.error && locations.length === 0}
          emptyLabel="No locations were returned for the map."
        >
          <div className="grid g3">
            {locations.map((m) => (
              <button
                key={`${m.name}-${m.latitude}`}
                className="location-card"
                style={{ cursor: "pointer", textAlign: "left", width: "100%" }}
                onClick={() => openLocation(m.latitude, m.longitude, m.name, m.timezone, m.admin_area)}
              >
                <div className="left">
                  <span className="loc-icon">{m.icon || "📍"}</span>
                  <div>
                    <strong>{m.name}</strong>
                    <small>
                      {m.condition || "no live values"} ·{" "}
                      {m.temp_c != null ? `${Math.round(m.temp_c)}°C` : "—"}
                      {m.saved ? " · saved" : ""}
                    </small>
                  </div>
                </div>
                <Icon name="map" size={16} />
              </button>
            ))}
          </div>
        </AsyncPanel>
      </Panel>

      <div className="footer-note">
        Map data © OpenStreetMap contributors, rendered with Leaflet under the OSM tile usage policy. The map
        is bounded to Australia and Tasmania.
      </div>
    </section>
  );
}

/** Minimal HTML escaping for popup strings (values still originate server-side). */
function escapeHtml(value) {
  return String(value == null ? "" : value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
