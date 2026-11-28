import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {
  Button, ConfirmDialog, ErrorLine, Field, Form, FormGrid, Input, MenuItem, MultiGroupSelect, Overlay, toast,
} from "@/ui";

const privacyResult = z.object({privacy_mode: z.string()});
const shadowResult = z.object({id: z.number(), name: z.string(), platform: z.string(), type: z.string()});

// 菜单弹层关闭后会卸载内容，使用方的 MoreActions 需要传 keepMounted，弹窗才能留在菜单项里。
export function AccountPrivacyAction({accountId, onChanged}: {accountId: number; onChanged?: () => void}) {
  const [open, setOpen] = useState(false);
  const op = useAction();
  return (
    <>
      <MenuItem onSelect={() => setOpen(true)}>隐私设置</MenuItem>
      <ConfirmDialog
        open={open}
        title="设置账号隐私"
        description="将通过上游为该账号重新应用隐私模式（关闭训练数据共享），完成后显示最终的隐私模式。"
        confirm="应用隐私设置"
        busy={op.busy}
        error={op.error}
        onClose={() => setOpen(false)}
        onConfirm={() => void op.run(async () => {
          const res = await api(`/admin/accounts/${accountId}/set-privacy`, privacyResult, "POST", {});
          setOpen(false);
          toast(res.privacy_mode ? `隐私模式：${res.privacy_mode}` : "已提交隐私设置");
          onChanged?.();
        })}
      />
    </>
  );
}

// 只对 OpenAI OAuth 账号显示。
export function ShadowAccountAction({accountId, platform, type, onChanged}: {
  accountId: number;
  platform: string;
  type: string;
  onChanged?: () => void;
}) {
  const [open, setOpen] = useState(false);
  if (platform !== "openai" || type !== "oauth") return null;
  return (
    <>
      <MenuItem onSelect={() => setOpen(true)}>Spark 影子账号</MenuItem>
      <ShadowDialog open={open} accountId={accountId} onClose={() => setOpen(false)} onCreated={() => { setOpen(false); onChanged?.(); }} />
    </>
  );
}

function ShadowDialog({open, accountId, onClose, onCreated}: {open: boolean; accountId: number; onClose: () => void; onCreated: () => void}) {
  const formID = useId();
  const [groupIDs, setGroupIDs] = useState<number[]>([]);
  const op = useAction();
  return (
    <Overlay open={open} title="创建 Spark 影子账号" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>创建影子账号</Button>
      </>
    }>
      <Form id={formID} busy={op.busy} onSubmit={e => {
        const fd = new FormData(e.currentTarget);
        void op.run(async () => {
          const created = await api(`/admin/accounts/${accountId}/shadow`, shadowResult, "POST", {
            name: String(fd.get("name") ?? "").trim(),
            priority: Number(fd.get("priority") || 0),
            concurrency: Number(fd.get("concurrency") || 0),
            group_ids: groupIDs,
          });
          toast(`已创建影子账号 ${created.name}`);
          onCreated();
        });
      }}>
        <Field label="账号名称（留空沿用上游默认）"><Input name="name" maxLength={100} /></Field>
        <FormGrid>
          <Field label="优先级"><Input name="priority" type="number" min="0" max="100000" defaultValue="0" /></Field>
          <Field label="并发上限"><Input name="concurrency" type="number" min="0" max="100000" defaultValue="0" /></Field>
        </FormGrid>
        <Field label="绑定分组（可选）"><MultiGroupSelect value={groupIDs} onChange={setGroupIDs} /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
