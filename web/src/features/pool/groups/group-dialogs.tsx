import {useId, useState} from "react";
import {api} from "@/lib/http";
import {adminGroupRow} from "@/schemas";
import {useAction} from "@/lib/requests";
import {
  Button,
  ErrorLine,
  Field,
  Form,
  FormGrid,
  Input,
  InsetGroup,
  InsetRow,
  MultiGroupSelect,
  Overlay,
  PlatformSelect,
  Segmented,
  Switch,
} from "@/ui";
import {type GroupRow} from "@/features/pool/shared";

export function CreateGroupDialog({open, revision = 0, onClose, onCreated}: {open: boolean; revision?: number; onClose: () => void; onCreated: (grp: GroupRow) => void}) {
  const formID = useId();
  const [platform, setPlatform] = useState("openai");
  const [subType, setSubType] = useState("standard");
  const [exclusive, setExclusive] = useState(false);
  const [copyFromGroupIDs, setCopyFromGroupIDs] = useState<number[]>([]);
  const op = useAction();

  return (
    <Overlay open={open} title="新建调度分组" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>创建分组</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            const created = await api("/admin/groups", adminGroupRow, "POST", {
              name: String(fd.get("name") ?? "").trim(),
              description: String(fd.get("description") ?? "").trim(),
              platform,
              subscription_type: subType,
              rate_multiplier: Number(fd.get("rate_multiplier") || 1),
              is_exclusive: exclusive,
              ...(copyFromGroupIDs.length > 0 ? {copy_accounts_from_group_ids: copyFromGroupIDs} : {}),
            });
            onCreated(created);
          });
        }}
      >
        <Field label="分组名称"><Input name="name" required maxLength={100} placeholder="例如 Claude-Pro-主力组" /></Field>
        <FormGrid>
          <Field label="所属平台">
            <PlatformSelect value={platform} includeComposite clearable={false} onChange={setPlatform} />
          </Field>
          <Field label="基础计费倍率"><Input name="rate_multiplier" type="number" min="0" step="0.01" defaultValue="1" required /></Field>
        </FormGrid>
        <Field label="计费类型">
          <Segmented label="计费类型" value={subType as "standard" | "subscription"} options={[["standard", "按量计费"], ["subscription", "套餐订阅"]] as const} onChange={setSubType} />
        </Field>
        <Field label="分组说明（可选）"><Input name="description" maxLength={200} /></Field>
        <Field label="从已有分组复制账号（可选）">
          <MultiGroupSelect value={copyFromGroupIDs} revision={revision} onChange={setCopyFromGroupIDs} />
        </Field>
        <InsetGroup>
          <InsetRow label="设为专属分组" hint="仅授权给指定用户或开通订阅后可见">
            <Switch label="专属分组" checked={exclusive} onChange={setExclusive} />
          </InsetRow>
        </InsetGroup>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
