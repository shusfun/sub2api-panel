import {useId, useState} from "react";
import {api} from "@/lib/http";
import {channelRowSchema} from "@/schemas";
import {useAction} from "@/lib/requests";
import {Button, ErrorLine, Field, Form, Input, MultiGroupSelect, Overlay, Select} from "@/ui";
import {type ChannelRow} from "@/features/pool/shared";

export function CreateChannelDialog({open, revision, onClose, onCreated}: {open: boolean; revision: number; onClose: () => void; onCreated: (ch: ChannelRow) => void}) {
  const formID = useId();
  const [groupIDs, setGroupIDs] = useState<number[]>([]);
  const op = useAction();

  return (
    <Overlay open={open} title="新建渠道调价模板" onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>创建并配置价目</Button>
      </>
    }>
      <Form id={formID}
        busy={op.busy}
        onSubmit={e => {
          const fd = new FormData(e.currentTarget);
          void op.run(async () => {
            const created = await api("/admin/channels", channelRowSchema, "POST", {
              name: String(fd.get("name") ?? "").trim(),
              description: String(fd.get("description") ?? "").trim(),
              billing_model_source: String(fd.get("billing_model_source") ?? "requested"),
              group_ids: groupIDs,
            });
            onCreated(created);
          });
        }}
      >
        <Field label="模板名称"><Input name="name" required maxLength={100} placeholder="企业专属阶梯价目" /></Field>
        <Field label="计费模型名称基准">
          <Select name="billing_model_source" defaultValue="requested">
            <option value="requested">按客户端请求模型名 (requested)</option>
            <option value="upstream">按实际发往上游模型名 (upstream)</option>
            <option value="response_model">按上游响应返回模型名 (response_model)</option>
          </Select>
        </Field>
        <Field label="绑定调度分组"><MultiGroupSelect value={groupIDs} revision={revision} onChange={setGroupIDs} /></Field>
        <Field label="说明（可选）"><Input name="description" maxLength={200} /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
