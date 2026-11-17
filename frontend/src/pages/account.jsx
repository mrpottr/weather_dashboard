import { useState } from "react";
import {
  FALLBACK_LOCATIONS,
  useAuth,
  useClock,
  useLocationContext,
  useTheme,
  useToast,
} from "../api/client.jsx";
import { Field, Panel, Tag, Toggle, useModal } from "../components/UI.jsx";

export default function AccountPage() {
  const { user, updateMe, deleteMe, updatePreferences } = useAuth();
  const { theme, setTheme } = useTheme();
  const { format, setClockFormat } = useClock();
  const { setLocation, meta } = useLocationContext();
  const toast = useToast();
  const { openModal, closeModal } = useModal();

  const [form, setForm] = useState({
    first_name: user?.first_name || "",
    last_name: user?.last_name || "",
    email: user?.email || "",
    default_location: user?.default_location || FALLBACK_LOCATIONS[0].name,
    current_password: "",
    password: "",
    confirm: "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }));

  const saveProfile = async (e) => {
    e.preventDefault();
    setError("");
    setOk("");
    const payload = {
      first_name: form.first_name.trim(),
      last_name: form.last_name.trim(),
      email: form.email.trim(),
      default_location: form.default_location,
    };
    if (form.password) {
      if (form.password !== form.confirm) {
        setError("The two new passwords do not match.");
        return;
      }
      payload.current_password = form.current_password;
      payload.password = form.password;
    }
    setSaving(true);
    try {
      const me = await updateMe(payload);
      const chosen = (meta || FALLBACK_LOCATIONS).find((l) => l.name === me.default_location);
      if (chosen) setLocation(chosen);
      setForm((f) => ({ ...f, current_password: "", password: "", confirm: "" }));
      setOk("Your account details were saved.");
      toast("Account updated", me.email, "✓");
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = () => {
    openModal(
      "Delete this account?",
      <div>
        <p style={{ fontSize: 11, color: "var(--muted)", lineHeight: 1.7, marginTop: 0 }}>
          Your account, saved locations and preferences are removed from the database immediately. Retrieval
          history that has already been written to the audit files is retained for reporting. This cannot be
          undone.
        </p>
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 12 }}>
          <button className="ghost-btn" onClick={closeModal}>
            Keep my account
          </button>
          <button
            className="danger-btn"
            onClick={async () => {
              try {
                await deleteMe();
                closeModal();
                toast("Account deleted", "Your workspace has been removed.", "✓");
              } catch (err) {
                closeModal();
                toast("Could not delete account", err.message, "!");
              }
            }}
          >
            Delete permanently
          </button>
        </div>
      </div>
    );
  };

  return (
    <section className="page active">
      <div className="page-title">Account centre</div>
      <p className="page-sub">Update your profile, credentials and interface preferences.</p>

      <div className="grid g2">
        <Panel title="Profile" kicker="Name, email and default location">
          <form onSubmit={saveProfile}>
            <div className="form-grid">
              <Field label="First name" htmlFor="accFirst">
                <input id="accFirst" className="input" value={form.first_name} onChange={set("first_name")} required />
              </Field>
              <Field label="Last name" htmlFor="accLast">
                <input id="accLast" className="input" value={form.last_name} onChange={set("last_name")} required />
              </Field>
              <div className="full">
                <Field label="Email" htmlFor="accEmail">
                  <input id="accEmail" className="input" type="email" value={form.email} onChange={set("email")} required />
                </Field>
              </div>
              <div className="full">
                <Field label="Default Australian location" htmlFor="accLocation">
                  <select id="accLocation" className="select" value={form.default_location} onChange={set("default_location")}>
                    {(meta || FALLBACK_LOCATIONS).map((l) => (
                      <option key={l.name} value={l.name}>
                        {l.name}
                      </option>
                    ))}
                  </select>
                </Field>
              </div>
            </div>

            <div className="panel-title" style={{ marginTop: 6 }}>
              Change password
            </div>
            <div className="form-grid">
              <div className="full">
                <Field label="Current password" htmlFor="accCur">
                  <input id="accCur" className="input" type="password" value={form.current_password} onChange={set("current_password")} placeholder="Required to set a new password" />
                </Field>
              </div>
              <Field label="New password" htmlFor="accNew">
                <input id="accNew" className="input" type="password" value={form.password} onChange={set("password")} />
              </Field>
              <Field label="Confirm new password" htmlFor="accNew2">
                <input id="accNew2" className="input" type="password" value={form.confirm} onChange={set("confirm")} />
              </Field>
            </div>

            <div className="error-note" role="alert">
              {error}
            </div>
            <div className="success-note" role="status">
              {ok}
            </div>

            <button className="primary-btn" type="submit" disabled={saving}>
              {saving ? "Saving…" : "Save changes"}
            </button>
          </form>
        </Panel>

        <div>
          <Panel title="Account record" kicker="Server-side values for this session">
            <div className="location-card" style={{ marginBottom: 10 }}>
              <div className="left">
                <span className="avatar">{((user?.first_name || "?")[0] + (user?.last_name || "")[0]).toUpperCase()}</span>
                <div>
                  <strong>
                    {user?.first_name} {user?.last_name}
                  </strong>
                  <small>{user?.email}</small>
                </div>
              </div>
              <Tag variant={user?.role === "admin" ? "accent" : ""}>
                {(user?.role || "standard").toUpperCase()}
              </Tag>
            </div>
            <div className="chip-row">
              <Tag variant={user?.status === "active" ? "success" : "danger"}>
                {(user?.status || "active").toUpperCase()}
              </Tag>
              <Tag>Default · {user?.default_location || "not set"}</Tag>
              <Tag>
                Created {user?.created_at ? new Date(user.created_at).toLocaleDateString("en-AU") : "—"}
              </Tag>
            </div>
            <p className="panel-kicker" style={{ marginTop: 12, lineHeight: 1.7 }}>
              Sessions use a signed JSON Web Token (24-hour expiry). Changing your password does not invalidate
              the current token; signing out clears it locally.
            </p>
          </Panel>

          <Panel className="section-gap" title="Preferences" kicker="Stored per user in the database">
            <Toggle
              label="Light theme"
              hint="Switch between the dark aurora theme and the light theme with an animated reveal."
              checked={theme === "light"}
              onChange={(v) => setTheme(v ? "light" : "dark")}
            />
            <Toggle
              label="24-hour clock"
              hint="Applies to the sidebar clock, forecast hours, sunrise and sunset."
              checked={format === "24h"}
              onChange={(v) => setClockFormat(v ? "24h" : "12h")}
            />
            <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <button
                className="tiny-btn"
                onClick={async () => {
                  const r = await updatePreferences({ theme, clock_format: format });
                  toast(r ? "Preferences saved" : "Preferences not saved", r ? "Stored on your account." : "Try again while signed in.", r ? "✓" : "!");
                }}
              >
                Save preferences now
              </button>
              <Tag>Theme and clock also save automatically when changed.</Tag>
            </div>
          </Panel>

          <Panel className="section-gap" title="Danger zone" kicker="Delete your account and stored locations">
            <p className="panel-kicker" style={{ lineHeight: 1.7 }}>
              Deletion removes your account row, saved locations and preferences. It cannot be reversed.
            </p>
            <button className="danger-btn" style={{ marginTop: 12 }} onClick={confirmDelete}>
              Delete my account
            </button>
          </Panel>
        </div>
      </div>
    </section>
  );
}
