import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {adminSubscriptionRow, userRow} from "@/schemas";
import {
  Button, ErrorLine, Field, Form, FormGrid, GroupDialog, Input,
  MultiGroupSelect, Overlay, PasswordInput, UserSelect,
} from "@/ui";
import type {UserItem} from "./index";

export function CreateUserDialog({open, revision, onClose, onCreated}: {open: boolean; revision: number; onClose: () => void; onCreated: (u: UserItem) => void}) {
  const formID = useId();
  const [allowedGroups, setAllowedGroups] = useState<number[]>([]);
  const op = useAction();

  return (
    <Overlay
      open={open}
      title="创建新用户"
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={op.busy}>创建用户</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            const created = await api("/admin/users", userRow, "POST", {
              email: String(fd.get("email") ?? "").trim(),
              password: String(fd.get("password") ?? ""),
              username: String(fd.get("username") ?? "").trim(),
              balance: Number(fd.get("balance") || 0),
              concurrency: Number(fd.get("concurrency") || 5),
              notes: String(fd.get("notes") ?? "").trim(),
              ...(allowedGroups.length ? {allowed_groups: allowedGroups} : {}),
            });
            onCreated(created);
          });
        }}
      >
        <FormGrid>
          <Field label="登录邮箱"><Input name="email" type="email" required /></Field>
          <Field label="初始密码（至少 6 位）"><PasswordInput name="password" minLength={6} required /></Field>
          <Field label="显示昵称（可选）"><Input name="username" /></Field>
          <Field label="初始余额"><Input name="balance" type="number" suffix="USD" min="0" step="0.01" defaultValue="0" /></Field>
          <Field label="最大并发数"><Input name="concurrency" type="number" min="0" defaultValue="5" /></Field>
          <Field label="备注"><Input name="notes" /></Field>
        </FormGrid>
        <Field label="授权专属分组（可选）">
          <MultiGroupSelect privateOnly value={allowedGroups} revision={revision} onChange={setAllowedGroups} />
        </Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

export function BatchUserLimitsDialog({open, userIDs, onClose, onSaved}: {open: boolean; userIDs: readonly number[]; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  return (
    <Overlay
      open={open}
      title={`批量设置 ${userIDs.length} 位用户限流`}
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={op.busy}>应用限流</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          const c = String(fd.get("concurrency") ?? "");
          const r = String(fd.get("rpm_limit") ?? "");
          void op.run(async () => {
            await api("/admin/users/batch-limits", z.object({affected: z.number()}), "POST", {
              user_ids: userIDs,
              ...(c !== "" ? {concurrency: Number(c)} : {}),
              ...(r !== "" ? {rpm_limit: Number(r)} : {}),
            });
            onSaved();
          });
        }}
      >
        <FormGrid>
          <Field label="覆写并发上限（留空不改）"><Input name="concurrency" type="number" min="0" /></Field>
          <Field label="覆写 RPM 上限（留空不改）"><Input name="rpm_limit" type="number" min="0" /></Field>
        </FormGrid>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}

export function AssignSubscriptionDialog({open, revision, onClose, onSaved, fixedUserID}: {open: boolean; revision: number; onClose: () => void; onSaved: () => void; fixedUserID?: number}) {
  return open ? <AssignSubscriptionForm revision={revision} fixedUserID={fixedUserID} onClose={onClose} onSaved={onSaved} /> : null;
}

function AssignSubscriptionForm({revision, onClose, onSaved, fixedUserID}: {revision: number; onClose: () => void; onSaved: () => void; fixedUserID?: number}) {
  const [userID, setUserID] = useState("");
  const op = useAction();
  return (
    <GroupDialog
      title="分配订阅"
      source={{subscriptionOnly: true, revision}}
      activeOnly
      onClose={onClose}
      applyLabel="开通订阅"
      applyDisabled={!fixedUserID && !userID}
      busy={op.busy}
      error={op.error}
      onApply={([groupID], fields) => void op.run(async () => {
        await api("/admin/subscriptions/assign", adminSubscriptionRow, "POST", {
          user_id: fixedUserID ?? Number(userID),
          group_id: groupID,
          validity_days: Number(fields.get("validity_days")),
          notes: String(fields.get("notes") ?? "").trim(),
        });
        onSaved();
      })}
    >
      {!fixedUserID && <Field label="目标用户"><UserSelect value={userID} placeholder="搜索用户邮箱…" clearable={false} onChange={setUserID} /></Field>}
      <FormGrid>
        <Field label="有效天数"><Input name="validity_days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
        <Field label="备注（选填）"><Input name="notes" maxLength={200} /></Field>
      </FormGrid>
    </GroupDialog>
  );
}

export function ExtendSubscription({id, onClose, onSaved}: {id: number; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  return (
    <Overlay
      open
      title="订阅续期"
      onClose={onClose}
      busy={op.busy}
      footer={<><Button variant="outline" onClick={onClose}>取消</Button><Button type="submit" form={formID} busy={op.busy}>确认续期</Button></>}
    >
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={event => {
          const days = Number(new FormData(event.currentTarget).get("days"));
          void op.run(async () => { await api(`/admin/subscriptions/${id}/extend`, adminSubscriptionRow, "POST", {days}); onSaved(); });
        }}
      >
        <Field label="续期天数"><Input name="days" type="number" min="1" max="3650" defaultValue="30" required /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
