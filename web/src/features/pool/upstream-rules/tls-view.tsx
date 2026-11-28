import {useId, useState, type ReactNode} from "react";
import {Plus} from "lucide-react";
import {z} from "zod";
import {api} from "@/lib/http";
import {empty, tlsProfileSchema} from "@/schemas";
import {useAction} from "@/lib/requests";
import {
  Badge, Button, ConfirmDialog, EmptyState, ErrorLine, Field, FilterBar, Form, FormGrid, Input, Overlay,
  RefreshButton, RemoteTable, RowActions, Switch, TagInput, Textarea, WorkspaceStage, toast, type Column,
} from "@/ui";
import {joinNumbers, parseNumbers, validNumbers} from "./rule-helpers";

type Profile = z.infer<typeof tlsProfileSchema>;

export function TlsProfilesView({switcher}: {switcher: ReactNode}) {
  const [revision, revise] = useState(0);
  const [editing, setEditing] = useState<Profile | "new">();
  const [deleting, setDeleting] = useState<Profile>();
  const del = useAction();
  const refresh = () => revise(v => v + 1);

  const columns: Column<Profile>[] = [
    {
      id: "name",
      title: "档案",
      render: row => (
        <div>
          <strong className="compact-text" title={row.name}>{row.name}</strong>
          {row.description && <div className="muted text-fine">{row.description}</div>}
        </div>
      ),
    },
    {id: "ciphers", title: "加密套件", align: "number", render: row => row.cipher_suites.length},
    {id: "curves", title: "曲线", align: "number", hideWhenNarrow: true, render: row => row.curves.length},
    {id: "ext", title: "扩展", align: "number", hideWhenNarrow: true, render: row => row.extensions.length},
    {id: "alpn", title: "ALPN", hideWhenCompact: true, render: row => row.alpn_protocols.join(", ") || "—"},
    {id: "grease", title: "GREASE", align: "center", hideWhenCompact: true, render: row => <Badge variant="soft" tone={row.enable_grease ? "ok" : "neutral"}>{row.enable_grease ? "开启" : "关闭"}</Badge>},
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
              <RefreshButton iconOnly size="default" variant="ghost" label="刷新档案" onClick={refresh} />
              <Button onClick={() => setEditing("new")}><Plus aria-hidden />新建档案</Button>
            </div>
          </div>
        </FilterBar>
        <div className="swift-pane-table">
          <RemoteTable
            path="/admin/tls-fingerprint-profiles"
            schema={tlsProfileSchema}
            columns={columns}
            rowID={row => row.id}
            label="TLS 指纹档案"
            revision={revision}
            onRowClick={row => setEditing(row)}
            empty={<EmptyState title="没有 TLS 指纹档案" description="账号的防封控设置里可以选择这里创建的档案。" action={<Button size="sm" onClick={() => setEditing("new")}><Plus aria-hidden />新建档案</Button>} />}
          />
        </div>
      </WorkspaceStage>

      {editing && (
        <ProfileDialog
          key={editing === "new" ? "new" : editing.id}
          profile={editing === "new" ? undefined : editing}
          onClose={() => setEditing(undefined)}
          onSaved={() => { setEditing(undefined); toast("档案已保存"); refresh(); }}
        />
      )}
      <ConfirmDialog
        open={!!deleting}
        title="删除 TLS 指纹档案"
        description={`确定删除档案「${deleting?.name ?? ""}」吗？已引用该档案的账号需要重新选择。`}
        confirm="删除"
        critical
        busy={del.busy}
        error={del.error}
        onClose={() => setDeleting(undefined)}
        onConfirm={() => void del.run(async () => {
          if (!deleting) return;
          await api(`/admin/tls-fingerprint-profiles/${deleting.id}`, empty, "DELETE");
          setDeleting(undefined);
          toast("档案已删除");
          refresh();
        })}
      />
    </>
  );
}

const numberFields = [
  ["cipher_suites", "加密套件 (cipher_suites)"],
  ["curves", "曲线 (curves)"],
  ["point_formats", "点格式 (point_formats)"],
  ["signature_algorithms", "签名算法 (signature_algorithms)"],
  ["supported_versions", "支持版本 (supported_versions)"],
  ["key_share_groups", "密钥共享组 (key_share_groups)"],
  ["psk_modes", "PSK 模式 (psk_modes)"],
  ["extensions", "扩展 (extensions)"],
] as const;

type NumberKey = (typeof numberFields)[number][0];

function ProfileDialog({profile, onClose, onSaved}: {profile?: Profile; onClose: () => void; onSaved: () => void}) {
  const formID = useId();
  const op = useAction();
  const [name, setName] = useState(profile?.name ?? "");
  const [description, setDescription] = useState(profile?.description ?? "");
  const [grease, setGrease] = useState(profile?.enable_grease ?? false);
  const [alpn, setAlpn] = useState<string[]>(profile?.alpn_protocols ?? []);
  const [lists, setLists] = useState<Record<NumberKey, string>>(() => Object.fromEntries(
    numberFields.map(([key]) => [key, joinNumbers(profile?.[key] ?? [])]),
  ) as Record<NumberKey, string>);

  return (
    <Overlay open title={profile ? "编辑 TLS 指纹档案" : "新建 TLS 指纹档案"} onClose={onClose} busy={op.busy} footer={
      <>
        <Button variant="outline" onClick={onClose}>取消</Button>
        <Button type="submit" form={formID} busy={op.busy}>保存档案</Button>
      </>
    }>
      <Form id={formID} busy={op.busy} onSubmit={() => void op.run(async () => {
        const parsed = {} as Record<NumberKey, number[]>;
        for (const [key, label] of numberFields) {
          parsed[key] = parseNumbers(lists[key]);
          if (!validNumbers(parsed[key], 0, 65535)) throw new Error(`${label} 需为 0-65535 的整数，用逗号分隔`);
        }
        const body = {
          name: name.trim(),
          description: description.trim() || null,
          enable_grease: grease,
          alpn_protocols: alpn,
          ...parsed,
        };
        if (profile) await api(`/admin/tls-fingerprint-profiles/${profile.id}`, tlsProfileSchema, "PUT", body);
        else await api("/admin/tls-fingerprint-profiles", tlsProfileSchema, "POST", body);
        onSaved();
      })}>
        <Field label="档案名称"><Input value={name} required maxLength={100} onChange={e => setName(e.target.value)} /></Field>
        <Field label="说明（可选）"><Input value={description} maxLength={500} onChange={e => setDescription(e.target.value)} /></Field>
        <Field label="启用 GREASE"><Switch label="启用 GREASE" checked={grease} onChange={setGrease} /></Field>
        <Field label="ALPN 协议"><TagInput value={alpn} placeholder="如 h2、http/1.1，回车添加" onChange={setAlpn} /></Field>
        <FormGrid>
          {numberFields.map(([key, label]) => (
            <Field key={key} label={label}>
              <Textarea rows={2} value={lists[key]} placeholder="十进制数值，逗号分隔" onChange={e => setLists({...lists, [key]: e.target.value})} />
            </Field>
          ))}
        </FormGrid>
        <ErrorLine value={op.error} />
      </Form>
    </Overlay>
  );
}
