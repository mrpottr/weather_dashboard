import React, { useId, useMemo } from "react";

const WIDTH = 760;
const HEIGHT = 230;
const PAD_L = 34;
const PAD_R = 12;
const PAD_T = 18;
const PAD_B = 26;

/**
 * LineChart draws the temperature (or any numeric) trend with an area fill,
 * point markers, grid lines and axis labels.
 *
 * values: number[]  labels: string[] (shown on the x axis)
 */
export default function LineChart({ values = [], labels = [], ariaLabel = "Temperature trend" }) {
  const uid = useId().replace(/:/g, "");
  const gradId = `fill-${uid}`;

  const model = useMemo(() => {
    const clean = values.filter((v) => typeof v === "number" && Number.isFinite(v));
    if (clean.length < 2) return null;
    const min = Math.min(...clean) - 2;
    const max = Math.max(...clean) + 2;
    const span = max - min || 1;
    const pts = clean.map((v, i) => {
      const x = PAD_L + (i * (WIDTH - PAD_L - PAD_R)) / (clean.length - 1);
      const y = PAD_T + ((max - v) * (HEIGHT - PAD_T - PAD_B)) / span;
      return [x, y];
    });
    const line = pts.map((p) => p.map((n) => n.toFixed(2)).join(",")).join(" ");
    const area = `${pts[0][0].toFixed(2)},${HEIGHT - PAD_B} ${line} ${pts[pts.length - 1][0].toFixed(2)},${HEIGHT - PAD_B}`;
    const grid = [0, 1, 2, 3].map((i) => PAD_T + (i * (HEIGHT - PAD_T - PAD_B)) / 3);
    const ticks = (labels.length ? labels : clean.map((_, i) => String(i))).map((l, i, arr) => {
      const x = PAD_L + (i * (WIDTH - PAD_L - PAD_R)) / Math.max(1, arr.length - 1);
      return { x, label: l };
    });
    return { pts, line, area, grid, ticks, min: clean[0], max: clean[clean.length - 1] };
  }, [values, labels]);

  if (!model) {
    return (
      <div className="chart-wrap">
        <div className="empty" style={{ padding: "60px 10px" }}>
          Not enough hourly data to draw a trend yet.
        </div>
      </div>
    );
  }

  return (
    <div className="chart-wrap">
      <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} className="chart-svg" role="img" aria-label={ariaLabel}>
        <defs>
          <linearGradient id={gradId} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="var(--chart-line)" stopOpacity=".25" />
            <stop offset="100%" stopColor="var(--chart-line)" stopOpacity="0" />
          </linearGradient>
        </defs>
        {model.grid.map((y, i) => (
          <line key={i} x1={PAD_L} y1={y} x2={WIDTH - PAD_R} y2={y} className="chart-grid-line" />
        ))}
        <polygon points={model.area} fill={`url(#${gradId})`} opacity=".85" />
        <polyline points={model.line} className="chart-line" />
        {model.pts.map((p, i) => (
          <circle key={i} cx={p[0]} cy={p[1]} r="3.4" className="chart-point" />
        ))}
        {model.ticks.map((t, i) => (
          <text key={i} x={t.x} y={HEIGHT - 7} textAnchor="middle" className="axis">
            {t.label}
          </text>
        ))}
      </svg>
    </div>
  );
}
