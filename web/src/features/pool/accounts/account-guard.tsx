import {useState} from "react";
import {api} from "@/lib/http";
import {account, page, tlsProfileSchema} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Button,
  ErrorLine,
  Field,
  Form,
  FormActions,
  FormGrid,
  FormSection,
  Input,
  InsetRow,
  Select,
  Switch,
  Textarea,
} from "@/ui";
import {type AccountRow} from "@/features/pool/shared";

export function AccountGuardForm({
  acc,
  onSaved,
}: {
  acc: AccountRow;
  onSaved: () => void;
}) {
  const save = useAction();
  const extra = acc.extra;
  const [windowCostLimit, setWindowCostLimit] = useState(extra?.window_cost_limit != null ? String(extra.window_cost_limit) : "");
  const [windowCostStickyReserve, setWindowCostStickyReserve] = useState(extra?.window_cost_sticky_reserve != null ? String(extra.window_cost_sticky_reserve) : "");
  const [dailyQuota, setDailyQuota] = useState(extra?.daily_quota != null ? String(extra.daily_quota) : "");
  const [totalQuota, setTotalQuota] = useState(extra?.total_quota != null ? String(extra.total_quota) : "");
  const [maxSessions, setMaxSessions] = useState(extra?.max_sessions != null ? String(extra.max_sessions) : "");
  const [rpmLimit, setRpmLimit] = useState(extra?.rpm_limit != null ? String(extra.rpm_limit) : "");
  const [codexCLIOnly, setCodexCLIOnly] = useState(extra?.codex_cli_only ?? false);
  const [tlsFingerprint, setTlsFingerprint] = useState(extra?.tls_fingerprint ?? false);
  const [tlsProfileID, setTlsProfileID] = useState(extra?.tls_fingerprint_profile_id != null ? String(extra.tls_fingerprint_profile_id) : "");
  const profiles = useResource(() => api("/admin/tls-fingerprint-profiles", page(tlsProfileSchema)), "tls-profiles-select", true);
  const [interceptWarmup, setInterceptWarmup] = useState(extra?.intercept_warmup ?? false);
  const [allowlistText, setAllowlistText] = useState((extra?.model_allowlist ?? []).join("\n"));
  const [expiresDate, setExpiresDate] = useState(acc.expires_at ? new Date(acc.expires_at * 1000).toISOString().slice(0, 10) : "");
  const [autoPauseOnExpired, setAutoPauseOnExpired] = useState(acc.auto_pause_on_expired);

  return (
    <Form
      busy={save.busy}
      onSubmit={() => void save.run(async () => {
        const expEpoch = expiresDate ? Math.floor(new Date(expiresDate + "T23:59:59Z").getTime() / 1000) : undefined;
        const allowlist = allowlistText.split(/\r?\n/).map(s => s.trim()).filter(Boolean);
        await api(`/admin/accounts/${acc.id}`, account, "PUT", {
          ...(expEpoch ? {expires_at: expEpoch} : acc.expires_at ? {clear_expires: true} : {}),
          auto_pause_on_expired: autoPauseOnExpired,
          extra: {
            ...(windowCostLimit.trim() !== "" ? {window_cost_limit: Number(windowCostLimit)} : {}),
            ...(windowCostStickyReserve.trim() !== "" ? {window_cost_sticky_reserve: Number(windowCostStickyReserve)} : {}),
            ...(dailyQuota.trim() !== "" ? {daily_quota: Number(dailyQuota)} : {}),
            ...(totalQuota.trim() !== "" ? {total_quota: Number(totalQuota)} : {}),
            ...(maxSessions.trim() !== "" ? {max_sessions: Number(maxSessions)} : {}),
            ...(rpmLimit.trim() !== "" ? {rpm_limit: Number(rpmLimit)} : {}),
            codex_cli_only: codexCLIOnly,
            tls_fingerprint: tlsFingerprint,
            tls_fingerprint_profile_id: tlsFingerprint && tlsProfileID ? Number(tlsProfileID) : null,
            intercept_warmup: interceptWarmup,
            model_allowlist: allowlist,
          },
        });
        onSaved();
      })}
    >
      <FormSection title="成本限额与会话控量">
        <FormGrid>
          <Field label="窗口成本上限 (USD)"><Input type="number" min="0" step="0.01" value={windowCostLimit} onChange={e => setWindowCostLimit(e.target.value)} placeholder="不限" /></Field>
          <Field label="粘性会话预留 (USD)"><Input type="number" min="0" step="0.01" value={windowCostStickyReserve} onChange={e => setWindowCostStickyReserve(e.target.value)} placeholder="0" /></Field>
          <Field label="每日配额上限 (USD)"><Input type="number" min="0" step="0.01" value={dailyQuota} onChange={e => setDailyQuota(e.target.value)} placeholder="不限" /></Field>
          <Field label="累计配额上限 (USD)"><Input type="number" min="0" step="0.01" value={totalQuota} onChange={e => setTotalQuota(e.target.value)} placeholder="不限" /></Field>
          <Field label="最大并发会话数 (max_sessions)"><Input type="number" min="0" value={maxSessions} onChange={e => setMaxSessions(e.target.value)} placeholder="不限" /></Field>
          <Field label="单号 RPM 限制"><Input type="number" min="0" value={rpmLimit} onChange={e => setRpmLimit(e.target.value)} placeholder="不限" /></Field>
        </FormGrid>
      </FormSection>

      <FormSection title="防封特征与模型白名单">
        <InsetRow label="仅允许 Codex CLI 流量"><Switch label="仅 Codex CLI" checked={codexCLIOnly} onChange={setCodexCLIOnly} /></InsetRow>
        <InsetRow label="启用 TLS 指纹伪装"><Switch label="TLS 指纹伪装" checked={tlsFingerprint} onChange={setTlsFingerprint} /></InsetRow>
        {tlsFingerprint && (
          <InsetRow label="TLS 指纹档案" hint="不选择时由上游使用默认档案；档案在「上游规则」页维护">
            <Select value={tlsProfileID} aria-label="TLS 指纹档案" onChange={e => setTlsProfileID(e.target.value)}>
              <option value="">上游默认档案</option>
              {(profiles?.value?.items ?? []).map(p => <option key={p.id} value={String(p.id)}>{p.name}</option>)}
              {tlsProfileID && !(profiles?.value?.items ?? []).some(p => String(p.id) === tlsProfileID) && <option value={tlsProfileID}>档案 #{tlsProfileID}</option>}
            </Select>
          </InsetRow>
        )}
        <InsetRow label="拦截预热探测请求 (intercept_warmup)"><Switch label="拦截预热请求" checked={interceptWarmup} onChange={setInterceptWarmup} /></InsetRow>
        <Field label="单号模型白名单（每行一个，留空不限）">
          <Textarea rows={3} value={allowlistText} onChange={e => setAllowlistText(e.target.value)} placeholder="claude-sonnet-4-5" />
        </Field>
      </FormSection>

      <FormSection title="账号生命周期">
        <FormGrid>
          <Field label="到期日期（留空表示永不过期）">
            <Input type="date" value={expiresDate} onChange={e => setExpiresDate(e.target.value)} />
          </Field>
        </FormGrid>
        <InsetRow label="到期后自动暂停调度">
          <Switch label="到期自动暂停" checked={autoPauseOnExpired} onChange={setAutoPauseOnExpired} />
        </InsetRow>
      </FormSection>

      <ErrorLine value={save.error} />
      <FormActions>
        <Button type="submit" size="sm" busy={save.busy}>保存防封与生命周期</Button>
      </FormActions>
    </Form>
  );
}
