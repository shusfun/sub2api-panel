# 面板组件索引

页面只组合 `ui/` 中的交互组件。交互尺寸、焦点、状态和主题样式在对应公共组件及 `ui/styles/` 中维护。

| 组件 | 入口 | 页面示例 |
|---|---|---|
| 按钮、输入、下拉、字段、表单 | `button.tsx`, `input.tsx`, `select.tsx`, `field.tsx`, `form.tsx` | `../pages/keys.tsx` 的 Key 编辑表单 |
| 工具栏、区块、状态与反馈 | `layout.tsx`, `status.tsx`, `feedback.tsx` | `../pages/accounts.tsx` 的筛选工具栏 |
| 具名列数据表和分页 | `data-table.tsx`, `pagination.tsx` | `../pages/accounts.tsx` 的额度详情表 |
| 筛选列表与远端分页 | 页面适配器 `../pages/remote-table.tsx` | `../pages/users.tsx` 的用户筛选表 |
| 抽屉、弹窗、确认弹窗 | `overlay.tsx` | `../pages/users.tsx` 的用户详情抽屉 |
| 状态标签、复制结果、图标按钮 | `status.tsx`, `copy-value.tsx`, `copy-icon.tsx`, `tooltip.tsx` | `../pages/codes.tsx` 的发码记录 |

新增或调整交互时优先扩展这些入口。列表列必须提供稳定 `id`、标题、可选对齐方式和单行渲染函数。异步状态使用 `Resource`、`Feedback` 与 `ErrorLine`。页面样式读取主题 token，不另行定义控件高度、焦点和状态样式。
