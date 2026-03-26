function safeString(value, fallbackValue) {
  if (typeof value === "string") {
    var trimmed = value.replace(/^\s+|\s+$/g, "");
    if (trimmed) {
      return trimmed;
    }
  }
  return fallbackValue || "";
}

function safeBoolean(value) {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    var normalized = value.toLowerCase();
    return normalized === "true" || normalized === "1" || normalized === "yes";
  }
  return Boolean(value);
}

function firstNonEmpty() {
  var index;
  for (index = 0; index < arguments.length; index += 1) {
    var value = arguments[index];
    if (typeof value === "string" && value.replace(/^\s+|\s+$/g, "")) {
      return value.replace(/^\s+|\s+$/g, "");
    }
    if (typeof value === "number" && isFinite(value)) {
      return String(value);
    }
  }
  return "";
}

function toPositiveInteger(value) {
  var numeric = Number(value);
  if (!isFinite(numeric) || numeric <= 0) {
    return 0;
  }
  return Math.floor(numeric);
}

function lowerText(value) {
  return safeString(value, "").toLowerCase();
}

function escapeHTML(value) {
  return String(value || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function storageKey(prefix, value) {
  return prefix + ":" + String(value || "");
}

function readStoredJSON(key) {
  var storage = AuraLogic && AuraLogic.storage;
  if (!storage || typeof storage.get !== "function") {
    return null;
  }
  var raw = storage.get(key);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw);
  } catch (error) {
    return null;
  }
}

function writeStoredJSON(key, value) {
  var storage = AuraLogic && AuraLogic.storage;
  if (!storage || typeof storage.set !== "function") {
    return false;
  }
  return storage.set(key, JSON.stringify(value || {}));
}

function buildOrderLookup(order) {
  return {
    order_id: toPositiveInteger(order && order.id),
    order_no: safeString(order && order.order_no, "")
  };
}

function readPaidRecord(order) {
  var lookup = buildOrderLookup(order);
  var record = null;
  if (lookup.order_no) {
    record = readStoredJSON(storageKey("payment_by_order_no", lookup.order_no));
  }
  if (!record && lookup.order_id > 0) {
    record = readStoredJSON(storageKey("payment_by_order_id", lookup.order_id));
  }
  return record;
}

function formatAmount(order) {
  var amountMinor = Number(order && order.total_amount_minor);
  if (!isFinite(amountMinor)) {
    amountMinor = 0;
  }
  return (amountMinor / 100).toFixed(2) + " " + safeString(order && order.currency, "USD");
}

function readWebhookPayload() {
  var webhook = AuraLogic && AuraLogic.webhook;
  if (!webhook) {
    return {};
  }
  try {
    if (typeof webhook.json === "function") {
      var parsed = webhook.json();
      if (parsed && typeof parsed === "object") {
        return parsed;
      }
    }
  } catch (error) {
  }
  return {};
}

function webhookQuery(name) {
  var webhook = AuraLogic && AuraLogic.webhook;
  if (!webhook || typeof webhook.query !== "function") {
    return "";
  }
  return safeString(webhook.query(name), "");
}

function webhookHeader(name) {
  var webhook = AuraLogic && AuraLogic.webhook;
  if (!webhook || typeof webhook.header !== "function") {
    return "";
  }
  return safeString(webhook.header(name), "");
}

function buildWebhookRecord(payload, config) {
  var orderID = toPositiveInteger(payload.order_id || payload.orderId || webhookQuery("order_id") || webhookQuery("orderId"));
  var orderNo = firstNonEmpty(payload.order_no, payload.orderNo, webhookQuery("order_no"), webhookQuery("orderNo"));
  var transactionID = firstNonEmpty(
    payload.transaction_id,
    payload.transactionId,
    payload.tx_id,
    payload.txId,
    webhookHeader("x-payment-transaction-id")
  );
  var status = lowerText(firstNonEmpty(payload.status, payload.payment_status, payload.state, payload.result));
  var successStatus = lowerText(config && config.success_status) || "paid";
  var paid = status === successStatus || status === "paid" || status === "success" || status === "succeeded";
  var callbackMode = safeString(config && config.callback_mode, "mark_paid");
  var record = {
    order_id: orderID,
    order_no: orderNo,
    transaction_id: transactionID,
    status: status || "pending",
    paid: paid,
    provider_label: safeString(config && config.provider_label, "Demo Gateway"),
    callback_mode: callbackMode,
    received_at: new Date().toISOString(),
    raw_payload: payload
  };
  return record;
}

function persistWebhookRecord(record) {
  if (record.order_no) {
    writeStoredJSON(storageKey("payment_by_order_no", record.order_no), record);
  }
  if (record.order_id > 0) {
    writeStoredJSON(storageKey("payment_by_order_id", record.order_id), record);
  }
  if (record.transaction_id) {
    writeStoredJSON(storageKey("payment_by_transaction", record.transaction_id), record);
  }
}

function onGeneratePaymentCard(order, config) {
  var checkoutTitle = safeString(config && config.checkout_title, "Manual Review Checkout");
  var providerLabel = safeString(config && config.provider_label, "Demo Gateway");
  var instructions = safeString(
    config && config.payment_instructions,
    "After your upstream payment succeeds, the provider should call the declared webhook endpoint."
  );
  var callbackMode = safeString(config && config.callback_mode, "mark_paid");
  var callbackUrl = "";
  if (AuraLogic && AuraLogic.system && typeof AuraLogic.system.getWebhookUrl === "function") {
    callbackUrl = safeString(AuraLogic.system.getWebhookUrl("payment.notify"), "");
  }

  if (AuraLogic && AuraLogic.order && typeof AuraLogic.order.updatePaymentData === "function") {
    AuraLogic.order.updatePaymentData({
      provider_label: providerLabel,
      callback_mode: callbackMode,
      callback_url: callbackUrl,
      expected_status: safeString(config && config.success_status, "paid")
    });
  }

  return {
    title: checkoutTitle,
    description: providerLabel + " checkout",
    cache_ttl: 60,
    data: {
      callback_mode: callbackMode,
      callback_url: callbackUrl,
      provider_label: providerLabel
    },
    html: "" +
      '<div style="display:grid;gap:12px;padding:16px;border:1px solid #d7dee7;border-radius:14px;background:#ffffff;font-family:Arial,sans-serif;color:#0f172a;">' +
        '<div style="display:grid;gap:4px;">' +
          '<strong style="font-size:16px;line-height:1.4;">' + escapeHTML(checkoutTitle) + '</strong>' +
          '<span style="font-size:13px;color:#475569;">Provider: ' + escapeHTML(providerLabel) + '</span>' +
        '</div>' +
        '<div style="padding:12px;border-radius:12px;background:#f8fafc;border:1px solid #e2e8f0;display:grid;gap:6px;">' +
          '<span style="font-size:12px;color:#64748b;text-transform:uppercase;letter-spacing:0.04em;">Order</span>' +
          '<strong style="font-size:15px;">' + escapeHTML(safeString(order && order.order_no, "-")) + '</strong>' +
          '<span style="font-size:14px;color:#0f172a;">Amount: ' + escapeHTML(formatAmount(order)) + '</span>' +
        '</div>' +
        '<div style="display:grid;gap:6px;">' +
          '<span style="font-size:13px;color:#475569;">' + escapeHTML(instructions) + '</span>' +
          '<span style="font-size:12px;color:#64748b;">Callback mode: ' + escapeHTML(callbackMode) + '</span>' +
        '</div>' +
        '<div style="display:grid;gap:6px;padding:12px;border-radius:12px;border:1px dashed #cbd5e1;background:#f8fafc;">' +
          '<span style="font-size:12px;color:#64748b;">Declared webhook URL</span>' +
          '<code style="font-size:12px;line-height:1.5;word-break:break-all;">' + escapeHTML(callbackUrl || "Configure after upload") + '</code>' +
          '<span style="font-size:12px;color:#64748b;">Header auth uses X-Payment-Webhook-Token from config.webhook_token.</span>' +
        '</div>' +
      '</div>'
  };
}

function onCheckPaymentStatus(order, config) {
  var paidRecord = readPaidRecord(order);
  if (paidRecord && paidRecord.paid) {
    return {
      paid: true,
      transaction_id: safeString(paidRecord.transaction_id, ""),
      message: "Payment confirmed from stored webhook record.",
      data: paidRecord
    };
  }

  if (safeBoolean(config && config.simulate_paid)) {
    return {
      paid: true,
      transaction_id: "SIM-" + safeString(order && order.order_no, "UNKNOWN"),
      message: "Simulated payment approval is enabled.",
      data: {
        simulated: true,
        provider_label: safeString(config && config.provider_label, "Demo Gateway")
      }
    };
  }

  return {
    paid: false,
    message: "Waiting for payment confirmation webhook or polling result."
  };
}

function onRefund(order, config) {
  var refundRecord = {
    order_id: toPositiveInteger(order && order.id),
    order_no: safeString(order && order.order_no, ""),
    requested_at: new Date().toISOString(),
    provider_label: safeString(config && config.provider_label, "Demo Gateway")
  };
  writeStoredJSON(storageKey("refund_request", refundRecord.order_no || refundRecord.order_id), refundRecord);
  return {
    success: true,
    message: "Refund request recorded for manual processing.",
    data: refundRecord
  };
}

function onWebhook(hook, config) {
  var normalizedHook = safeString(hook, "payment.notify");
  if (normalizedHook !== "payment.notify") {
    return {
      ack_status: 404,
      ack_body: {
        ok: false,
        error: "unknown webhook"
      }
    };
  }

  var payload = readWebhookPayload();
  var record = buildWebhookRecord(payload, config || {});
  var callbackMode = safeString(config && config.callback_mode, "mark_paid");

  if (!record.order_no && record.order_id === 0) {
    return {
      ack_status: 202,
      ack_body: {
        ok: false,
        message: "missing order identifier"
      },
      paid: false,
      queue_polling: true,
      message: "Webhook accepted but an order identifier is still required."
    };
  }

  persistWebhookRecord(record);

  return {
    ack_status: 200,
    ack_body: {
      ok: true,
      recorded: true,
      paid: record.paid,
      order_no: record.order_no || null,
      transaction_id: record.transaction_id || null
    },
    paid: callbackMode === "mark_paid" ? record.paid : false,
    order_id: record.order_id > 0 ? record.order_id : undefined,
    order_no: record.order_no || undefined,
    transaction_id: record.transaction_id || undefined,
    message: record.paid
      ? "Webhook recorded and payment is ready to confirm."
      : "Webhook recorded; payment remains pending.",
    data: record,
    queue_polling: callbackMode === "queue_polling" || !record.paid
  };
}
