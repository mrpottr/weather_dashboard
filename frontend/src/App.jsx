import { Component, useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate, useLocation } from "react-router-dom";
import "./styles.css";
import { AppProviders, useAuth } from "./api/client.jsx";
import { IconSprite } from "./components/UI.jsx";
import Layout, { Splash, RequireAuth, RequireAdmin } from "./components/Layout.jsx";
import AuthPage from "./pages/page_auth.jsx";
import OverviewPage from "./pages/overview.jsx";
import ForecastPage from "./pages/forecast.jsx";
import InsightsPage from "./pages/insights.jsx";
import LocationsPage from "./pages/locations.jsx";
import HistoryPage from "./pages/history.jsx";
import ExportsPage from "./pages/exports.jsx";
import MapPage from "./pages/map.jsx";
import IntelligencePage from "./pages/intelligence.jsx";
import AlertsPage from "./pages/alerts.jsx";
import AccountPage from "./pages/account.jsx";
import AdminPage from "./pages/admin.jsx";
import UsersPage from "./pages/users.jsx";

/* Catches render errors that would otherwise produce a blank screen. */
class ErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { error: null };
  }
  static getDerivedStateFromError(error) {
    return { error };
  }
  componentDidCatch(error, info) {
    console.error("ATMOS render error:", error, info);
  }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 40, fontFamily: "monospace", color: "#f87171", background: "#0d1117", minHeight: "100dvh" }}>
          <h2 style={{ marginBottom: 16 }}>Something went wrong</h2>
          <pre style={{ whiteSpace: "pre-wrap", fontSize: 13 }}>{this.state.error.message}</pre>
          <button
            style={{ marginTop: 24, padding: "8px 18px", background: "#1e293b", color: "#fff", border: "1px solid #334155", borderRadius: 6, cursor: "pointer" }}
            onClick={() => this.setState({ error: null })}
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

/* Keeps the document title in sync with the route, like the prototype. */
function DocumentTitle() {
  const { pathname } = useLocation();
  useEffect(() => {
    const titles = {
      "/login": "Sign in",
      "/overview": "Overview",
      "/forecast": "Forecast",
      "/map": "Live map",
      "/intelligence": "Intelligence",
      "/insights": "Insights",
      "/locations": "Locations",
      "/history": "History",
      "/exports": "Exports",
      "/alerts": "Alerts",
      "/account": "Account",
      "/admin": "Admin",
      "/admin/users": "Admin · Users",
    };
    document.title = `${titles[pathname] || "ATMOS"} · ATMOS`;
  }, [pathname]);
  return null;
}

function AppRoutes() {
  const { booting, user } = useAuth();
  if (booting) return <Splash label="Restoring your session" />;
  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/overview" replace /> : <AuthPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/overview" replace />} />
          <Route path="/overview" element={<OverviewPage />} />
          <Route path="/forecast" element={<ForecastPage />} />
          <Route path="/map" element={<MapPage />} />
          <Route path="/intelligence" element={<IntelligencePage />} />
          <Route path="/insights" element={<InsightsPage />} />
          <Route path="/locations" element={<LocationsPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/exports" element={<ExportsPage />} />
          <Route path="/alerts" element={<AlertsPage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route element={<RequireAdmin />}>
            <Route path="/admin" element={<AdminPage />} />
            <Route path="/admin/users" element={<UsersPage />} />
          </Route>
        </Route>
      </Route>
      <Route path="*" element={<Navigate to={user ? "/overview" : "/login"} replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <ErrorBoundary>
      <AppProviders>
        <IconSprite />
        <BrowserRouter>
          <DocumentTitle />
          <AppRoutes />
        </BrowserRouter>
      </AppProviders>
    </ErrorBoundary>
  );
}

