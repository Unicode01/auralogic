import React from 'react';

export default function PageHeader({
  eyebrow,
  title,
  description,
  meta = [],
  actions = null,
}) {
  const visibleMeta = Array.isArray(meta) ? meta.filter((item) => item && item.label && item.value !== undefined && item.value !== null && item.value !== '') : [];

  return (
    <div className="market-page-header">
      <div className="market-page-header-copy">
        {eyebrow ? <div className="market-page-eyebrow">{eyebrow}</div> : null}
        <h1 className="market-page-title">{title}</h1>
        {description ? <p className="market-page-description">{description}</p> : null}
        {visibleMeta.length > 0 ? (
          <div className="market-page-meta">
            {visibleMeta.map((item) => (
              <div key={`${item.label}-${String(item.value)}`} className="market-meta-pill">
                <span className="market-meta-pill-label">{item.label}</span>
                <span>{item.value}</span>
              </div>
            ))}
          </div>
        ) : null}
      </div>
      {actions ? <div className="market-page-actions">{actions}</div> : null}
    </div>
  );
}
