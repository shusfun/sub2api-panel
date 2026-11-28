package testkit

import "net/http"

// 夹具注册点（第 0 步）：账本相关的模拟上游。
//
// 采样与站点识别（probe）都走 core.dialBalanceHost，只允许 https 公网地址，预览里的夹具上游访问不到，
// 所以预览里无法演示 probe 和实时采样，也不放宽这个限制。预览的账本历史由 seedPreviewLedger 直接写库。
func (f *fixture) servePreviewLedger(w http.ResponseWriter, r *http.Request, path, role string) bool {
	_, _, _, _ = w, r, path, role
	return false
}
