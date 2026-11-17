import { useState } from "react";
import { FALLBACK_LOCATIONS, useAuth, useTheme } from "../api/client.jsx";
import { Icon } from "../components/UI.jsx";

const DEMO = {
  standard: { email: "user@weather.local", password: "user123" },
  admin: { email: "admin@weather.local", password: "admin123" },
};

export default function AuthPage() {
  const { login, register } = useAuth();
  const { theme, toggleTheme } = useTheme();

  const [mode, setMode] = useState("login"); // login | create
  const [role, setRole] = useState("standard");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const [form, setForm] = useState({
    first_name: "",
    last_name: "",
    email: "",
    password: "",
    confirm: "",
    default_location: FALLBACK_LOCATIONS[0].name,
  });

  const onLogin = async (e) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await login(email.trim(), password);
    } catch (err) {
      setError(err.message || "Sign-in failed. Please try again.");
    } finally {
      setBusy(false);
    }
  };

  const onCreate = async (e) => {
    e.preventDefault();
    setError("");
    if (form.password !== form.confirm) {
      setError("The two passwords do not match.");
      return;
    }
    if (form.password.length < 6) {
      setError("Choose a password of at least 6 characters.");
      return;
    }
    const base = FALLBACK_LOCATIONS.find((l) => l.name === form.default_location) || FALLBACK_LOCATIONS[0];
    setBusy(true);
    try {
      await register({
        first_name: form.first_name.trim(),
        last_name: form.last_name.trim(),
        email: form.email.trim(),
        password: form.password,
        default_location: base.name,
        latitude: base.latitude,
        longitude: base.longitude,
        timezone: base.timezone,
        admin_area: base.admin_area,
      });
    } catch (err) {
      setError(err.message || "The account could not be created.");
    } finally {
      setBusy(false);
    }
  };

  const fillDemo = (which) => {
    setMode("login");
    setRole(which);
    setEmail(DEMO[which].email);
    setPassword(DEMO[which].password);
    setError("");
  };

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

  return (
    <main className="auth-wrap">
      <section className="auth-card">
        <div
          className="auth-visual"
          style={{
            backgroundImage: `url('${mode === "login"
                ? "https://images.unsplash.com/photo-1534088568595-a066f410bcda?auto=format&fit=crop&w=2200&q=85"
                : "https://images.unsplash.com/photo-1485236715568-ddc5ee6ca227?auto=format&fit=crop&w=2200&q=85"
              }')`,
          }}
        >
          <div className="mini-brand">
            <span className="dot">
              <Icon name="cloud-sun" />
            </span>
            ATMOS // BOM
          </div>
          <div className="visual-quote">
            <div className="eyebrow">
              {mode === "login" ? "Forecast without friction" : "Personal weather workspace"}
            </div>
            <h2>
              {mode === "login"
                ? "See the weather before the weather sees you."
                : "Build a view that follows your day."}
            </h2>
            <p>
              {mode === "login"
                ? "Australian location search, live BOM forecast intelligence, alerts, saved locations, history, activity suitability and a full administrative control layer."
                : "Save places, compare historical conditions, export forecasts and keep weather intelligence one glance away."}
            </p>
            <div className="visual-chips">
              {(mode === "login"
                ? ["BOM forecast source", "Role-aware interface", "Cached-data ready"]
                : ["Saved locations", "Forecast history", "Outdoor ratings"]
              ).map((c) => (
                <span className="chip" key={c}>
                  {c}
                </span>
              ))}
            </div>
          </div>
        </div>

        {mode === "login" ? (
          <form className="auth-panel" onSubmit={onLogin}>
            <div className="eyebrow">Secure access</div>
            <h3>Welcome back.</h3>
            <p>Sign in to access your personalised weather workspace.</p>

            <div className="role-switch">
              <button
                type="button"
                className={role === "standard" ? "active" : ""}
                onClick={() => setRole("standard")}
              >
                Standard user
              </button>
              <button
                type="button"
                className={role === "admin" ? "active" : ""}
                onClick={() => setRole("admin")}
              >
                Administrator
              </button>
            </div>

            <div className="field">
              <label htmlFor="email">Email</label>
              <input
                id="email"
                className="input"
                type="email"
                placeholder="you@example.com"
                autoComplete="username"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className="field">
              <label htmlFor="password">Password</label>
              <div className="input-wrap">
                <input
                  id="password"
                  className="input"
                  type={showPassword ? "text" : "password"}
                  placeholder="Enter password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
                <button
                  className="password-eye"
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? "Hide password" : "Show password"}
                >
                  {showPassword ? "hide" : "show"}
                </button>
              </div>
            </div>
            <div className="error-note" role="alert">
              {error}
            </div>

            <button className="primary-btn" type="submit" disabled={busy}>
              {busy ? "Signing in…" : "Enter dashboard"}
            </button>

            <div className="demo-row">
              <button className="demo-btn" type="button" onClick={() => fillDemo("standard")}>
                Use standard demo
              </button>
              <button className="demo-btn" type="button" onClick={() => fillDemo("admin")}>
                Use admin demo
              </button>
            </div>
            <button
              className="back-link"
              type="button"
              onClick={() => {
                setMode("create");
                setError("");
              }}
            >
              New here? Create an account
            </button>
            <div style={{ marginTop: 14, textAlign: "center" }}>
              <button className="tiny-btn" type="button" onClick={(e) => toggleTheme(e.currentTarget)}>
                {theme === "light" ? "◐ Dark theme" : "◐ Light theme"}
              </button>
            </div>
          </form>
        ) : (
          <form className="auth-panel" onSubmit={onCreate}>
            <div className="eyebrow">Identity setup</div>
            <h3>Create account.</h3>
            <p>Your default location is saved to your favourites automatically.</p>
            <div className="form-grid">
              <div className="field">
                <label htmlFor="newFirst">First name</label>
                <input id="newFirst" className="input" placeholder="Minindu" value={form.first_name} onChange={set("first_name")} required />
              </div>
              <div className="field">
                <label htmlFor="newLast">Last name</label>
                <input id="newLast" className="input" placeholder="Baranasuriya" value={form.last_name} onChange={set("last_name")} required />
              </div>
              <div className="field full">
                <label htmlFor="newEmail">Email</label>
                <input id="newEmail" className="input" type="email" placeholder="you@example.com" value={form.email} onChange={set("email")} required />
              </div>
              <div className="field">
                <label htmlFor="newPass">Password</label>
                <input id="newPass" className="input" type="password" placeholder="Create password" value={form.password} onChange={set("password")} required />
              </div>
              <div className="field">
                <label htmlFor="newPass2">Confirm</label>
                <input id="newPass2" className="input" type="password" placeholder="Repeat password" value={form.confirm} onChange={set("confirm")} required />
              </div>
              <div className="field full">
                <label htmlFor="newLocation">Default Australian location</label>
                <select id="newLocation" className="select" value={form.default_location} onChange={set("default_location")}>
                  {FALLBACK_LOCATIONS.map((l) => (
                    <option key={l.name} value={l.name}>
                      {l.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            <div className="error-note" role="alert">
              {error}
            </div>
            <button className="primary-btn" type="submit" disabled={busy}>
              {busy ? "Creating…" : "Create workspace"}
            </button>
            <button
              className="back-link"
              type="button"
              onClick={() => {
                setMode("login");
                setError("");
              }}
            >
              ← Back to sign in
            </button>
          </form>
        )}
      </section>
    </main>
  );
}
