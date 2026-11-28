import {useId, useState, type ReactNode} from "react";
import {Plus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, passthroughRuleSchema} from "@/schemas";
import {useAction} from "@/lib/requests";
import {
  Badge, Button, ConfirmDialog, EmptyState, ErrorLine, Field, FilterBar, Form, FormGrid, Input, Overlay,
  RefreshButton, RemoteTable, RowActions, Select, Switch, TagInput, Textarea, WorkspaceStage, toast, type Column,
} from "@/ui";
import {joinNumbers, parseNumbers, validNumbers} from "./rule-helpers";

type Rule = z.infer<typeof passthroughRuleSchema>;

export function PassthroughView({switcher}: {switcher: ReactNode}) {
  const [revision, revise] = useState(0);
  const [editing, setEditing] = useState<Rule | "new">();
  const [deleting, setDeleting] = useState<Rule>();
  const del = useAction();
  const refresh = () => revise(v => v + 1);

  const columns: Column<Rule>[] = [
    {
      id: "name",
      title: "规则",
      render: row => (
        <div>
          <strong className="compact-text" title={row.name}>{row.name}</strong>
          {row.description && <div className="muted text-fine">{row.description}</div>}
        </div>
      ),
    },
    {id: "priority", title: "优先级", align: "number", render: row => row.priority},
    {
      id: "match",
      title: "匹配条件",
      hideWhenNarrow: true,
      render: row => (
        <span className="code-mono text-fine">
          {row.match_mode === "all" ? "全部满足" : "任一满足"} · {row.error_codes.length ? row.error_codes.join("/") : "任意状态码"}{row.keywords.length ? ` · ${row.keywords.length} 个关键词` : ""}
        </span>
      ),
    },
    {id: "platforms", title: "平台", hideWhenCompact: true, render: row => row.platforms.length ? row.platforms.join(", ") : <span className="muted">全部平台</span>},
    {
      id: "result",
      title: "响应处理",
      hideWhenNarrow: true,
      render: row => `${row.passthrough_code ? "透传状态码" : `状态码 ${row.response_code ?? "—"}`} · ${row.passthrough_body ? "透传正文" : "自定义正文"}`,
    },
    {id: "enabled", title: "状态", align: "center", render: row => <Badge variant="soft" tone={row.enabled ? "ok" : "neutral"}>{row.enabled ? "启用" : "停用"}</Badge>},
    {
      id: "actions",
      title: "操作",
      align: "end",
      render: row => (
        <RowActions>
          <Button size="xs" variant="outline" onClick={() => setEditing(row)}>编辑</Button>
          <Button size="xs" variant="destructive" onClick={() => setDeleting(row)}>删除</Button>
        </RowActions>
      ),
    },
  ];

  return (
    <>
      <WorkspaceStage flush>
        <FilterBar sticky>
          <div className="obs-filter-row">
            {switcher}
            <div className="obs-filter-spacer">
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新规则" onClick={refresh} />
              <Button onClick={() => setEditing("new")}><Plus aria-hidden />新建规则</Button>
            </div>
          </div>
        </FilterBar>
        <div className="swift-pane-table">
          <RemoteTable
            path="/admin/error-passthrough-rules"
            schema={passthroughRuleSchema}
            columns={columns}
            rowID={row => row.id}
            label="错误透传规则"
            revision={revision}
            onRowClick={row => setEditing(row)}
            empty={<EmptyState title="没有错误透传规则" description="未命中任何规则时，上游错误按默认方式处理。" action={<Button size="sm" onClick={() => setEditing("new")}><Plus aria-hidden />新建规则</Button>} />}
          />
        </div>
      </WorkspaceStage>

      {editing && (
        <RuleDialog
          key={editing === "new" ? "new" : editing.id}
          rule={editing === "new" ? undefined : editing}
          onClose={() => setEditing(undefined)}
          onSaved={() => { setEditing(undefined); toast("规则已保存"); refresh(); }}
        />
      )}
      <ConfirmDialog
        open={!!deleting}
        title="删除错误透传规则"
        description={`确定删除规则「${deleting?.name ?? ""}」吗？`}
        confirm="删除"
        critical
        busy={del.busy}
        error={del.error}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => void del.run(async () => {
          if (!deleting) return;
          await api(`/admin/error-passthrough-rules/${deleting.id}`, empty, "DELETE");
          setDeleting(undefined);
          toast("规则已删除");
          refresh();
        })}
      />
    </>
  );
}

function RuleDialog({rule, onClose, onSaved}: {rule?: Rule; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  const [name, setName] = useState(rule?.name ?? "");
  const [description, setDescription] = useState(rule?.description ?? "");
  const [enabled, setEnabled] = useState(rule?.enabled ?? true);
  const [priority, setPriority] = useState(String(rule?.priority ?? 0));
  const [codes, setCodes] = useState(joinNumbers(rule?.error_codes ?? []));
  const [keywords, setKeywords] = useState<string[]>(rule?.keywords ?? []);
  const [matchMode, setMatchMode] = useState(rule?.match_mode ?? "any");
  const [platforms, setPlatforms] = useState<string[]>(rule?.platforms ?? []);
  const [passCode, setPassCode] = useState(rule?.passthrough_code ?? true);
  const [responseCode, setResponseCode] = useState(rule?.response_code != null ? String(rule.response_code) : "");
  const [passBody, setPassBody] = useState(rule?.passthrough_body ?? true);
  const [message, setMessage] = useState(rule?.custom_message ?? "");
  const [skipMonitoring, setSkipMonitoring] = useState(rule?.skip_monitoring ?? false);

  return (
    <Overlay open title={rule ? "编辑错误透传规则" : "新建错误透传规则"} onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>保存规则</Button>
      </>
    }>
      <Form id={formID} busy={op.busy} onSubmit={() => void op.run(async () => {
        const errorCodes = parseNumbers(codes);
        if (!validNumbers(errorCodes, 100, 599)) throw new Error("状态码需为 100-599 的整数");
        if (errorCodes.length === 0 && keywords.length === 0) throw new Error("至少填写一个状态码或关键词");
        const body = {
          name: name.trim(),
          description: description.trim() || null,
          enabled,
          priority: Number(priority || 0),
          error_codes: errorCodes,
          keywords,
          match_mode: matchMode,
          platforms,
          passthrough_code: passCode,
          response_code: passCode || responseCode.trim() === "" ? null : Number(responseCode),
          passthrough_body: passBody,
          custom_message: passBody || message.trim() === "" ? null : message.trim(),
          skip_monitoring: skipMonitoring,
        };
        if (rule) await api(`/admin/error-passthrough-rules/${rule.id}`, passthroughRuleSchema, "PUT", body);
        else await api("/admin/error-passthrough-rules", passthroughRuleSchema, "POST", body);
        onSaved();
      })}>
        <Field label="规则名称"><Input value={name} required maxLength={100} onChange={e => setName(e.target.value)} /></Field>
        <Field label="说明（可选）"><Input value={description} maxLength={500} onChange={e => setDescription(e.target.value)} /></Field>
        <FormGrid>
          <Field label="优先级（数字小的先匹配）"><Input type="number" min="0" max="100000" value={priority} onChange={e => setPriority(e.target.value)} /></Field>
          <Field label="多条件关系">
            <Select value={matchMode} onChange={e => setMatchMode(e.target.value)}>
              <option value="any">任一条件命中</option>
              <option value="all">全部条件命中</option>
            </Select>
          </Field>
        </FormGrid>
        <Field label="上游状态码（逗号分隔）"><Input value={codes} placeholder="429, 503" onChange={e => setCodes(e.target.value)} /></Field>
        <Field label="错误关键词"><TagInput value={keywords} onChange={setKeywords} /></Field>
        <Field label="作用平台（留空为全部）"><TagInput value={platforms} placeholder="如 openai，回车添加" onChange={setPlatforms} /></Field>
        <Field label="透传上游状态码"><Switch label="透传上游状态码" checked={passCode} onChange={setPassCode} /></Field>
        {!passCode && <Field label="自定义响应状态码"><Input type="number" min="100" max="599" value={responseCode} onChange={e => setResponseCode(e.target.value)} /></Field>}
        <Field label="透传上游错误正文"><Switch label="透传上游错误正文" checked={passBody} onChange={setPassBody} /></Field>
        {!passBody && <Field label="自定义错误消息"><Textarea value={message} maxLength={2000} rows={3} onChange={e => setMessage(e.target.value)} /></Field>}
        <Field label="命中后不计入监控"><Switch label="跳过监控" checked={skipMonitoring} onChange={setSkipMonitoring} /></Field>
        <Field label="启用规则"><Switch label="启用规则" checked={enabled} onChange={setEnabled} /></Field>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
