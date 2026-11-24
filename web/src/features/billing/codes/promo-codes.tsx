import {useId, useState} from "react";
import {History, Pencil, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {count, date, dateTimeInputSeconds, dateTimeInputValue, money} from "@/lib/format";
import {useAction, useDebouncedValue} from "@/lib/requests";
import {
  Button, ConfirmDialog, EntityLink, ErrorLine, Field, FilterBar, FilterSlot, Form, FormGrid,
  Input, MaskedSecret, Overlay, RefreshButton, RemoteTable, SearchInput, SecretRevealDialog,
  Select, Switch, Textarea, WorkspaceStage, toast, type Column,
} from "@/ui";

const base = "/admin/promo-codes";
const promoCode = z.object({
  id: z.number(), masked: z.string(), status: z.string(), bonus_amount: z.number(), max_uses: z.number(),
  used_count: z.number(), expires_at: z.string().nullish(), notes: z.string(),
});
const newPromoCode = z.object({code: z.string().min(1)});
const usage = z.object({id: z.number(), user_id: z.number(), user_email: z.string().optional(), bonus_amount: z.number(), used_at: z.string()});
type PromoCode = z.infer<typeof promoCode>;

const usageColumns: Column<z.infer<typeof usage>>[] = [
  {id: "user", title: "兑换用户", render: row => <EntityLink kind="user" id={row.user_id} label={row.user_email || "用户 #" + row.user_id} />},
  {id: "bonus", title: "入账金额", align: "number", render: row => money(row.bonus_amount)},
  {id: "date", title: "使用时间", render: row => date(row.used_at)},
];

export function PromoCodes() {
  const [revision, revise] = useState(0);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [editing, edit] = useState<{row?: PromoCode}>();
  const [usageFor, setUsage] = useState<PromoCode>();
  const [removing, remove] = useState<PromoCode>();
  const [generated, setGenerated] = useState("");
  const removal = useAction();
  const debounced = useDebouncedValue(search.trim());
  const path = base + "?" + new URLSearchParams({search: debounced, status});

  const columns: Column<PromoCode>[] = [
    {id: "code", title: "优惠码", render: row => <MaskedSecret masked={row.masked} />},
    {
      id: "status",
      title: "启用",
      align: "center",
      render: row => (
        <Switch
          checked={row.status === "active"}
          disabled={removal.busy}
          label={`${row.status === "active" ? "停用" : "启用"}优惠码 ${row.masked}`}
          onChange={checked => void removal.run(async () => {
            await api(base + "/" + row.id, empty, "PUT", {
              bonus_amount: row.bonus_amount,
              max_uses: row.max_uses,
              notes: row.notes,
              expires_at: row.expires_at ? Math.floor(new Date(row.expires_at).getTime() / 1000) : 0,
              status: checked ? "active" : "disabled",
            });
            revise(v => v + 1);
            toast(checked ? "优惠码已启用" : "优惠码已停用");
          })}
        />
      ),
    },
    {id: "amount", title: "赠送余额", align: "number", render: row => money(row.bonus_amount)},
    {id: "uses", title: "已用 / 上限", align: "number", hideWhenCompact: true, render: row => count.format(row.used_count) + " / " + (row.max_uses ? count.format(row.max_uses) : "不限")},
    {id: "expiry", title: "到期时间", hideWhenNarrow: true, render: row => row.expires_at ? date(row.expires_at) : "长期"},
    {
      id: "actions",
      title: "操作",
      align: "end",
      render: row => (
        <div className="row-actions">
          <Button variant="ghost" size="icon-sm" title="使用记录" aria-label="使用记录" onClick={() => setUsage(row)}><History aria-hidden /></Button>
          <Button variant="ghost" size="icon-sm" title="编辑优惠码" aria-label="编辑优惠码" onClick={() => edit({row})}><Pencil aria-hidden /></Button>
          <Button variant="ghost" size="icon-sm" title="删除优惠码" aria-label="删除优惠码" onClick={() => { removal.clearError(); remove(row); }}><Trash2 aria-hidden /></Button>
        </div>
      ),
    },
  ];

  return <WorkspaceStage flush>
    <FilterBar sticky>
      <div className="obs-filter-row">
        <SearchInput value={search} onChange={setSearch} placeholder="搜索优惠码或备注" />
        <FilterSlot>
          <Select aria-label="优惠码状态" value={status} onChange={event => setStatus(event.target.value)}>
            <option value="">全部状态</option><option value="active">已启用</option><option value="disabled">已停用</option>
          </Select>
        </FilterSlot>
        <div className="obs-filter-spacer">
          <RefreshButton iconOnly size="default" variant="ghost" label="刷新优惠码" onClick={() => revise(v => v + 1)} />
          <Button onClick={() => edit({})}>创建优惠码</Button>
        </div>
      </div>
    </FilterBar>
    <div className="swift-pane-table">
      <RemoteTable path={path} schema={promoCode} columns={columns} rowID={row => row.id} pinActions label="优惠码" revision={revision} />
    </div>
    {editing && <PromoEditor row={editing.row} onClose={() => edit(undefined)} onSaved={code => {
      edit(undefined);
      revise(value => value + 1);
      if (code) setGenerated(code);
      else toast("优惠码已保存");
    }} />}
    <SecretRevealDialog
      open={Boolean(generated)}
      title="优惠码已创建"
      label="注册优惠码"
      value={generated}
      onClose={() => setGenerated("")}
    />
    {usageFor && <Overlay open drawer title="优惠码使用记录" onClose={() => setUsage(undefined)}>
      <RemoteTable path={base + "/" + usageFor.id + "/usages"} schema={usage} columns={usageColumns} rowID={row => row.id} label="优惠码使用记录" />
    </Overlay>}
    {removing && <ConfirmDialog open title="删除优惠码" description="删除后，此优惠码将无法继续用于注册赠送。" confirm="删除优惠码"
      busy={removal.busy} error={removal.error} onClose={() => remove(undefined)} onConfirm={() => void removal.run(async () => {
        await api(base + "/" + removing.id, empty, "DELETE");
        remove(undefined);
        revise(value => value + 1);
        toast("优惠码已删除");
      })} />}
  </WorkspaceStage>;
}

function PromoEditor({row, onClose, onSaved}: {row?: PromoCode; onClose: () => void; onSaved: (code?: string) => void}) {
  const formID = useId();
  const action = useAction();
  return <Overlay open drawer title={row ? "编辑优惠码" : "创建优惠码"} busy={action.busy} onClose={onClose}
    footer={<>
      <Button variant="ghost" onClick={onClose}>取消</Button>
      <Button type="submit" form={formID} busy={action.busy}>{row ? "保存优惠码" : "生成优惠码"}</Button>
    </>}>
    <Form id={formID} busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
      const data = new FormData(event.currentTarget);
      void action.run(async () => {
        const code = String(data.get("code") ?? "").trim();
        const input = {
          code: code || undefined, bonus_amount: Number(data.get("bonus_amount")),
          max_uses: Number(data.get("max_uses")), notes: String(data.get("notes") ?? ""),
          expires_at: dateTimeInputSeconds(data.get("expires_at")),
        };
        if (row) {
          await api(base + "/" + row.id, empty, "PUT", {...input, expires_at: input.expires_at ?? 0, status: data.get("status")});
          onSaved();
        } else {
          const created = await api(base, newPromoCode, "POST", input);
          onSaved(created.code);
        }
      });
    }}>
      <Field label={row ? "更换优惠码" : "自定义优惠码"} hint={row ? "留空保留现有优惠码" : "留空由系统随机生成"}>
        <Input name="code" autoComplete="off" />
      </Field>
      <FormGrid>
        <Field label="赠送余额"><Input type="number" name="bonus_amount" suffix="USD" step="any" min={row ? 0 : 0.000001} defaultValue={row?.bonus_amount} required /></Field>
        <Field label="使用次数上限" hint="0 表示不限次数"><Input type="number" name="max_uses" suffix="次" min={0} defaultValue={row?.max_uses ?? 0} required /></Field>
        <Field label="到期时间" hint="留空长期有效"><Input type="datetime-local" step={1} name="expires_at" defaultValue={dateTimeInputValue(row?.expires_at)} /></Field>
        {row && <Field label="状态"><Select name="status" defaultValue={row.status === "disabled" ? "disabled" : "active"}>
          <option value="active">已启用</option><option value="disabled">已停用</option>
        </Select></Field>}
      </FormGrid>
      <Field label="备注"><Textarea name="notes" defaultValue={row?.notes} /></Field>
      <ErrorLine value={action.error} />
    </Form>
  </Overlay>;
}
