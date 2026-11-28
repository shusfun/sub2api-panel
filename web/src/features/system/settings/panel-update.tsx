import {useEffect, useState} from "react";
import {z} from "zod";
import {api} from "@/lib/http";
import {useAction, useResource} from "@/lib/requests";
import {Badge, Button, Feedback, InlineStack, InsetRow, RefreshButton, StepUpDialog} from "@/ui";

const endpoint = "/admin/settings-tools/panel-update";
const phaseNames = ["queued", "download", "maintenance", "backup", "migrate", "switching", "starting", "recovering", "succeeded", "failed", "rolled_back", "intervention"] as const;
const phases: Record<(typeof phaseNames)[number], string> = {
  queued: "等待执行",
  download: "正在下载和校验",
  maintenance: "正在进入维护",
  backup: "正在备份数据库",
  migrate: "正在迁移数据库",
  switching: "正在切换版本",
  starting: "正在检查服务",
  recovering: "正在恢复上一版本",
  succeeded: "已更新",
  failed: "更新失败",
  rolled_back: "已恢复上一版本",
  intervention: "等待运维处理",
};

const jobSchema = z.object({
  id: z.string(),
  from: z.string(),
  to: z.string(),
  phase: z.enum(phaseNames),
  failure: z.enum([...phaseNames, "interrupted"]).optional(),
  updated_at: z.string(),
});
const statusSchema = z.object({configured: z.boolean(), job: jobSchema.optional()});
const releaseSchema = z.object({
  version: z.string(),
  url: z.string().url().startsWith("https://github.com/shusfun/sub2api-panel/releases/tag/"),
  notes: z.string(),
  availability: z.enum(["ready", "unverified", "unsupported_source", "manual"]),
  migration: z.boolean(),
});
const checkSchema = z.object({release: releaseSchema.optional()});

const blockReasons = {
  unverified: "发布包缺少可验证的更新文件",
  unsupported_source: "此版本尚未验证从当前版本直接升级",
  manual: "此版本需要由运维执行迁移",
} as const;

function running(job: z.infer<typeof jobSchema> | undefined) {
  return job !== undefined && !["succeeded", "failed", "rolled_back", "intervention"].includes(job.phase);
}

export function PanelUpdateRows({version, commit}: {version: string; commit: string}) {
  const [revision, refresh] = useState(0);
  const [submitted, setSubmitted] = useState<z.infer<typeof jobSchema>>();
  const [checked, setChecked] = useState<z.infer<typeof checkSchema>>();
  const [target, setTarget] = useState<z.infer<typeof releaseSchema>>();
  const check = useAction();
  const submit = useAction();
  const status = useResource(() => api(endpoint, statusSchema), "panel-update-status", true, revision);
  const serverJob = status?.value?.job;
  const showingPrevious = serverJob && submitted && serverJob.id !== submitted.id &&
    Date.parse(serverJob.updated_at) < Date.parse(submitted.updated_at);
  const job = !serverJob || showingPrevious ? submitted : serverJob;
  const active = running(job);
  const locked = active || job?.phase === "intervention";
  const availability = checked?.release?.availability;

  useEffect(() => {
    if (!active || status?.loading) return;
    const timer = window.setTimeout(() => refresh(value => value + 1), 3000);
    return () => window.clearTimeout(timer);
  }, [active, status?.loading, revision]);

  function checkRelease() {
    void check.run(async () => {
      setChecked(await api(endpoint + "/release", checkSchema));
    });
  }

  return <>
    <InsetRow label="面板版本" hint={commit}>
      <InlineStack wrap>
        <span>{version}</span>
        {status?.value?.configured && <RefreshButton size="xs" variant="ghost" label="检查面板更新" busy={check.busy} disabled={locked} onClick={checkRelease} />}
      </InlineStack>
    </InsetRow>
    {status?.error && !active && <InsetRow label="更新状态"><InlineStack wrap>
      <Feedback tone="error">{status.error}</Feedback>
      <RefreshButton size="xs" variant="ghost" label="重新读取更新状态" busy={status.loading} onClick={() => refresh(value => value + 1)} />
    </InlineStack></InsetRow>}
    {status?.value && !status.value.configured && <InsetRow label="更新状态">部署未启用在线更新</InsetRow>}
    {check.error && <InsetRow label="检查更新"><Feedback tone="error">{check.error}</Feedback></InsetRow>}
    {!check.error && checked && !checked.release && <InsetRow label="检查更新">当前版本已是最新</InsetRow>}
    {checked?.release && <InsetRow label="可用面板版本" hint={availability && availability !== "ready" ? blockReasons[availability] : undefined}>
      <InlineStack wrap>
        <span>{checked.release.version}</span>
        <a href={checked.release.url} target="_blank" rel="noreferrer">发布说明</a>
        {availability === "ready" && <Button size="sm" variant="outline" disabled={locked || check.busy || Boolean(check.error)} onClick={() => {
          submit.clearError();
          setTarget(checked.release);
        }}>更新面板</Button>}
      </InlineStack>
    </InsetRow>}
    {job && <InsetRow label="最近一次更新" hint={job.phase === "intervention" ? "面板需要运维恢复，请查看服务器上的更新记录" : undefined}>
      <InlineStack wrap>
        <span>{job.from} → {job.to}</span>
        <Badge variant="soft" tone={job.phase === "succeeded" ? "ok" : job.failure ? "bad" : "neutral"}>{phases[job.phase]}</Badge>
        {job.failure && <span>{job.failure === "interrupted" ? "执行器曾中断" : "失败阶段 · " + phases[job.failure]}</span>}
      </InlineStack>
    </InsetRow>}
    {target && <StepUpDialog
      title={"更新面板至 " + target.version}
      labels={[
        target.migration ? "面板将备份并迁移数据库。更新期间面板短暂维护，恢复后请重新登录。" : "更新期间面板短暂维护，恢复后请重新登录。",
      ]}
      description={target.notes ? <pre className="whitespace-pre-wrap break-words">{target.notes}</pre> : undefined}
      critical
      confirmLabel="确认更新"
      busy={submit.busy}
      error={submit.error}
      fieldErrors={submit.fieldErrors}
      onClose={() => setTarget(undefined)}
      onConfirm={({totp}) => void submit.run(async () => {
        const queued = await api(endpoint, jobSchema, "POST", {version: target.version, totp});
        setSubmitted(queued);
        setTarget(undefined);
        setChecked(undefined);
        refresh(value => value + 1);
      })}
    />}
  </>;
}
