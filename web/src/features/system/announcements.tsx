import {useId, useState} from "react";
import {CheckCheck, Eye, Pencil, Trash2} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty} from "@/schemas";
import {count, date, dateTimeInputSeconds, dateTimeInputValue} from "@/lib/format";
import {useAction, useDebouncedValue, useResource} from "@/lib/requests";
import {
  AudienceRuleBuilder, Badge, Button, ConfirmDialog, EllipsisText, EmptyState, EntityLink, ErrorLine, Field, FilterBar, FilterSlot, Form, FormGrid,
  InlineStack, InsetGroup, InsetRow, Input, MetricStrip, Overlay, RefreshButton, Resource, SearchInput, Select, Stack, Status,
  Textarea, WorkspaceContent, WorkspaceStage, toast, type AudienceCondition, type Column,
} from "@/ui";
import {RemoteTable} from "@/ui";

const userAnnouncement = z.object({id: z.number(), title: z.string(), content: z.string(), notify_mode: z.string(), read_at: z.string().nullish(), created_at: z.string()});
const userAnnouncements = z.array(userAnnouncement);
type Notice = z.infer<typeof userAnnouncement>;
const statuses = {draft: "草稿", active: "已发布", archived: "已归档"};
const announcement = z.object({
  id: z.number(), title: z.string(), status: z.enum(["draft", "active", "archived"]), notify_mode: z.enum(["silent", "popup"]),
  starts_at: z.string().nullish(), ends_at: z.string().nullish(),
});
const balanceOperator = z.enum(["gt", "gte", "lt", "lte", "eq"]);
const condition = z.discriminatedUnion("type", [
  z.object({type: z.literal("subscription"), operator: z.literal("in"), group_ids: z.array(z.number())}),
  z.object({type: z.literal("balance"), operator: balanceOperator, value: z.number().default(0)}),
]);
const audience = z.array(z.object({all_of: z.array(condition)}));
const announcementDetail = announcement.extend({content: z.string(), targeting: z.object({any_of: audience.optional()})});
const readStatus = z.object({user_id: z.number(), username: z.string(), email: z.string(), eligible: z.boolean(), read_at: z.string().nullish()});
type Announcement = z.infer<typeof announcement>;

function NoticeDialog({item, onClose, onRead}: {item: Notice; onClose: () => void; onRead: () => void}) {
  const action = useAction();
  return <Overlay open onClose={onClose} title={item.title} busy={action.busy} footer={!item.read_at &&
    <Button busy={action.busy} onClick={() => void action.run(async () => {
      await api("/announcements/" + item.id + "/read", empty, "POST");
      onRead();
      onClose();
    })}>标为已读</Button>}>
    <div className="preserve-lines">{item.content}</div>
    <ErrorLine value={action.error} />
  </Overlay>;
}

export function Announcements() {
  const [revision, revise] = useState(0);
  const [unread, setUnread] = useState(false);
  const [item, open] = useState<Notice>();
  const readAll = useAction();
  const path = "/announcements" + (unread ? "?unread_only=true" : "");
  const rows = useResource(() => api(path, userAnnouncements), path, true, revision);
  const items = rows?.value ?? [];
  const unreadCount = items.filter(r => !r.read_at).length;
  return <WorkspaceStage flush>
    <MetricStrip
      flush
      items={[
        {id: "a-total", label: "可见公告", value: count.format(items.length)},
        {id: "a-unread", label: "未读公告", value: count.format(unreadCount), tone: unreadCount > 0 ? "accent" : "ok"},
        {id: "a-latest", label: "最新发布", value: items[0] ? date(items[0].created_at) : "—"},
      ]}
    />
    <WorkspaceContent>
      <InsetGroup
        actions={
          <InlineStack density="compact">
            {unreadCount > 0 && (
              <Button
                size="sm"
                variant="outline"
                busy={readAll.busy}
                onClick={() => void readAll.run(async () => {
                  await api("/announcements/read-all", empty, "POST", {});
                  toast("已全部标为已读");
                  revise(v => v + 1);
                })}
              >
                <CheckCheck aria-hidden />全部标为已读
              </Button>
            )}
            <Select aria-label="公告筛选" value={unread ? "unread" : "all"} onChange={event => setUnread(event.target.value === "unread")}>
              <option value="all">全部公告</option><option value="unread">未读公告</option>
            </Select>
          </InlineStack>
        }
      >
        <Resource state={rows}>{list => list.length ? list.map(row => <InsetRow key={row.id} label={row.title} hint={`${date(row.created_at)} · ${row.content.slice(0, 72)}${row.content.length > 72 ? "…" : ""}`}>
          {!row.read_at && <Badge variant="soft" tone="info">未读</Badge>}
          <Button size="sm" variant="outline" onClick={() => open(row)}>查看</Button>
        </InsetRow>) : <EmptyState title={unread ? "没有未读公告" : "暂无公告"} />}</Resource>
      </InsetGroup>
      <ErrorLine value={readAll.error} />
      {item && <NoticeDialog item={item} onClose={() => open(undefined)} onRead={() => { toast("已标为已读"); revise(value => value + 1); }} />}
    </WorkspaceContent>
  </WorkspaceStage>;
}

export function AnnouncementNotice({revision}: {revision: number}) {
  const [dismissed, dismiss] = useState<number[]>([]);
  const data = useResource(() => api("/announcements?unread_only=true", userAnnouncements), "unread-announcements", true, revision);
  const next = data?.value?.find(row => row.notify_mode === "popup" && !row.read_at && !dismissed.includes(row.id));
  if (!next) return null;
  return <NoticeDialog key={next.id} item={next} onClose={() => dismiss(ids => [...ids, next.id])} onRead={() => {}} />;
}

const readColumns: Column<z.infer<typeof readStatus>>[] = [
  {id: "user", title: "用户", render: row => <EntityLink kind="user" id={row.user_id} label={row.username || row.email} />},
  {id: "eligible", title: "匹配投放条件", align: "center", render: row => <Badge variant="soft" tone={row.eligible ? "ok" : "neutral"}>{row.eligible ? "匹配" : "不匹配"}</Badge>},
  {id: "read", title: "阅读状态", render: row => row.read_at ? date(row.read_at) : <span className="muted">未读</span>},
];

export function ManageAnnouncements({onChanged}: {onChanged: () => void}) {
  const [revision, revise] = useState(0);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [editing, edit] = useState<number | null>(null);
  const [reading, read] = useState<Announcement>();
  const [removing, remove] = useState<Announcement>();
  const [loaded, setLoaded] = useState<Announcement[]>([]);
  const removal = useAction();
  const debounced = useDebouncedValue(search.trim());
  const base = "/admin/announcements";
  const path = base + "?" + new URLSearchParams({search: debounced, status});

  const saved = (message: string) => {
    revise(value => value + 1);
    toast(message);
    onChanged();
  };
  const columns: Column<Announcement>[] = [
    {id: "title", title: "标题", render: row => <EllipsisText text={row.title} />},
    {id: "status", title: "状态", align: "center", render: row => <Status value={row.status} label={statuses[row.status]} />},
    {id: "notify", title: "提醒方式", hideWhenCompact: true, render: row => row.notify_mode === "popup" ? "登录后弹窗" : "站内查看"},
    {id: "starts", title: "开始时间", hideWhenNarrow: true, render: row => row.starts_at ? date(row.starts_at) : "立即"},
    {id: "ends", title: "结束时间", hideWhenNarrow: true, render: row => row.ends_at ? date(row.ends_at) : "长期"},
    {id: "actions", title: "操作", align: "end", render: row => <div className="row-actions">
      <Button size="icon-xs" variant="ghost" title="阅读情况" aria-label={`查看 ${row.title} 阅读情况`} onClick={() => read(row)}><Eye /></Button>
      <Button size="icon-xs" variant="ghost" title="编辑公告" aria-label={`编辑 ${row.title}`} onClick={() => edit(row.id)}><Pencil /></Button>
      <Button size="icon-xs" variant="ghost" title="删除公告" aria-label={`删除 ${row.title}`} onClick={() => { removal.clearError(); remove(row); }}><Trash2 /></Button>
    </div>},
  ];

  const activeCount = loaded.filter(r => r.status === "active").length;
  const popupCount = loaded.filter(r => r.notify_mode === "popup").length;
  const draftCount = loaded.filter(r => r.status === "draft").length;

  return <WorkspaceStage flush>
    <MetricStrip
      flush
      items={[
        {id: "m-total", label: "当前页公告", value: count.format(loaded.length)},
        {id: "m-active", label: "已发布", value: count.format(activeCount), tone: "ok", onClick: () => setStatus("active")},
        {id: "m-popup", label: "登录弹窗", value: count.format(popupCount), tone: "accent"},
        {id: "m-draft", label: "草稿", value: count.format(draftCount), onClick: () => setStatus("draft")},
      ]}
    />
    <FilterBar sticky>
      <div className="obs-filter-row">
        <SearchInput value={search} onChange={setSearch} placeholder="搜索公告标题" />
        <FilterSlot>
          <Select aria-label="发布状态" value={status} onChange={event => setStatus(event.target.value)}>
            <option value="">全部状态</option>
            {Object.entries(statuses).map(([value, label]) => <option value={value} key={value}>{label}</option>)}
          </Select>
        </FilterSlot>
        <div className="obs-filter-spacer">
          <RefreshButton iconOnly size="default" variant="ghost" label="刷新公告" onClick={() => revise(v => v + 1)} />
          <Button onClick={() => edit(0)}>新建公告</Button>
        </div>
      </div>
    </FilterBar>
    <div className="swift-pane-table">
      <RemoteTable path={path} schema={announcement} columns={columns} rowID={row => row.id} onItemsLoaded={setLoaded} pinActions label="公告管理" revision={revision} />
    </div>
    {editing !== null && <AnnouncementEditor key={editing} id={editing} onClose={() => edit(null)} onSaved={() => { edit(null); saved("公告已保存"); }} />}
    {reading && <Overlay open drawer title={reading.title + " · 阅读情况"} onClose={() => read(undefined)}>
      <RemoteTable path={base + "/" + reading.id + "/read-status"} schema={readStatus} columns={readColumns} rowID={row => row.user_id} label="阅读情况" />
    </Overlay>}
    {removing && <ConfirmDialog open title="删除公告" description={"删除「" + removing.title + "」后，用户将无法继续查阅。"} confirm="删除公告"
      busy={removal.busy} error={removal.error} onClose={() => remove(undefined)} onConfirm={() => void removal.run(async () => {
        await api(base + "/" + removing.id, empty, "DELETE");
        remove(undefined);
        saved("公告已删除");
      })} />}
  </WorkspaceStage>;
}

function AnnouncementEditor({id, onClose, onSaved}: {id: number; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const action = useAction();
  const path = "/admin/announcements" + (id ? "/" + id : "");
  const detail = useResource(() => api(path, announcementDetail), path, Boolean(id));
  const [canSave, setCanSave] = useState(true);
  const renderForm = (item?: z.infer<typeof announcementDetail>) => (
    <AnnouncementForm
      formID={formID}
      item={item}
      busy={action.busy}
      error={action.error}
      fieldErrors={action.fieldErrors}
      onValidityChange={setCanSave}
      onSave={(data, targeting) =>
        void action.run(async () => {
          await api(path, empty, id ? "PUT" : "POST", {
            title: String(data.get("title") ?? "").trim(),
            content: String(data.get("content") ?? ""),
            status: data.get("status"),
            notify_mode: data.get("notify_mode"),
            starts_at: dateTimeInputSeconds(data.get("starts_at")) ?? 0,
            ends_at: dateTimeInputSeconds(data.get("ends_at")) ?? 0,
            targeting,
          });
          onSaved();
        })
      }
    />
  );
  return (
    <Overlay
      open
      drawer
      title={id ? "编辑公告" : "新建公告"}
      busy={action.busy}
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button type="submit" form={formID} busy={action.busy} disabled={!canSave}>保存公告</Button>
        </>
      }
    >
      {id ? <Resource state={detail}>{renderForm}</Resource> : renderForm()}
    </Overlay>
  );
}

function AnnouncementForm({
  formID,
  item,
  onSave,
  onValidityChange,
  busy,
  error,
  fieldErrors,
}: {
  formID: string;
  item?: z.infer<typeof announcementDetail>;
  onSave: (form: FormData, targeting: {any_of: {all_of: AudienceCondition[]}[]}) => void;
  onValidityChange: (ok: boolean) => void;
  busy: boolean;
  error: string;
  fieldErrors?: Record<string, string>;
}) {
  const [groups, setGroups] = useState<AudienceCondition[][]>(() => (item?.targeting.any_of ?? []).map(g => g.all_of));
  const [targeted, setTargeted] = useState(groups.length > 0);

  const checkValid = (isTargeted: boolean, currentGroups: AudienceCondition[][]) => {
    const incomplete =
      isTargeted &&
      (currentGroups.length === 0 ||
        currentGroups.some(group => group.some(rule => rule.type === "subscription" && rule.group_ids.length === 0)));
    onValidityChange(!incomplete);
  };

  return (
    <Form
      id={formID}
      busy={busy}
      errors={fieldErrors}
      onSubmit={event =>
        onSave(new FormData(event.currentTarget), {
          any_of: targeted ? groups.map(all_of => ({all_of})) : [],
        })
      }
    >
      <Field label="标题" name="title"><Input name="title" defaultValue={item?.title} required /></Field>
      <Field label="正文" name="content"><Textarea name="content" rows={6} defaultValue={item?.content} required /></Field>
      <FormGrid>
        <Field label="发布状态"><Select name="status" defaultValue={item?.status ?? "draft"}>
          {Object.entries(statuses).map(([value, label]) => <option value={value} key={value}>{label}</option>)}
        </Select></Field>
        <Field label="提醒方式"><Select name="notify_mode" defaultValue={item?.notify_mode ?? "silent"}>
          <option value="silent">站内查看</option><option value="popup">登录后弹窗</option>
        </Select></Field>
        <Field label="开始时间" hint="留空立即生效"><Input type="datetime-local" step={1} name="starts_at" defaultValue={dateTimeInputValue(item?.starts_at)} /></Field>
        <Field label="结束时间" hint="留空长期有效"><Input type="datetime-local" step={1} name="ends_at" defaultValue={dateTimeInputValue(item?.ends_at)} /></Field>
      </FormGrid>
      <Field label="投放受众">
        <Select
          value={targeted ? "targeted" : "all"}
          onChange={event => {
            const filtered = event.target.value === "targeted";
            setTargeted(filtered);
            const nextGroups = filtered && groups.length === 0 ? [[{type: "balance" as const, operator: "lt" as const, value: 0}]] : groups;
            if (nextGroups !== groups) setGroups(nextGroups);
            checkValid(filtered, nextGroups);
          }}
        >
          <option value="all">全部登录用户</option>
          <option value="targeted">按余额或订阅条件筛选</option>
        </Select>
      </Field>
      {targeted && (
        <Stack>
          <div className="muted text-fine">满足任一条件组即可收到此公告，同一条件组内需同时满足。</div>
          <AudienceRuleBuilder
            value={groups}
            onChange={next => {
              setGroups(next);
              checkValid(targeted, next);
            }}
          />
        </Stack>
      )}
      <ErrorLine value={error} />
    </Form>
  );
}
