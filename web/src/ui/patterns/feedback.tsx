import {Component, type ErrorInfo, type ReactNode} from "react";
import {AlertTriangle, Inbox} from "lucide-react";
import {cn} from "cn";
import {Button} from "../primitives/button";

export function Feedback({children, tone = "neutral"}: {children: ReactNode; tone?: "neutral" | "error" | "success"}) {
  return <p className="feedback" data-tone={tone} role={tone === "error" ? "alert" : "status"}>{children}</p>;
}

export function ErrorLine({value}: {value: string}) {
  return value ? <Feedback tone="error">{value}</Feedback> : null;
}

export function Skeleton({className, height}: {className?: string; height?: number | string}) {
  return <div className={cn("obs-skeleton", className)} style={height !== undefined ? {height} : undefined} aria-hidden />;
}

export function StatsSkeleton({count = 5}: {count?: number}) {
  return (
    <div className="obs-metrics-grid" role="status" aria-label="正在加载指标">
      {Array.from({length: count}, (_, i) => (
        <div key={i} className="obs-metric">
          <Skeleton className="obs-skeleton-metric-label" />
          <Skeleton className="obs-skeleton-metric-value" />
          <Skeleton className="obs-skeleton-metric-detail" />
        </div>
      ))}
    </div>
  );
}

export function ChartSkeleton({height = "var(--weir-skeleton-chart)"}: {height?: number | string}) {
  return (
    <div className="obs-panel" role="status" aria-label="正在加载图表">
      <div className="obs-panel-body">
        <div className="obs-skeleton-chart-head">
          <Skeleton className="obs-skeleton-chart-title" />
          <Skeleton className="obs-skeleton-chart-legend" />
        </div>
        <Skeleton height={height} className="w-full" />
      </div>
    </div>
  );
}

export function TableSkeleton({rows = 5}: {rows?: number}) {
  return (
    <div className="obs-table-skeleton" role="status" aria-label="正在加载列表">
      <Skeleton className="obs-skeleton-table-head" />
      {Array.from({length: rows}, (_, i) => <Skeleton key={i} className="obs-skeleton-table-row" />)}
    </div>
  );
}

export function EmptyState({
  title = "暂无匹配数据",
  description,
  action,
  icon,
}: {
  title?: string;
  description?: string;
  action?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <div className="obs-empty" role="status">
      <div className="obs-empty-icon" aria-hidden>{icon ?? <Inbox />}</div>
      <div className="obs-empty-title">{title}</div>
      {description && <p className="obs-empty-desc">{description}</p>}
      {action && <div className="obs-empty-action">{action}</div>}
    </div>
  );
}

export function Resource<T>({state, fallback, children}: {
  state?: {value?: T; error?: string; loading?: boolean; stale?: boolean};
  fallback?: ReactNode;
  children: (value: T, meta: {stale: boolean; loading: boolean}) => ReactNode;
}) {
  if (state?.error && state?.value === undefined) return <ErrorLine value={state.error} />;
  if (state?.value === undefined) return fallback ?? <TableSkeleton rows={3} />;
  return <>
    {state.error && <ErrorLine value={state.error} />}
    {children(state.value, {stale: !!state.stale, loading: !!state.loading})}
  </>;
}

type BoundaryProps = {
  resetKey?: string | number;
  children: ReactNode;
};

type BoundaryState = {
  failed: boolean;
  resetKey?: string | number;
};

export class WorkspaceErrorBoundary extends Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = {failed: false, resetKey: this.props.resetKey};

  static getDerivedStateFromError(): Partial<BoundaryState> {
    return {failed: true};
  }

  static getDerivedStateFromProps(props: BoundaryProps, state: BoundaryState): Partial<BoundaryState> | null {
    if (props.resetKey !== state.resetKey) {
      return {failed: false, resetKey: props.resetKey};
    }
    return null;
  }

  componentDidCatch(_error: Error, _info: ErrorInfo) {
    // 保留侧栏与顶栏外壳，不将原始堆栈暴露给终端用户。
  }

  render() {
    if (this.state.failed) {
      return (
        <EmptyState
          icon={<AlertTriangle />}
          title="当前视图加载遇到异常"
          description="可能是热更新或异步资源已更新，点击下方按钮即可恢复。"
          action={
            <Button variant="outline" size="sm" onClick={() => this.setState({failed: false})}>
              重新加载视图
            </Button>
          }
        />
      );
    }
    return this.props.children;
  }
}
