import {api} from "@/lib/http";
import {useResource} from "@/lib/requests";
import {ledgerSummary} from "@/schemas/ledger";
import {accountProblem} from "./account-helpers";
import {type AccountRow} from "@/features/pool/shared";

// 页头摘要：GET /admin/ledger/summary，只取 show_profit 与 totals，毛利字段由后端按 show_profit 返回。
export function useLedgerTotals(revision: number) {
  const state = useResource(() => api("/admin/ledger/summary", ledgerSummary), "ledger-summary", true, revision);
  return state?.value ? {showProfit: state.value.show_profit, totals: state.value.totals} : undefined;
}

// “需要处理”：账号自身有问题状态，或账本带有告警徽标。
export function needsAttention(row: AccountRow) {
  return Boolean(accountProblem(row) || (row.ledger?.alerts.length ?? 0) > 0);
}
