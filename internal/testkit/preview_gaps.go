package testkit

import "net/http"

// 夹具注册点（第 0 步）：线 C 负责。补齐原版缺口接口的样本：错误透传规则、TLS 指纹档案、提示词审计、
// 风控补全、推广返利、订阅批量、账号隐私与影子账号、监控运行/历史/复制/模板、用量清理任务、生图对象存储。
func (f *fixture) servePreviewGaps(w http.ResponseWriter, r *http.Request, path, role string) bool {
	_, _, _, _ = w, r, path, role
	return false
}
