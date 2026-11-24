import {useRef, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {named} from "@/schemas";
import {date, money} from "@/lib/format";
import {useAction} from "@/lib/requests";
import {
  Button, DataTable, DetailHeader, EllipsisText, EmptyState, ErrorLine, Field, FilePicker, Form, FormGrid, InlineStack, Input,
  InsetGroup, InsetRow, MaskedSecret, Overlay, Pagination, RefreshButton, Select, Status, Textarea,
  WorkspaceContent, toast, type Column,
} from "@/ui";
import {RemoteTable} from "@/ui";

const keyRow = named.extend({masked: z.string(), batch_image: z.boolean().optional(), status: z.string()});
const job = z.object({
  id: z.string(), task_name: z.string(), status: z.string(), model: z.string(), item_count: z.number(), success_count: z.number(),
  fail_count: z.number(), estimated_cost: z.number(), hold_amount: z.number(), actual_cost: z.number().nullable(), created_at: z.number(),
  parent_batch_id: z.string().nullish(), provider: z.string().optional(),
});
const jobs = z.object({data: z.array(job), has_more: z.boolean()});
const models = z.object({data: z.array(z.object({id: z.string()}))});
const item = z.object({custom_id: z.string(), status: z.string(), image_count: z.number(),
  prompt_preview: z.string().nullish(),
  error: z.object({code: z.string()}).nullish()});
const items = z.object({data: z.array(item), has_more: z.boolean()});
type Job = z.infer<typeof job>;
const base = "/v1/images/batches";
const pageSize = 20;
const referenceMimes = ["image/png", "image/jpeg", "image/webp"];
const active = new Set(["queued", "running", "indexing", "processing_results", "settling"]);

async function gatewayRequest(gateway: string, path: string, key: string, init?: RequestInit) {
  const headers = new Headers(init?.headers);
  headers.set("Authorization", "Bearer " + key);
  let response: Response;
  try {
    response = await fetch(new URL(path, gateway), {...init, headers, credentials: "omit", cache: "no-store",
      redirect: "error", signal: AbortSignal.timeout(25000)});
  } catch {
    throw new Error("无法连接官方网关。请检查是否允许当前站点跨域访问。");
  }
  if (response.status === 401 || response.status === 403) throw new Error("当前密钥、分组或请求来源无权访问批量生图。");
  if (!response.ok) throw new Error("官方批量生图请求失败（" + response.status + "）");
  return response;
}

function referenceData(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("无法读取参考图。"));
    reader.onload = () => resolve(String(reader.result).split(",", 2)[1]);
    reader.readAsDataURL(file);
  });
}

function pagedPath(path: string, page: number) {
  return path + "?" + new URLSearchParams({limit: String(pageSize), cursor: String((page - 1) * pageSize)});
}

export function BatchImages({gateway}: {gateway: string}) {
  const [selected, select] = useState<{id: number; name: string; masked: string}>();
  return <WorkspaceContent>
    {selected ? <BatchJobWorkspace key={selected.id} gateway={gateway} selected={selected} onBack={() => select(undefined)} /> :
      <RemoteTable path="/keys" schema={keyRow} rowID={row => row.id} columns={[]} label="批量生图密钥"
        renderItems={rows => <InsetGroup title="选择已开启批量生图的 Gemini 密钥">
          {rows.filter(row => row.batch_image && row.status === "active").map(row => (
            <InsetRow key={row.id} label={row.name} onClick={() => select(row)}>
              <MaskedSecret masked={row.masked} />
            </InsetRow>
          ))}
          {!rows.some(row => row.batch_image && row.status === "active") && <EmptyState title="当前页没有可用于批量生图的密钥"
            action={<Button variant="outline" render={<a href="/keys" />}>打开我的密钥</Button>} />}
        </InsetGroup>} />}
  </WorkspaceContent>;
}

function BatchJobWorkspace({gateway, selected, onBack}: {gateway: string; selected: {id: number; name: string; masked: string}; onBack: () => void}) {
  const [catalog, setCatalog] = useState<string[]>();
  const [history, setHistory] = useState<z.infer<typeof jobs>>();
  const [historyPage, setHistoryPage] = useState(1);
  const [opened, open] = useState<Job>();
  const [refFiles, setRefFiles] = useState<File[]>([]);
  const action = useAction();
  const attempt = useRef<{body: string; id: string} | undefined>(undefined);
  const secret = () => api("/keys/" + selected.id + "/secret", z.object({key: z.string().min(1)})).then(row => row.key);
  const request = async (path: string, init?: RequestInit) => gatewayRequest(gateway, path, await secret(), init);
  const refresh = async (page = historyPage) => {
    setHistory(jobs.parse(await (await request(pagedPath(base, page))).json()));
    setHistoryPage(page);
  };
  const columns: Column<Job>[] = [
    {id: "name", title: "任务", render: row => <EllipsisText text={row.task_name || row.id} />},
    {id: "model", title: "模型", hideWhenCompact: true, render: row => <EllipsisText text={row.model} mono />},
    {id: "state", title: "状态", align: "center", render: row => <Status value={row.status} />},
    {id: "progress", title: "成功 / 失败 / 合计", align: "number", hideWhenCompact: true, render: row => [row.success_count, row.fail_count, row.item_count].join(" / ")},
    {id: "cost", title: "冻结 / 实扣", align: "number", hideWhenNarrow: true, render: row => money(row.hold_amount) + " / " + (row.actual_cost == null ? "—" : money(row.actual_cost))},
    {id: "action", title: "操作", align: "end", render: row => <Button variant="ghost" size="sm" onClick={() => open(row)}>任务详情</Button>},
  ];
  return <>
    <DetailHeader
      backLabel="更换密钥"
      title={selected.name}
      badges={<MaskedSecret masked={selected.masked} />}
      onBack={onBack}
    />
    <InsetGroup title="提交批量生图任务">
      {!catalog ? <InsetRow label="初始化模型目录与历史任务"><Button busy={action.busy} onClick={() => void action.run(async () => {
        const loaded = models.parse(await (await request(base + "/models")).json());
        setCatalog(loaded.data.map(row => row.id));
        await refresh();
      })}>加载可用模型</Button></InsetRow> :
        catalog.length ? <InsetRow label="新建批次" vertical><Form busy={action.busy} onSubmit={event => {
          const fields = new FormData(event.currentTarget);
          void action.run(async () => {
            const prompts = String(fields.get("prompts") ?? "").split(/\r?\n/).map(line => line.trim()).filter(Boolean);
            if (!prompts.length || prompts.length > 100 || prompts.some(prompt => prompt.length > 8000)) throw new Error("请输入 1 到 100 行提示词，每行最多 8,000 字符。");
            const model = String(fields.get("model"));
            const referenceLimit = model.includes("pro-image") ? 14 : 3;
            if (refFiles.length > referenceLimit || refFiles.some(file => !referenceMimes.includes(file.type))) {
              throw new Error("当前模型最多接受 " + referenceLimit + " 张 PNG、JPEG 或 WebP 参考图。");
            }
            const total = refFiles.reduce((size, file) => size + file.size, 0) * prompts.length;
            if (total > 128 * 1024 * 1024 || refFiles.length * prompts.length > 1000) throw new Error("参考图总量超出官方批次上限。");
            const reference_images = await Promise.all(refFiles.map(async file => ({mime_type: file.type, data: await referenceData(file)})));
            const body = JSON.stringify({task_name: fields.get("task_name"), model, image_size: fields.get("image_size"),
              items: prompts.map((prompt, index) => ({custom_id: "image-" + (index + 1), prompt, output_count: 1,
                ...(reference_images.length ? {reference_images} : {})}))});
            if (attempt.current?.body !== body) attempt.current = {body, id: crypto.randomUUID()};
            const result = job.parse(await (await request(base, {method: "POST", body,
              headers: {"Content-Type": "application/json", "Idempotency-Key": attempt.current.id}})).json());
            attempt.current = undefined;
            setRefFiles([]);
            toast(`已提交 ${result.item_count} 项生图任务，冻结 ${money(result.hold_amount)}`);
            await refresh(1);
          });
        }}>
          <FormGrid cols={3}>
            <Field label="任务名称"><Input name="task_name" maxLength={100} /></Field>
            <Field label="模型"><Select name="model">{catalog.map(model => <option key={model}>{model}</option>)}</Select></Field>
            <Field label="输出分辨率"><Select name="image_size">{["1K", "2K", "4K"].map(size => <option key={size}>{size}</option>)}</Select></Field>
          </FormGrid>
          <Field label="提示词（每行生成一张图）"><Textarea name="prompts" rows={5} required /></Field>
          <Field label="参考图（选填，应用于本批次全部条目）">
            <FilePicker
              accept={referenceMimes.join(",")}
              multiple
              maxFiles={14}
              hint="支持 PNG、JPEG 或 WebP"
              value={refFiles}
              onChange={setRefFiles}
            />
          </Field>
          <Button type="submit" busy={action.busy}>提交批量任务</Button>
        </Form></InsetRow> : <InsetRow label="当前密钥没有可用的批量生图模型" />}
      <ErrorLine value={action.error} />
    </InsetGroup>
    {history && <InsetGroup title="最近任务" actions={<RefreshButton size="sm" busy={action.busy} onClick={() => void action.run(() => refresh())} />}>
      <DataTable label="最近批量生图任务" rows={history.data} rowID={row => row.id} columns={columns} />
      <Pagination label="批量任务分页" number={historyPage} more={history.has_more} onChange={page => void action.run(() => refresh(page))} />
    </InsetGroup>}
    {opened && <JobDetail key={opened.id} initial={opened} request={request} onChanged={() => void action.run(() => refresh())} onClose={() => open(undefined)} />}
  </>;
}

function JobDetail({initial, request, onChanged, onClose}: {initial: Job; request: (path: string, init?: RequestInit) => Promise<Response>;
  onChanged: () => void; onClose: () => void}) {
  const [row, setRow] = useState(initial);
  const [detail, setDetail] = useState<z.infer<typeof items>>();
  const [page, setPage] = useState(1);
  const [retry, setRetry] = useState<z.infer<typeof item>>();
  const attempt = useRef<{body: string; id: string} | undefined>(undefined);
  const action = useAction();
  const path = base + "/" + encodeURIComponent(row.id);
  const loadItems = async (number: number) => {
    setDetail(items.parse(await (await request(pagedPath(path + "/items", number))).json()));
    setPage(number);
  };
  return <Overlay open drawer title={row.task_name || "批量任务"} onClose={onClose} busy={action.busy}>
    <InsetGroup>
      <InsetRow label="状态"><Status value={row.status} /></InsetRow>
      <InsetRow label="提交时间" hint={date(row.created_at * 1000)} />
      <InsetRow label="预计费用" hint={money(row.estimated_cost)} />
      <InsetRow label="任务操作"><InlineStack wrap>
        <RefreshButton size="sm" busy={action.busy} label="刷新并看明细" onClick={() => void action.run(async () => {
          setRow(job.parse(await (await request(path)).json()));
          await loadItems(page);
          onChanged();
        })} />
        {active.has(row.status) && <Button size="sm" variant="danger-outline" busy={action.busy} onClick={() => void action.run(async () => {
          setRow(job.parse(await (await request(path + "/cancel", {method: "POST"})).json()));
          toast("已请求取消批量任务");
          onChanged();
        })}>取消任务</Button>}
        {row.status === "completed" && row.success_count > 0 && <Button size="sm" busy={action.busy} onClick={() => void action.run(async () => {
          const uri = URL.createObjectURL(await (await request(path + "/download")).blob());
          const link = document.createElement("a");
          link.href = uri;
          link.download = "batch-images-" + row.id.replace(/[^a-zA-Z0-9_-]/g, "") + ".zip";
          link.click();
          setTimeout(() => URL.revokeObjectURL(uri), 1000);
        })}>下载图片包</Button>}
      </InlineStack></InsetRow>
      <ErrorLine value={action.error} />
    </InsetGroup>
    {retry && <InsetGroup title="重新提交失败条目"><Form busy={action.busy} onSubmit={event => {
      const prompt = String(new FormData(event.currentTarget).get("retry_prompt") ?? "").trim();
      void action.run(async () => {
        if (!prompt) throw new Error("请输入用于重试的提示词。");
        const body = JSON.stringify({parent_batch_id: row.parent_batch_id || row.id, model: row.model, provider: row.provider,
          image_size: "1K", items: [{custom_id: retry.custom_id, prompt, output_count: 1}]});
        if (attempt.current?.body !== body) attempt.current = {body, id: crypto.randomUUID()};
        const result = job.parse(await (await request(base, {method: "POST", body,
          headers: {"Content-Type": "application/json", "Idempotency-Key": attempt.current.id}})).json());
        attempt.current = undefined;
        toast("已提交补跑任务 " + result.id);
        setRetry(undefined);
        onChanged();
      });
    }}>
      <Field label="重试提示词" hint="生成一张 1K 图片并按新任务计费。">
        <Textarea key={retry.custom_id} name="retry_prompt" defaultValue={retry.prompt_preview ?? ""} required maxLength={8000} rows={4} />
      </Field>
      <InlineStack><Button variant="ghost" onClick={() => setRetry(undefined)}>取消重试</Button><Button type="submit" busy={action.busy}>提交补跑任务</Button></InlineStack>
    </Form></InsetGroup>}
    {detail && <DataTable rows={detail.data} rowID={item => item.custom_id} label="批量任务明细" columns={[
      {id: "id", title: "条目", render: item => <span className="code-mono">{item.custom_id}</span>},
      {id: "result", title: "状态", align: "center", render: item => <Status value={item.status} />},
      {id: "count", title: "生成图片", align: "number", render: item => item.image_count},
      {id: "error", title: "异常代码", render: item => item.error?.code ?? "—"},
      {id: "retry", title: "操作", align: "end", render: item => !active.has(row.status) && item.status === "failed" &&
        <Button size="sm" variant="ghost" onClick={() => { action.clearError(); setRetry(item); }}>重试</Button>},
    ]} />}
    {detail && <Pagination label="条目分页" number={page} more={detail.has_more} onChange={number => void action.run(() => loadItems(number))} />}
  </Overlay>;
}
