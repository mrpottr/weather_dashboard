import { useEffect, useMemo, useRef, useState } from "react";
import { api, saveBlob, useAsync, useAuth, useToast } from "../api/client.jsx";
import { AsyncPanel, Icon, Panel, Tag } from "../components/UI.jsx";

const ROLE_FILTERS = [
  { value: "", label: "All roles" },
  { value: "standard", label: "Standard" },
  { value: "admin", label: "Admin" },
];

const STATUS_FILTERS = [
  { value: "", label: "All statuses" },
  { value: "active", label: "Active" },
  { value: "suspended", label: "Suspended" },
  { value: "deactivated", label: "Deactivated" },
];

/** Human label for a backend status value. */
const statusLabel = (s) => {
  const v = (s || "").toLowerCase();
  if (v === "active") return "Active";
  if (v === "suspended") return "Suspended";
  if (v === "deactivated") return "Deactivated";
  return s || "Unknown";
};

/** Tag variant conveying the account state at a glance. */
const statusVariant = (s) => {
  const v = (s || "").toLowerCase();
  if (v === "active") return "success";
  if (v === "suspended") return "warn";
  return "danger";
};

/** Two-letter initials for the avatar bubble. */
function initials(first, last, email) {
  const a = (first || "").trim();
  const b = (last || "").trim();
  if (a || b) return `${a[0] || ""}${b[0] || ""}`.toUpperCase() || "?";
  return (email || "?").slice(0, 2).toUpperCase();
}

export default function UsersPage() {
  const { user } = useAuth();
  const toast = useToast();

  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [role, setRole] = useState("");
  const [status, setStatus] = useState("");
  const timer = useRef(0);

  // Debounce the free-text search so every keystroke does not hit the API.
  useEffect(() => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setDebounced(query.trim()), 300);
    return () => window.clearTimeout(timer.current);
  }, [query]);

  const users = useAsync(
    () => api.adminUsers({ q: debounced || undefined, role: role || undefined, status: status || undefined }),
    [debounced, role, status]
  );

  const rows = users.data?.users || [];
  const filtered = debounced || role || status;

  const [pendingId, setPendingId] = useState(null);

  /** Applies a status change through the API and updates the table in place. */
  const changeStatus = async (row, next) => {
    setPendingId(row.id);
    try {
      const res = await api.adminUpdateUserStatus(row.id, next);
      const saved = res?.user || { ...row, status: next };
      // Update the visible table immediately, then reconcile with the server.
      users.setData((d) => ({
        ...(d || {}),
        users: (d?.users || []).map((u) =>
          u.id === row.id ? { ...u, status: saved.status ?? next } : u
        ),
      }));
      toast(
        `Account ${statusLabel(next).toLowerCase()}`,
        `${row.first_name} ${row.last_name} · ${row.email}`,
        "✓"
      );
      users.reload();
    } catch (e) {
      toast("Could not update the account", e.message, "!");
    } finally {
      setPendingId(null);
    }
  };

  const exportUsers = async () => {
    try {
      const res = await api.adminUsersExport();
      const name = await saveBlob(res, "atmos-users.csv");
      toast("User export ready", name, "⤓");
    } catch (e) {
      toast("User export failed", e.message, "!");
    }
  };

  // Admin totals summarise the currently visible set, so the header always
  // reflects exactly what the filters returned.
  const summary = useMemo(() => {
    let active = 0;
    let admins = 0;
    for (const u of rows) {
      if ((u.status || "").toLowerCase() === "active") active += 1;
      if ((u.role || "").toLowerCase() === "admin") admins += 1;
    }
    return { total: rows.length, active, admins };
  }, [rows]);

  return (
    <section className="page active">
      <div className="page-title">User management</div>
      <p className="page-sub">Account administration and role-aware access control.</p>

      <div className="panel">
        <div className="toolbar">
          <div className="searchbox">
            <Icon name="search" />
            <input
              className="input"
              placeholder="Search by name or email"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Search users by name or email"
            />
          </div>
          <select
            className="select"
            style={{ width: "auto" }}
            value={role}
            onChange={(e) => setRole(e.target.value)}
            aria-label="Filter by role"
          >
            {ROLE_FILTERS.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </select>
          <select
            className="select"
            style={{ width: "auto" }}
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            aria-label="Filter by status"
          >
            {STATUS_FILTERS.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
          <button className="ghost-btn" onClick={exportUsers}>
            <Icon name="download" size={14} />
            Export users
          </button>
        </div>
      </div>

      <Panel
        className="section-gap"
        title="Account directory"
        kicker="Every registered account, filterable by name, role and status"
        right={
          <div className="chip-row">
            <Tag>{summary.total} shown</Tag>
            <Tag variant="success">{summary.active} active</Tag>
            <Tag variant="warn">{summary.admins} admin</Tag>
          </div>
        }
      >
        <AsyncPanel
          loading={users.loading}
          error={users.error}
          onRetry={users.reload}
          empty={!users.loading && !users.error && rows.length === 0}
          emptyLabel={
            filtered
              ? "No accounts match the current search and filters."
              : "No user accounts were found."
          }
        >
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>User</th>
                  <th>Role</th>
                  <th>Location</th>
                  <th>Status</th>
                  <th>Last active</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((u) => {
                  const name = `${u.first_name} ${u.last_name}`.trim() || u.email;
                  const isSelf = user?.id === u.id;
                  return (
                    <tr key={u.id}>
                      <td>
                        <div className="user-row">
                          <div className="avatar" aria-hidden="true">
                            {initials(u.first_name, u.last_name, u.email)}
                          </div>
                          <div>
                            <strong>{name}</strong>
                            <div style={{ fontSize: 9, color: "var(--muted)", marginTop: 2 }}>
                              {u.email}
                              {isSelf ? " · you" : ""}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td>
                        <Tag variant={(u.role || "").toLowerCase() === "admin" ? "warn" : ""}>
                          {u.role === "admin" ? "Admin" : "Standard"}
                        </Tag>
                      </td>
                      <td>{u.location || "Not set"}</td>
                      <td>
                        <Tag variant={statusVariant(u.status)}>{statusLabel(u.status)}</Tag>
                      </td>
                      <td>{u.last_active || "never"}</td>
                      <td>
                        <ManageMenu
                          row={u}
                          isSelf={isSelf}
                          busy={pendingId === u.id}
                          onChange={(next) => changeStatus(u, next)}
                        />
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p className="panel-kicker" style={{ marginTop: 12, lineHeight: 1.7 }}>
            Suspended and deactivated accounts are blocked from signing in. Changing a role is reserved for the
            backend; status changes take effect immediately and are recorded in the system log.
          </p>
        </AsyncPanel>
      </Panel>
    </section>
  );
}

/**
 * Per-row Manage dropdown. Offers the transitions that make sense for the
 * account's current status, and explains why the signed-in administrator's own
 * account is locked.
 */
function ManageMenu({ row, isSelf, busy, onChange }) {
  const [open, setOpen] = useState(false);
  const wrap = useRef(null);

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (wrap.current && !wrap.current.contains(e.target)) setOpen(false);
    };
    const onKey = (e) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const current = (row.status || "active").toLowerCase();
  const options = [];
  if (current !== "active") options.push({ status: "active", label: "Activate", danger: false });
  if (current !== "suspended") options.push({ status: "suspended", label: "Suspend", danger: true });
  if (current !== "deactivated") options.push({ status: "deactivated", label: "Deactivate", danger: true });

  return (
    <div className="manage-wrap" ref={wrap}>
      <button
        className="tiny-btn"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={busy}
      >
        {busy ? "Saving…" : "Manage"}
      </button>
      {open ? (
        <div className="manage-menu" role="menu">
          {isSelf ? (
            <div className="menu-note">You cannot change the status of your own account.</div>
          ) : (
            options.map((o) => (
              <button
                key={o.status}
                type="button"
                role="menuitem"
                className={o.danger ? "danger-item" : ""}
                onClick={() => {
                  setOpen(false);
                  onChange(o.status);
                }}
              >
                {o.label}
              </button>
            ))
          )}
        </div>
      ) : null}
    </div>
  );
}
