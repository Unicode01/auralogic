package service

import (
	"sort"
	"strings"
)

type hookPhase string

const (
	hookPhaseBefore   hookPhase = "before"
	hookPhaseAfter    hookPhase = "after"
	hookPhaseEvent    hookPhase = "event"
	hookPhaseFrontend hookPhase = "frontend"
)

type hookDefinition struct {
	Name                  string
	Phase                 hookPhase
	RestrictPayloadPatch  bool
	WritablePayloadFields map[string]struct{}
}

var hookDefinitions = map[string]hookDefinition{
	"frontend.bootstrap":   newReadOnlyHookDefinition("frontend.bootstrap", hookPhaseFrontend),
	"frontend.slot.render": newReadOnlyHookDefinition("frontend.slot.render", hookPhaseFrontend),

	"auth.login.after":               newReadOnlyHookDefinition("auth.login.after", hookPhaseAfter),
	"auth.login.before":              newRestrictedHookDefinition("auth.login.before", hookPhaseBefore, "email", "phone", "password", "code"),
	"auth.bind_email.confirm.after":  newReadOnlyHookDefinition("auth.bind_email.confirm.after", hookPhaseAfter),
	"auth.bind_email.confirm.before": newRestrictedHookDefinition("auth.bind_email.confirm.before", hookPhaseBefore, "email", "code"),
	"auth.bind_email.request.after":  newReadOnlyHookDefinition("auth.bind_email.request.after", hookPhaseAfter),
	"auth.bind_email.request.before": newRestrictedHookDefinition("auth.bind_email.request.before", hookPhaseBefore, "email"),
	"auth.bind_phone.confirm.after":  newReadOnlyHookDefinition("auth.bind_phone.confirm.after", hookPhaseAfter),
	"auth.bind_phone.confirm.before": newRestrictedHookDefinition("auth.bind_phone.confirm.before", hookPhaseBefore, "phone", "code"),
	"auth.bind_phone.request.after":  newReadOnlyHookDefinition("auth.bind_phone.request.after", hookPhaseAfter),
	"auth.bind_phone.request.before": newRestrictedHookDefinition("auth.bind_phone.request.before", hookPhaseBefore, "phone"),
	"auth.password.change.after":     newReadOnlyHookDefinition("auth.password.change.after", hookPhaseAfter),
	"auth.password.change.before":    newRestrictedHookDefinition("auth.password.change.before", hookPhaseBefore, "old_password", "new_password"),
	"auth.password.reset.after":      newReadOnlyHookDefinition("auth.password.reset.after", hookPhaseAfter),
	"auth.password.reset.before":     newRestrictedHookDefinition("auth.password.reset.before", hookPhaseBefore, "email", "phone", "token", "code", "new_password"),
	"auth.preferences.update.after":  newReadOnlyHookDefinition("auth.preferences.update.after", hookPhaseAfter),
	"auth.preferences.update.before": newRestrictedHookDefinition(
		"auth.preferences.update.before",
		hookPhaseBefore,
		"locale",
		"country",
		"email_notify_order",
		"email_notify_ticket",
		"email_notify_marketing",
		"sms_notify_marketing",
	),
	"auth.register.after":  newReadOnlyHookDefinition("auth.register.after", hookPhaseAfter),
	"auth.register.before": newRestrictedHookDefinition("auth.register.before", hookPhaseBefore, "email", "phone", "name", "password"),

	"apikey.create.after":  newReadOnlyHookDefinition("apikey.create.after", hookPhaseAfter),
	"apikey.create.before": newRestrictedHookDefinition("apikey.create.before", hookPhaseBefore, "key_name", "platform", "scopes", "rate_limit", "expires_at"),
	"apikey.delete.after":  newReadOnlyHookDefinition("apikey.delete.after", hookPhaseAfter),
	"apikey.delete.before": newReadOnlyHookDefinition("apikey.delete.before", hookPhaseBefore),
	"apikey.update.after":  newReadOnlyHookDefinition("apikey.update.after", hookPhaseAfter),
	"apikey.update.before": newRestrictedHookDefinition("apikey.update.before", hookPhaseBefore, "is_active", "rate_limit", "key_name"),

	"announcement.create.after":  newReadOnlyHookDefinition("announcement.create.after", hookPhaseAfter),
	"announcement.create.before": newRestrictedHookDefinition("announcement.create.before", hookPhaseBefore, "title", "content", "category", "send_email", "send_sms", "is_mandatory", "require_full_read"),
	"announcement.delete.after":  newReadOnlyHookDefinition("announcement.delete.after", hookPhaseAfter),
	"announcement.delete.before": newReadOnlyHookDefinition("announcement.delete.before", hookPhaseBefore),
	"announcement.read.after":    newReadOnlyHookDefinition("announcement.read.after", hookPhaseAfter),
	"announcement.update.after":  newReadOnlyHookDefinition("announcement.update.after", hookPhaseAfter),
	"announcement.update.before": newRestrictedHookDefinition("announcement.update.before", hookPhaseBefore, "title", "content", "category", "send_email", "send_sms", "is_mandatory", "require_full_read"),
	"announcement.view.after":    newReadOnlyHookDefinition("announcement.view.after", hookPhaseAfter),

	"cart.add.after":     newReadOnlyHookDefinition("cart.add.after", hookPhaseAfter),
	"cart.add.before":    newRestrictedHookDefinition("cart.add.before", hookPhaseBefore, "quantity", "attributes"),
	"cart.clear.after":   newReadOnlyHookDefinition("cart.clear.after", hookPhaseAfter),
	"cart.remove.after":  newReadOnlyHookDefinition("cart.remove.after", hookPhaseAfter),
	"cart.update.after":  newReadOnlyHookDefinition("cart.update.after", hookPhaseAfter),
	"cart.update.before": newRestrictedHookDefinition("cart.update.before", hookPhaseBefore, "quantity"),

	"email_template.update.after":  newReadOnlyHookDefinition("email_template.update.after", hookPhaseAfter),
	"email_template.update.before": newRestrictedHookDefinition("email_template.update.before", hookPhaseBefore, "content"),
	"email.send.after":             newReadOnlyHookDefinition("email.send.after", hookPhaseAfter),
	"email.send.before":            newRestrictedHookDefinition("email.send.before", hookPhaseBefore, "to", "subject", "content"),

	"inventory.adjust.after":                newReadOnlyHookDefinition("inventory.adjust.after", hookPhaseAfter),
	"inventory.adjust.before":               newRestrictedHookDefinition("inventory.adjust.before", hookPhaseBefore, "stock_delta", "available_quantity_delta", "reason", "notes"),
	"inventory.binding.batch_create.after":  newReadOnlyHookDefinition("inventory.binding.batch_create.after", hookPhaseAfter),
	"inventory.binding.batch_create.before": newRestrictedHookDefinition("inventory.binding.batch_create.before", hookPhaseBefore, "bindings"),
	"inventory.binding.create.after":        newReadOnlyHookDefinition("inventory.binding.create.after", hookPhaseAfter),
	"inventory.binding.create.before":       newRestrictedHookDefinition("inventory.binding.create.before", hookPhaseBefore, "inventory_id", "is_random", "priority", "notes"),
	"inventory.binding.delete.after":        newReadOnlyHookDefinition("inventory.binding.delete.after", hookPhaseAfter),
	"inventory.binding.delete.before":       newReadOnlyHookDefinition("inventory.binding.delete.before", hookPhaseBefore),
	"inventory.binding.replace.after":       newReadOnlyHookDefinition("inventory.binding.replace.after", hookPhaseAfter),
	"inventory.binding.replace.before":      newRestrictedHookDefinition("inventory.binding.replace.before", hookPhaseBefore, "bindings"),
	"inventory.binding.update.after":        newReadOnlyHookDefinition("inventory.binding.update.after", hookPhaseAfter),
	"inventory.binding.update.before":       newRestrictedHookDefinition("inventory.binding.update.before", hookPhaseBefore, "is_random", "priority", "notes"),
	"inventory.create.after":                newReadOnlyHookDefinition("inventory.create.after", hookPhaseAfter),
	"inventory.create.before": newRestrictedHookDefinition(
		"inventory.create.before",
		hookPhaseBefore,
		"name",
		"sku",
		"attributes",
		"stock",
		"available_quantity",
		"safety_stock",
		"alert_email",
		"notes",
	),
	"inventory.delete.after":   newReadOnlyHookDefinition("inventory.delete.after", hookPhaseAfter),
	"inventory.delete.before":  newReadOnlyHookDefinition("inventory.delete.before", hookPhaseBefore),
	"inventory.release.after":  newReadOnlyHookDefinition("inventory.release.after", hookPhaseAfter),
	"inventory.reserve.after":  newReadOnlyHookDefinition("inventory.reserve.after", hookPhaseAfter),
	"inventory.reserve.before": newRestrictedHookDefinition("inventory.reserve.before", hookPhaseBefore, "inventory_id"),
	"inventory.update.after":   newReadOnlyHookDefinition("inventory.update.after", hookPhaseAfter),
	"inventory.update.before": newRestrictedHookDefinition(
		"inventory.update.before",
		hookPhaseBefore,
		"stock",
		"available_quantity",
		"safety_stock",
		"is_active",
		"alert_email",
		"notes",
	),

	"knowledge.article.create.after":   newReadOnlyHookDefinition("knowledge.article.create.after", hookPhaseAfter),
	"knowledge.article.create.before":  newRestrictedHookDefinition("knowledge.article.create.before", hookPhaseBefore, "category_id", "title", "content", "sort_order"),
	"knowledge.article.delete.after":   newReadOnlyHookDefinition("knowledge.article.delete.after", hookPhaseAfter),
	"knowledge.article.delete.before":  newReadOnlyHookDefinition("knowledge.article.delete.before", hookPhaseBefore),
	"knowledge.article.update.after":   newReadOnlyHookDefinition("knowledge.article.update.after", hookPhaseAfter),
	"knowledge.article.update.before":  newRestrictedHookDefinition("knowledge.article.update.before", hookPhaseBefore, "category_id", "title", "content", "sort_order"),
	"knowledge.article.view.after":     newReadOnlyHookDefinition("knowledge.article.view.after", hookPhaseAfter),
	"knowledge.category.create.after":  newReadOnlyHookDefinition("knowledge.category.create.after", hookPhaseAfter),
	"knowledge.category.create.before": newRestrictedHookDefinition("knowledge.category.create.before", hookPhaseBefore, "parent_id", "name", "sort_order"),
	"knowledge.category.delete.after":  newReadOnlyHookDefinition("knowledge.category.delete.after", hookPhaseAfter),
	"knowledge.category.delete.before": newReadOnlyHookDefinition("knowledge.category.delete.before", hookPhaseBefore),
	"knowledge.category.update.after":  newReadOnlyHookDefinition("knowledge.category.update.after", hookPhaseAfter),
	"knowledge.category.update.before": newRestrictedHookDefinition("knowledge.category.update.before", hookPhaseBefore, "parent_id", "name", "sort_order"),

	"landing_page.reset.after":   newReadOnlyHookDefinition("landing_page.reset.after", hookPhaseAfter),
	"landing_page.reset.before":  newReadOnlyHookDefinition("landing_page.reset.before", hookPhaseBefore),
	"landing_page.update.after":  newReadOnlyHookDefinition("landing_page.update.after", hookPhaseAfter),
	"landing_page.update.before": newRestrictedHookDefinition("landing_page.update.before", hookPhaseBefore, "html_content"),

	"marketing.batch.enqueue.after": newReadOnlyHookDefinition("marketing.batch.enqueue.after", hookPhaseAfter),
	"marketing.batch.start.after":   newReadOnlyHookDefinition("marketing.batch.start.after", hookPhaseAfter),
	"marketing.preview.after":       newReadOnlyHookDefinition("marketing.preview.after", hookPhaseAfter),
	"marketing.preview.before": newRestrictedHookDefinition(
		"marketing.preview.before",
		hookPhaseBefore,
		"title",
		"content",
		"user_id",
		"user_ids",
		"audience_mode",
		"audience_query",
		"sample_limit",
	),
	"marketing.send.after": newReadOnlyHookDefinition("marketing.send.after", hookPhaseAfter),
	"marketing.send.before": newRestrictedHookDefinition(
		"marketing.send.before",
		hookPhaseBefore,
		"title",
		"content",
		"send_email",
		"send_sms",
		"target_all",
		"user_ids",
		"audience_mode",
		"audience_query",
	),
	"marketing.task.dispatch.after": newReadOnlyHookDefinition("marketing.task.dispatch.after", hookPhaseAfter),
	"marketing.task.dispatch.before": newRestrictedHookDefinition(
		"marketing.task.dispatch.before",
		hookPhaseBefore,
		"title",
		"content",
		"email_subject",
		"email_html",
		"sms_text",
	),

	"log.email.retry.after":  newReadOnlyHookDefinition("log.email.retry.after", hookPhaseAfter),
	"log.email.retry.before": newRestrictedHookDefinition("log.email.retry.before", hookPhaseBefore, "email_ids"),

	"order.admin.cancel.after":           newReadOnlyHookDefinition("order.admin.cancel.after", hookPhaseAfter),
	"order.admin.cancel.before":          newRestrictedHookDefinition("order.admin.cancel.before", hookPhaseBefore, "reason"),
	"order.admin.complete.after":         newReadOnlyHookDefinition("order.admin.complete.after", hookPhaseAfter),
	"order.admin.complete.before":        newRestrictedHookDefinition("order.admin.complete.before", hookPhaseBefore, "remark", "admin_remark"),
	"order.admin.delete.after":           newReadOnlyHookDefinition("order.admin.delete.after", hookPhaseAfter),
	"order.admin.delete.before":          newReadOnlyHookDefinition("order.admin.delete.before", hookPhaseBefore),
	"order.admin.deliver_virtual.after":  newReadOnlyHookDefinition("order.admin.deliver_virtual.after", hookPhaseAfter),
	"order.admin.deliver_virtual.before": newRestrictedHookDefinition("order.admin.deliver_virtual.before", hookPhaseBefore, "mark_only_shipped"),
	"order.admin.mark_paid.after":        newReadOnlyHookDefinition("order.admin.mark_paid.after", hookPhaseAfter),
	"order.admin.mark_paid.before":       newRestrictedHookDefinition("order.admin.mark_paid.before", hookPhaseBefore, "admin_remark", "skip_auto_delivery"),
	"order.admin.refund_finalize.after":  newReadOnlyHookDefinition("order.admin.refund_finalize.after", hookPhaseAfter),
	"order.admin.refund_finalize.before": newRestrictedHookDefinition("order.admin.refund_finalize.before", hookPhaseBefore, "remark", "transaction_id"),
	"order.admin.refund.after":           newReadOnlyHookDefinition("order.admin.refund.after", hookPhaseAfter),
	"order.admin.refund.before":          newRestrictedHookDefinition("order.admin.refund.before", hookPhaseBefore, "reason"),
	"order.admin.update_price.after":     newReadOnlyHookDefinition("order.admin.update_price.after", hookPhaseAfter),
	"order.admin.update_price.before":    newRestrictedHookDefinition("order.admin.update_price.before", hookPhaseBefore, "total_amount_minor"),
	"order.admin.update_shipping.after":  newReadOnlyHookDefinition("order.admin.update_shipping.after", hookPhaseAfter),
	"order.admin.update_shipping.before": newRestrictedHookDefinition(
		"order.admin.update_shipping.before",
		hookPhaseBefore,
		"receiver_name",
		"phone_code",
		"receiver_phone",
		"receiver_email",
		"receiver_country",
		"receiver_province",
		"receiver_city",
		"receiver_district",
		"receiver_address",
		"receiver_postcode",
	),
	"order.auto_cancel.after":      newReadOnlyHookDefinition("order.auto_cancel.after", hookPhaseAfter),
	"order.auto_cancel.before":     newRestrictedHookDefinition("order.auto_cancel.before", hookPhaseBefore, "admin_remark", "reason"),
	"order.complete.after":         newReadOnlyHookDefinition("order.complete.after", hookPhaseAfter),
	"order.complete.before":        newRestrictedHookDefinition("order.complete.before", hookPhaseBefore, "feedback"),
	"order.create.after":           newReadOnlyHookDefinition("order.create.after", hookPhaseAfter),
	"order.create.before":          newRestrictedHookDefinition("order.create.before", hookPhaseBefore, "items", "remark", "promo_code"),
	"order.status.changed.after":   newReadOnlyHookDefinition("order.status.changed.after", hookPhaseAfter),
	"payment.confirm.after":        newReadOnlyHookDefinition("payment.confirm.after", hookPhaseAfter),
	"payment.confirm.before":       newRestrictedHookDefinition("payment.confirm.before", hookPhaseBefore, "transaction_id", "payment_result"),
	"payment.market.install.after": newReadOnlyHookDefinition("payment.market.install.after", hookPhaseAfter),
	"payment.market.install.before": newRestrictedHookDefinition(
		"payment.market.install.before",
		hookPhaseBefore,
		"payment_method_id",
		"name",
		"description",
		"icon",
		"entry",
		"config",
		"poll_interval",
	),
	"payment.method.select.after":  newReadOnlyHookDefinition("payment.method.select.after", hookPhaseAfter),
	"payment.method.select.before": newRestrictedHookDefinition("payment.method.select.before", hookPhaseBefore, "payment_method_id"),
	"payment.method.create.after":  newReadOnlyHookDefinition("payment.method.create.after", hookPhaseAfter),
	"payment.method.create.before": newRestrictedHookDefinition(
		"payment.method.create.before",
		hookPhaseBefore,
		"name",
		"description",
		"icon",
		"script",
		"config",
		"poll_interval",
		"enabled",
	),
	"payment.method.delete.after":  newReadOnlyHookDefinition("payment.method.delete.after", hookPhaseAfter),
	"payment.method.delete.before": newReadOnlyHookDefinition("payment.method.delete.before", hookPhaseBefore),
	"payment.method.enable.after":  newReadOnlyHookDefinition("payment.method.enable.after", hookPhaseAfter),
	"payment.method.enable.before": newRestrictedHookDefinition("payment.method.enable.before", hookPhaseBefore, "enabled"),
	"payment.method.reorder.after": newReadOnlyHookDefinition("payment.method.reorder.after", hookPhaseAfter),
	"payment.method.test.after":    newReadOnlyHookDefinition("payment.method.test.after", hookPhaseAfter),
	"payment.method.test.before":   newRestrictedHookDefinition("payment.method.test.before", hookPhaseBefore, "script", "config"),
	"payment.method.update.after":  newReadOnlyHookDefinition("payment.method.update.after", hookPhaseAfter),
	"payment.method.update.before": newRestrictedHookDefinition(
		"payment.method.update.before",
		hookPhaseBefore,
		"name",
		"description",
		"icon",
		"script",
		"config",
		"poll_interval",
		"enabled",
	),
	"payment.method.webhook.after":  newReadOnlyHookDefinition("payment.method.webhook.after", hookPhaseAfter),
	"payment.method.webhook.before": newRestrictedHookDefinition("payment.method.webhook.before", hookPhaseBefore, "payment_result", "transaction_id", "message", "data"),
	"payment.package.import.after":  newReadOnlyHookDefinition("payment.package.import.after", hookPhaseAfter),
	"payment.package.import.before": newRestrictedHookDefinition(
		"payment.package.import.before",
		hookPhaseBefore,
		"payment_method_id",
		"name",
		"description",
		"icon",
		"entry",
		"config",
		"package_version",
		"poll_interval",
	),

	"payment.polling.failed":    newReadOnlyHookDefinition("payment.polling.failed", hookPhaseEvent),
	"payment.polling.succeeded": newReadOnlyHookDefinition("payment.polling.succeeded", hookPhaseEvent),

	"plugin.create.after": newReadOnlyHookDefinition("plugin.create.after", hookPhaseAfter),
	"plugin.create.before": newRestrictedHookDefinition(
		"plugin.create.before",
		hookPhaseBefore,
		"display_name",
		"description",
		"type",
		"runtime",
		"address",
		"package_path",
		"config",
		"runtime_params",
		"capabilities",
		"version",
		"enabled",
	),
	"plugin.delete.after":                newReadOnlyHookDefinition("plugin.delete.after", hookPhaseAfter),
	"plugin.delete.before":               newReadOnlyHookDefinition("plugin.delete.before", hookPhaseBefore),
	"plugin.lifecycle.hot_reload.after":  newReadOnlyHookDefinition("plugin.lifecycle.hot_reload.after", hookPhaseAfter),
	"plugin.lifecycle.hot_reload.before": newRestrictedHookDefinition("plugin.lifecycle.hot_reload.before", hookPhaseBefore, "version_id", "auto_start"),
	"plugin.lifecycle.install.after":     newReadOnlyHookDefinition("plugin.lifecycle.install.after", hookPhaseAfter),
	"plugin.lifecycle.install.before":    newRestrictedHookDefinition("plugin.lifecycle.install.before", hookPhaseBefore, "version_id", "auto_start"),
	"plugin.lifecycle.pause.after":       newReadOnlyHookDefinition("plugin.lifecycle.pause.after", hookPhaseAfter),
	"plugin.lifecycle.pause.before":      newReadOnlyHookDefinition("plugin.lifecycle.pause.before", hookPhaseBefore),
	"plugin.lifecycle.restart.after":     newReadOnlyHookDefinition("plugin.lifecycle.restart.after", hookPhaseAfter),
	"plugin.lifecycle.restart.before":    newReadOnlyHookDefinition("plugin.lifecycle.restart.before", hookPhaseBefore),
	"plugin.lifecycle.resume.after":      newReadOnlyHookDefinition("plugin.lifecycle.resume.after", hookPhaseAfter),
	"plugin.lifecycle.resume.before":     newRestrictedHookDefinition("plugin.lifecycle.resume.before", hookPhaseBefore, "version_id", "auto_start"),
	"plugin.lifecycle.retire.after":      newReadOnlyHookDefinition("plugin.lifecycle.retire.after", hookPhaseAfter),
	"plugin.lifecycle.retire.before":     newReadOnlyHookDefinition("plugin.lifecycle.retire.before", hookPhaseBefore),
	"plugin.lifecycle.start.after":       newReadOnlyHookDefinition("plugin.lifecycle.start.after", hookPhaseAfter),
	"plugin.lifecycle.start.before":      newRestrictedHookDefinition("plugin.lifecycle.start.before", hookPhaseBefore, "version_id", "auto_start"),
	"plugin.market.install.after":        newReadOnlyHookDefinition("plugin.market.install.after", hookPhaseAfter),
	"plugin.market.install.before": newRestrictedHookDefinition(
		"plugin.market.install.before",
		hookPhaseBefore,
		"activate",
		"auto_start",
		"granted_permissions",
		"note",
	),
	"plugin.package.upload.after": newReadOnlyHookDefinition("plugin.package.upload.after", hookPhaseAfter),
	"plugin.package.upload.before": newRestrictedHookDefinition(
		"plugin.package.upload.before",
		hookPhaseBefore,
		"name",
		"display_name",
		"description",
		"type",
		"runtime",
		"address",
		"entry",
		"version",
		"config",
		"runtime_params",
		"capabilities",
		"granted_permissions",
		"changelog",
		"activate",
		"auto_start",
	),
	"plugin.secret.update.after":  newReadOnlyHookDefinition("plugin.secret.update.after", hookPhaseAfter),
	"plugin.secret.update.before": newReadOnlyHookDefinition("plugin.secret.update.before", hookPhaseBefore),
	"plugin.update.after":         newReadOnlyHookDefinition("plugin.update.after", hookPhaseAfter),
	"plugin.update.before": newRestrictedHookDefinition(
		"plugin.update.before",
		hookPhaseBefore,
		"display_name",
		"description",
		"type",
		"runtime",
		"address",
		"package_path",
		"config",
		"runtime_params",
		"capabilities",
		"version",
		"enabled",
	),
	"plugin.version.activate.after":  newReadOnlyHookDefinition("plugin.version.activate.after", hookPhaseAfter),
	"plugin.version.activate.before": newRestrictedHookDefinition("plugin.version.activate.before", hookPhaseBefore, "auto_start"),
	"plugin.version.delete.after":    newReadOnlyHookDefinition("plugin.version.delete.after", hookPhaseAfter),
	"plugin.version.delete.before":   newReadOnlyHookDefinition("plugin.version.delete.before", hookPhaseBefore),

	"product.create.after": newReadOnlyHookDefinition("product.create.after", hookPhaseAfter),
	"product.create.before": newRestrictedHookDefinition(
		"product.create.before",
		hookPhaseBefore,
		"sku",
		"name",
		"product_code",
		"product_type",
		"description",
		"short_description",
		"category",
		"tags",
		"price_minor",
		"original_price_minor",
		"stock",
		"max_purchase_limit",
		"images",
		"attributes",
		"status",
		"sort_order",
		"is_featured",
		"is_recommended",
		"remark",
		"auto_delivery",
	),
	"product.delete.after":                 newReadOnlyHookDefinition("product.delete.after", hookPhaseAfter),
	"product.delete.before":                newRestrictedHookDefinition("product.delete.before", hookPhaseBefore, "delete_images"),
	"product.inventory_mode.update.after":  newReadOnlyHookDefinition("product.inventory_mode.update.after", hookPhaseAfter),
	"product.inventory_mode.update.before": newRestrictedHookDefinition("product.inventory_mode.update.before", hookPhaseBefore, "inventory_mode"),
	"product.list.query.after":             newReadOnlyHookDefinition("product.list.query.after", hookPhaseAfter),
	"product.list.query.before":            newReadOnlyHookDefinition("product.list.query.before", hookPhaseBefore),
	"product.status.update.after":          newReadOnlyHookDefinition("product.status.update.after", hookPhaseAfter),
	"product.status.update.before":         newRestrictedHookDefinition("product.status.update.before", hookPhaseBefore, "status"),
	"product.update.after":                 newReadOnlyHookDefinition("product.update.after", hookPhaseAfter),
	"product.update.before": newRestrictedHookDefinition(
		"product.update.before",
		hookPhaseBefore,
		"sku",
		"name",
		"product_code",
		"product_type",
		"description",
		"short_description",
		"category",
		"tags",
		"price_minor",
		"original_price_minor",
		"stock",
		"max_purchase_limit",
		"images",
		"attributes",
		"status",
		"sort_order",
		"is_featured",
		"is_recommended",
		"remark",
		"auto_delivery",
	),
	"product.view.after": newReadOnlyHookDefinition("product.view.after", hookPhaseAfter),

	"promo.admin.create.after": newReadOnlyHookDefinition("promo.admin.create.after", hookPhaseAfter),
	"promo.admin.create.before": newRestrictedHookDefinition(
		"promo.admin.create.before",
		hookPhaseBefore,
		"code",
		"name",
		"description",
		"discount_type",
		"discount_value_minor",
		"max_discount_minor",
		"min_order_amount_minor",
		"total_quantity",
		"product_ids",
		"product_scope",
		"status",
		"expires_at",
	),
	"promo.admin.delete.after":  newReadOnlyHookDefinition("promo.admin.delete.after", hookPhaseAfter),
	"promo.admin.delete.before": newReadOnlyHookDefinition("promo.admin.delete.before", hookPhaseBefore),
	"promo.admin.update.after":  newReadOnlyHookDefinition("promo.admin.update.after", hookPhaseAfter),
	"promo.admin.update.before": newRestrictedHookDefinition(
		"promo.admin.update.before",
		hookPhaseBefore,
		"name",
		"description",
		"discount_type",
		"discount_value_minor",
		"max_discount_minor",
		"min_order_amount_minor",
		"total_quantity",
		"product_ids",
		"product_scope",
		"status",
		"expires_at",
	),
	"promo.validate.after":  newReadOnlyHookDefinition("promo.validate.after", hookPhaseAfter),
	"promo.validate.before": newRestrictedHookDefinition("promo.validate.before", hookPhaseBefore, "code", "product_ids", "amount_minor"),

	"serial.batch_delete.after":  newReadOnlyHookDefinition("serial.batch_delete.after", hookPhaseAfter),
	"serial.batch_delete.before": newRestrictedHookDefinition("serial.batch_delete.before", hookPhaseBefore, "ids"),
	"serial.create.after":        newReadOnlyHookDefinition("serial.create.after", hookPhaseAfter),
	"serial.delete.after":        newReadOnlyHookDefinition("serial.delete.after", hookPhaseAfter),
	"serial.delete.before":       newReadOnlyHookDefinition("serial.delete.before", hookPhaseBefore),
	"serial.verify.after":        newReadOnlyHookDefinition("serial.verify.after", hookPhaseAfter),
	"serial.verify.before":       newRestrictedHookDefinition("serial.verify.before", hookPhaseBefore, "serial_number"),

	"settings.update.after":  newReadOnlyHookDefinition("settings.update.after", hookPhaseAfter),
	"settings.update.before": newRestrictedHookDefinition("settings.update.before", hookPhaseBefore, "app", "smtp", "sms", "security", "plugin", "maintenance"),

	"order.export.after":  newReadOnlyHookDefinition("order.export.after", hookPhaseAfter),
	"order.export.before": newRestrictedHookDefinition("order.export.before", hookPhaseBefore, "status", "search", "country", "product_search", "promo_code", "promo_code_id"),
	"order.import.after":  newReadOnlyHookDefinition("order.import.after", hookPhaseAfter),
	"order.import.before": newRestrictedHookDefinition("order.import.before", hookPhaseBefore, "entries"),

	"template.package.import.after":  newReadOnlyHookDefinition("template.package.import.after", hookPhaseAfter),
	"template.package.import.before": newRestrictedHookDefinition("template.package.import.before", hookPhaseBefore, "expected_kind", "target_key"),

	"ticket.assign.after":             newReadOnlyHookDefinition("ticket.assign.after", hookPhaseAfter),
	"ticket.assign.before":            newRestrictedHookDefinition("ticket.assign.before", hookPhaseBefore, "assigned_to"),
	"ticket.attachment.upload.after":  newReadOnlyHookDefinition("ticket.attachment.upload.after", hookPhaseAfter),
	"ticket.attachment.upload.before": newRestrictedHookDefinition("ticket.attachment.upload.before", hookPhaseBefore, "filename", "content_type"),
	"ticket.auto_close.after":         newReadOnlyHookDefinition("ticket.auto_close.after", hookPhaseAfter),
	"ticket.auto_close.before":        newRestrictedHookDefinition("ticket.auto_close.before", hookPhaseBefore, "content", "message"),
	"ticket.create.after":             newRestrictedHookDefinition("ticket.create.after", hookPhaseAfter, "auto_reply", "ticket_auto_reply"),
	"ticket.create.before":            newRestrictedHookDefinition("ticket.create.before", hookPhaseBefore, "subject", "content", "category", "priority", "order_id"),
	"ticket.message.admin.after":      newReadOnlyHookDefinition("ticket.message.admin.after", hookPhaseAfter),
	"ticket.message.admin.before":     newRestrictedHookDefinition("ticket.message.admin.before", hookPhaseBefore, "content", "content_type"),
	"ticket.message.read.admin.after": newReadOnlyHookDefinition("ticket.message.read.admin.after", hookPhaseAfter),
	"ticket.message.read.user.after":  newReadOnlyHookDefinition("ticket.message.read.user.after", hookPhaseAfter),
	"ticket.message.user.after":       newReadOnlyHookDefinition("ticket.message.user.after", hookPhaseAfter),
	"ticket.message.user.before":      newRestrictedHookDefinition("ticket.message.user.before", hookPhaseBefore, "content", "content_type"),
	"ticket.order.share.after":        newReadOnlyHookDefinition("ticket.order.share.after", hookPhaseAfter),
	"ticket.status.user.after":        newReadOnlyHookDefinition("ticket.status.user.after", hookPhaseAfter),
	"ticket.status.user.before":       newRestrictedHookDefinition("ticket.status.user.before", hookPhaseBefore, "status"),
	"ticket.update.admin.after":       newReadOnlyHookDefinition("ticket.update.admin.after", hookPhaseAfter),
	"ticket.update.admin.before":      newRestrictedHookDefinition("ticket.update.admin.before", hookPhaseBefore, "status", "priority", "assigned_to"),

	"user.admin.create.after":        newReadOnlyHookDefinition("user.admin.create.after", hookPhaseAfter),
	"user.admin.create.before":       newRestrictedHookDefinition("user.admin.create.before", hookPhaseBefore, "email", "name", "role", "is_active", "password"),
	"user.admin.delete.after":        newReadOnlyHookDefinition("user.admin.delete.after", hookPhaseAfter),
	"user.admin.delete.before":       newReadOnlyHookDefinition("user.admin.delete.before", hookPhaseBefore),
	"user.admin.update.after":        newReadOnlyHookDefinition("user.admin.update.after", hookPhaseAfter),
	"user.admin.update.before":       newRestrictedHookDefinition("user.admin.update.before", hookPhaseBefore, "name", "role", "is_active", "password"),
	"user.permissions.update.after":  newReadOnlyHookDefinition("user.permissions.update.after", hookPhaseAfter),
	"user.permissions.update.before": newRestrictedHookDefinition("user.permissions.update.before", hookPhaseBefore, "permissions"),

	"virtual_inventory.batch.delete.after":    newReadOnlyHookDefinition("virtual_inventory.batch.delete.after", hookPhaseAfter),
	"virtual_inventory.batch.delete.before":   newRestrictedHookDefinition("virtual_inventory.batch.delete.before", hookPhaseBefore, "batch_no"),
	"virtual_inventory.binding.create.after":  newReadOnlyHookDefinition("virtual_inventory.binding.create.after", hookPhaseAfter),
	"virtual_inventory.binding.create.before": newRestrictedHookDefinition("virtual_inventory.binding.create.before", hookPhaseBefore, "virtual_inventory_id", "is_random", "priority", "notes"),
	"virtual_inventory.binding.delete.after":  newReadOnlyHookDefinition("virtual_inventory.binding.delete.after", hookPhaseAfter),
	"virtual_inventory.binding.delete.before": newReadOnlyHookDefinition("virtual_inventory.binding.delete.before", hookPhaseBefore),
	"virtual_inventory.binding.save.after":    newReadOnlyHookDefinition("virtual_inventory.binding.save.after", hookPhaseAfter),
	"virtual_inventory.binding.save.before":   newRestrictedHookDefinition("virtual_inventory.binding.save.before", hookPhaseBefore, "bindings"),
	"virtual_inventory.binding.update.after":  newReadOnlyHookDefinition("virtual_inventory.binding.update.after", hookPhaseAfter),
	"virtual_inventory.binding.update.before": newRestrictedHookDefinition("virtual_inventory.binding.update.before", hookPhaseBefore, "is_random", "priority", "notes"),
	"virtual_inventory.create.after":          newReadOnlyHookDefinition("virtual_inventory.create.after", hookPhaseAfter),
	"virtual_inventory.create.before": newRestrictedHookDefinition(
		"virtual_inventory.create.before",
		hookPhaseBefore,
		"name",
		"sku",
		"type",
		"script",
		"script_config",
		"description",
		"total_limit",
		"is_active",
		"notes",
	),
	"virtual_inventory.delete.after":                newReadOnlyHookDefinition("virtual_inventory.delete.after", hookPhaseAfter),
	"virtual_inventory.delete.before":               newReadOnlyHookDefinition("virtual_inventory.delete.before", hookPhaseBefore),
	"virtual_inventory.stock.create.after":          newReadOnlyHookDefinition("virtual_inventory.stock.create.after", hookPhaseAfter),
	"virtual_inventory.stock.create.before":         newRestrictedHookDefinition("virtual_inventory.stock.create.before", hookPhaseBefore, "content", "remark"),
	"virtual_inventory.stock.delete.after":          newReadOnlyHookDefinition("virtual_inventory.stock.delete.after", hookPhaseAfter),
	"virtual_inventory.stock.delete.before":         newReadOnlyHookDefinition("virtual_inventory.stock.delete.before", hookPhaseBefore),
	"virtual_inventory.stock.import.after":          newReadOnlyHookDefinition("virtual_inventory.stock.import.after", hookPhaseAfter),
	"virtual_inventory.stock.import.before":         newRestrictedHookDefinition("virtual_inventory.stock.import.before", hookPhaseBefore, "import_type", "content"),
	"virtual_inventory.stock.release.manual.after":  newReadOnlyHookDefinition("virtual_inventory.stock.release.manual.after", hookPhaseAfter),
	"virtual_inventory.stock.release.manual.before": newReadOnlyHookDefinition("virtual_inventory.stock.release.manual.before", hookPhaseBefore),
	"virtual_inventory.stock.reserve.manual.after":  newReadOnlyHookDefinition("virtual_inventory.stock.reserve.manual.after", hookPhaseAfter),
	"virtual_inventory.stock.reserve.manual.before": newRestrictedHookDefinition("virtual_inventory.stock.reserve.manual.before", hookPhaseBefore, "remark"),
	"virtual_inventory.update.after":                newReadOnlyHookDefinition("virtual_inventory.update.after", hookPhaseAfter),
	"virtual_inventory.update.before": newRestrictedHookDefinition(
		"virtual_inventory.update.before",
		hookPhaseBefore,
		"name",
		"sku",
		"type",
		"script",
		"script_config",
		"description",
		"total_limit",
		"is_active",
		"notes",
	),

	"sms.send.after":  newReadOnlyHookDefinition("sms.send.after", hookPhaseAfter),
	"sms.send.before": newRestrictedHookDefinition("sms.send.before", hookPhaseBefore, "phone", "phone_code", "code", "message"),

	"upload.image.after":         newReadOnlyHookDefinition("upload.image.after", hookPhaseAfter),
	"upload.image.before":        newReadOnlyHookDefinition("upload.image.before", hookPhaseBefore),
	"upload.image.delete.after":  newReadOnlyHookDefinition("upload.image.delete.after", hookPhaseAfter),
	"upload.image.delete.before": newReadOnlyHookDefinition("upload.image.delete.before", hookPhaseBefore),
}

func resolveHookDefinition(hook string) hookDefinition {
	normalized := normalizeHookName(hook)
	if normalized == "" {
		return hookDefinition{
			Name:                  "",
			Phase:                 hookPhaseEvent,
			RestrictPayloadPatch:  false,
			WritablePayloadFields: nil,
		}
	}

	if definition, exists := hookDefinitions[normalized]; exists {
		return definition
	}

	return hookDefinition{
		Name:                  normalized,
		Phase:                 inferHookPhase(normalized),
		RestrictPayloadPatch:  false,
		WritablePayloadFields: nil,
	}
}

func sanitizeHookPayloadPatch(hook string, payload map[string]interface{}) (map[string]interface{}, int) {
	if payload == nil {
		return nil, 0
	}

	definition := resolveHookDefinition(hook)
	if !definition.RestrictPayloadPatch {
		return clonePayloadMap(payload), 0
	}

	if len(definition.WritablePayloadFields) == 0 {
		return nil, len(payload)
	}

	filtered := make(map[string]interface{})
	dropped := 0
	for key, value := range payload {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if normalizedKey == "" {
			dropped++
			continue
		}
		if _, allowed := definition.WritablePayloadFields[normalizedKey]; !allowed {
			dropped++
			continue
		}
		filtered[normalizedKey] = clonePayloadValue(value)
	}

	if len(filtered) == 0 {
		return nil, dropped
	}
	return filtered, dropped
}

func normalizeHookName(hook string) string {
	return strings.ToLower(strings.TrimSpace(hook))
}

func inferHookPhase(hook string) hookPhase {
	normalized := normalizeHookName(hook)
	switch {
	case strings.HasSuffix(normalized, ".before"):
		return hookPhaseBefore
	case strings.HasSuffix(normalized, ".after"):
		return hookPhaseAfter
	case strings.HasPrefix(normalized, "frontend."):
		return hookPhaseFrontend
	default:
		return hookPhaseEvent
	}
}

func newRestrictedHookDefinition(name string, phase hookPhase, writableFields ...string) hookDefinition {
	fields := make(map[string]struct{}, len(writableFields))
	for _, field := range writableFields {
		normalized := strings.ToLower(strings.TrimSpace(field))
		if normalized == "" {
			continue
		}
		fields[normalized] = struct{}{}
	}
	return hookDefinition{
		Name:                  normalizeHookName(name),
		Phase:                 phase,
		RestrictPayloadPatch:  true,
		WritablePayloadFields: fields,
	}
}

func newReadOnlyHookDefinition(name string, phase hookPhase) hookDefinition {
	return hookDefinition{
		Name:                  normalizeHookName(name),
		Phase:                 phase,
		RestrictPayloadPatch:  true,
		WritablePayloadFields: map[string]struct{}{},
	}
}

func ListSupportedPluginHooks() []string {
	hooks := make([]string, 0, len(hookDefinitions))
	for name := range hookDefinitions {
		hooks = append(hooks, name)
	}
	sort.Strings(hooks)
	return hooks
}
