package platform

type GrantWire struct {
	Invites int64   `json:"invites"`
	Budget  int64   `json:"budget_cents"`
	Amounts []int64 `json:"amounts_cents"`
}

type Grant = GrantWire

type GrantChange struct {
	Invites int64   `json:"invites"`
	Budget  int64   `json:"budget_cents"`
	Amounts []int64 `json:"amounts_cents"`
	TOTP    string  `json:"totp"`
}

type IssueInput struct {
	Operation string `json:"operation_id"`
	Kind      string `json:"kind"`
	Amount    int64  `json:"amount_cents"`
}

type IssueResult struct {
	Code string `json:"code"`
}

type IssueEvent struct {
	Operation  string `json:"operation_id"`
	Actor      int64  `json:"actor_id,omitempty"`
	Kind       string `json:"kind"`
	Amount     int64  `json:"amount_cents"`
	State      string `json:"state"`
	CodeID     *int64 `json:"code_id"`
	Created    int64  `json:"created_at"`
	QuotaDelta int64  `json:"quota_delta"`
	RequestID  string `json:"request_id,omitempty"`
}

type Reconciliation struct {
	CodeID *int64 `json:"code_id"`
}
