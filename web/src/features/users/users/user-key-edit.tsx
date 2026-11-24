import {useId, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {keyRow} from "@/schemas";
import {
  Button, ErrorLine, Field, Form, FormGrid, Input, MultiGroupSelect, Overlay, Switch, TagInput,
} from "@/ui";

type KeyItem = z.infer<typeof keyRow>;

export function AdminEditKeyDialog({
  item,
  revision,
  onClose,
  onSaved,
}: {
  item: KeyItem;
  revision: number;
  onClose: () => void;
  onSaved: (msg: string) => void;
}) {
  const formID = useId();
  const op = useAction();
  const [active, setActive] = useState(item.status === "active");
  const [resetUsage, setResetUsage] = useState(false);
  const [allowedGroups, setAllowedGroups] = useState<number[]>(item.group_id ? [item.group_id] : []);
  const [whitelist, setWhitelist] = useState<string[]>(item.ip_whitelist);
  const [blacklist, setBlacklist] = useState<string[]>(item.ip_blacklist);

  return (
    <Overlay
      open
      title={`编辑代管密钥 · ${item.name}`}
      onClose={onClose}
      busy={op.busy}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={op.busy}>保存设置</Button>
        </>
      }
    >
      <Form
        id={formID}
        busy={op.busy}
        onSubmit={event => {
          const fd = new FormData(event.currentTarget);
          const quotaRaw = String(fd.get("quota") ?? "").trim();
          const multRaw = String(fd.get("rate_multiplier") ?? "").trim();
          void op.run(async () => {
            await api(`/admin/api-keys/${item.id}`, z.object({auto_granted_group_access: z.boolean().optional(), granted_group_name: z.string().optional()}), "PUT", {
              status: active ? "active" : "disabled",
              ...(quotaRaw !== "" ? {quota: Number(quotaRaw)} : {}),
              ...(resetUsage ? {reset_usage: true} : {}),
              ...(multRaw !== "" ? {rate_multiplier: Number(multRaw)} : {}),
              allowed_groups: allowedGroups,
              ip_whitelist: whitelist,
              ip_blacklist: blacklist,
            });
            onSaved(`已更新密钥「${item.name}」配置`);
          });
        }}
      >
        <Switch checked={active} label="启用该密钥" onChange={setActive} />
        <FormGrid>
          <Field label="额度上限（USD，0 为不限）">
            <Input name="quota" type="number" min="0" step="0.01" defaultValue={String(item.quota)} />
          </Field>
          <Field label="倍率（选填）">
            <Input name="rate_multiplier" type="number" min="0.01" max="100" step="0.01" placeholder="默认继承分组" />
          </Field>
        </FormGrid>
        <Switch checked={resetUsage} label="保存时清零已用额度" onChange={setResetUsage} />
        <Field label="允许访问分组（选填）">
          <MultiGroupSelect value={allowedGroups} revision={revision} onChange={setAllowedGroups} />
        </Field>
        <Field label="IP 白名单（回车添加）">
          <TagInput value={whitelist} placeholder="例如 1.2.3.4 或 10.0.0.0/8" onChange={setWhitelist} />
        </Field>
        <Field label="IP 黑名单（回车添加）">
          <TagInput value={blacklist} placeholder="例如 5.6.7.8" onChange={setBlacklist} />
        </Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
