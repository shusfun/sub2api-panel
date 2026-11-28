import {useId, useState} from "react";
import {api} from "@/lib/http";
import {useAction} from "@/lib/requests";
import {affiliateBatchResult} from "@/schemas";
import {BatchBar, Button, ErrorLine, Field, Form, Input, Overlay, toast} from "@/ui";

export function BatchRateBar({userIDs, onClear, onDone}: {userIDs: number[]; onClear: () => void; onDone: () => void}) {
  const [open, setOpen] = useState(false);
  const formID = useId();
  const op = useAction();
  const [rate, setRate] = useState("");

  const submit = (body: Record<string, unknown>) => void op.run(async () => {
    const res = await api("/admin/affiliates/users/batch-rate", affiliateBatchResult, "POST", {user_ids: userIDs, ...body});
    setOpen(false);
    onClear();
    toast(`已更新 ${res.affected} 位用户的返利比例`);
    onDone();
  });

  return (
    <>
      <BatchBar count={userIDs.length} onClear={onClear}>
        <Button size="xs" variant="outline" onClick={() => setOpen(true)}>批量设置返利比例</Button>
      </BatchBar>
      <Overlay open={open} title="批量设置返利比例" onClose={() => setOpen(false)} busy={op.busy} footer={
        <>
          <Button variant="ghost" disabled={op.busy} onClick={() => submit({clear: true})}>恢复全局比例</Button>
          <Button variant="outline" onClick={() => setOpen(false)}>取消</Button>
          <Button type="submit" form={formID} busy={op.busy}>应用到 {userIDs.length} 位用户</Button>
        </>
      }>
        <Form id={formID} busy={op.busy} onSubmit={() => submit({aff_rebate_rate_percent: Number(rate)})}>
          <Field label="返利比例（0-100 %）"><Input type="number" min="0" max="100" step="0.1" value={rate} required onChange={e => setRate(e.target.value)} /></Field>
          <ErrorLine value={op.error} />
        </Form>
      </Overlay>
    </>
  );
}
