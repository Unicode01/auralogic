package service

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
	"gorm.io/gorm"
)

func TestBuildPluginHostAccessClaimsPrefersExplicitOperatorUserID(t *testing.T) {
	subjectUserID := uint(7)
	operatorUserID := uint(42)

	claims := BuildPluginHostAccessClaims(nil, &ExecutionContext{
		OperatorUserID: &operatorUserID,
		UserID:         &subjectUserID,
	}, time.Minute)
	if claims.OperatorUserID != operatorUserID {
		t.Fatalf("expected operator user id %d, got %d", operatorUserID, claims.OperatorUserID)
	}
}

func TestExecutePluginHostActionMasksPrivacyProtectedOrder(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "privacy-user@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)
	order.PrivacyProtected = true
	order.ReceiverName = "Alice"
	order.ReceiverPhone = "13800138000"
	order.ReceiverEmail = "alice@example.com"
	order.ReceiverAddress = "No. 1 Demo Street"
	if err := db.Save(&order).Error; err != nil {
		t.Fatalf("update test order failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.view"},
	}, "host.order.get", map[string]interface{}{
		"id": order.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if masked, ok := result["privacy_masked"].(bool); !ok || !masked {
		t.Fatalf("expected privacy_masked=true, got %#v", result["privacy_masked"])
	}
	if got := result["receiver_name"]; got != "***" {
		t.Fatalf("expected masked receiver_name, got %#v", got)
	}
	if got := result["receiver_phone"]; got != "138****8000" {
		t.Fatalf("expected masked receiver_phone, got %#v", got)
	}
	if got := result["receiver_email"]; got != "alice@example.com" {
		t.Fatalf("expected receiver_email to remain visible, got %#v", got)
	}
	if got := result["receiver_address"]; got != "***" {
		t.Fatalf("expected masked receiver_address, got %#v", got)
	}
}

func TestExecutePluginHostActionRequiresOperatorPermission(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "forbidden-user@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)

	_, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"user.view"},
	}, "host.order.get", map[string]interface{}{
		"id": order.ID,
	})
	if err == nil {
		t.Fatalf("expected operator permission denial")
	}

	var hostErr *PluginHostActionError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected PluginHostActionError, got %T (%v)", err, err)
	}
	if hostErr.Status != 403 {
		t.Fatalf("expected 403 status, got %+v", hostErr)
	}
	if hostErr.Message != "operator permission denied" {
		t.Fatalf("unexpected host error message: %+v", hostErr)
	}
}

func TestExecutePluginHostActionAllowsWorkspaceAppendWithoutOperatorScope(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	const pluginID uint = 21
	plugin := &models.Plugin{ID: pluginID, Name: "runtime", Runtime: PluginRuntimeJSWorker}
	manager := &PluginManagerService{
		workspaceBuffers: map[uint]*pluginWorkspaceBuffer{
			pluginID: newPluginWorkspaceBuffer(pluginID, plugin.Name, plugin.Runtime, 8),
		},
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	runtime := NewPluginHostRuntime(db, nil, manager)

	result, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		PluginID:           pluginID,
		ExecutionID:        "pex_runtime_eval",
		ScopeAuthenticated: false,
	}, "host.workspace.append", map[string]interface{}{
		"command_id": "pex_runtime_eval",
		"entries": []pluginipc.WorkspaceBufferEntry{{
			Message: "async",
			Channel: "stdout",
			Level:   "info",
			Source:  "console.log",
		}},
	})
	if err != nil {
		t.Fatalf("expected workspace append without operator scope to succeed, got %v", err)
	}

	if got := interfaceToTestInt64(result["appended"]); got != 1 {
		t.Fatalf("expected appended=1, got %#v", result["appended"])
	}
	snapshot := manager.GetPluginWorkspaceSnapshot(plugin, 10)
	if snapshot.EntryCount != 1 || len(snapshot.Entries) != 1 {
		t.Fatalf("expected one appended workspace entry, got %+v", snapshot)
	}
	if snapshot.Entries[0].Message != "async" {
		t.Fatalf("expected appended async entry to be retained, got %+v", snapshot.Entries[0])
	}
}

func TestExecutePluginHostActionAllowsPrivacyReadWithDoublePermission(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "privacy-full@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)
	order.PrivacyProtected = true
	order.ReceiverName = "Bob"
	order.ReceiverPhone = "13900139000"
	order.ReceiverAddress = "No. 9 Privacy Road"
	if err := db.Save(&order).Error; err != nil {
		t.Fatalf("update test order failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderRead, PluginPermissionHostOrderPrivacy},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.view", "order.view_privacy"},
	}, "host.order.get", map[string]interface{}{
		"id": order.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if masked, ok := result["privacy_masked"].(bool); !ok || masked {
		t.Fatalf("expected privacy_masked=false, got %#v", result["privacy_masked"])
	}
	if got := result["receiver_name"]; got != "Bob" {
		t.Fatalf("expected raw receiver_name, got %#v", got)
	}
	if got := result["receiver_phone"]; got != "13900139000" {
		t.Fatalf("expected raw receiver_phone, got %#v", got)
	}
	if got := result["receiver_address"]; got != "No. 9 Privacy Road" {
		t.Fatalf("expected raw receiver_address, got %#v", got)
	}
}

func TestExecutePluginHostActionAssignsTrackingByID(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "tracking-id@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderAssignTracking},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.assign_tracking"},
	}, "host.order.assign_tracking", map[string]interface{}{
		"id":          order.ID,
		"tracking_no": "TRACK-ID-001",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["id"]); got != int64(order.ID) {
		t.Fatalf("expected id=%d, got %#v", order.ID, got)
	}
	if got := result["order_no"]; got != order.OrderNo {
		t.Fatalf("expected order_no=%q, got %#v", order.OrderNo, got)
	}
	if got := result["tracking_no"]; got != "TRACK-ID-001" {
		t.Fatalf("expected tracking_no=TRACK-ID-001, got %#v", got)
	}
	if got := result["status"]; got != string(models.OrderStatusShipped) {
		t.Fatalf("expected status=shipped, got %#v", got)
	}
	if result["shipped_at"] == nil {
		t.Fatalf("expected shipped_at to be populated, got %#v", result["shipped_at"])
	}
}

func TestExecutePluginHostActionAssignsTrackingByOrderNo(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "tracking-order-no@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderAssignTracking},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.assign_tracking"},
	}, "host.order.assign_tracking", map[string]interface{}{
		"order_no":    order.OrderNo,
		"tracking_no": "TRACK-ORDER-NO-001",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["id"]); got != int64(order.ID) {
		t.Fatalf("expected id=%d, got %#v", order.ID, got)
	}
	if got := result["order_no"]; got != order.OrderNo {
		t.Fatalf("expected order_no=%q, got %#v", order.OrderNo, got)
	}
	if got := result["tracking_no"]; got != "TRACK-ORDER-NO-001" {
		t.Fatalf("expected tracking_no=TRACK-ORDER-NO-001, got %#v", got)
	}
	if got := result["status"]; got != string(models.OrderStatusShipped) {
		t.Fatalf("expected status=shipped, got %#v", got)
	}
	if result["shipped_at"] == nil {
		t.Fatalf("expected shipped_at to be populated, got %#v", result["shipped_at"])
	}
}

func TestExecutePluginHostActionRequestsResubmitByID(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "resubmit-id@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderRequestResubmit},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.edit"},
	}, "host.order.request_resubmit", map[string]interface{}{
		"id":     order.ID,
		"reason": "Need complete shipping details",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["id"]); got != int64(order.ID) {
		t.Fatalf("expected id=%d, got %#v", order.ID, got)
	}
	if got := result["order_no"]; got != order.OrderNo {
		t.Fatalf("expected order_no=%q, got %#v", order.OrderNo, got)
	}
	if got := result["status"]; got != string(models.OrderStatusNeedResubmit) {
		t.Fatalf("expected status=need_resubmit, got %#v", got)
	}
	if result["form_expires_at"] == nil {
		t.Fatalf("expected form_expires_at to be populated, got %#v", result["form_expires_at"])
	}
	if _, exists := result["new_form_token"]; exists {
		t.Fatalf("expected new_form_token to stay hidden, got %#v", result["new_form_token"])
	}
}

func TestExecutePluginHostActionRejectsResubmitForInvalidStatus(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "resubmit-invalid@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)
	order.Status = models.OrderStatusShipped
	if err := db.Save(&order).Error; err != nil {
		t.Fatalf("update test order failed: %v", err)
	}

	_, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderRequestResubmit},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.edit"},
	}, "host.order.request_resubmit", map[string]interface{}{
		"id":     order.ID,
		"reason": "Need correction",
	})
	if err == nil {
		t.Fatalf("expected invalid status error")
	}

	var hostErr *PluginHostActionError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected PluginHostActionError, got %T (%v)", err, err)
	}
	if hostErr.Status != 400 {
		t.Fatalf("expected 400 status, got %+v", hostErr)
	}
}

func TestExecutePluginHostActionMarksOrderPaidByOrderNo(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}, &models.VirtualProductStock{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "mark-paid-user@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)
	order.Status = models.OrderStatusPendingPayment
	order.TotalAmount = 1999
	order.ReceiverName = "Plugin Host"
	order.ReceiverAddress = "1 Host Street"
	if err := db.Save(&order).Error; err != nil {
		t.Fatalf("update test order failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderMarkPaid},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.status_update"},
	}, "host.order.mark_paid", map[string]interface{}{
		"order_no":    order.OrderNo,
		"adminRemark": "Paid via plugin reconciliation",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["id"]); got != int64(order.ID) {
		t.Fatalf("expected id=%d, got %#v", order.ID, got)
	}
	if got := result["order_no"]; got != order.OrderNo {
		t.Fatalf("expected order_no=%q, got %#v", order.OrderNo, got)
	}
	if got := result["status"]; got != string(models.OrderStatusPending) {
		t.Fatalf("expected status=pending, got %#v", got)
	}
	if got := interfaceToTestInt64(result["total_amount_minor"]); got != 1999 {
		t.Fatalf("expected total_amount_minor=1999, got %#v", got)
	}
	if !interfaceIsNil(result["shipped_at"]) {
		t.Fatalf("expected shipped_at to remain nil for physical order, got %#v", result["shipped_at"])
	}

	var updated models.Order
	if err := db.First(&updated, order.ID).Error; err != nil {
		t.Fatalf("reload updated order failed: %v", err)
	}
	if updated.Status != models.OrderStatusPending {
		t.Fatalf("expected persisted status=pending, got %s", updated.Status)
	}
	if updated.AdminRemark != "Paid via plugin reconciliation" {
		t.Fatalf("expected admin remark to persist, got %#v", updated.AdminRemark)
	}
}

func TestExecutePluginHostActionUpdatesOrderPriceByOrderNo(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "update-price-user@example.com", "user")
	order := createPluginHostTestOrder(t, db, user.ID)
	order.Status = models.OrderStatusPendingPayment
	order.TotalAmount = 1299
	if err := db.Save(&order).Error; err != nil {
		t.Fatalf("update test order failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostOrderUpdatePrice},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"order.edit"},
	}, "host.order.update_price", map[string]interface{}{
		"order_no":         order.OrderNo,
		"totalAmountMinor": 2599,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["id"]); got != int64(order.ID) {
		t.Fatalf("expected id=%d, got %#v", order.ID, got)
	}
	if got := result["status"]; got != string(models.OrderStatusPendingPayment) {
		t.Fatalf("expected status=pending_payment, got %#v", got)
	}
	if got := interfaceToTestInt64(result["total_amount_minor"]); got != 2599 {
		t.Fatalf("expected total_amount_minor=2599, got %#v", got)
	}

	var updated models.Order
	if err := db.First(&updated, order.ID).Error; err != nil {
		t.Fatalf("reload updated order failed: %v", err)
	}
	if updated.TotalAmount != 2599 {
		t.Fatalf("expected persisted total_amount_minor=2599, got %d", updated.TotalAmount)
	}
}

func TestExecutePluginHostActionGetsProductBySKU(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Product{}); err != nil {
		t.Fatalf("auto migrate product host api models failed: %v", err)
	}

	product := createPluginHostTestProduct(t, db, "SKU-HOST-PRODUCT-1")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostProductRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.product.get", map[string]interface{}{
		"sku": product.SKU,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["sku"]; got != product.SKU {
		t.Fatalf("expected sku=%q, got %#v", product.SKU, got)
	}
	if got := interfaceToTestInt64(result["price_minor"]); got != int64(product.Price) {
		t.Fatalf("expected price_minor=%d, got %#v", product.Price, got)
	}
}

func TestExecutePluginHostActionGetsInventoryBySKU(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Inventory{}); err != nil {
		t.Fatalf("auto migrate inventory host api models failed: %v", err)
	}

	inventory := createPluginHostTestInventory(t, db, "INV-HOST-1")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostInventoryRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.inventory.get", map[string]interface{}{
		"sku": inventory.SKU,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["sku"]; got != inventory.SKU {
		t.Fatalf("expected sku=%q, got %#v", inventory.SKU, got)
	}
	if got := interfaceToTestInt64(result["remaining_stock"]); got != int64(inventory.GetRemainingStock()) {
		t.Fatalf("expected remaining_stock=%d, got %#v", inventory.GetRemainingStock(), got)
	}
}

func TestExecutePluginHostActionGetsPromoCodeByCode(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PromoCode{}); err != nil {
		t.Fatalf("auto migrate promo host api models failed: %v", err)
	}

	promo := createPluginHostTestPromoCode(t, db, "HOSTPROMO")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPromoRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.promo.get", map[string]interface{}{
		"code": promo.Code,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["code"]; got != promo.Code {
		t.Fatalf("expected code=%q, got %#v", promo.Code, got)
	}
	if got := interfaceToTestInt64(result["available_quantity"]); got != int64(promo.GetAvailableQuantity()) {
		t.Fatalf("expected available_quantity=%d, got %#v", promo.GetAvailableQuantity(), got)
	}
}

func TestExecutePluginHostActionListsTickets(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Ticket{}); err != nil {
		t.Fatalf("auto migrate ticket host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "ticket-user@example.com", "user")
	ticket := createPluginHostTestTicket(t, db, user.ID)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostTicketList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"ticket.view"},
		OperatorUserID:     77,
	}, "host.ticket.list", map[string]interface{}{
		"search": ticket.TicketNo,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one ticket item, got %#v", result["items"])
	}
	if got := items[0]["ticket_no"]; got != ticket.TicketNo {
		t.Fatalf("expected ticket_no=%q, got %#v", ticket.TicketNo, got)
	}
}

func TestExecutePluginHostActionRepliesTicket(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Ticket{}, &models.TicketMessage{}); err != nil {
		t.Fatalf("auto migrate ticket host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "ticket-reply-user@example.com", "user")
	operator := createPluginHostTestUser(t, db, "ticket-reply-admin@example.com", "admin")
	ticket := createPluginHostTestTicket(t, db, user.ID)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostTicketReply},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"ticket.reply"},
		OperatorUserID:     operator.ID,
	}, "host.ticket.reply", map[string]interface{}{
		"id":      ticket.ID,
		"content": "Reply from plugin",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["ticket_id"]); got != int64(ticket.ID) {
		t.Fatalf("expected ticket_id=%d, got %#v", ticket.ID, got)
	}
	if got := result["ticket_no"]; got != ticket.TicketNo {
		t.Fatalf("expected ticket_no=%q, got %#v", ticket.TicketNo, got)
	}
	if got := result["status"]; got != string(models.TicketStatusProcessing) {
		t.Fatalf("expected status=processing, got %#v", got)
	}
	if got := interfaceToTestInt64(result["assigned_to"]); got != int64(operator.ID) {
		t.Fatalf("expected assigned_to=%d, got %#v", operator.ID, got)
	}
	if got := result["content_type"]; got != "text" {
		t.Fatalf("expected content_type=text, got %#v", got)
	}
	if result["created_at"] == nil {
		t.Fatalf("expected created_at to be populated, got %#v", result["created_at"])
	}
	if _, exists := result["content"]; exists {
		t.Fatalf("expected content to stay hidden, got %#v", result["content"])
	}
}

func TestExecutePluginHostActionRejectsReplyToClosedTicket(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Ticket{}, &models.TicketMessage{}); err != nil {
		t.Fatalf("auto migrate ticket host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "ticket-closed-user@example.com", "user")
	operator := createPluginHostTestUser(t, db, "ticket-closed-admin@example.com", "admin")
	ticket := createPluginHostTestTicket(t, db, user.ID)
	ticket.Status = models.TicketStatusClosed
	if err := db.Save(&ticket).Error; err != nil {
		t.Fatalf("update test ticket failed: %v", err)
	}

	_, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostTicketReply},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"ticket.reply"},
		OperatorUserID:     operator.ID,
	}, "host.ticket.reply", map[string]interface{}{
		"id":      ticket.ID,
		"content": "Reply from plugin",
	})
	if err == nil {
		t.Fatalf("expected closed ticket error")
	}

	var hostErr *PluginHostActionError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected PluginHostActionError, got %T (%v)", err, err)
	}
	if hostErr.Status != 400 {
		t.Fatalf("expected 400 status, got %+v", hostErr)
	}
}

func TestExecutePluginHostActionUpdatesTicketAndClearsClosedAtWhenReopened(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Ticket{}); err != nil {
		t.Fatalf("auto migrate ticket host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "ticket-update-user@example.com", "user")
	operator := createPluginHostTestUser(t, db, "ticket-update-admin@example.com", "admin")
	ticket := createPluginHostTestTicket(t, db, user.ID)

	resolveResult, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostTicketUpdate},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"ticket.status_update"},
	}, "host.ticket.update", map[string]interface{}{
		"id":         ticket.ID,
		"status":     "resolved",
		"priority":   "high",
		"assignedTo": operator.ID,
	})
	if err != nil {
		t.Fatalf("resolve ExecutePluginHostAction returned error: %v", err)
	}

	if got := resolveResult["status"]; got != string(models.TicketStatusResolved) {
		t.Fatalf("expected resolved status, got %#v", got)
	}
	if got := resolveResult["priority"]; got != string(models.TicketPriorityHigh) {
		t.Fatalf("expected high priority, got %#v", got)
	}
	if got := interfaceToTestInt64(resolveResult["assigned_to"]); got != int64(operator.ID) {
		t.Fatalf("expected assigned_to=%d, got %#v", operator.ID, got)
	}
	if resolveResult["closed_at"] == nil {
		t.Fatalf("expected closed_at to be populated after resolve, got %#v", resolveResult["closed_at"])
	}

	reopenResult, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostTicketUpdate},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"ticket.status_update"},
	}, "host.ticket.update", map[string]interface{}{
		"ticket_no":     ticket.TicketNo,
		"status":        "open",
		"priority":      "urgent",
		"clearAssignee": true,
	})
	if err != nil {
		t.Fatalf("reopen ExecutePluginHostAction returned error: %v", err)
	}

	if got := reopenResult["status"]; got != string(models.TicketStatusOpen) {
		t.Fatalf("expected open status, got %#v", got)
	}
	if got := reopenResult["priority"]; got != string(models.TicketPriorityUrgent) {
		t.Fatalf("expected urgent priority, got %#v", got)
	}
	if !interfaceIsNil(reopenResult["assigned_to"]) {
		t.Fatalf("expected assigned_to to be cleared, got %#v", reopenResult["assigned_to"])
	}
	if !interfaceIsNil(reopenResult["closed_at"]) {
		t.Fatalf("expected closed_at to be cleared after reopen, got %#v", reopenResult["closed_at"])
	}

	var updated models.Ticket
	if err := db.First(&updated, ticket.ID).Error; err != nil {
		t.Fatalf("reload updated ticket failed: %v", err)
	}
	if updated.Status != models.TicketStatusOpen {
		t.Fatalf("expected persisted status=open, got %s", updated.Status)
	}
	if updated.Priority != models.TicketPriorityUrgent {
		t.Fatalf("expected persisted priority=urgent, got %s", updated.Priority)
	}
	if updated.AssignedTo != nil {
		t.Fatalf("expected persisted assigned_to to be nil, got %#v", updated.AssignedTo)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("expected persisted closed_at to be nil, got %#v", updated.ClosedAt)
	}
}

func TestExecutePluginHostActionGetsSerialByNumber(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Order{}, &models.Product{}, &models.ProductSerial{}); err != nil {
		t.Fatalf("auto migrate serial host api models failed: %v", err)
	}

	user := createPluginHostTestUser(t, db, "serial-user@example.com", "user")
	product := createPluginHostTestProduct(t, db, "SKU-HOST-SERIAL-1")
	order := createPluginHostTestOrder(t, db, user.ID)
	serial := createPluginHostTestSerial(t, db, product.ID, order.ID, product.ProductCode)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostSerialRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"serial.view"},
	}, "host.serial.get", map[string]interface{}{
		"serial_number": serial.SerialNumber,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["serial_number"]; got != serial.SerialNumber {
		t.Fatalf("expected serial_number=%q, got %#v", serial.SerialNumber, got)
	}
	if got := result["has_been_viewed"]; got != false {
		t.Fatalf("expected has_been_viewed=false, got %#v", got)
	}
}

func TestExecutePluginHostActionGetsAnnouncementByID(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Announcement{}); err != nil {
		t.Fatalf("auto migrate announcement host api models failed: %v", err)
	}

	announcement := createPluginHostTestAnnouncement(t, db, "Host Announcement")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostAnnouncementRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"announcement.view"},
	}, "host.announcement.get", map[string]interface{}{
		"id": announcement.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["title"]; got != announcement.Title {
		t.Fatalf("expected title=%q, got %#v", announcement.Title, got)
	}
	if _, exists := result["is_marketing"]; exists {
		t.Fatalf("expected is_marketing to be removed, got %#v", result["is_marketing"])
	}
}

func TestExecutePluginHostActionListsAnnouncements(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Announcement{}); err != nil {
		t.Fatalf("auto migrate announcement host api models failed: %v", err)
	}

	announcement := createPluginHostTestAnnouncement(t, db, "Host Announcement List")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostAnnouncementList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"announcement.view"},
	}, "host.announcement.list", map[string]interface{}{
		"search":   announcement.Title,
		"category": "marketing",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one announcement item, got %#v", result["items"])
	}
	if got := items[0]["title"]; got != announcement.Title {
		t.Fatalf("expected title=%q, got %#v", announcement.Title, got)
	}
}

func TestExecutePluginHostActionGetsKnowledgeArticleByID(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.KnowledgeCategory{}, &models.KnowledgeArticle{}); err != nil {
		t.Fatalf("auto migrate knowledge host api models failed: %v", err)
	}

	category, article := createPluginHostTestKnowledgeArticle(t, db, "Host Knowledge Article")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostKnowledgeRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"knowledge.view"},
	}, "host.knowledge.get", map[string]interface{}{
		"id": article.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["title"]; got != article.Title {
		t.Fatalf("expected title=%q, got %#v", article.Title, got)
	}
	if got := interfaceToTestInt64(result["category_id"]); got != int64(category.ID) {
		t.Fatalf("expected category_id=%d, got %#v", category.ID, got)
	}
	if got := result["category_name"]; got != category.Name {
		t.Fatalf("expected category_name=%q, got %#v", category.Name, got)
	}
}

func TestExecutePluginHostActionListsKnowledgeArticles(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.KnowledgeCategory{}, &models.KnowledgeArticle{}); err != nil {
		t.Fatalf("auto migrate knowledge host api models failed: %v", err)
	}

	category, article := createPluginHostTestKnowledgeArticle(t, db, "Host Knowledge List")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostKnowledgeList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"knowledge.view"},
	}, "host.knowledge.list", map[string]interface{}{
		"category_id": category.ID,
		"search":      article.Title,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one knowledge item, got %#v", result["items"])
	}
	if got := items[0]["title"]; got != article.Title {
		t.Fatalf("expected title=%q, got %#v", article.Title, got)
	}
}

func TestExecutePluginHostActionGetsKnowledgeCategories(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.KnowledgeCategory{}, &models.KnowledgeArticle{}); err != nil {
		t.Fatalf("auto migrate knowledge category host api models failed: %v", err)
	}

	category, _ := createPluginHostTestKnowledgeArticle(t, db, "Host Knowledge Tree")
	child := models.KnowledgeCategory{
		ParentID:  &category.ID,
		Name:      "Host Knowledge Child",
		SortOrder: 20,
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create test knowledge child category failed: %v", err)
	}
	grandchild := models.KnowledgeCategory{
		ParentID:  &child.ID,
		Name:      "Host Knowledge Grandchild",
		SortOrder: 30,
	}
	if err := db.Create(&grandchild).Error; err != nil {
		t.Fatalf("create test knowledge grandchild category failed: %v", err)
	}
	grandchildArticle := models.KnowledgeArticle{
		CategoryID: &grandchild.ID,
		Title:      "Host Knowledge Tree Leaf",
		Content:    "Host knowledge tree leaf content",
		SortOrder:  10,
	}
	if err := db.Create(&grandchildArticle).Error; err != nil {
		t.Fatalf("create test knowledge grandchild article failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostKnowledgeCategories},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"knowledge.view"},
	}, "host.knowledge.categories", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["total_roots"]); got != 1 {
		t.Fatalf("expected total_roots=1, got %#v", got)
	}
	if got := interfaceToTestInt64(result["total_entries"]); got != 3 {
		t.Fatalf("expected total_entries=3, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one root knowledge category item, got %#v", result["items"])
	}
	if got := items[0]["name"]; got != category.Name {
		t.Fatalf("expected root category name=%q, got %#v", category.Name, got)
	}
	if got := interfaceToTestInt64(items[0]["total_article_count"]); got != 2 {
		t.Fatalf("expected root total_article_count=2, got %#v", got)
	}
	children, ok := items[0]["children"].([]interface{})
	if !ok || len(children) != 1 {
		t.Fatalf("expected one child knowledge category item, got %#v", items[0]["children"])
	}
	childItem, ok := children[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected child item to be object, got %#v", children[0])
	}
	if got := childItem["name"]; got != child.Name {
		t.Fatalf("expected child category name=%q, got %#v", child.Name, got)
	}
	grandchildren, ok := childItem["children"].([]interface{})
	if !ok || len(grandchildren) != 1 {
		t.Fatalf("expected one grandchild knowledge category item, got %#v", childItem["children"])
	}
	grandchildItem, ok := grandchildren[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected grandchild item to be object, got %#v", grandchildren[0])
	}
	if got := grandchildItem["name"]; got != grandchild.Name {
		t.Fatalf("expected grandchild category name=%q, got %#v", grandchild.Name, got)
	}
	if got := interfaceToTestInt64(grandchildItem["article_count"]); got != 1 {
		t.Fatalf("expected grandchild article_count=1, got %#v", got)
	}
}

func TestExecutePluginHostActionListsKnowledgeArticlesAcrossDescendantCategories(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.KnowledgeCategory{}, &models.KnowledgeArticle{}); err != nil {
		t.Fatalf("auto migrate knowledge host api models failed: %v", err)
	}

	category, article := createPluginHostTestKnowledgeArticle(t, db, "Host Knowledge Root")
	child := models.KnowledgeCategory{
		ParentID:  &category.ID,
		Name:      "Host Knowledge Nested Child",
		SortOrder: 20,
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create test knowledge child category failed: %v", err)
	}
	grandchild := models.KnowledgeCategory{
		ParentID:  &child.ID,
		Name:      "Host Knowledge Nested Grandchild",
		SortOrder: 30,
	}
	if err := db.Create(&grandchild).Error; err != nil {
		t.Fatalf("create test knowledge grandchild category failed: %v", err)
	}
	grandchildArticle := models.KnowledgeArticle{
		CategoryID: &grandchild.ID,
		Title:      "Host Knowledge Nested Leaf",
		Content:    "Host nested knowledge article content",
		SortOrder:  10,
	}
	if err := db.Create(&grandchildArticle).Error; err != nil {
		t.Fatalf("create test knowledge grandchild article failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostKnowledgeList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"knowledge.view"},
	}, "host.knowledge.list", map[string]interface{}{
		"category_id": category.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(2) {
		t.Fatalf("expected total=2, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected two knowledge items, got %#v", result["items"])
	}
	titles := map[string]bool{}
	for _, item := range items {
		titles[item["title"].(string)] = true
	}
	if !titles[article.Title] {
		t.Fatalf("expected root article title=%q to be listed", article.Title)
	}
	if !titles[grandchildArticle.Title] {
		t.Fatalf("expected grandchild article title=%q to be listed", grandchildArticle.Title)
	}
}

func TestExecutePluginHostActionGetsPaymentMethodByIDWithoutSensitiveFields(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}); err != nil {
		t.Fatalf("auto migrate payment method host api models failed: %v", err)
	}

	method := createPluginHostTestPaymentMethod(t, db, "Host Payment Method")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPaymentMethodRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"payment_method.view"},
	}, "host.payment_method.get", map[string]interface{}{
		"id": method.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["name"]; got != method.Name {
		t.Fatalf("expected name=%q, got %#v", method.Name, got)
	}
	if got := result["has_script"]; got != true {
		t.Fatalf("expected has_script=true, got %#v", got)
	}
	if got := result["has_config"]; got != true {
		t.Fatalf("expected has_config=true, got %#v", got)
	}
	if _, exists := result["script"]; exists {
		t.Fatalf("script should not be exposed to plugin host consumers")
	}
	if _, exists := result["config"]; exists {
		t.Fatalf("config should not be exposed to plugin host consumers")
	}
}

func TestExecutePluginHostActionGetsPaymentMethodByCamelCaseID(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}); err != nil {
		t.Fatalf("auto migrate payment method host api models failed: %v", err)
	}

	method := createPluginHostTestPaymentMethod(t, db, "Host Payment Method Camel")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPaymentMethodRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"payment_method.view"},
	}, "host.payment_method.get", map[string]interface{}{
		"paymentMethodId": method.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["name"]; got != method.Name {
		t.Fatalf("expected name=%q, got %#v", method.Name, got)
	}
}

func TestExecutePluginHostActionListsPaymentMethods(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}); err != nil {
		t.Fatalf("auto migrate payment method host api models failed: %v", err)
	}

	method := createPluginHostTestPaymentMethod(t, db, "Host Payment Method List")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPaymentMethodList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"payment_method.view"},
	}, "host.payment_method.list", map[string]interface{}{
		"search":       method.Name,
		"enabled_only": true,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one payment method item, got %#v", result["items"])
	}
	if got := items[0]["name"]; got != method.Name {
		t.Fatalf("expected name=%q, got %#v", method.Name, got)
	}
}

func TestExecutePluginHostActionGetsInventoryBindingByID(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.Inventory{}, &models.ProductInventoryBinding{}); err != nil {
		t.Fatalf("auto migrate inventory binding host api models failed: %v", err)
	}

	product, inventory, binding := createPluginHostTestInventoryBinding(t, db)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostInventoryBindingRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.inventory_binding.get", map[string]interface{}{
		"id": binding.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["product_id"]); got != int64(product.ID) {
		t.Fatalf("expected product_id=%d, got %#v", product.ID, got)
	}
	if got := interfaceToTestInt64(result["inventory_id"]); got != int64(inventory.ID) {
		t.Fatalf("expected inventory_id=%d, got %#v", inventory.ID, got)
	}
	productData, ok := result["product"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested product summary, got %#v", result["product"])
	}
	if got := productData["sku"]; got != product.SKU {
		t.Fatalf("expected nested product sku=%q, got %#v", product.SKU, got)
	}
	inventoryData, ok := result["inventory"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested inventory summary, got %#v", result["inventory"])
	}
	if got := inventoryData["sku"]; got != inventory.SKU {
		t.Fatalf("expected nested inventory sku=%q, got %#v", inventory.SKU, got)
	}
}

func TestExecutePluginHostActionListsInventoryBindings(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.Inventory{}, &models.ProductInventoryBinding{}); err != nil {
		t.Fatalf("auto migrate inventory binding host api models failed: %v", err)
	}

	product, _, binding := createPluginHostTestInventoryBinding(t, db)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostInventoryBindingList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.inventory_binding.list", map[string]interface{}{
		"product_id": product.ID,
		"is_random":  true,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one inventory binding item, got %#v", result["items"])
	}
	if got := interfaceToTestInt64(items[0]["id"]); got != int64(binding.ID) {
		t.Fatalf("expected binding id=%d, got %#v", binding.ID, got)
	}
}

func TestExecutePluginHostActionGetsVirtualInventoryByIDWithoutSensitiveFields(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.VirtualInventory{}, &models.VirtualProductStock{}); err != nil {
		t.Fatalf("auto migrate virtual inventory host api models failed: %v", err)
	}

	inventory := createPluginHostTestVirtualInventory(t, db, "Host Virtual Inventory")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostVirtualInventoryRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.virtual_inventory.get", map[string]interface{}{
		"id": inventory.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["name"]; got != inventory.Name {
		t.Fatalf("expected name=%q, got %#v", inventory.Name, got)
	}
	if got := result["has_script"]; got != true {
		t.Fatalf("expected has_script=true, got %#v", got)
	}
	if got := result["has_script_config"]; got != true {
		t.Fatalf("expected has_script_config=true, got %#v", got)
	}
	if got := interfaceToTestInt64(result["available"]); got != 4 {
		t.Fatalf("expected available=4, got %#v", got)
	}
	if _, exists := result["script"]; exists {
		t.Fatalf("script should not be exposed to plugin host consumers")
	}
	if _, exists := result["script_config"]; exists {
		t.Fatalf("script_config should not be exposed to plugin host consumers")
	}
}

func TestExecutePluginHostActionListsVirtualInventories(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.VirtualInventory{}, &models.VirtualProductStock{}); err != nil {
		t.Fatalf("auto migrate virtual inventory host api models failed: %v", err)
	}

	inventory := createPluginHostTestVirtualInventory(t, db, "Host Virtual Inventory List")

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostVirtualInventoryList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.virtual_inventory.list", map[string]interface{}{
		"search": inventory.Name,
		"type":   string(models.VirtualInventoryTypeScript),
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one virtual inventory item, got %#v", result["items"])
	}
	if got := items[0]["name"]; got != inventory.Name {
		t.Fatalf("expected name=%q, got %#v", inventory.Name, got)
	}
}

func TestExecutePluginHostActionGetsVirtualInventoryBindingByIDWithoutSensitiveFields(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.VirtualInventory{}, &models.VirtualProductStock{}, &models.ProductVirtualInventoryBinding{}); err != nil {
		t.Fatalf("auto migrate virtual inventory binding host api models failed: %v", err)
	}

	product, inventory, binding := createPluginHostTestVirtualInventoryBinding(t, db)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostVirtualInventoryBindingRead},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.virtual_inventory_binding.get", map[string]interface{}{
		"id": binding.ID,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := interfaceToTestInt64(result["product_id"]); got != int64(product.ID) {
		t.Fatalf("expected product_id=%d, got %#v", product.ID, got)
	}
	virtualInventoryData, ok := result["virtual_inventory"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested virtual inventory summary, got %#v", result["virtual_inventory"])
	}
	if got := virtualInventoryData["name"]; got != inventory.Name {
		t.Fatalf("expected virtual inventory name=%q, got %#v", inventory.Name, got)
	}
	if _, exists := virtualInventoryData["script"]; exists {
		t.Fatalf("nested virtual inventory script should not be exposed")
	}
	if _, exists := virtualInventoryData["script_config"]; exists {
		t.Fatalf("nested virtual inventory script_config should not be exposed")
	}
}

func TestExecutePluginHostActionListsVirtualInventoryBindings(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.VirtualInventory{}, &models.VirtualProductStock{}, &models.ProductVirtualInventoryBinding{}); err != nil {
		t.Fatalf("auto migrate virtual inventory binding host api models failed: %v", err)
	}

	product, _, binding := createPluginHostTestVirtualInventoryBinding(t, db)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostVirtualInventoryBindingList},
		ScopeAuthenticated: true,
		ScopePermissions:   []string{"product.view"},
	}, "host.virtual_inventory_binding.list", map[string]interface{}{
		"product_id": product.ID,
		"is_random":  true,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	if got := result["total"]; got != int64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	items, ok := result["items"].([]map[string]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected one virtual inventory binding item, got %#v", result["items"])
	}
	if got := interfaceToTestInt64(items[0]["id"]); got != int64(binding.ID) {
		t.Fatalf("expected binding id=%d, got %#v", binding.ID, got)
	}
}

func createPluginHostTestUser(t *testing.T, db *gorm.DB, email string, role string) models.User {
	t.Helper()
	user := models.User{
		UUID:          "uuid-" + email,
		Email:         email,
		Name:          "Host Test User",
		Role:          role,
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create test user failed: %v", err)
	}
	return user
}

func createPluginHostTestOrder(t *testing.T, db *gorm.DB, userID uint) models.Order {
	t.Helper()
	order := models.Order{
		OrderNo: "ORD-HOST-" + t.Name(),
		UserID:  &userID,
		Items: []models.OrderItem{
			{
				SKU:      "SKU-HOST-1",
				Name:     "Host Test SKU",
				Quantity: 1,
			},
		},
		Status: models.OrderStatusPending,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create test order failed: %v", err)
	}
	return order
}

func createPluginHostTestProduct(t *testing.T, db *gorm.DB, sku string) models.Product {
	t.Helper()
	product := models.Product{
		SKU:         sku,
		Name:        "Host Test Product",
		ProductCode: "PHT",
		ProductType: models.ProductTypePhysical,
		Price:       1999,
		Status:      models.ProductStatusActive,
		Stock:       12,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create test product failed: %v", err)
	}
	return product
}

func createPluginHostTestInventory(t *testing.T, db *gorm.DB, sku string) models.Inventory {
	t.Helper()
	inventory := models.Inventory{
		Name:              "Host Test Inventory",
		SKU:               sku,
		Attributes:        models.JSON(`{"region":"us"}`),
		Stock:             10,
		AvailableQuantity: 8,
		SoldQuantity:      1,
		ReservedQuantity:  2,
		SafetyStock:       3,
		IsActive:          true,
	}
	if err := db.Create(&inventory).Error; err != nil {
		t.Fatalf("create test inventory failed: %v", err)
	}
	return inventory
}

func createPluginHostTestPromoCode(t *testing.T, db *gorm.DB, code string) models.PromoCode {
	t.Helper()
	promo := models.PromoCode{
		Code:          code,
		Name:          "Host Promo",
		DiscountType:  models.DiscountTypeFixed,
		DiscountValue: 500,
		TotalQuantity: 100,
		UsedQuantity:  2,
		Status:        models.PromoCodeStatusActive,
		ProductScope:  "all",
	}
	if err := db.Create(&promo).Error; err != nil {
		t.Fatalf("create test promo code failed: %v", err)
	}
	return promo
}

func createPluginHostTestTicket(t *testing.T, db *gorm.DB, userID uint) models.Ticket {
	t.Helper()
	ticket := models.Ticket{
		TicketNo: "TKT-HOST-" + t.Name(),
		UserID:   userID,
		Subject:  "Host Test Ticket",
		Content:  "Need help with host bridge",
		Priority: models.TicketPriorityNormal,
		Status:   models.TicketStatusOpen,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create test ticket failed: %v", err)
	}
	return ticket
}

func createPluginHostTestSerial(t *testing.T, db *gorm.DB, productID uint, orderID uint, productCode string) models.ProductSerial {
	t.Helper()
	serial := models.ProductSerial{
		SerialNumber:        productCode + "-0001-ABCD",
		ProductID:           productID,
		OrderID:             orderID,
		ProductCode:         productCode,
		SequenceNumber:      1,
		AntiCounterfeitCode: "ABCD",
	}
	if err := db.Create(&serial).Error; err != nil {
		t.Fatalf("create test serial failed: %v", err)
	}
	return serial
}

func createPluginHostTestAnnouncement(t *testing.T, db *gorm.DB, title string) models.Announcement {
	t.Helper()
	announcement := models.Announcement{
		Title:           title,
		Content:         "Host announcement content",
		Category:        "marketing",
		IsMandatory:     true,
		RequireFullRead: true,
	}
	if err := db.Create(&announcement).Error; err != nil {
		t.Fatalf("create test announcement failed: %v", err)
	}
	return announcement
}

func createPluginHostTestKnowledgeArticle(t *testing.T, db *gorm.DB, title string) (models.KnowledgeCategory, models.KnowledgeArticle) {
	t.Helper()
	category := models.KnowledgeCategory{
		Name:      "Host Knowledge",
		SortOrder: 10,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create test knowledge category failed: %v", err)
	}

	article := models.KnowledgeArticle{
		CategoryID: &category.ID,
		Title:      title,
		Content:    "Host knowledge article content",
		SortOrder:  5,
	}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("create test knowledge article failed: %v", err)
	}
	return category, article
}

func createPluginHostTestPaymentMethod(t *testing.T, db *gorm.DB, name string) models.PaymentMethod {
	t.Helper()
	method := models.PaymentMethod{
		Name:         name,
		Description:  "Host payment method description",
		Type:         models.PaymentMethodTypeCustom,
		Enabled:      true,
		Script:       "function onGeneratePaymentCard() { return {}; }",
		Config:       `{"api_key":"demo-secret"}`,
		Icon:         "CreditCard",
		SortOrder:    1,
		PollInterval: 30,
	}
	if err := db.Create(&method).Error; err != nil {
		t.Fatalf("create test payment method failed: %v", err)
	}
	return method
}

func createPluginHostTestInventoryBinding(t *testing.T, db *gorm.DB) (models.Product, models.Inventory, models.ProductInventoryBinding) {
	t.Helper()

	product := createPluginHostTestProduct(t, db, "SKU-HOST-BINDING")
	inventory := createPluginHostTestInventory(t, db, "INV-HOST-BINDING")
	binding := models.ProductInventoryBinding{
		ProductID:      product.ID,
		InventoryID:    inventory.ID,
		Attributes:     models.JSON(`{"region":"north"}`),
		AttributesHash: "hash-host-binding",
		IsRandom:       true,
		Priority:       3,
		Notes:          "Host binding notes",
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create test inventory binding failed: %v", err)
	}
	return product, inventory, binding
}

func createPluginHostTestVirtualInventory(t *testing.T, db *gorm.DB, name string) models.VirtualInventory {
	t.Helper()

	inventory := models.VirtualInventory{
		Name:         name,
		SKU:          "VINV-" + t.Name(),
		Type:         models.VirtualInventoryTypeScript,
		Script:       "function deliver() { return []; }",
		ScriptConfig: `{"token":"secret-demo"}`,
		Description:  "Host virtual inventory description",
		TotalLimit:   5,
		IsActive:     true,
		Notes:        "Host virtual inventory notes",
	}
	if err := db.Create(&inventory).Error; err != nil {
		t.Fatalf("create test virtual inventory failed: %v", err)
	}

	soldStock := models.VirtualProductStock{
		VirtualInventoryID: inventory.ID,
		Content:            "HOST-DELIVERED-CODE",
		Status:             models.VirtualStockStatusSold,
		OrderNo:            "ORD-HOST-VIRTUAL",
	}
	if err := db.Create(&soldStock).Error; err != nil {
		t.Fatalf("create test virtual inventory sold stock failed: %v", err)
	}

	return inventory
}

func createPluginHostTestVirtualInventoryBinding(t *testing.T, db *gorm.DB) (models.Product, models.VirtualInventory, models.ProductVirtualInventoryBinding) {
	t.Helper()

	product := createPluginHostTestProduct(t, db, "SKU-HOST-VIRTUAL-BINDING")
	virtualInventory := createPluginHostTestVirtualInventory(t, db, "Host Virtual Inventory Binding")
	binding := models.ProductVirtualInventoryBinding{
		ProductID:          product.ID,
		VirtualInventoryID: virtualInventory.ID,
		Attributes:         models.JSONMap{"region": "global"},
		AttributesHash:     "hash-host-virtual-binding",
		IsRandom:           true,
		Priority:           2,
		Notes:              "Host virtual binding notes",
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create test virtual inventory binding failed: %v", err)
	}
	return product, virtualInventory, binding
}

func interfaceToTestInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func interfaceIsNil(value interface{}) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
