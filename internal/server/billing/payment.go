package billing

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func RegisterRoutes(mux *http.ServeMux, s *core.Server) {
	h := &handler{s: s}
	h.registerPaymentRoutes(mux)
	h.registerRedeemAdminRoutes(mux)
	h.registerRedeemAndGrantRoutes(mux)
}

type handler struct {
	s *core.Server
}

type checkoutPlan struct {
	platform.NamedItem
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Duration    int     `json:"validity_days"`
	Unit        string  `json:"validity_unit"`
}

type checkoutMethod struct {
	Available bool    `json:"available"`
	Name      string  `json:"display_name"`
	Currency  string  `json:"currency"`
	Min       float64 `json:"single_min"`
	Max       float64 `json:"single_max"`
	Fee       float64 `json:"fee_rate"`
}

type paymentCheckout struct {
	Plans           []checkoutPlan            `json:"plans"`
	Methods         map[string]checkoutMethod `json:"methods"`
	BalanceDisabled bool                      `json:"balance_disabled"`
}

type paymentOrder struct {
	ID        int64      `json:"id"`
	Amount    float64    `json:"amount"`
	Paid      float64    `json:"pay_amount"`
	Currency  string     `json:"currency"`
	Method    string     `json:"payment_type"`
	Type      string     `json:"order_type"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}

type adminPaymentOrder struct {
	paymentOrder
	UserID       int64   `json:"user_id"`
	UserEmail    string  `json:"user_email"`
	RefundAmount float64 `json:"refund_amount"`
}

type paymentRefund struct {
	Success      bool    `json:"success"`
	RequireForce bool    `json:"require_force"`
	Balance      float64 `json:"balance_deducted"`
	Days         int     `json:"subscription_days_deducted"`
}

const adminOrderPath = "/admin/payment/orders"

func (h *handler) registerPaymentRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/payment/checkout", false, h.paymentCheckout)
	h.s.Route(mux, "GET /api/payment/orders", false, h.listPaymentOrders)
	h.s.Route(mux, "POST /api/payment/orders", false, h.createPaymentOrder)
	h.s.Route(mux, "GET /api/payment/orders/{id}", false, h.paymentOrderDetail)
	h.s.Route(mux, "POST /api/payment/orders/{id}/cancel", false, h.cancelPaymentOrder)
	h.s.Public(mux, "POST /api/public/payment/result", h.publicPaymentResult)
	h.s.Route(mux, "GET /api"+adminOrderPath, true, h.adminPaymentOrders)
	h.s.Route(mux, "GET /api"+adminOrderPath+"/{id}", true, h.adminPaymentOrder)
	h.s.Route(mux, "POST /api"+adminOrderPath+"/{id}/{action}", true, h.adminPaymentAction)
	h.s.Route(mux, "POST /api"+adminOrderPath+"/{id}/refund/query", true, h.adminPaymentRefundQuery)
}

func (h *handler) adminPaymentOrders(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "status", "order_type", "payment_type", "user_id", "keyword")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, adminOrderPath, page, query,
		func(row adminPaymentOrder) (adminPaymentOrder, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) adminPaymentOrder(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[struct {
		Order adminPaymentOrder `json:"order"`
	}](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("%s/%d", adminOrderPath, id), a.Tokens.Access, "", "", nil)
	if err == nil && row.Order.ID != id {
		err = platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	return platform.Send(w, http.StatusOK, row.Order, err)
}

func (h *handler) adminPaymentAction(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	action := r.PathValue("action")
	path := fmt.Sprintf("%s/%d/%s", adminOrderPath, id, action)
	switch action {
	case "cancel", "retry":
		result, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPost, path, a.Tokens.Access, "", "", nil)
		return platform.Send(w, http.StatusOK, result, err)
	case "refund":
		input, err := platform.DecodeBody[struct {
			Amount float64 `json:"amount"`
			Reason string  `json:"reason"`
			Deduct bool    `json:"deduct_balance"`
			Force  bool    `json:"force"`
		}](r)
		if err != nil {
			return err
		}
		if input.Amount <= 0 || input.Amount > 1e9 || len(input.Reason) > 500 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		result, err := platform.Upstream[paymentRefund](r.Context(), h.s.Upstream, http.MethodPost, path, a.Tokens.Access, "", "", input)
		return platform.Send(w, http.StatusOK, result, err)
	default:
		return platform.Fail(http.StatusNotFound, "not_found")
	}
}

func (h *handler) adminPaymentRefundQuery(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	result, err := platform.Upstream[paymentRefund](r.Context(), h.s.Upstream, http.MethodPost,
		fmt.Sprintf("%s/%d/refund/query", adminOrderPath, id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) requirePayment(r *http.Request) error {
	settings, err := h.s.Settings(r.Context())
	if err == nil && !settings.Payment {
		return platform.Fail(http.StatusForbidden, "not_allowed")
	}
	return err
}

func supportedHostedMethod(method string) bool {
	switch method {
	case "alipay", "wxpay", "easypay", "alipay_direct", "wxpay_direct":
		return true
	default:
		return false
	}
}

func (h *handler) paymentCheckout(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if err := h.requirePayment(r); err != nil {
		return err
	}
	result, err := platform.Upstream[paymentCheckout](r.Context(), h.s.Upstream, http.MethodGet, "/payment/checkout-info", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if result.Plans == nil {
		result.Plans = []checkoutPlan{}
	}
	methods := make(map[string]checkoutMethod)
	for name, method := range result.Methods {
		if supportedHostedMethod(name) && method.Available {
			methods[name] = method
		}
	}
	result.Methods = methods
	return platform.Send(w, http.StatusOK, result, nil)
}

func (h *handler) listPaymentOrders(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "status", "order_type")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/payment/orders/my", page, query,
		func(row paymentOrder) (paymentOrder, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) paymentOrderDetail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[paymentOrder](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("/payment/orders/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) cancelPaymentOrder(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	result, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPost, fmt.Sprintf("/payment/orders/%d/cancel", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) createPaymentOrder(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if err := h.requirePayment(r); err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Amount float64 `json:"amount"`
		Method string  `json:"payment_type"`
		Type   string  `json:"order_type"`
		PlanID int64   `json:"plan_id,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	if !supportedHostedMethod(input.Method) || input.Type != "balance" && input.Type != "subscription" ||
		input.Type == "subscription" && input.PlanID <= 0 || input.Type == "balance" && (input.Amount <= 0 || input.Amount > 1e9 ||
		math.Abs(math.Round(input.Amount*100)-input.Amount*100) > 0.00001 || input.PlanID != 0) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	trace := platform.Meta(r.Context())
	trace.Referer = h.s.Origin + "/purchase"
	ctx := platform.WithMeta(r.Context(), trace)
	result, err := platform.Upstream[struct {
		ID       int64     `json:"order_id"`
		PayURL   string    `json:"pay_url,omitempty"`
		QR       string    `json:"qr_code,omitempty"`
		Pay      float64   `json:"pay_amount"`
		Currency string    `json:"currency"`
		Expires  time.Time `json:"expires_at"`
	}](ctx, h.s.Upstream, http.MethodPost, "/payment/orders", a.Tokens.Access, "", "", map[string]any{
		"amount": input.Amount, "order_type": input.Type, "plan_id": input.PlanID, "payment_type": input.Method,
		"return_url": h.s.Origin + "/payment/result", "payment_source": "hosted_redirect",
	})
	if err != nil {
		return err
	}
	if result.PayURL != "" {
		u, err := url.Parse(result.PayURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			result.PayURL = ""
		}
	}
	return platform.Send(w, http.StatusOK, result, platform.RequiredID(result.ID))
}

func (h *handler) publicPaymentResult(w http.ResponseWriter, r *http.Request) error {
	input, err := platform.DecodeBody[struct {
		Token string `json:"resume_token"`
	}](r)
	if err != nil {
		return err
	}
	if len(input.Token) == 0 || len(input.Token) > 2048 || strings.ContainsAny(input.Token, " \t\r\n") {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	h.s.RuntimeMu.RLock()
	client, setup := h.s.Upstream, h.s.Setup
	h.s.RuntimeMu.RUnlock()
	if setup {
		return platform.Fail(http.StatusServiceUnavailable, "setup_required")
	}
	result, err := platform.Upstream[paymentOrder](r.Context(), client, http.MethodPost, "/payment/public/orders/resolve", "", "", "", input)
	return platform.Send(w, http.StatusOK, result, err)
}
