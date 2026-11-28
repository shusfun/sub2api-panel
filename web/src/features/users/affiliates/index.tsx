import {useMemo, useState} from "react";
import {useDebouncedValue} from "@/lib/requests";
import {affiliateInviteRow, affiliateRebateRow, affiliateTransferRow, affiliateUserRow} from "@/schemas";
import {EmptyState, FilterBar, RefreshButton, RemoteTable, SearchInput, Segmented, WorkspaceStage} from "@/ui";
import {inviteColumns, rebateColumns, transferColumns, userColumns} from "./affiliate-tables";
import {BatchRateBar} from "./batch-rate";

export {AffiliateUserPane} from "./affiliate-user-pane";

type View = "users" | "invites" | "rebates" | "transfers";
const views = [["users", "专属用户"], ["invites", "邀请关系"], ["rebates", "返利记录"], ["transfers", "转出记录"]] as const;

export function AffiliatesWorkspace({queryString = ""}: {queryString?: string}) {
  const initial = useMemo(() => new URLSearchParams(queryString).get("view"), [queryString]);
  const [view, setView] = useState<View>(() => views.some(([v]) => v === initial) ? initial as View : "users");
  const [search, setSearch] = useState("");
  const [revision, revise] = useState(0);
  const [selected, setSelected] = useState<number[]>([]);
  const debounced = useDebouncedValue(search.trim(), 240);
  const refresh = () => revise(v => v + 1);
  const path = (base: string) => `/admin/affiliates/${base}` + (debounced ? `?search=${encodeURIComponent(debounced)}` : "");
  const none = <EmptyState title="暂无记录" />;

  return (
    <WorkspaceStage flush>
      <FilterBar sticky>
        <div className="obs-filter-row">
          <Segmented label="推广视图" value={view} options={views} onChange={next => { setView(next); setSelected([]); }} />
          <SearchInput value={search} onChange={setSearch} placeholder="搜索邮箱、用户名或邀请码" ariaLabel="搜索推广记录" />
          <div className="obs-filter-spacer"><RefreshButton iconOnly size="default" variant="ghost" label="刷新推广记录" onClick={refresh} /></div>
        </div>
      </FilterBar>
      {view === "users" && <BatchRateBar userIDs={selected} onClear={() => setSelected([])} onDone={refresh} />}
      <div className="swift-pane-table">
        {view === "users" && <RemoteTable key={`u-${debounced}`} path={path("users")} schema={affiliateUserRow} columns={userColumns} rowID={row => row.user_id} label="推广专属用户" revision={revision} selection={{selected, onChange: setSelected}} empty={none} />}
        {view === "invites" && <RemoteTable key={`i-${debounced}`} path={path("invites")} schema={affiliateInviteRow} columns={inviteColumns} rowID={row => `${row.inviter_id}-${row.invitee_id}`} label="邀请关系" revision={revision} empty={none} />}
        {view === "rebates" && <RemoteTable key={`r-${debounced}`} path={path("rebates")} schema={affiliateRebateRow} columns={rebateColumns} rowID={row => `${row.out_trade_no}-${row.inviter_id}-${row.order_id ?? 0}`} label="返利记录" revision={revision} empty={none} />}
        {view === "transfers" && <RemoteTable key={`t-${debounced}`} path={path("transfers")} schema={affiliateTransferRow} columns={transferColumns} rowID={row => row.ledger_id} label="转出记录" revision={revision} empty={none} />}
      </div>
    </WorkspaceStage>
  );
}
