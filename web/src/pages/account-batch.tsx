import {useState} from "react";
import {z} from "zod";
import {account, api, describeError, named, operationTrace, testAccount} from "../api";
import {useAction} from "../requests";
import {Button, ErrorLine, InsetGroup, InsetRow, Overlay, Segmented} from "../ui";

type Target = z.infer<typeof named>;
type BatchOperation = "test" | "refresh" | "recover";
type BatchResult = {ok: boolean; message: string};
const operations = [["test", "测活"], ["refresh", "刷新凭据"], ["recover", "恢复调度"]] as const;

export function AccountBatch({targets, onClose, onChanged}: {targets: Target[]; onClose: () => void; onChanged: () => void}) {
  const action = useAction();
  const [operation, setOperation] = useState<BatchOperation>("test");
  const [results, setResults] = useState<Record<number, BatchResult>>({});
  const completed = Object.keys(results).length;
  const failed = targets.filter(target => results[target.id]?.ok === false);
  const run = (rows: Target[]) => void action.run(async () => {
    const trace = operationTrace();
    for (const row of rows) {
      let result: BatchResult;
      try {
        if (operation === "test") {
          let passed = false;
          await testAccount(row.id, "", status => { passed = status === "passed"; }, trace);
          if (!passed) throw new Error("测活失败");
        } else {
          await api("/admin/accounts/" + row.id + "/" + operation, account, "POST", {}, undefined, trace);
        }
        result = {ok: true, message: operation === "test" ? "测活通过" : operation === "refresh" ? "凭据已刷新" : "已恢复调度"};
      } catch (error) {
        result = {ok: false, message: describeError(error)};
      }
      setResults(previous => ({...previous, [row.id]: result}));
    }
    onChanged();
  });
  return <Overlay open title="批量处理上游账号" onClose={onClose} busy={action.busy} footer={<>
    <Button variant="outline" onClick={onClose}>关闭</Button>
    {completed === 0 && <Button onClick={() => run(targets)}>开始{operations.find(([value]) => value === operation)?.[1]}</Button>}
    {completed > 0 && failed.length > 0 && <Button onClick={() => run(failed)}>重试失败项（{failed.length}）</Button>}
  </>}>
    {completed === 0 && !action.busy && <Segmented label="批量操作类型" options={operations} value={operation} onChange={setOperation} />}
    <InsetGroup title={"共 " + targets.length + " 个账号"}>
      {targets.map(row => <InsetRow key={row.id} label={row.name}>
        {results[row.id]?.ok === false ? <ErrorLine value={results[row.id].message} /> : results[row.id]?.message ?? "待处理"}
      </InsetRow>)}
    </InsetGroup>
    <ErrorLine value={action.error} />
  </Overlay>;
}
