package mcpserver_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommerceTools_Catalog(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		args        map[string]any
		getBody     string
		getStatus   int
		putBody     string
		putStatus   int
		wantErr     bool
		wantText    string
		wantPayload map[string]any
	}{
		{
			name:      "success - catalog_get returns product",
			tool:      "catalog_get",
			args:      map[string]any{"contentId": 11},
			getBody:   `{"data":{"product":{"contentId":11,"price":1000}}}`,
			getStatus: http.StatusOK,
			wantErr:   false,
			wantText:  `"price":1000`,
		},
		{
			name:      "error - catalog_get invalid id",
			tool:      "catalog_get",
			args:      map[string]any{"contentId": 0},
			getBody:   `{}`,
			getStatus: http.StatusOK,
			wantErr:   true,
			wantText:  "INVALID_ID",
		},
		{
			name:      "success - catalog_upsert overlays onto current record",
			tool:      "catalog_upsert",
			args:      map[string]any{"contentId": 11, "price": 2000},
			getBody:   `{"data":{"contentId":11,"price":1000,"sku":"old","stock":5}}`,
			getStatus: http.StatusOK,
			putBody:   `{"data":{"contentId":11}}`,
			putStatus: http.StatusOK,
			wantErr:   false,
			wantText:  `"contentId":11`,
			wantPayload: map[string]any{
				"price": float64(2000),
				"sku":   "old",
				"stock": float64(5),
			},
		},
		{
			name:      "success - catalog_upsert creates from empty when price given",
			tool:      "catalog_upsert",
			args:      map[string]any{"contentId": 12, "price": 500},
			getBody:   `{"error":{"code":"NOT_FOUND","message":"no record"}}`,
			getStatus: http.StatusNotFound,
			putBody:   `{"data":{"contentId":12}}`,
			putStatus: http.StatusOK,
			wantErr:   false,
			wantText:  `"contentId":12`,
			wantPayload: map[string]any{
				"price": float64(500),
			},
		},
		{
			name:      "error - catalog_upsert needs price without a record",
			tool:      "catalog_upsert",
			args:      map[string]any{"contentId": 12, "sku": "x"},
			getBody:   `{"error":{"code":"NOT_FOUND","message":"no record"}}`,
			getStatus: http.StatusNotFound,
			wantErr:   true,
			wantText:  "VALIDATION_ERROR",
		},
		{
			name:      "error - catalog_upsert needs a field",
			tool:      "catalog_upsert",
			args:      map[string]any{"contentId": 11},
			getBody:   `{}`,
			getStatus: http.StatusOK,
			wantErr:   true,
			wantText:  "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.getBody, status: tt.getStatus}
			if tt.putBody != "" {
				h.bodies = map[string]struct {
					body   string
					status int
				}{
					"GET /api/v1/catalog/products/11": {body: tt.getBody, status: tt.getStatus},
					"GET /api/v1/catalog/products/12": {body: tt.getBody, status: tt.getStatus},
					"PUT /api/v1/catalog/products/11": {body: tt.putBody, status: tt.putStatus},
					"PUT /api/v1/catalog/products/12": {body: tt.putBody, status: tt.putStatus},
				}
			}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			if tt.wantPayload != nil {
				for k, v := range tt.wantPayload {
					assert.Equal(t, v, h.gotPayload[k], "payload key %s", k)
				}
			}
		})
	}
}

func TestCommerceTools_Orders(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		body     string
		status   int
		wantErr  bool
		wantText string
	}{
		{
			name:     "success - orders_list filters by status",
			tool:     "orders_list",
			args:     map[string]any{"status": "pending"},
			body:     `{"data":[{"id":1}],"meta":{"pagination":{"hasMore":false}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":1`,
		},
		{
			name:     "success - orders_get returns detail",
			tool:     "orders_get",
			args:     map[string]any{"id": 42},
			body:     `{"data":{"order":{"id":42}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":42`,
		},
		{
			name:     "success - orders_transition moves status",
			tool:     "orders_transition",
			args:     map[string]any{"id": 42, "status": "shipped"},
			body:     `{"data":{"order":{"id":42,"status":"shipped"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"shipped"`,
		},
		{
			name:     "error - orders_transition surfaces illegal move",
			tool:     "orders_transition",
			args:     map[string]any{"id": 42, "status": "paid"},
			body:     `{"error":{"code":"ILLEGAL_TRANSITION","message":"nope"}}`,
			status:   http.StatusConflict,
			wantErr:  true,
			wantText: "ILLEGAL_TRANSITION",
		},
		{
			name:     "success - orders_set_shipping sets the line",
			tool:     "orders_set_shipping",
			args:     map[string]any{"id": 42, "amount": 900},
			body:     `{"data":{"order":{"id":42}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":42`,
		},
		{
			name:     "error - orders_get invalid id",
			tool:     "orders_get",
			args:     map[string]any{"id": 0},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "INVALID_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
		})
	}
}

func TestCommerceTools_PaymentsInvoice(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		args        map[string]any
		body        string
		status      int
		wantErr     bool
		wantText    string
		wantPayload map[string]any
	}{
		{
			name:     "success - payments_list returns proofs",
			tool:     "payments_list",
			args:     map[string]any{},
			body:     `{"data":[{"id":3}],"meta":{"pagination":{"hasMore":false}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":3`,
		},
		{
			name:     "success - payments_get returns proof with order",
			tool:     "payments_get",
			args:     map[string]any{"id": 3},
			body:     `{"data":{"payment":{"id":3}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"id":3`,
		},
		{
			name:     "success - payments_verify accepts proof",
			tool:     "payments_verify",
			args:     map[string]any{"id": 3},
			body:     `{"data":{"payment":{"id":3,"status":"verified"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"verified"`,
		},
		{
			name:     "success - payments_reject rejects proof",
			tool:     "payments_reject",
			args:     map[string]any{"id": 3},
			body:     `{"data":{"payment":{"id":3,"status":"rejected"}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"rejected"`,
		},
		{
			name:     "success - invoice_settings_get returns block",
			tool:     "invoice_settings_get",
			args:     map[string]any{},
			body:     `{"data":{"settings":{"taxRate":0.11}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"taxRate"`,
		},
		{
			name:     "success - invoice_settings_set saves block",
			tool:     "invoice_settings_set",
			args:     map[string]any{"settings": map[string]any{"taxRate": 0.12}},
			body:     `{"data":{"settings":{"taxRate":0.12}}}`,
			status:   http.StatusOK,
			wantErr:  false,
			wantText: `"taxRate"`,
			wantPayload: map[string]any{
				"taxRate": 0.12,
			},
		},
		{
			name:     "error - invoice_settings_set needs settings",
			tool:     "invoice_settings_set",
			args:     map[string]any{},
			body:     `{}`,
			status:   http.StatusOK,
			wantErr:  true,
			wantText: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &backendHandler{body: tt.body, status: tt.status}
			csess := testSession(t, h)
			text, isErr := callTool(t, csess, tt.tool, tt.args)
			assert.Equal(t, tt.wantErr, isErr)
			assert.Contains(t, text, tt.wantText)
			if tt.wantPayload != nil {
				assert.Equal(t, tt.wantPayload, h.gotPayload)
			}
		})
	}
}
