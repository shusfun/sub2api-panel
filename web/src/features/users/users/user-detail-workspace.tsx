import {useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {date, money} from "@/lib/format";
import {useAction, useResource} from "@/lib/requests";
import {balanceHistoryRow, empty, userRow} from "@/schemas";
import {navigateTo} from "@/app/route";
import {
  Badge, Button, ConfirmDialog, DetailHeader, EmptyState, ErrorLine, GroupDialog, IdentityBadge,
  InsetGroup, InsetRow, RemoteTable, Resource, Status, Tabs, Toolbar, toast, useGroups,
} from "@/ui";
import {AffiliateUserPane} from "../affiliates";
import {AssignSubscriptionDialog} from "./user-dialogs";
import {UserAttributes} from "./user-attributes";
import {UserQuotas} from "./user-quotas";
import {GrantEditor, ReplaceUserGroup, UserChangeDialog, UserKeysPane, UserSubscriptionsPane} from "./user-panes";

type DetailTab = "overview" | "subs" | "keys" | "balance" | "extras" | "affiliate";

export function UserDetailWorkspace({
  userID,
  initialTab,
  onBack,
}: {
  userID: number;
  initialTab?: string;
  onBack: () => void;
}) {
  const [tab, setTab] = useState<DetailTab>(() => {
    if (initialTab === "subs" || initialTab === "keys" || initialTab === "balance" || initialTab === "extras" || initialTab === "affiliate") return initialTab;
    if (initialTab === "grant") return "extras";
    return "overview";
  });
  const [revision, revise] = useState(0);
  const [task, setTask] = useState<"balance" | "limits" | "quotas" | "groups" | "replace-group" | "assign" | "profile" | "delete">();
  const detail = useResource(() => api(`/admin/users/${userID}`, userRow), `usr-standalone-${userID}`, true, revision);
  const groups = useGroups({revision});
  const op = useAction();

  const finish = (message: string) => {
    toast(message);
    setTask(undefined);
    revise(n => n + 1);
  };

  return (
    <Resource state={detail}>
      {usr => (
        <div className="swift-pane-stack">
          <DetailHeader
            backLabel="返回用户列表"
            onBack={onBack}
            title={
              <IdentityBadge
                name={usr.email}
                meta={usr.username ? `${usr.username} · #${usr.id}` : `用户 #${usr.id}`}
                tone={usr.role === "admin" ? "accent" : usr.status === "active" ? "ok" : "bad"}
              />
            }
            badges={
              <>
                <Status value={usr.status} />
                {usr.role === "admin" && <Badge variant="soft" tone="info">管理员</Badge>}
              </>
            }
            actions={
              <>
                <Button size="sm" variant="outline" onClick={() => setTask("profile")}>编辑账号</Button>
                <Button size="sm" onClick={() => setTask("assign")}>分配订阅</Button>
              </>
            }
          />

          <Tabs
            label="用户详情标签页"
            value={tab}
            onChange={setTab}
            items={[
              {value: "overview", label: "概览与限额"},
              {value: "subs", label: "订阅与专属分组"},
              {value: "keys", label: "API Key 代管"},
              {value: "balance", label: "余额与流水"},
              {value: "extras", label: "扩展属性与发码授权"},
              {value: "affiliate", label: "推广"},
            ]}
          />

          {tab === "overview" && (
            <>
              <InsetGroup title="账号概览与限额">
                <InsetRow label="登录邮箱">{usr.email}</InsetRow>
                <InsetRow label="显示昵称">{usr.username || "—"}</InsetRow>
                <InsetRow label="角色">{usr.role === "admin" ? "管理员" : "普通用户"}</InsetRow>
                <InsetRow label="公开分组限制">{usr.restrict_public_groups ? "受限（仅专属分组）" : "可访问公开分组"}</InsetRow>
                <InsetRow label="可用余额" onClick={() => setTask("balance")}><strong className="weir-digits">{money(usr.balance)}</strong></InsetRow>
                <InsetRow label="并发 / RPM" onClick={() => setTask("limits")}><span className="weir-digits">{usr.current_concurrency}/{usr.concurrency || "∞"} · {usr.rpm_limit || "不限"} RPM</span></InsetRow>
                <InsetRow label="平台配额" onClick={() => setTask("quotas")} />
                <InsetRow label="备注">{usr.notes || "—"}</InsetRow>
              </InsetGroup>
              <Toolbar>
                <Button variant="outline" onClick={() => setTask("limits")}>修改限流</Button>
                <Button variant="link" onClick={() => navigateTo("usage", {user: usr.id})}>查看用量</Button>
                <Button variant="link" onClick={() => navigateTo("ops", {tab: "errors", user_id: usr.id})}>查看报错</Button>
              </Toolbar>
              {usr.role !== "admin" && (
                <InsetGroup><InsetRow label="删除用户" tone="bad" onClick={() => setTask("delete")} /></InsetGroup>
              )}
            </>
          )}

          {tab === "subs" && (
            <>
              <InsetGroup title="专属分组授权">
                <InsetRow label="已授权私有分组" onClick={() => setTask("groups")}>
                  {(groups?.value ?? []).filter(g => g.is_exclusive && usr.allowed_groups.includes(g.id)).length || "未授权"}
                </InsetRow>
                <InsetRow label="替换专属分组" onClick={() => setTask("replace-group")} />
              </InsetGroup>
              <UserSubscriptionsPane userID={usr.id} revision={revision} onChanged={finish} />
            </>
          )}

          {tab === "keys" && (
            <UserKeysPane userID={usr.id} revision={revision} onChanged={finish} />
          )}

          {tab === "balance" && (
            <>
              <Toolbar>
                <strong className="weir-digits">当前余额：{money(usr.balance)}</strong>
                <Button size="sm" onClick={() => setTask("balance")}>调整余额</Button>
              </Toolbar>
              <RemoteTable
                path={`/admin/users/${usr.id}/balance-history`}
                schema={balanceHistoryRow}
                revision={revision}
                label="余额流水"
                rowID={r => r.id}
                empty={<EmptyState title="暂无余额流水记录" />}
                columns={[
                  {id: "type", title: "类型", render: r => <Badge variant="soft" tone="neutral">{r.type}</Badge>},
                  {id: "value", title: "金额", align: "number", render: r => <strong className="weir-digits">{money(r.value)}</strong>},
                  {id: "status", title: "状态", align: "center", render: r => <Status value={r.status} />},
                  {id: "notes", title: "备注", render: r => r.notes || "—"},
                  {id: "time", title: "时间", render: r => <span className="code-mono text-fine">{date(r.used_at || r.created_at)}</span>},
                ]}
              />
            </>
          )}

          {tab === "affiliate" && <AffiliateUserPane userId={usr.id} />}

          {tab === "extras" && (
            <>
              <UserAttributes userID={usr.id} inline />
              {usr.role !== "admin" && <GrantEditor user={usr.id} />}
            </>
          )}

          {task === "groups" && (
            <GroupDialog
              title="授权私有分组"
              source={{privateOnly: true, revision}}
              value={usr.allowed_groups}
              multiple
              allowEmpty
              activeOnly
              busy={op.busy}
              error={op.error}
              onClose={() => setTask(undefined)}
              onApply={ids => void op.run(async () => {
                await api(`/admin/users/${usr.id}/private-groups`, userRow, "PUT", {group_ids: ids});
                finish("私有分组已更新");
              })}
            />
          )}
          {task === "replace-group" && (
            <ReplaceUserGroup user={usr} groups={groups?.value ?? []} onClose={() => setTask(undefined)} onSaved={finish} />
          )}
          {task === "quotas" && <UserQuotas userID={usr.id} onClose={() => setTask(undefined)} />}
          {task === "assign" && <AssignSubscriptionDialog open revision={revision} fixedUserID={usr.id} onClose={() => setTask(undefined)} onSaved={() => finish("已开通订阅")} />}
          {(task === "balance" || task === "limits" || task === "profile") && (
            <UserChangeDialog task={task} usr={usr} onClose={() => setTask(undefined)} onSaved={finish} />
          )}
          <ConfirmDialog
            open={task === "delete"}
            title="删除用户"
            description={`确定删除用户「${usr.email}」吗？`}
            confirm="删除用户"
            busy={op.busy}
            error={op.error}
            onClose={() => setTask(undefined)}
            onConfirm={() => void op.run(async () => {
              await api(`/admin/users/${usr.id}`, empty, "DELETE");
              toast("用户已删除");
              onBack();
            })}
          />
          <ErrorLine value={op.error} />
        </div>
      )}
    </Resource>
  );
}
