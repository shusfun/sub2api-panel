# 面板组件索引

页面必须且只能通过 `ui/`（`import {...} from "../ui"`）组合公共组件。控件尺寸、状态、焦点、动效和三层主题变量集中在 `ui/styles/` 维护，`styles.css` 只负责应用外壳和页面级网格编排。开发与固定预览环境下可访问 `/_ui` 交互验收全部公共组件。

| 组件分类 | 入口文件与导出 | 适用场景与契约 |
|---|---|---|
| 外壳、分栏与详情页头 | `split-view.tsx`, `inspector.tsx`, `public-layout.tsx`, `tabs.tsx` (`WorkspaceStage`, `Inspector`, `PublicLayout`, `StageCard`, `Tabs`, `DetailHeader`) | 后台主从速览、独立详情页返回头、视图级标签页、公开页顶栏/页脚及窄/中/宽三档聚焦卡片舞台（`sm`/`md`/`lg`） |
| 凹槽分组、折叠与开关 | `inset-group.tsx`, `switch.tsx` (`InsetGroup`, `InsetRow`, `CollapsibleSection`, `Switch`) | 检查器键值、设置开关行与按需展开高级区；页面严禁手写原生 `<details>` |
| 单通道反馈、容错边界与状态 | `toast.tsx`, `feedback.tsx`, `status.tsx`, `tooltip.tsx` (`ToastViewport`, `toast`, `Feedback`, `ErrorLine`, `EmptyState`, `Resource`, `WorkspaceErrorBoundary`, `Status`, `TooltipProvider`, `Tooltip`, `EllipsisText`, 各类 `Skeleton`) | 写操作成功统一调 `toast(...)` 自动消退；工作区舞台由 `WorkspaceErrorBoundary` 兜底渲染异常保留侧栏导航；超长文本统一用 `EllipsisText`（溢出时自动浮出石板 `Tooltip`），禁止生硬原生 `title`；所有状态枚举统一走 `<Status>` |
| 弹窗、分级确认与二次验证 | `overlay.tsx`, `hold-button.tsx`, `step-up-dialog.tsx`, `otp-input.tsx` (`Overlay`, `ConfirmDialog`, `HoldButton`, `StepUpDialog`, `OtpInput`) | 常规删除走 `ConfirmDialog`（幽灵取消 + `danger-outline` 红色描边即点生效）；覆盖恢复数据库、清空审计、重启上游、轮换/吊销管理密钥等高危中断操作传 `critical` 启用按住 1 秒 `HoldButton`；敏感操作统一走 `StepUpDialog`（面板请求体字段固定为 `totp`） |
| 按钮、输入、密码与表单 | `button.tsx`, `input.tsx`, `password-input.tsx`, `textarea.tsx`, `select.tsx`, `checkbox.tsx`, `segmented.tsx`, `field.tsx`, `form.tsx`, `save-bar.tsx` (`Button`, `Input`, `PasswordInput`, `Field`, `Form`, `FormGrid`, `FormSection`, `FormActions`, `SaveBar`) | 单位与货币放进 `Input` 的 `prefix`/`suffix`，右侧动作放进 `endAddon`；密码与密钥输入一律用 `PasswordInput`（内置单图标连续形变 `EyeMorphIcon`）；长配置表单底部统一用粘性 `SaveBar`；服务端字段级 `fields` 错误通过 `Form.errors={action.fieldErrors}` 自动标红定位 |
| 选择卡、文件、多值胶囊与权限规则 | `choice-cards.tsx`, `file-picker.tsx`, `tag-input.tsx`, `permission-picker.tsx`, `setting-inputs.tsx` (`ChoiceCards`, `FilePicker`, `TagInput`, `PermissionPicker`, `AccessRuleBuilder`, `AudienceRuleBuilder`, `SecretInput`, `ListInput`, `RowsEditor`) | 带价格/说明的方案或模式单选用整张可点 `ChoiceCards`（含键盘方向键导航）；文件上传一律用 `FilePicker`；IP/CIDR、模型或面额列表用 `TagInput`；权限与投放条件使用结构化规则器，严禁让用户手填 JSON |
| 连续形变图标、掩码凭证与二维码 | `icons.tsx`, `theme-icon.tsx`, `menu-toggle.tsx`, `copy-icon.tsx`, `copy-button.tsx`, `copy-value.tsx`, `secret-box.tsx`, `qr-code.tsx`, `tabs.tsx` (`EyeMorphIcon`, `ChevronMorphIcon`, `PlayPauseIcon`, `RefreshMorphIcon`, `RefreshButton`, `CopyButton`, `CopyValue`, `MaskedSecret`, `SecretRevealDialog`, `QrCode`) | 状态切换图标使用单 SVG 路径连续形变，禁止双图标硬切；列表掩码展示用 `MaskedSecret`，创建/轮换仅展示一次的明文统一走 `SecretRevealDialog`；扫码绑定与支付统一走带亮底衬板与过期遮罩的 `QrCode` |
| 筛选栏、具名列数据表与批量操作 | `combobox.tsx`, `filter-bar.tsx`, `date-range.tsx`, `data-table.tsx`, `batch-bar.tsx`, `pagination.tsx`, `../pages/remote-table.tsx` | 具名列 `Column<T>`（状态居中 `center`、数值右对齐 `number`、操作右对齐 `end`），远端分页统一走 `RemoteTable` |
| 跨工作台业务实体与遥测图表 | `entity.tsx`, `entity-select.tsx`, `entity-card.tsx`, `plan-picker.tsx`, `model-select.tsx`, `telemetry.tsx`, `telemetry-chart.tsx`, `spark-bars.tsx`, `donut-chart.tsx`, `plugin-sandbox.tsx` | 用户/分组/账号/代理/Key/请求跳转与选择器、60 格巡检健康柱、指标切换面积图与隔离插件沙箱帧 |

## Agent 开发硬性约束（由 `pnpm --dir web check` 与 `go test` 自动拦截）

1. 页面代码（`src/` 下除 `src/ui/` 外）禁止手写原生 `<button>`、`<details>`、`<fieldset>`、`<dialog>`、`<table>`、`<input type="file">`、`<input type="password">`，也禁止对 `<Input>` 传 `type="file"` 或 `type="password"`（必须使用 `FilePicker` 与 `PasswordInput`）。
2. 禁止引入 `EyeOff` 等双图标切换写法，禁止页面写内联 `style`、`<style>` 或任意值工具类（`w-[...]` 等）。
3. 颜色、圆角、字号、间距一律引用已定义的 `--weir-*` / `--panel-*` 变量，写死尺寸保持奇数刻度。
4. 面板敏感接口统一使用 `totp` 字段，前端通过 `useAction()` 自动继承单次操作根 `traceparent` 并把服务端 `fieldErrors` 传入 `<Form errors={action.fieldErrors}>`。
