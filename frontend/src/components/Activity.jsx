import React from "react";
import { Tag } from "./UI.jsx";

/** Maps a rubric rating to the prototype's tag variant. */
export function ratingVariant(score) {
  if (score >= 65) return "success";
  if (score >= 50) return "warn";
  return "danger";
}

/** The circular score ring. `score` is 0..100. */
export function ActivityRing({ score = 0, label }) {
  const clamped = Math.max(0, Math.min(100, Math.round(score || 0)));
  return (
    <div className="ring" style={{ "--score": `${clamped}%` }}>
      <span>{label != null ? label : clamped}</span>
    </div>
  );
}

/**
 * ActivityDetail renders one ActivityScore in full: ring, headline, the
 * documented factors with their exact point contributions, the practical
 * advice list and the computed best outdoor window.
 */
export function ActivityDetail({ activity }) {
  if (!activity) return null;
  const {
    emoji,
    activity: name,
    score,
    rating,
    tag,
    title,
    summary,
    best_window: bestWindow,
    date,
    location,
  } = activity;

  // The backend sends `null` (not `[]`) for an empty factors/advice/best-hours
  // list, and destructuring defaults only cover `undefined`. Normalise here so
  // a sparse day can never crash the panel.
  const factors = activity.factors || [];
  const advice = activity.advice || [];
  const bestHours = activity.best_hours || [];

  const variant = ratingVariant(score);

  return (
    <div className="panel">
      <div className="panel-head">
        <div>
          <div className="panel-title">
            {emoji} {name} suitability
          </div>
          <div className="panel-kicker">
            {location}
            {date ? ` · ${date}` : ""}
          </div>
        </div>
        <Tag variant={variant}>{tag || rating?.toUpperCase()}</Tag>
      </div>

      <div className="suitability">
        <ActivityRing score={score} />
        <div>
          <strong>{title}</strong>
          <p style={{ fontSize: 11, color: "var(--muted)", lineHeight: 1.65, margin: "8px 0 0" }}>
            {summary}
          </p>
        </div>
      </div>

      <div className="factor-list">
        {factors.length === 0 ? (
          <div className="empty" style={{ padding: 16 }}>
            No factor pushed the score away from the 100-point base for this day.
          </div>
        ) : (
          factors.map((f, i) => (
            <div className="factor" key={i}>
              <div>
                <strong>
                  {f.factor} · {f.reading}
                </strong>
                <p>{f.reason}</p>
              </div>
              <span className={"f-impact " + (f.points > 0 ? "plus" : f.points < 0 ? "minus" : "neutral")}>
                {f.points > 0 ? "+" : ""}
                {f.points} pts
              </span>
            </div>
          ))
        )}
      </div>

      {advice.length > 0 ? (
        <>
          <div className="panel-title" style={{ marginTop: 16 }}>
            Practical advice
          </div>
          <ul className="advice-list">
            {advice.map((a, i) => (
              <li key={i}>{a}</li>
            ))}
          </ul>
        </>
      ) : null}

      <div style={{ marginTop: 14 }}>
        <div className="panel-title">Best outdoor window</div>
        <div className="best-window">
          <span className="window-chip">{bestWindow || "No comfortable window computed"}</span>
          {bestHours.slice(0, 12).map((h, i) => (
            <span className="window-chip" key={i}>
              {h}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * ActivityGrid renders every scored activity as a selectable card. The chosen
 * key is highlighted so the parent can drive an ActivityDetail panel.
 */
export function ActivityGrid({ activities = [], selected, onSelect }) {
  return (
    <div className="activity-grid">
      {activities.map((a) => {
        const isSelected = selected === a.activity_key;
        return (
          <button
            key={a.activity_key}
            className={"activity" + (isSelected ? " active" : "")}
            onClick={() => onSelect && onSelect(a.activity_key)}
            aria-pressed={isSelected}
          >
            <div className="emoji">{a.emoji}</div>
            <strong>{a.activity}</strong>
            <div className="score">
              {a.score != null && a.rating !== "No data" ? `${a.score}/100 · ${a.rating}` : "No data"}
            </div>
          </button>
        );
      })}
    </div>
  );
}

/**
 * SuitabilitySummary is the compact overview-panel version: ring + headline +
 * one-line copy, used on the Overview page next to the forecast strip.
 */
export function SuitabilitySummary({ activity, onManage }) {
  if (!activity) {
    return (
      <div className="empty" style={{ padding: 28 }}>
        Weather intelligence appears once a forecast is loaded.
      </div>
    );
  }
  return (
    <div className="suitability">
      <ActivityRing score={activity.score} />
      <div>
        <strong>{activity.title}</strong>
        <p style={{ fontSize: 11, color: "var(--muted)", lineHeight: 1.55, margin: "6px 0 0" }}>
          {activity.summary}
        </p>
        {onManage ? (
          <button className="tiny-btn" style={{ marginTop: 9 }} onClick={onManage}>
            Change activity
          </button>
        ) : null}
      </div>
    </div>
  );
}
