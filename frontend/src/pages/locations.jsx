import { useEffect, useRef, useState } from "react";
import {
  api,
  pickLocationFromInput,
  useAsync,
  useLocationContext,
  useToast,
} from "../api/client.jsx";
import { AsyncPanel, Empty, Field, Icon, Panel, Tag, useModal } from "../components/UI.jsx";

export default function LocationsPage() {
  const { setLocation } = useLocationContext();
  const toast = useToast();
  const { openModal, closeModal } = useModal();

  const [filter, setFilter] = useState("");
  const [debounced, setDebounced] = useState("");
  const timer = useRef(0);

  useEffect(() => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setDebounced(filter.trim()), 300);
    return () => window.clearTimeout(timer.current);
  }, [filter]);

  const favourites = useAsync(() => api.listLocations(debounced || undefined), [debounced]);

  const open = (l) => {
    setLocation({
      name: l.name,
      latitude: l.latitude,
      longitude: l.longitude,
      timezone: l.timezone,
      admin_area: l.admin_area,
    });
    toast("Workspace location updated", l.name, "◎");
    window.history.pushState({}, "", "/overview");
    window.dispatchEvent(new PopStateEvent("popstate"));
  };

  const remove = async (l) => {
    try {
      await api.deleteLocation(l.id);
      favourites.reload();
      toast("Location removed", l.name, "✓");
    } catch (e) {
      toast("Could not remove location", e.message, "!");
    }
  };

  const addLocation = () => {
    openModal(
      "Add an Australian location",
      <AddLocationForm
        onCancel={closeModal}
        onSaved={(l) => {
          closeModal();
          favourites.reload();
          toast("Location saved", l.name, "✓");
        }}
      />
    );
  };

  const locations = favourites.data?.locations || [];

  return (
    <section className="page active">
      <div className="page-title">Saved locations</div>
      <p className="page-sub">Search, filter and open your favourite Australian forecast locations.</p>

      <div className="panel">
        <div className="toolbar">
          <div className="searchbox">
            <Icon name="search" />
            <input
              className="input"
              placeholder="Filter saved locations"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              aria-label="Filter saved locations"
            />
          </div>
          <button className="primary-btn" style={{ width: "auto" }} onClick={addLocation}>
            + Add location
          </button>
        </div>
        <p className="panel-kicker" style={{ marginTop: 10 }}>
          {locations.length} saved location{locations.length === 1 ? "" : "s"} stored for your account.
        </p>
      </div>

      <div className="grid g2 section-gap">
        <AsyncPanel
          loading={favourites.loading}
          error={favourites.error}
          onRetry={favourites.reload}
          empty={!favourites.loading && !favourites.error && locations.length === 0}
          emptyLabel={
            debounced
              ? `No saved locations match “${debounced}”.`
              : "No saved locations yet. Use “Add location” to save your first Australian place."
          }
        >
          {locations.map((l) => (
            <div className="location-card" key={l.id}>
              <div className="left">
                <span className="loc-icon">
                  <Icon name="heart" size={16} />
                </span>
                <div>
                  <strong>{l.name}</strong>
                  <small>
                    {l.admin_area || "Australia"} · {l.latitude.toFixed(3)}, {l.longitude.toFixed(3)} ·{" "}
                    {l.timezone || "timezone resolved on fetch"}
                  </small>
                </div>
              </div>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <button className="tiny-btn" onClick={() => open(l)}>
                  Open
                </button>
                <button className="danger-btn" style={{ padding: "8px 10px", fontSize: 11 }} onClick={() => remove(l)}>
                  Remove
                </button>
              </div>
            </div>
          ))}
        </AsyncPanel>
      </div>

      <div className="footer-note">
        Saved locations feed the Overview preview, the alerts engine and the live map markers.
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ modal */

/** Geocoder-backed add form (city, postcode or "lat, lon"). */
function AddLocationForm({ onCancel, onSaved }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const search = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const res = await api.geocode(query.trim());
      setResults(res?.results || []);
      if (!res?.results?.length) setError("No Australian places matched that search.");
    } catch (err) {
      setError(err.message);
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

  const save = async (place) => {
    try {
      await api.createLocation({
        name: place.admin_area ? `${place.name}, ${place.admin_area}` : place.name,
        latitude: place.latitude,
        longitude: place.longitude,
        timezone: place.timezone,
        admin_area: place.admin_area,
      });
      onSaved({ name: place.name });
    } catch (err) {
      setError(err.message);
    }
  };

  const saveCoordinates = async () => {
    const picked = await pickLocationFromInput(query);
    if (!picked) {
      setError("Enter a city, postcode or coordinates inside Australia.");
      return;
    }
    try {
      await api.createLocation({
        name: picked.name,
        latitude: picked.latitude,
        longitude: picked.longitude,
        timezone: picked.timezone,
        admin_area: picked.admin_area,
      });
      onSaved(picked);
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <div>
      <form onSubmit={search} className="toolbar">
        <div className="searchbox">
          <Icon name="search" />
          <input
            className="input"
            placeholder="City, postcode or “-37.81, 144.96”"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            aria-label="Search Australian location to save"
          />
        </div>
        <button className="ghost-btn" type="submit" disabled={loading}>
          {loading ? "Searching…" : "Search"}
        </button>
      </form>

      <div className="error-note" role="alert">
        {error}
      </div>

      {results.length ? (
        <div className="location-list" style={{ marginTop: 10 }}>
          {results.map((p, i) => (
            <div className="location-card" key={`${p.latitude}-${p.longitude}-${i}`}>
              <div className="left">
                <span className="loc-icon">
                  <Icon name="map" size={16} />
                </span>
                <div>
                  <strong>{p.name}</strong>
                  <small>
                    {p.admin_area || p.country} · {p.latitude.toFixed(3)}, {p.longitude.toFixed(3)}
                    {p.postcode ? ` · ${p.postcode}` : ""}
                  </small>
                </div>
              </div>
              <button className="tiny-btn" onClick={() => save(p)}>
                Save
              </button>
            </div>
          ))}
        </div>
      ) : (
        <Empty message="Search a city, postcode or paste coordinates to preview matches." />
      )}

      <Field label="Offline / coordinate entry">
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button className="ghost-btn" type="button" onClick={saveCoordinates} disabled={!query.trim()}>
            Save entered text directly
          </button>
          <button className="ghost-btn" type="button" onClick={onCancel}>
            Cancel
          </button>
        </div>
      </Field>

      <div className="chip-row" style={{ marginTop: 10 }}>
        <Tag>Search is restricted to Australia by the backend geocoder.</Tag>
      </div>
    </div>
  );
}
