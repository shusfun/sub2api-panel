import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {createdRedeemCode, empty} from "@/schemas";
import {
  Button, ErrorLine, Field, Form, FormGrid, GroupSelect, Input, InsetRow,
  Overlay, Segmented, Select, Switch, UserSelect,
} from "@/ui";

export type CreatedCode = z.infer<typeof createdRedeemCode>;

export function EditCodesBatch({ids, onClose, onSaved}: {ids: number[]; onClose: () => void; onSaved: (total: number) => void}) {
  const formID = useId();
  const action = useAction();
  const [expiry, setExpiry] = useState("");
  const [group, setGroup] = useState("");
  const [editGroup, setEditGroup] = useState(false);
  const [editNotes, setEditNotes] = useState(false);
  return (
    <Overlay
      open
      title="批量编辑兑换码"
      onClose={onClose}
      busy={action.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={action.busy}>保存 {ids.length} 张兑换码</Button>
        </>
      }
    >
      <Form
        id={formID}
        onSubmit={event => {
          const values = new FormData(event.currentTarget);
          void action.run(async () => {
            const fields: Record<string, unknown> = {};
            if (values.get("status")) fields.status = values.get("status");
            if (expiry === "clear") fields.expires_at = null;
            if (expiry === "set") {
              const dt = new Date(String(values.get("expires_at") ?? ""));
              if (!Number.isFinite(dt.getTime())) throw new Error("请输入有效的到期时间");
              fields.expires_at = dt.toISOString();
            }
            if (editGroup) fields.group_id = group ? Number(group) : null;
            if (editNotes) fields.notes = String(values.get("notes") ?? "");
            if (!Object.keys(fields).length) throw new Error("请选择要修改的字段");
            const result = await api("/admin/redeem-codes/batch-update", z.object({updated: z.number()}), "POST", {ids, fields});
            onSaved(result.updated);
          });
        }}
      >
        <Field label="状态"><Select name="status"><option value="">保持原值</option><option value="unused">启用</option><option value="disabled">停用</option></Select></Field>
        <Field label="到期时间"><Select value={expiry} onChange={event => setExpiry(event.target.value)}><option value="">保持原值</option><option value="set">设置时间</option><option value="clear">清除</option></Select></Field>
        {expiry === "set" && <Field label="到期于"><Input type="datetime-local" name="expires_at" required /></Field>}
        <InsetRow label="更改订阅分组"><Switch label="更改订阅分组" checked={editGroup} onChange={setEditGroup} /></InsetRow>
        {editGroup && <Field label="订阅分组"><GroupSelect subscriptionOnly value={group} onChange={setGroup} placeholder="清除分组" /></Field>}
        <InsetRow label="更改备注"><Switch label="更改备注" checked={editNotes} onChange={setEditNotes} /></InsetRow>
        {editNotes && <Field label="新备注"><Input name="notes" maxLength={255} /></Field>}
        <ErrorLine value={action.error} />
      </Form>
    </Overlay>
  );
}

export function GenerateRedeemDialog({
  open,
  revision,
  onClose,
  onGenerated,
  onDirectRedeemed,
}: {
  open: boolean;
  revision: number;
  onClose: () => void;
  onGenerated: (codes: CreatedCode[]) => void;
  onDirectRedeemed: (userID: number) => void;
}) {
  const formID = useId();
  const [mode, setMode] = useState<"batch" | "direct">("batch");
  const [kind, setKind] = useState<"balance" | "subscription" | "concurrency" | "invitation">("balance");
  const [groupID, setGroupID] = useState("");
  const [targetUser, setTargetUser] = useState("");
  const op = useAction();

  return (
    <Overlay
      open={open}
      title="生成兑换码"
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} disabled={(mode === "direct" && !targetUser) || (kind === "subscription" && !groupID)} busy={op.busy}>
            {mode === "direct" ? "充值" : "生成"}
          </Button>
        </>
      }
    >
      <Segmented
        label="发卡模式"
        value={mode}
        options={[["batch", "批量生成"], ["direct", "直接充给用户"]] as const}
        onChange={next => {
          setMode(next);
          if (next === "direct" && kind === "invitation") setKind("balance");
        }}
      />
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            if (mode === "direct") {
              const uid = Number(targetUser);
              await api("/admin/redeem-codes/create-and-redeem", empty, "POST", {
                user_id: uid,
                type: kind,
                value: kind === "subscription" ? 1 : Number(fd.get("value") || 10),
                ...(kind === "subscription" ? {group_id: Number(groupID), validity_days: Number(fd.get("validity_days") || 30)} : {}),
                notes: String(fd.get("notes") ?? "").trim(),
                operation_id: crypto.randomUUID(),
              });
              onDirectRedeemed(uid);
              return;
            }
            const res = await api("/admin/redeem-codes/generate", z.object({codes: z.array(createdRedeemCode)}), "POST", {
              count: Number(fd.get("count") || 1),
              type: kind,
              value: kind === "invitation" ? 0 : kind === "subscription" ? 1 : Number(fd.get("value") || 10),
              ...(kind === "subscription" ? {group_id: Number(groupID), validity_days: Number(fd.get("validity_days") || 30)} : {}),
              expires_in_days: Number(fd.get("expires_in_days") || 30),
              operation_id: crypto.randomUUID(),
            });
            onGenerated(res.codes);
          });
        }}
      >
        {mode === "direct" && (
          <Field label="目标用户"><UserSelect value={targetUser} clearable={false} placeholder="搜索用户邮箱…" onChange={v => setTargetUser(v)} /></Field>
        )}
        <FormGrid>
          <Field label="类型">
            <Select value={kind} onChange={e => setKind(e.target.value as "balance" | "subscription" | "concurrency" | "invitation")}>
              <option value="balance">余额充值码 (USD)</option>
              <option value="subscription">套餐订阅码</option>
              <option value="concurrency">并发扩容码</option>
              {mode === "batch" && <option value="invitation">注册邀请码</option>}
            </Select>
          </Field>
          {mode === "batch" && (
            <Field label="生成张数 (1–100)"><Input name="count" type="number" min="1" max="100" defaultValue="5" required /></Field>
          )}
          {(kind === "balance" || kind === "concurrency") && (
            <Field label={kind === "balance" ? "单张金额 (USD)" : "增加并发数"}>
              <Input name="value" type="number" min="0.01" step={kind === "balance" ? "0.01" : "1"} defaultValue="10" required />
            </Field>
          )}
          {mode === "batch" && (
            <Field label="兑换码有效天数"><Input name="expires_in_days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
          )}
        </FormGrid>
        {kind === "subscription" && (
          <FormGrid>
            <Field label="订阅分组"><GroupSelect value={groupID} subscriptionOnly revision={revision} clearable={false} placeholder="选择订阅组…" onChange={setGroupID} /></Field>
            <Field label="开通有效天数"><Input name="validity_days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
          </FormGrid>
        )}
        {mode === "direct" && <Field label="备注（选填）"><Input name="notes" maxLength={200} /></Field>}
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
