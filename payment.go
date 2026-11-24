package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type checkoutPlan struct {
	namedItem
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

func (p *panel) paymentRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/payment/checkout", false, p.paymentCheckout)
	p.route(mux, "GET /api/payment/orders", false, p.listPaymentOrders)
	p.route(mux, "POST /api/payment/orders", false, p.createPaymentOrder)
	p.route(mux, "GET /api/payment/orders/{id}", false, p.paymentOrderDetail)
	p.route(mux, "POST /api/payment/orders/{id}/cancel", false, p.cancelPaymentOrder)
	p.public(mux, "POST /api/public/payment/result", p.publicPaymentResult)
	p.route(mux, "GET /api"+adminOrderPath, true, p.adminPaymentOrders)
	p.route(mux, "GET /api"+adminOrderPath+"/{id}", true, p.adminPaymentOrder)
	p.route(mux, "POST /api"+adminOrderPath+"/{id}/{action}", true, p.adminPaymentAction)
	p.route(mux, "POST /api"+adminOrderPath+"/{id}/refund/query", true, p.adminPaymentRefundQuery)
}

func (p *panel) adminPaymentOrders(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "status", "order_type", "payment_type", "user_id", "keyword")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, adminOrderPath, page, query,
		func(row adminPaymentOrder) (adminPaymentOrder, error) { return row, requiredID(row.ID) })
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) adminPaymentOrder(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	row, err := upstream[struct {
		Order adminPaymentOrder `json:"order"`
	}](r.Context(), p.upstream, http.MethodGet, fmt.Sprintf("%s/%d", adminOrderPath, id), a.tokens.Access, "", "", nil)
	if err == nil && row.Order.ID != id {
		err = fail(http.StatusBadGateway, "invalid_upstream")
	}
	return send(w, http.StatusOK, row.Order, err)
}

func (p *panel) adminPaymentAction(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	action := r.PathValue("action")
	path := fmt.Sprintf("%s/%d/%s", adminOrderPath, id, action)
	switch action {
	case "cancel", "retry":
		result, err := upstream[struct{}](r.Context(), p.upstream, http.MethodPost, path, a.tokens.Access, "", "", nil)
		return send(w, http.StatusOK, result, err)
	case "refund":
		input, err := decodeBody[struct {
			Amount float64 `json:"amount"`
			Reason string  `json:"reason"`
			Deduct bool    `json:"deduct_balance"`
			Force  bool    `json:"force"`
		}](r)
		if err != nil {
			return err
		}
		if input.Amount <= 0 || input.Amount > 1e9 || len(input.Reason) > 500 {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		result, err := upstream[paymentRefund](r.Context(), p.upstream, http.MethodPost, path, a.tokens.Access, "", "", input)
		return send(w, http.StatusOK, result, err)
	default:
		return fail(http.StatusNotFound, "not_found")
	}
}

func (p *panel) adminPaymentRefundQuery(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	result, err := upstream[paymentRefund](r.Context(), p.upstream, http.MethodPost,
		fmt.Sprintf("%s/%d/refund/query", adminOrderPath, id), a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, result, err)
}

func (p *panel) requirePayment(r *http.Request) error {
	settings, err := p.settings(r.Context())
	if err == nil && !settings.Payment {
		return fail(http.StatusForbidden, "not_allowed")
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

func (p *panel) paymentCheckout(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if err := p.requirePayment(r); err != nil {
		return err
	}
	result, err := upstream[paymentCheckout](r.Context(), p.upstream, http.MethodGet, "/payment/checkout-info", a.tokens.Access, "", "", nil)
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
	return send(w, http.StatusOK, result, nil)
}

func (p *panel) listPaymentOrders(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "status", "order_type")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/payment/orders/my", page, query,
		func(row paymentOrder) (paymentOrder, error) { return row, requiredID(row.ID) })
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) paymentOrderDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	row, err := upstream[paymentOrder](r.Context(), p.upstream, http.MethodGet, fmt.Sprintf("/payment/orders/%d", id), a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, row, err)
}

func (p *panel) cancelPaymentOrder(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	result, err := upstream[struct{}](r.Context(), p.upstream, http.MethodPost, fmt.Sprintf("/payment/orders/%d/cancel", id), a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, result, err)
}

func (p *panel) createPaymentOrder(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if err := p.requirePayment(r); err != nil {
		return err
	}
	input, err := decodeBody[struct {
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
		return fail(http.StatusBadRequest, "invalid_input")
	}
	trace := meta(r.Context())
	trace.Referer = p.origin + "/purchase"
	ctx := context.WithValue(r.Context(), metaKey{}, trace)
	result, err := upstream[struct {
		ID       int64     `json:"order_id"`
		PayURL   string    `json:"pay_url,omitempty"`
		QR       string    `json:"qr_code,omitempty"`
		Pay      float64   `json:"pay_amount"`
		Currency string    `json:"currency"`
		Expires  time.Time `json:"expires_at"`
	}](ctx, p.upstream, http.MethodPost, "/payment/orders", a.tokens.Access, "", "", map[string]any{
		"amount": input.Amount, "order_type": input.Type, "plan_id": input.PlanID, "payment_type": input.Method,
		"return_url": p.origin + "/payment/result", "payment_source": "hosted_redirect",
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
	return send(w, http.StatusOK, result, requiredID(result.ID))
}

func (p *panel) publicPaymentResult(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeBody[struct {
		Token string `json:"resume_token"`
	}](r)
	if err != nil {
		return err
	}
	if len(input.Token) == 0 || len(input.Token) > 2048 || strings.ContainsAny(input.Token, " \t\r\n") {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	p.runtimeMu.RLock()
	client, setup := p.upstream, p.setup
	p.runtimeMu.RUnlock()
	if setup {
		return fail(http.StatusServiceUnavailable, "setup_required")
	}
	result, err := upstream[paymentOrder](r.Context(), client, http.MethodPost, "/payment/public/orders/resolve", "", "", "", input)
	return send(w, http.StatusOK, result, err)
}
