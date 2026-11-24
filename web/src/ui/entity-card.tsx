import {Fragment, type ReactNode} from "react";
import {Checkbox} from "./checkbox";
import {EllipsisText} from "./tooltip";

export type EntityCardResult = {tone: "ok" | "bad"; text: string};

export function EntityMetrics({items}: {items: readonly {label: string; value: ReactNode}[]}) {
  return <div className="entity-card-metrics">
    {items.map(item => <div key={item.label} className="entity-card-metric">
      <EllipsisText text={item.label} className="entity-card-metric-label" />
      {typeof item.value === "string"
        ? <EllipsisText text={item.value} className="entity-card-metric-val" />
        : <strong>{item.value}</strong>}
    </div>)}
  </div>;
}

export function EntityMeters({items}: {items: readonly {label: string; bar: ReactNode; reset?: string}[]}) {
  return <div className="entity-card-meters" role={items.length > 3 ? "region" : undefined}
    tabIndex={items.length > 3 ? 0 : undefined} aria-label={items.length > 3 ? "配额水位" : undefined}>
    {items.map(item => <div key={item.label} className="entity-card-meter">
      <EllipsisText text={item.label} className="entity-card-meter-label" />
      {item.bar}
      {item.reset && <EllipsisText text={item.reset} className="entity-card-meter-reset" />}
    </div>)}
  </div>;
}

export function EntityResult({result, fallback}: {result?: EntityCardResult; fallback?: string}) {
  const text = result?.text ?? fallback ?? "";
  return (
    <div className="entity-card-result" data-tone={result?.tone} role={result?.tone === "bad" ? "alert" : "status"}>
      {text ? <EllipsisText text={text} /> : null}
    </div>
  );
}

// 卡片内嵌微胶囊切换（配额/用量、今日/7d/30d 等局部视角切换），不抢工具栏主控件视觉权重。
export function MiniPills<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly (readonly [T, ReactNode])[];
  onChange: (next: T) => void;
}) {
  return (
    <div className="entity-card-pills" role="group" aria-label={label} onClick={event => event.stopPropagation()}>
      {options.map(([optionValue, optionLabel]) => {
        const selected = optionValue === value;
        return (
          <button
            key={optionValue}
            type="button"
            aria-pressed={selected}
            data-selected={selected || undefined}
            className="entity-card-pill"
            onClick={() => onChange(optionValue)}
          >
            {optionLabel}
          </button>
        );
      })}
    </div>
  );
}

// 通用对象卡片：头部（勾选、状态、名称、开关）→ 徽章行 → 行内异常恢复条 → 可切换遥测主视区 → 单行紧凑元信息 → 反馈位 → 底部操作。
// 整张可点打开速览；头部控件、遥测切换、元信息交互与底部操作各自拦截点击。
export function EntityCard({
  name, lead, toggle, selected, onSelect, active, dim, onOpen,
  badges, banner, bannerTone, panelHead, panelBody, panelSize, metrics, meters, meta, relations, result, reserveResult, actions, more,
}: {
  name: string;
  lead?: ReactNode;
  toggle?: ReactNode;
  selected?: boolean;
  onSelect?: (next: boolean) => void;
  active?: boolean;
  dim?: boolean;
  onOpen: () => void;
  badges?: ReactNode;
  banner?: ReactNode;
  bannerTone?: "bad" | "warn";
  panelHead?: ReactNode;
  panelBody?: ReactNode;
  panelSize?: "quota" | "summary";
  metrics?: readonly {label: string; value: ReactNode}[];
  meters?: readonly {label: string; bar: ReactNode; reset?: string}[];
  meta?: readonly ReactNode[];
  relations?: ReactNode;
  result?: EntityCardResult;
  reserveResult?: boolean;
  actions?: ReactNode;
  more?: ReactNode;
}) {
  return (
    <article
      className="entity-card"
      data-active={active || undefined}
      data-dim={dim || undefined}
      tabIndex={0}
      aria-label={name}
      onClick={onOpen}
      onKeyDown={event => {
        if (event.target === event.currentTarget && (event.key === "Enter" || event.key === " ")) {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <header className="entity-card-head" onClick={event => event.stopPropagation()}>
        {onSelect && <Checkbox checked={!!selected} onChange={onSelect} label={`选择 ${name}`} />}
        {lead}
        <EllipsisText text={name} className="entity-card-name" onClick={onOpen} />
        {toggle}
      </header>
      {badges && <div className="entity-card-badges">{badges}</div>}
      {banner && <div className="entity-card-banner" data-tone={bannerTone ?? "neutral"} onClick={event => event.stopPropagation()}>{banner}</div>}
      {(panelHead || panelBody) && (
        <div className="entity-card-well" data-size={panelSize} onClick={event => event.stopPropagation()}>
          {panelHead && <div className="entity-card-well-head">{panelHead}</div>}
          {panelBody && <div className="entity-card-well-body">{panelBody}</div>}
        </div>
      )}
      {metrics && <EntityMetrics items={metrics} />}
      {meters && <EntityMeters items={meters} />}
      {meta && meta.length > 0 && (
        <div className="entity-card-meta" onClick={event => event.stopPropagation()}>
          {meta.map((item, idx) => (
            <Fragment key={idx}>
              {idx > 0 && <span className="entity-card-meta-sep" aria-hidden>·</span>}
              <span className="entity-card-meta-item">{item}</span>
            </Fragment>
          ))}
        </div>
      )}
      {relations && <div className="entity-card-relations" onClick={event => event.stopPropagation()}>{relations}</div>}
      {(result || reserveResult) && <EntityResult result={result} />}
      {(actions || more) && (
        <footer className="entity-card-foot" onClick={event => event.stopPropagation()}>
          {actions}
          <span className="entity-card-foot-spacer" />
          {more}
        </footer>
      )}
    </article>
  );
}

export function EntityGrid({children, label}: {children: ReactNode; label: string}) {
  return <div className="entity-grid" role="list" aria-label={label}>{children}</div>;
}
