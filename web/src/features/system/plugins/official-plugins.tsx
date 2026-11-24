import {useState} from "react";
import {Plus, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, named} from "@/schemas";
import {useAction, useResource} from "@/lib/requests";
import {
  Button, Checkbox, DataTable, EllipsisText, EmptyState, ErrorLine, Field, FilePicker, Form, InlineStack, Input, InsetGroup, InsetRow,
  Resource, SecretInput, Stack, Status, StepUpDialog, Switch, Textarea, toast, type Column,
} from "@/ui";

const base = "/admin/plugins/upstream";
const plugin = named.extend({
  version: z.string(),
  description: z.string(),
  state: z.string(),
  runtime_healthy: z.boolean(),
  compatibility: z.object({status: z.string()}),
});
const configFieldSchema = z.object({
  key: z.string(),
  kind: z.enum(["secret", "bool", "number", "string", "json"]),
  value: z.unknown().optional(),
  configured: z.boolean().optional(),
});
const configViewSchema = z.object({fields: z.array(configFieldSchema)});

type Plugin = z.infer<typeof plugin>;
type ConfigField = z.infer<typeof configFieldSchema>;
type Action = "enable" | "disable" | "test" | "config" | "remove";
const actions: Record<Action, string> = {enable: "启用", disable: "停用", test: "测试", config: "配置", remove: "卸载"};

export function OfficialPlugins({stepUpEnabled = false}: {stepUpEnabled?: boolean}) {
  const [revision, revise] = useState(0);
  const [selection, select] = useState<{row: Plugin; action: Action}>();
  const [accept, setAccept] = useState(false);
  const [rollout, setRollout] = useState("100");
  const [uploading, setUploading] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const op = useAction();
  const upload = useAction();
  const rows = useResource(() => api(base, z.array(plugin)), base, true, revision);
  const pick = (row: Plugin, action: Action) => {
    setAccept(false);
    setRollout("100");
    op.clearError();
    select({row, action});
  };

  const columns: Column<Plugin>[] = [
    {id: "name", title: "插件", render: row => <div><EllipsisText text={row.name} className="font-semibold" />{row.description && <EllipsisText text={row.description} className="muted text-fine" />}</div>},
    {id: "version", title: "版本", hideWhenNarrow: true, render: row => <span className="code-mono">{row.version}</span>},
    {id: "compat", title: "兼容性", align: "center", hideWhenCompact: true, render: row => <Status value={row.compatibility.status} />},
    {id: "state", title: "运行状态", align: "center", render: row => <Status value={row.state} />},
    {id: "actions", title: "操作", align: "end", render: row => <InlineStack>
      {row.state === "enabled" ? <>
        <Button size="sm" variant="ghost" onClick={() => pick(row, "test")}>测试</Button>
        <Button size="sm" variant="ghost" onClick={() => pick(row, "disable")}>停用</Button>
      </> : <Button size="sm" variant="ghost" disabled={row.compatibility.status === "incompatible"} onClick={() => pick(row, "enable")}>启用</Button>}
      <Button size="sm" variant="ghost" onClick={() => pick(row, "config")}>配置</Button>
      <Button size="sm" variant="destructive" onClick={() => pick(row, "remove")}>卸载</Button>
    </InlineStack>},
  ];

  return (
    <InsetGroup title="官方网关内核插件" actions={<Button size="sm" variant="outline" onClick={() => { upload.clearError(); setFiles([]); setUploading(true); }}>安装插件</Button>}>
      <Resource state={rows}>{items => items.length ? <DataTable label="官方网关插件" rows={items} rowID={row => row.id} columns={columns} /> :
        <EmptyState title="官方网关尚未安装插件" />}</Resource>

      {uploading && (
        <StepUpDialog
          title="安装官方网关插件"
          requireTotp={stepUpEnabled}
          confirmLabel="上传并安装"
          busy={upload.busy}
          error={upload.error}
          fieldErrors={upload.fieldErrors}
          onClose={() => setUploading(false)}
          onConfirm={({totp}) =>
            void upload.run(async () => {
              const file = files[0];
              if (!file || !file.name.toLowerCase().endsWith(".s2plugin") || !file.size || file.size > 128 * 1024 * 1024) {
                throw new Error("请选择 128 MiB 以内的有效 .s2plugin 安装包。");
              }
              if (totp) await api(base + "/upload/authorize", empty, "POST", {totp});
              await api(base + "/upload", plugin, "POST", file);
              setUploading(false);
              setFiles([]);
              toast("网关插件已上传安装");
              revise(value => value + 1);
            })
          }
        >
          <Field label="插件安装包" name="plugin">
            <FilePicker
              name="plugin"
              accept=".s2plugin"
              maxBytes={128 * 1024 * 1024}
              hint="仅支持 .s2plugin 包，单包最大 128 MiB"
              value={files}
              onChange={setFiles}
              required
            />
          </Field>
        </StepUpDialog>
      )}

      {selection && selection.action === "config" && (
        <PluginConfigEditor
          row={selection.row}
          stepUpEnabled={stepUpEnabled}
          onClose={() => select(undefined)}
          onSaved={() => {
            select(undefined);
            toast("插件配置已保存");
            revise(v => v + 1);
          }}
        />
      )}

      {selection && selection.action !== "config" && (
        <StepUpDialog
          title={`${actions[selection.action]}「${selection.row.name}」`}
          description={selection.action === "remove" ? "卸载将移除此官方网关插件及其安装记录。" : undefined}
          requireTotp={stepUpEnabled}
          critical={selection.action === "remove"}
          confirmLabel={actions[selection.action] + "插件"}
          busy={op.busy}
          error={op.error}
          fieldErrors={op.fieldErrors}
          onClose={() => select(undefined)}
          onConfirm={({totp}) =>
            void op.run(async () => {
              const selectionPath = base + "/" + selection.row.id;
              const actionPath = selectionPath + "/" + selection.action;
              const payload = {...(totp ? {totp} : {}), rollout_percent: Number(rollout || 100), accept_untested: accept};
              if (selection.action === "remove") {
                await api(selectionPath, empty, "DELETE", totp ? {totp} : {});
                toast("插件已卸载");
              } else if (selection.action === "test") {
                const result = await api(actionPath, z.object({success: z.boolean(), latency_ms: z.number()}), "POST", payload);
                if (!result.success) throw new Error("官方插件测试未通过");
                toast(`插件测试通过（${result.latency_ms} ms）`);
              } else {
                await api(actionPath, empty, "POST", payload);
                toast(`插件已${actions[selection.action]}`);
              }
              select(undefined);
              revise(value => value + 1);
            })
          }
        >
          {selection.action === "enable" && (
            <>
              <Field label="灰度启用比例" name="rollout_percent">
                <Input
                  name="rollout_percent"
                  type="number"
                  suffix="%"
                  min={1}
                  max={100}
                  value={rollout}
                  onChange={e => setRollout(e.currentTarget.value)}
                  required
                />
              </Field>
              {selection.row.compatibility.status === "untested" && (
                <Checkbox label="接受当前版本未验证" checked={accept} onChange={setAccept} required>
                  接受当前官方版本未经兼容性验证
                </Checkbox>
              )}
            </>
          )}
        </StepUpDialog>
      )}
    </InsetGroup>
  );
}

type DraftItem = {
  key: string;
  kind: ConfigField["kind"];
  configured?: boolean;
  rawText: string;
  boolVal: boolean;
};

function toDraftItems(fields: readonly ConfigField[]): DraftItem[] {
  return fields.map(f => ({
    key: f.key,
    kind: f.kind,
    configured: f.configured,
    boolVal: f.kind === "bool" ? Boolean(f.value) : false,
    rawText:
      f.kind === "secret"
        ? ""
        : f.kind === "string"
          ? String(f.value ?? "")
          : f.kind === "number"
            ? String(f.value ?? 0)
            : f.kind === "json"
              ? JSON.stringify(f.value ?? null, null, 2)
              : "",
  }));
}

function PluginConfigEditor({
  row,
  stepUpEnabled,
  onClose,
  onSaved,
}: {
  row: Plugin;
  stepUpEnabled: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const endpoint = `${base}/${row.id}/config`;
  const loaded = useResource(() => api(endpoint, configViewSchema), endpoint, true);
  const action = useAction();

  return (
    <Resource state={loaded}>
      {data => (
        <PluginConfigDialogInner
          row={row}
          initial={toDraftItems(data.fields)}
          stepUpEnabled={stepUpEnabled}
          action={action}
          endpoint={endpoint}
          onClose={onClose}
          onSaved={onSaved}
        />
      )}
    </Resource>
  );
}

function PluginConfigDialogInner({
  row,
  initial,
  stepUpEnabled,
  action,
  endpoint,
  onClose,
  onSaved,
}: {
  row: Plugin;
  initial: DraftItem[];
  stepUpEnabled: boolean;
  action: ReturnType<typeof useAction>;
  endpoint: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [items, setItems] = useState<DraftItem[]>(initial);
  const [newKey, setNewKey] = useState("");

  const updateItem = (idx: number, patch: Partial<DraftItem>) =>
    setItems(prev => prev.map((it, i) => (i === idx ? {...it, ...patch} : it)));

  return (
    <StepUpDialog
      title={`配置「${row.name}」`}
      requireTotp={stepUpEnabled}
      confirmLabel="保存插件配置"
      busy={action.busy}
      error={action.error}
      fieldErrors={action.fieldErrors}
      onClose={onClose}
      onConfirm={({totp}) =>
        void action.run(async () => {
          const config: Record<string, unknown> = {};
          const preserve_secrets: string[] = [];
          for (const item of items) {
            const key = item.key.trim();
            if (!key) continue;
            if (item.kind === "secret") {
              if (item.rawText.trim() !== "") {
                config[key] = item.rawText;
              } else if (item.configured) {
                preserve_secrets.push(key);
              } else {
                config[key] = "";
              }
            } else if (item.kind === "bool") {
              config[key] = item.boolVal;
            } else if (item.kind === "number") {
              const num = Number(item.rawText);
              if (!Number.isFinite(num)) throw new Error(`字段「${key}」必须是有效数字`);
              config[key] = num;
            } else if (item.kind === "json") {
              try {
                config[key] = JSON.parse(item.rawText);
              } catch {
                throw new Error(`字段「${key}」必须是合法 JSON`);
              }
            } else {
              config[key] = item.rawText;
            }
          }
          await api(endpoint, empty, "PUT", {
            ...(totp ? {totp} : {}),
            config,
            preserve_secrets,
          });
          onSaved();
        })
      }
    >
      <Stack>
        {items.length === 0 && <div className="muted text-fine">当前插件暂无已保存配置，可在下方添加字段。</div>}
        {items.map((item, idx) =>
          item.kind === "bool" ? (
            <InsetRow key={item.key} label={item.key}>
              <Switch label={item.key} checked={item.boolVal} onChange={boolVal => updateItem(idx, {boolVal})} />
              <Button type="button" size="icon-xs" variant="ghost" title="移除字段" aria-label={`移除 ${item.key}`} onClick={() => setItems(prev => prev.filter((_, i) => i !== idx))}>
                <Trash2 aria-hidden />
              </Button>
            </InsetRow>
          ) : (
            <Field
              key={item.key}
              name={item.key}
              label={item.key}
            >
              {item.kind === "secret" ? (
                <InlineStack>
                  <div className="flex-1 min-w-0">
                    <SecretInput
                      label={item.key}
                      configured={item.configured}
                      value={item.rawText}
                      onChange={rawText => updateItem(idx, {rawText})}
                    />
                  </div>
                  <Button type="button" size="icon-xs" variant="ghost" title="移除字段" aria-label={`移除 ${item.key}`} onClick={() => setItems(prev => prev.filter((_, i) => i !== idx))}>
                    <Trash2 aria-hidden />
                  </Button>
                </InlineStack>
              ) : item.kind === "json" ? (
                <Stack>
                  <Textarea rows={4} value={item.rawText} onChange={e => updateItem(idx, {rawText: e.currentTarget.value})} />
                  <InlineStack>
                    <Button type="button" size="xs" variant="ghost" onClick={() => setItems(prev => prev.filter((_, i) => i !== idx))}>移除此 JSON 字段</Button>
                  </InlineStack>
                </Stack>
              ) : (
                <Input
                  type={item.kind === "number" ? "number" : "text"}
                  step={item.kind === "number" ? "any" : undefined}
                  value={item.rawText}
                  onChange={e => updateItem(idx, {rawText: e.currentTarget.value})}
                  endAddon={
                    <Button type="button" size="icon-xs" variant="ghost" title="移除字段" aria-label={`移除 ${item.key}`} onClick={() => setItems(prev => prev.filter((_, i) => i !== idx))}>
                      <Trash2 aria-hidden />
                    </Button>
                  }
                />
              )}
            </Field>
          ),
        )}
        <Field label="新增配置项">
          <Input
            value={newKey}
            placeholder="输入新字段名（含 key/secret 自动识别为密钥）"
            onChange={e => setNewKey(e.currentTarget.value)}
            endAddon={
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={!newKey.trim() || items.some(i => i.key === newKey.trim())}
                onClick={() => {
                  const key = newKey.trim();
                  const isSecret = /secret|token|password|api_key|apikey|credential/i.test(key);
                  setItems(prev => [...prev, {key, kind: isSecret ? "secret" : "string", boolVal: false, rawText: ""}]);
                  setNewKey("");
                }}
              >
                <Plus aria-hidden />添加
              </Button>
            }
          />
        </Field>
        <ErrorLine value={action.error} />
      </Stack>
    </StepUpDialog>
  );
}
