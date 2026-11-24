import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, named, type Config, type SessionInfo} from "@/schemas";
import {Brand} from "@/app/appearance";
import {currencyMoney, date, money} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {
  Button, Checkbox, ChoiceCards, EmptyState, EntityLink, ErrorLine, Feedback, Field, FilterBar, FilterSlot, Form, InlineStack,
  Input, InsetGroup, InsetRow, Overlay, QrCode, RefreshButton, Resource, SearchInput, Select, StageCard, Status,
  WorkspaceContent, WorkspaceStage, toast, type ChoiceOption, type Column,
} from "@/ui";
import {RemoteTable} from "@/ui";

const order = z.object({id: z.number(), amount: z.number(), pay_amount: z.number(), currency: z.string(), payment_type: z.string(),
  order_type: z.string(), status: z.string(), created_at: z.string(), expires_at: z.string(), paid_at: z.string().optional()});
const plan = named.extend({description: z.string(), price: z.number(), currency: z.string(), validity_days: z.number(), validity_unit: z.string()});
const checkout = z.object({plans: z.array(plan), balance_disabled: z.boolean(), methods: z.record(z.object({
  display_name: z.string(), currency: z.string(), single_min: z.number(), single_max: z.number(), fee_rate: z.number(),
}))});
const createdOrder = z.object({order_id: z.number(), pay_url: z.string().optional(), qr_code: z.string().optional(), pay_amount: z.number(),
  currency: z.string(), expires_at: z.string()});
const statuses: Record<string, string> = {PENDING: "待支付", PAID: "已付款", RECHARGING: "正在入账", COMPLETED: "已完成",
  EXPIRED: "已过期", CANCELLED: "已取消", FAILED: "失败", REFUND_REQUESTED: "已申请退款", REFUNDING: "退款中",
  REFUND_PENDING: "等待退款", PARTIALLY_REFUNDED: "部分退款", REFUNDED: "已退款", REFUND_FAILED: "退款失败"};
const methodNames: Record<string, string> = {alipay: "支付宝", alipay_direct: "支付宝", wxpay: "微信支付", wxpay_direct: "微信支付",
  easypay: "易支付", stripe: "Stripe", airwallex: "Airwallex"};
const base = "/payment/orders";
const adminBase = "/admin/payment/orders";
const adminOrder = order.extend({user_id: z.number(), user_email: z.string(), refund_amount: z.number()});
const refund = z.object({success: z.boolean(), require_force: z.boolean()});
type AdminOrder = z.infer<typeof adminOrder>;
type OrderAction = "cancel" | "retry" | "refund" | "refund/query";
const orderActions: Record<OrderAction, string> = {
  cancel: "取消订单", retry: "重试入账", refund: "退款", "refund/query": "查询退款",
};

export function PaymentOrders() {
  const [revision, revise] = useState(0);
  const [keyword, search] = useState("");
  const [status, setStatus] = useState("");
  const [method, setMethod] = useState("");
  const [selected, select] = useState<number>();
  const query = new URLSearchParams({status, payment_type: method, keyword: useDebouncedValue(keyword.trim())});
  const columns: Column<AdminOrder>[] = [
    {id: "id", title: "订单号", render: row => <span className="code-mono">#{row.id}</span>},
    {id: "user", title: "用户", hideWhenCompact: true, render: row => <OrderUser row={row} />},
    {id: "method", title: "支付方式", hideWhenCompact: true, render: row => methodNames[row.payment_type] ?? row.payment_type},
    {id: "amount", title: "支付金额", align: "number", render: row => currencyMoney(row.pay_amount, row.currency || "USD")},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {id: "created", title: "创建时间", hideWhenNarrow: true, render: row => date(row.created_at)},
    {id: "actions", title: "操作", align: "end", render: row => <Button size="sm" variant="ghost" onClick={() => select(row.id)}>查看</Button>},
  ];
  return <WorkspaceStage flush>
    <FilterBar sticky>
      <div className="obs-filter-row">
        <SearchInput value={keyword} onChange={search} placeholder="搜索订单号或邮箱" />
        <FilterSlot>
          <Select aria-label="订单状态" value={status} onChange={event => setStatus(event.target.value)}>
            <option value="">全部状态</option>
            {Object.entries(statuses).map(([id, title]) => <option key={id} value={id}>{title}</option>)}
          </Select>
        </FilterSlot>
        <FilterSlot>
          <Select aria-label="支付方式" value={method} onChange={event => setMethod(event.target.value)}>
            <option value="">全部支付方式</option>
            {Object.entries(methodNames).map(([id, title]) => <option key={id} value={id}>{title}{id.endsWith("_direct") ? "直连" : ""}</option>)}
          </Select>
        </FilterSlot>
        <div className="obs-filter-spacer">
          <RefreshButton iconOnly size="default" variant="ghost" label="刷新订单" onClick={() => revise(v => v + 1)} />
        </div>
      </div>
    </FilterBar>
    <div className="swift-pane-table">
      <RemoteTable path={adminBase + "?" + query} schema={adminOrder} columns={columns} rowID={row => row.id} onRowClick={row => select(row.id)} pinActions label="支付订单" revision={revision} />
    </div>
    {selected !== undefined && <AdminOrderDetail key={selected} id={selected} revision={revision} close={() => select(undefined)}
      update={message => { toast(message); revise(value => value + 1); }} />}
  </WorkspaceStage>;
}

function OrderUser({row}: {row: AdminOrder}) {
  return row.user_id > 0 ? <EntityLink kind="user" id={row.user_id} label={row.user_email || "用户 #" + row.user_id} /> : "—";
}

function AdminOrderDetail({id, revision, close, update}: {id: number; revision: number; close: () => void; update: (value: string) => void}) {
  const path = adminBase + "/" + id;
  const data = useResource(() => api(path, adminOrder), path, true, revision);
  const [operation, select] = useState<OrderAction>();
  const [deduct, setDeduct] = useState(true);
  const [force, setForce] = useState(false);
  const [requiresForce, requireForce] = useState(false);
  const action = useAction();
  const formID = useId();
  const reset = (next?: OrderAction) => {
    action.clearError();
    setDeduct(true);
    setForce(false);
    requireForce(false);
    select(next);
  };
  return <Overlay open title={"订单 #" + id} onClose={close} busy={action.busy}
    footer={operation && <>
      <Button variant="ghost" disabled={action.busy} onClick={() => reset()}>返回详情</Button>
      <Button type="submit" form={formID} busy={action.busy} disabled={operation === "refund" && requiresForce && !force}
        variant={operation === "cancel" || operation === "refund" ? "danger-outline" : "default"}>确认{orderActions[operation]}</Button>
    </>}>
    <Resource state={data}>{row => {
      const available: OrderAction | undefined = row.status === "PENDING" ? "cancel" : row.status === "FAILED" ? "retry" :
        row.status === "REFUND_PENDING" ? "refund/query" :
        ["REFUND_REQUESTED", "COMPLETED", "REFUND_FAILED"].includes(row.status) ? "refund" : undefined;
      const amount = currencyMoney(row.pay_amount, row.currency || "USD");
      return <>
        <InsetGroup>
          <InsetRow label="用户"><OrderUser row={row} /></InsetRow>
          <InsetRow label="类型" hint={row.order_type === "subscription" ? "订阅" : "充值"} />
          <InsetRow label="已付金额" hint={amount} />
          <InsetRow label="状态"><Status value={row.status} /></InsetRow>
          <InsetRow label="创建时间" hint={date(row.created_at)} />
          {row.refund_amount > 0 && <InsetRow label="退款金额" hint={currencyMoney(row.refund_amount, row.currency || "USD")} />}
        </InsetGroup>
        {!operation && available && <Button variant="outline" onClick={() => reset(available)}>{orderActions[available]}</Button>}
        {operation && <Form id={formID} busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
          const fields = new FormData(event.currentTarget);
          void action.run(async () => {
            if (operation === "refund") {
              const result = await api(path + "/refund", refund, "POST", {amount: Number(fields.get("amount")),
                reason: String(fields.get("reason")), deduct_balance: deduct, force});
              if (result.require_force) { requireForce(true); return; }
              if (!result.success) { reset(); update("退款尚未完成，已刷新订单状态"); return; }
            } else if (operation === "refund/query") {
              await api(path + "/refund/query", refund, "POST", {});
            } else {
              await api(path + "/" + operation, empty, "POST", {});
            }
            reset();
            update(operation === "refund/query" ? "退款状态已查询并刷新" : "已提交" + orderActions[operation]);
          });
        }}>
          {operation === "refund" && <>
            <Field label="退款原订单金额" hint="按订单入账金额填写，实际退回金额由官方按支付汇率换算。">
              <Input type="number" name="amount" suffix={row.currency || "USD"} min={0.01} max={row.amount} step={0.01}
                defaultValue={row.status === "REFUND_REQUESTED" ? row.refund_amount || row.amount : row.amount} required />
            </Field>
            <Field label="退款原因"><Input name="reason" maxLength={500} /></Field>
            <Checkbox label="扣回已入账的余额或订阅" checked={deduct} onChange={setDeduct}>扣回已入账的余额或订阅</Checkbox>
            {requiresForce && <Checkbox label="余额或订阅不足时仍强制退款" checked={force} onChange={setForce}>余额或订阅不足时仍强制退款</Checkbox>}
          </>}
          <ErrorLine value={action.error} />
        </Form>}
      </>;
    }}</Resource>
  </Overlay>;
}

export function Purchase({enabled}: {enabled: boolean}) {
  const [revision, revise] = useState(0);
  const [selected, select] = useState<number>();
  const offer = useResource(() => api("/payment/checkout", checkout), "checkout", enabled);
  const columns: Column<z.infer<typeof order>>[] = [
    {id: "id", title: "订单号", render: row => <span className="code-mono">#{row.id}</span>},
    {id: "kind", title: "类型", hideWhenCompact: true, render: row => row.order_type === "subscription" ? "订阅" : "充值"},
    {id: "pay", title: "支付金额", align: "number", render: row => currencyMoney(row.pay_amount, row.currency || "USD")},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {id: "created", title: "创建时间", hideWhenNarrow: true, render: row => date(row.created_at)},
    {id: "actions", title: "操作", align: "end", render: row => <Button size="sm" variant="ghost" onClick={() => select(row.id)}>查看</Button>},
  ];
  return <WorkspaceContent>
    {enabled && <Resource state={offer}>{options => <Checkout options={options} onCreated={() => revise(value => value + 1)} />}</Resource>}
    <InsetGroup title="我的订单" actions={<RefreshButton size="xs" variant="ghost" onClick={() => revise(v => v + 1)} />}>
      <RemoteTable path={base} schema={order} rowID={row => row.id} onRowClick={row => select(row.id)} pinActions columns={columns} label="我的订单" revision={revision} />
    </InsetGroup>
    {selected !== undefined && <OrderDetail key={selected} id={selected} onClose={() => select(undefined)} onChange={() => revise(value => value + 1)} />}
  </WorkspaceContent>;
}

function Checkout({options, onCreated}: {options: z.infer<typeof checkout>; onCreated: () => void}) {
  const [product, setProduct] = useState(options.balance_disabled ? String(options.plans[0]?.id ?? "") : "balance");
  const [method, setMethod] = useState(Object.keys(options.methods)[0] ?? "");
  const [created, create] = useState<z.infer<typeof createdOrder>>();
  const action = useAction();
  const selected = options.methods[method];
  if (!selected || !product) return <EmptyState title="暂无可购买的项目或可用支付方式" />;

  const productChoices: ChoiceOption<string>[] = [
    ...(!options.balance_disabled
      ? [{value: "balance", title: "钱包余额充值", description: "按需充值账户可用额度，适用于按量计费分组"}]
      : []),
    ...options.plans.map(item => ({
      value: String(item.id),
      title: item.name,
      meta: currencyMoney(item.price, item.currency || "USD"),
      description: item.description || `有效期 ${item.validity_days} 天`,
    })),
  ];

  const methodChoices: ChoiceOption<string>[] = Object.entries(options.methods).map(([id, data]) => {
    const range = data.single_max > 0 ? `限额 ${data.single_min || 0}–${data.single_max} ${data.currency}` : data.currency;
    return {
      value: id,
      title: data.display_name || methodNames[id] || id,
      meta: data.fee_rate > 0 ? `手续费 ${data.fee_rate}%` : "免手续费",
      description: range,
    };
  });

  return <InsetGroup title="购买订阅或充值">
    <InsetRow label="选择项目与支付通道" vertical>
      <Form busy={action.busy} errors={action.fieldErrors} onSubmit={event => {
        const fields = new FormData(event.currentTarget);
        void action.run(async () => {
          const result = await api(base, createdOrder, "POST", {
            payment_type: method,
            order_type: product === "balance" ? "balance" : "subscription",
            plan_id: product === "balance" ? undefined : Number(product),
            amount: product === "balance" ? Number(fields.get("amount")) : 0,
          });
          create(result);
          onCreated();
        });
      }}>
        <Field label="购买项目">
          <ChoiceCards label="购买项目" value={product} options={productChoices} columns={3} onChange={setProduct} />
        </Field>
        <Field label="支付通道">
          <ChoiceCards label="支付通道" value={method} options={methodChoices} columns={3} onChange={setMethod} />
        </Field>
        {product === "balance" && (
          <Field label="充值金额" name="amount">
            <div className="row-field">
              <Input
                name="amount"
                type="number"
                step="0.01"
                suffix={selected.currency || "USD"}
                min={selected.single_min || 0.01}
                max={selected.single_max || undefined}
                required
              />
            </div>
          </Field>
        )}
        <ErrorLine value={action.error} />
        <Button type="submit" busy={action.busy}>创建支付订单</Button>
      </Form>
    </InsetRow>
    {created && (
      <Overlay open title={`支付订单 #${created.order_id}`} onClose={() => create(undefined)}>
        <InsetGroup>
          <InsetRow label="应付金额" hint={currencyMoney(created.pay_amount, created.currency || "USD")} />
          <InsetRow label="过期时间" hint={date(created.expires_at)} />
          {created.pay_url && <InsetRow label="收银台"><Button render={<a href={created.pay_url} rel="noreferrer" />}>前往支付平台</Button></InsetRow>}
          {created.qr_code && <InsetRow label="扫码支付" vertical><QrCode value={created.qr_code} alt="支付二维码" /></InsetRow>}
          {!(created.qr_code || created.pay_url) && <Feedback>订单已创建。此支付方式未提供直接跳转入口，请在订单详情核对状态。</Feedback>}
          <InsetRow label="订单结果"><Button variant="outline" render={<a href={"/payment/result?order_id=" + created.order_id} />}>查看支付结果</Button></InsetRow>
        </InsetGroup>
      </Overlay>
    )}
  </InsetGroup>;
}

function OrderDetail({id, onClose, onChange}: {id: number; onClose: () => void; onChange: () => void}) {
  const [revision, revise] = useState(0);
  const action = useAction();
  const path = base + "/" + id;
  const data = useResource(() => api(path, order), path, true, revision);
  return <Overlay open title={"订单 #" + id} onClose={onClose} busy={action.busy}>
    <Resource state={data}>{row => <>
      <OrderSummary row={row} />
      {row.status === "PENDING" && <Button variant="danger-outline" busy={action.busy || data?.loading} onClick={() => void action.run(async () => {
        await api(path + "/cancel", empty, "POST");
        toast("订单已取消");
        revise(value => value + 1);
        onChange();
      })}>取消待支付订单</Button>}
    </>}</Resource>
    <ErrorLine value={action.error} />
  </Overlay>;
}

function OrderSummary({row}: {row: z.infer<typeof order>}) {
  return <InsetGroup>
    <InsetRow label="订单状态"><Status value={row.status} /></InsetRow>
    <InsetRow label="支付金额" hint={currencyMoney(row.pay_amount, row.currency || "USD")} />
    {row.order_type === "balance" && <InsetRow label="充值余额" hint={money(row.amount)} />}
    <InsetRow label="创建时间" hint={date(row.created_at)} />
    <InsetRow label="过期时间" hint={date(row.expires_at)} />
    {row.paid_at && <InsetRow label="支付时间" hint={date(row.paid_at)} />}
  </InsetGroup>;
}

export function PaymentResult({config, session, params}: {config: Config; session?: SessionInfo | null; params: URLSearchParams}) {
  const [revision, revise] = useState(0);
  const token = params.get("resume_token") ?? "";
  const id = params.get("order_id") ?? "";
  const validID = /^[1-9][0-9]{0,15}$/.test(id) && Number.isSafeInteger(Number(id));
  const canRead = Boolean(token || session && validID);
  const data = useResource(() => token ? api("/public/payment/result", order, "POST", {resume_token: token}, false) : api(base + "/" + id, order),
    id + ":" + token, canRead, revision);
  return (
    <StageCard
      size="md"
      title="支付结果"
      brand={<Brand name={config.site_name || "Sub2API"} logo={config.site_logo || undefined} />}
    >
      {canRead ? <Resource state={data}>{row => <OrderSummary row={row} />}</Resource> : <EmptyState title="请登录后查看订单" />}
      <InlineStack>
        {canRead && <RefreshButton busy={data?.loading} label="刷新订单状态" onClick={() => revise(value => value + 1)} />}
        <Button render={<a href="/purchase" />}>返回购买与订单</Button>
      </InlineStack>
    </StageCard>
  );
}
