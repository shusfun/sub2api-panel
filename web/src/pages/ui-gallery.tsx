import {useState} from "react";
import {navigatePublic} from "../route";
import {
  Badge,
  Button,
  ChevronMorphIcon,
  ChoiceCards,
  CollapsibleSection,
  ConfirmDialog,
  CopyButton,
  DetailHeader,
  EmptyState,
  EyeMorphIcon,
  Field,
  FilePicker,
  Form,
  FormGrid,
  HoldButton,
  InlineStack,
  Input,
  InsetGroup,
  InsetRow,
  MaskedSecret,
  OtpInput,
  PasswordInput,
  PermissionPicker,
  PlayPauseIcon,
  PublicLayout,
  QrCode,
  RefreshButton,
  SaveBar,
  SecretRevealDialog,
  Segmented,
  Stack,
  Status,
  Switch,
  Tabs,
  TagInput,
  toast,
} from "../ui";

const galleryPermissions = [
  {id: "accounts.read", label: "读取上游账号", category: "号池管理", scope: "scoped"},
  {id: "accounts.write", label: "修改上游账号", category: "号池管理", scope: "scoped"},
  {id: "settings.write", label: "修改站点设置", category: "系统治理", scope: "global"},
  {id: "audit.clear", label: "清空操作审计", category: "系统治理", scope: "global"},
] as const;

export function UIGallery({siteName = "Sub2API", siteLogo}: {siteName?: string; siteLogo?: string}) {
  const [tab, setTab] = useState<"controls" | "forms" | "feedback">("controls");
  const [choice, setChoice] = useState("pro");
  const [segment, setSegment] = useState<"day" | "hour">("day");
  const [eyeOpen, setEyeOpen] = useState(false);
  const [chevronOpen, setChevronOpen] = useState(false);
  const [playing, setPlaying] = useState(true);
  const [files, setFiles] = useState<File[]>([]);
  const [tags, setTags] = useState<string[]>(["10.0.0.0/8", "192.168.1.1"]);
  const [perms, setPerms] = useState<string[]>(["accounts.read"]);
  const [dirty, setDirty] = useState(true);
  const [confirmMode, setConfirmMode] = useState<"normal" | "critical">();
  const [revealOpen, setRevealOpen] = useState(false);
  const [qrExpired, setQrExpired] = useState(false);

  return (
    <PublicLayout
      siteName={siteName}
      siteLogo={siteLogo}
      subtitle="公共组件与交互契约画廊（仅开发与预览环境挂载）"
      actions={<Button size="sm" variant="outline" onClick={() => navigatePublic("home")}>返回站点首页</Button>}
    >
      <DetailHeader
        backLabel="首页"
        title="UI 公共组件验收台"
        badges={<Badge variant="soft" tone="info">低饱和石板 · 奇数刻度 · 连续形变</Badge>}
        actions={
          <>
            <RefreshButton size="sm" onClick={() => toast("刷新微动效已触发")} />
            <Button size="sm" onClick={() => toast("单通道轻量反馈：操作已生效")}>触发成功提示</Button>
          </>
        }
        onBack={() => navigatePublic("home")}
      />

      <Tabs
        label="画廊分区"
        value={tab}
        onChange={setTab}
        items={[
          {value: "controls", label: "按钮、形变图标与确认分级"},
          {value: "forms", label: "高密度表单、选择卡与规则器"},
          {value: "feedback", label: "状态枚举、凭证与二维码"},
        ]}
      />

      {tab === "controls" && (
        <Stack>
          <InsetGroup title="按钮语义层级与高危按住确认">
            <InsetRow label="常规与次级按钮">
              <InlineStack wrap>
                <Button size="sm">主操作</Button>
                <Button size="sm" variant="outline">描边次级</Button>
                <Button size="sm" variant="secondary">选中滑块</Button>
                <Button size="sm" variant="ghost">幽灵按钮</Button>
              </InlineStack>
            </InsetRow>
            <InsetRow label="危险操作分级（即点描边 vs 长按 1 秒读条）">
              <InlineStack wrap>
                <Button size="sm" variant="destructive" onClick={() => setConfirmMode("normal")}>常规删除确认框</Button>
                <Button size="sm" variant="danger-outline" onClick={() => setConfirmMode("critical")}>高危中断确认框</Button>
                <HoldButton size="sm" onConfirm={() => toast("高危长按 1 秒确认通过", "warn")}>直接长按重置</HoldButton>
              </InlineStack>
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="单图标连续路径形变（frontend-icons 契约）">
            <InsetRow label="眼睛开闭 / 折叠箭头 / 播放暂停 / 复制对勾">
              <InlineStack wrap>
                <Button size="icon" variant="outline" title={eyeOpen ? "隐藏" : "显示"} aria-label="切换眼睛形变" aria-pressed={eyeOpen} onClick={() => setEyeOpen(v => !v)}>
                  <EyeMorphIcon open={eyeOpen} />
                </Button>
                <Button size="icon" variant="outline" title="切换折叠形变" aria-label="切换折叠形变" aria-expanded={chevronOpen} onClick={() => setChevronOpen(v => !v)}>
                  <ChevronMorphIcon open={chevronOpen} />
                </Button>
                <Button size="icon" variant="outline" title={playing ? "暂停" : "继续"} aria-label="切换播放暂停形变" aria-pressed={playing} onClick={() => setPlaying(v => !v)}>
                  <PlayPauseIcon active={playing} />
                </Button>
                <CopyButton value="sub2api-panel-morph-check" label="测试复制形变" />
              </InlineStack>
            </InsetRow>
          </InsetGroup>

          <CollapsibleSection title="分段控件与可折叠详情面板" summary="点击展开">
            <Segmented
              label="时间粒度"
              value={segment}
              options={[["day", "按日聚合"], ["hour", "按小时明细"]] as const}
              onChange={setSegment}
            />
          </CollapsibleSection>
        </Stack>
      )}

      {tab === "forms" && (
        <Form onSubmit={() => { setDirty(false); toast("示例配置已保存"); }}>
          <InsetGroup title="整张可点选择卡（支持键盘方向键切换）">
            <InsetRow label="选择套餐方案" vertical>
              <ChoiceCards
                label="选择套餐方案"
                value={choice}
                columns={3}
                onChange={v => { setChoice(v); setDirty(true); }}
                options={[
                  {value: "payg", title: "按量钱包", meta: "免月费", description: "按实际输入与输出 Token 实时结算"},
                  {value: "pro", title: "专业订阅组", meta: "$29.00 / 月", description: "包含每日 $15 窗口配额与高优先级路由"},
                  {value: "team", title: "团队专属池", meta: "$99.00 / 月", description: "独立上游账号池隔离与并发保障"},
                ]}
              />
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="内联单位输入、密码形变框与验证码控件">
            <InsetRow label="组合输入控件" vertical>
              <FormGrid>
                <Field label="单次预算上限">
                  <Input type="number" suffix="USD" defaultValue="50" onChange={() => setDirty(true)} />
                </Field>
                <Field label="上游请求超时">
                  <Input type="number" suffix="ms" defaultValue="5000" onChange={() => setDirty(true)} />
                </Field>
                <Field label="连接凭据">
                  <PasswordInput defaultValue="sk-demo-credential-token" toggleLabel="示例凭据" onChange={() => setDirty(true)} />
                </Field>
                <Field label="6 位动态验证码">
                  <OtpInput onSendCode={() => { toast("测试验证码已发送"); return 60; }} />
                </Field>
              </FormGrid>
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="多值胶囊、文件拖拽选择器与权限树">
            <InsetRow label="来源网段白名单（TagInput）" vertical>
              <TagInput value={tags} onChange={next => { setTags(next); setDirty(true); }} />
            </InsetRow>
            <InsetRow label="文件选择与校验（FilePicker）" vertical>
              <FilePicker
                accept="image/png,image/jpeg,application/json,.json"
                multiple
                maxBytes={5 * 1024 * 1024}
                hint="支持拖拽 PNG / JPEG / JSON，单文件不超过 5 MiB"
                value={files}
                onChange={next => { setFiles(next); setDirty(true); }}
              />
            </InsetRow>
            <InsetRow label="分类权限选择器（PermissionPicker）" vertical>
              <PermissionPicker permissions={galleryPermissions} value={perms} onChange={next => { setPerms(next); setDirty(true); }} />
            </InsetRow>
          </InsetGroup>

          <SaveBar
            dirty={dirty}
            dirtyCount={dirty ? 3 : 0}
            onReset={() => setDirty(false)}
          />
        </Form>
      )}

      {tab === "feedback" && (
        <Stack>
          <InsetGroup title="全域统一状态语义（Status）">
            <InsetRow label="正常与完成态">
              <InlineStack wrap>
                <Status value="active" />
                <Status value="enabled" />
                <Status value="COMPLETED" />
                <Status value="compatible" />
              </InlineStack>
            </InsetRow>
            <InsetRow label="进行中与预警态">
              <InlineStack wrap>
                <Status value="PENDING" />
                <Status value="running" />
                <Status value="degraded" />
                <Status value="expired" />
              </InlineStack>
            </InsetRow>
            <InsetRow label="异常、拦截与免检态">
              <InlineStack wrap>
                <Status value="failed" />
                <Status value="block" />
                <Status value="revoked" />
                <Status value="skipped_idle" />
              </InlineStack>
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="掩码凭证、一次性明文弹窗与亮暗自适应二维码">
            <InsetRow label="掩码密钥与按需复制">
              <InlineStack wrap>
                <MaskedSecret masked="sk-panel****9f2a" getSecret={() => "sk-panel-demo-secret-9f2a"} />
                <Button size="sm" variant="outline" onClick={() => setRevealOpen(true)}>预览一次性明文弹窗</Button>
              </InlineStack>
            </InsetRow>
            <InsetRow label="二维码衬底与过期遮罩">
              <InlineStack wrap>
                <QrCode
                  value="https://panel.example.com/verify"
                  alt="示例二维码"
                  expired={qrExpired}
                  onRefresh={() => setQrExpired(false)}
                />
                <Switch label="模拟二维码过期" checked={qrExpired} onChange={setQrExpired} />
              </InlineStack>
            </InsetRow>
          </InsetGroup>

          <InsetGroup title="空状态引导">
            <EmptyState
              title="暂无匹配的观测样本"
              description="调整上方筛选范围或发起一次实时测活后即可在此查看明细。"
              action={<Button size="sm" variant="outline" onClick={() => toast("已触发下一步引导")}>立即探测</Button>}
            />
          </InsetGroup>
        </Stack>
      )}

      <ConfirmDialog
        open={confirmMode !== undefined}
        title={confirmMode === "critical" ? "清空并重置核心数据" : "删除单条记录"}
        description={confirmMode === "critical" ? "高危中断操作需按住确认键满 1 秒后才会执行。" : "常规单项删除采用幽灵取消配红色描边确认按钮。"}
        confirm={confirmMode === "critical" ? "确认重置" : "删除记录"}
        critical={confirmMode === "critical"}
        busy={false}
        error=""
        onClose={() => setConfirmMode(undefined)}
        onConfirm={() => {
          toast(confirmMode === "critical" ? "高危确认已完成" : "常规删除已完成");
          setConfirmMode(undefined);
        }}
      />

      <SecretRevealDialog
        open={revealOpen}
        title="管理访问密钥已生成"
        label="管理访问密钥"
        value="s2p_admin_live_98a7bc65d4e3f210"
        onClose={() => setRevealOpen(false)}
      />
    </PublicLayout>
  );
}
