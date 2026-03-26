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

function formatAmount(order) {
  var amountMinor = Number(order && order.total_amount_minor);
  if (!isFinite(amountMinor)) {
    amountMinor = 0;
  }
  return (amountMinor / 100).toFixed(2) + " " + safeString(order && order.currency, "USD");
}

function getTimestampSeconds() {
  if (AuraLogic && AuraLogic.system && typeof AuraLogic.system.getTimestamp === "function") {
    var timestampValue = Number(AuraLogic.system.getTimestamp());
    if (isFinite(timestampValue) && timestampValue > 0) {
      return Math.floor(timestampValue);
    }
  }
  return Math.floor(Date.now() / 1000);
}

function getWebhookUrl(hookKey) {
  if (AuraLogic && AuraLogic.system && typeof AuraLogic.system.getWebhookUrl === "function") {
    return safeString(AuraLogic.system.getWebhookUrl(hookKey), "");
  }
  return "";
}

function sessionStorageKey(order) {
  if (order && safeString(order.order_no, "")) {
    return storageKey("hosted_session_by_order_no", order.order_no);
  }
  return storageKey("hosted_session_by_order_id", toPositiveInteger(order && order.id));
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

function persistPaidRecord(record) {
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

function readCheckoutSession(order) {
  return readStoredJSON(sessionStorageKey(order));
}

function persistCheckoutSession(order, session) {
  writeStoredJSON(sessionStorageKey(order), session);
}

function isSessionExpired(session) {
  if (!session || typeof session !== "object") {
    return true;
  }
  var now = getTimestampSeconds();
  var expiresAt = Number(session.expires_at);
  if (isFinite(expiresAt) && expiresAt > 0) {
    return expiresAt <= now;
  }
  var createdAt = Number(session.created_at);
  var ttlSeconds = Number(session.ttl_seconds);
  if (isFinite(createdAt) && createdAt > 0 && isFinite(ttlSeconds) && ttlSeconds > 0) {
    return createdAt + ttlSeconds <= now;
  }
  return false;
}

function providerBase(config, key, fallbackValue) {
  var value = safeString(config && config[key], fallbackValue || "");
  return value.replace(/\/+$/g, "");
}

function buildProviderHeaders(config) {
  var headers = {
    "Content-Type": "application/json"
  };
  var apiKey = safeString(config && config.api_key, "");
  if (apiKey) {
    headers.Authorization = "Bearer " + apiKey;
  }
  return headers;
}

function generateSessionID(order) {
  if (AuraLogic && AuraLogic.utils && typeof AuraLogic.utils.generateId === "function") {
    var generated = safeString(AuraLogic.utils.generateId(), "");
    if (generated) {
      return generated;
    }
  }
  var seed = firstNonEmpty(order && order.order_no, order && order.id, getTimestampSeconds());
  return "sess-" + seed;
}

function buildCheckoutPayload(order, config, callbackUrl, sessionID) {
  return {
    session_id: sessionID,
    merchant_id: safeString(config && config.merchant_id, "demo-merchant"),
    order_id: toPositiveInteger(order && order.id),
    order_no: safeString(order && order.order_no, ""),
    amount_minor: toPositiveInteger(order && order.total_amount_minor),
    currency: safeString(order && order.currency, "USD"),
    callback_url: callbackUrl,
    provider_label: safeString(config && config.provider_label, "Demo Hosted Gateway")
  };
}

function encodeQueryPairs(entries) {
  var output = [];
  var index;
  for (index = 0; index < entries.length; index += 1) {
    var item = entries[index];
    if (!item || item.length < 2) {
      continue;
    }
    var key = String(item[0]);
    var value = item[1];
    if (value === undefined || value === null || value === "") {
      continue;
    }
    output.push(encodeURIComponent(key) + "=" + encodeURIComponent(String(value)));
  }
  return output.join("&");
}

function buildFallbackCheckoutUrl(order, config, callbackUrl, sessionID) {
  var base = providerBase(config, "provider_checkout_base", "https://checkout.example.test/pay");
  var query = encodeQueryPairs([
    ["session_id", sessionID],
    ["merchant_id", safeString(config && config.merchant_id, "demo-merchant")],
    ["order_no", safeString(order && order.order_no, "")],
    ["amount_minor", toPositiveInteger(order && order.total_amount_minor)],
    ["currency", safeString(order && order.currency, "USD")],
    ["callback_url", callbackUrl]
  ]);
  return base + (base.indexOf("?") === -1 ? "?" : "&") + query;
}

function responseData(response) {
  if (response && response.data && typeof response.data === "object") {
    return response.data;
  }
  if (response && typeof response.body === "string" && response.body) {
    try {
      var parsed = JSON.parse(response.body);
      if (parsed && typeof parsed === "object") {
        return parsed;
      }
    } catch (error) {
    }
  }
  return {};
}

function resolveSuccessStatus(config) {
  return lowerText(config && config.success_status) || "paid";
}

function resolvePaidState(statusText, fallbackBoolean, successStatus) {
  var normalized = lowerText(statusText);
  if (!normalized && typeof fallbackBoolean === "boolean") {
    return fallbackBoolean;
  }
  return normalized === successStatus || normalized === "paid" || normalized === "success" || normalized === "succeeded";
}

function createHostedSession(order, config) {
  var callbackUrl = getWebhookUrl("payment.notify");
  var sessionID = generateSessionID(order);
  var payload = buildCheckoutPayload(order, config, callbackUrl, sessionID);
  var baseUrl = providerBase(config, "provider_api_base", "");
  var http = AuraLogic && AuraLogic.http;
  var response = null;

  if (http && typeof http.post === "function" && baseUrl) {
    response = http.post(baseUrl + "/session", payload, buildProviderHeaders(config));
  }

  var data = responseData(response);
  var ttlSeconds = toPositiveInteger(data.ttl_seconds || data.expires_in || config && config.session_ttl_seconds || 900);
  if (ttlSeconds <= 0) {
    ttlSeconds = 900;
  }
  var createdAt = getTimestampSeconds();
  var remoteSessionID = firstNonEmpty(data.session_id, data.sessionId, data.id, sessionID);
  var checkoutUrl = firstNonEmpty(
    data.checkout_url,
    data.checkoutUrl,
    data.redirect_url,
    data.redirectUrl,
    data.payment_url,
    data.paymentUrl
  );
  if (!checkoutUrl) {
    checkoutUrl = buildFallbackCheckoutUrl(order, config, callbackUrl, remoteSessionID);
  }

  var session = {
    session_id: remoteSessionID,
    order_id: toPositiveInteger(order && order.id),
    order_no: safeString(order && order.order_no, ""),
    callback_url: callbackUrl,
    checkout_url: checkoutUrl,
    provider_reference: firstNonEmpty(data.provider_reference, data.provider_order_id, data.reference),
    remote_status: lowerText(firstNonEmpty(data.status, data.payment_status, data.state)) || "pending",
    created_at: createdAt,
    ttl_seconds: ttlSeconds,
    expires_at: createdAt + ttlSeconds
  };
  persistCheckoutSession(order, session);
  return session;
}

function ensureHostedSession(order, config) {
  var existing = readCheckoutSession(order);
  if (existing && existing.checkout_url && !isSessionExpired(existing)) {
    return existing;
  }
  return createHostedSession(order, config);
}

function buildHostedCardHTML(order, config, session) {
  var checkoutTitle = safeString(config && config.checkout_title, "Hosted Checkout");
  var providerLabel = safeString(config && config.provider_label, "Demo Hosted Gateway");
  return "" +
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
      '<div style="display:grid;gap:8px;padding:12px;border-radius:12px;border:1px dashed #cbd5e1;background:#f8fafc;">' +
        '<span style="font-size:12px;color:#64748b;">Hosted checkout session</span>' +
        '<code style="font-size:12px;line-height:1.5;word-break:break-all;">' + escapeHTML(session.session_id) + '</code>' +
        '<a href="' + escapeHTML(session.checkout_url) + '" target="_blank" rel="noopener noreferrer" style="display:inline-flex;align-items:center;justify-content:center;padding:11px 14px;border-radius:10px;background:#0f172a;color:#ffffff;text-decoration:none;font-size:14px;font-weight:700;">Open hosted checkout</a>' +
        '<span style="font-size:12px;color:#64748b;">If the gateway completes asynchronously, AuraLogic will reconcile payment from webhook and polling.</span>' +
      '</div>' +
      '<div style="display:grid;gap:6px;">' +
        '<span style="font-size:12px;color:#64748b;">Declared webhook URL</span>' +
        '<code style="font-size:12px;line-height:1.5;word-break:break-all;">' + escapeHTML(session.callback_url || "Configure after upload") + '</code>' +
      '</div>' +
    '</div>';
}

function updateOrderPaymentData(config, session) {
  if (AuraLogic && AuraLogic.order && typeof AuraLogic.order.updatePaymentData === "function") {
    AuraLogic.order.updatePaymentData({
      provider_label: safeString(config && config.provider_label, "Demo Hosted Gateway"),
      checkout_mode: "hosted",
      session_id: session.session_id,
      checkout_url: session.checkout_url,
      callback_url: session.callback_url,
      provider_reference: session.provider_reference || ""
    });
  }
}

function onGeneratePaymentCard(order, config) {
  var session = ensureHostedSession(order, config || {});
  updateOrderPaymentData(config || {}, session);
  return {
    title: safeString(config && config.checkout_title, "Hosted Checkout"),
    description: safeString(config && config.provider_label, "Demo Hosted Gateway") + " hosted checkout",
    cache_ttl: 30,
    data: {
      session_id: session.session_id,
      checkout_url: session.checkout_url,
      callback_url: session.callback_url
    },
    html: buildHostedCardHTML(order, config || {}, session)
  };
}

function buildStatusURL(config, session) {
  var baseUrl = providerBase(config, "provider_api_base", "");
  if (!baseUrl || !session || !session.session_id) {
    return "";
  }
  return baseUrl + "/session-status?" + encodeQueryPairs([
    ["session_id", session.session_id]
  ]);
}

function buildPaidRecordFromStatus(order, session, payload, source, config) {
  var statusText = firstNonEmpty(payload.status, payload.payment_status, payload.state, payload.result);
  return {
    order_id: toPositiveInteger(order && order.id) || toPositiveInteger(payload.order_id || payload.orderId),
    order_no: firstNonEmpty(payload.order_no, payload.orderNo, order && order.order_no),
    session_id: firstNonEmpty(payload.session_id, payload.sessionId, session && session.session_id),
    transaction_id: firstNonEmpty(
      payload.transaction_id,
      payload.transactionId,
      payload.provider_transaction_id,
      payload.providerTransactionId,
      payload.provider_reference,
      session && session.provider_reference
    ),
    status: lowerText(statusText) || "pending",
    paid: resolvePaidState(statusText, payload.paid === true, resolveSuccessStatus(config || {})),
    provider_label: safeString(config && config.provider_label, "Demo Hosted Gateway"),
    source: safeString(source, "unknown"),
    received_at: new Date().toISOString(),
    raw_payload: payload
  };
}

function onCheckPaymentStatus(order, config) {
  var paidRecord = readPaidRecord(order);
  if (paidRecord && paidRecord.paid) {
    return {
      paid: true,
      transaction_id: safeString(paidRecord.transaction_id, ""),
      message: "Payment already confirmed from stored hosted session data.",
      data: paidRecord
    };
  }

  if (safeBoolean(config && config.simulate_paid)) {
    return {
      paid: true,
      transaction_id: "SIM-" + safeString(order && order.order_no, "UNKNOWN"),
      message: "Simulated hosted payment approval is enabled.",
      data: {
        simulated: true,
        provider_label: safeString(config && config.provider_label, "Demo Hosted Gateway")
      }
    };
  }

  var session = readCheckoutSession(order);
  if (!session || !session.session_id) {
    return {
      paid: false,
      message: "Hosted checkout session has not been created yet."
    };
  }

  var http = AuraLogic && AuraLogic.http;
  var statusUrl = buildStatusURL(config || {}, session);
  if (!http || typeof http.get !== "function" || !statusUrl) {
    return {
      paid: false,
      message: "Hosted provider status API is not available."
    };
  }

  var response = http.get(statusUrl, buildProviderHeaders(config || {}));
  if (response && response.error) {
    return {
      paid: false,
      message: "Provider status query failed: " + safeString(response.error, "unknown error")
    };
  }
  if (!response || Number(response.status) < 200 || Number(response.status) >= 300) {
    return {
      paid: false,
      message: "Provider status query returned " + safeString(response && response.statusText, String(response && response.status || "unknown"))
    };
  }

  var payload = responseData(response);
  var record = buildPaidRecordFromStatus(order, session, payload, "polling", config || {});
  if (record.paid) {
    persistPaidRecord(record);
    return {
      paid: true,
      transaction_id: safeString(record.transaction_id, ""),
      message: "Hosted provider confirms payment success.",
      data: record
    };
  }

  return {
    paid: false,
    message: "Hosted provider status is still " + safeString(record.status, "pending") + ".",
    data: record
  };
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
  if (!payload.order_no && !payload.order_id && !payload.orderId) {
    var fallbackOrderNo = webhookQuery("order_no");
    if (fallbackOrderNo) {
      payload.order_no = fallbackOrderNo;
    }
  }

  var session = {
    session_id: firstNonEmpty(payload.session_id, payload.sessionId),
    provider_reference: firstNonEmpty(payload.provider_reference, payload.providerReference)
  };
  var record = buildPaidRecordFromStatus(null, session, payload, "webhook", config || {});
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

  persistPaidRecord(record);
  return {
    ack_status: 200,
    ack_body: {
      ok: true,
      recorded: true,
      paid: record.paid,
      order_no: record.order_no || null,
      transaction_id: record.transaction_id || null
    },
    paid: record.paid,
    order_id: record.order_id > 0 ? record.order_id : undefined,
    order_no: record.order_no || undefined,
    transaction_id: record.transaction_id || undefined,
    message: record.paid
      ? "Webhook recorded and hosted payment is ready to confirm."
      : "Webhook recorded; hosted payment remains pending.",
    data: record,
    queue_polling: !record.paid
  };
}

function onRefund(order, config) {
  var session = readCheckoutSession(order);
  var refundRecord = {
    order_id: toPositiveInteger(order && order.id),
    order_no: safeString(order && order.order_no, ""),
    session_id: session && session.session_id ? session.session_id : "",
    requested_at: new Date().toISOString(),
    provider_label: safeString(config && config.provider_label, "Demo Hosted Gateway")
  };

  var http = AuraLogic && AuraLogic.http;
  var baseUrl = providerBase(config, "provider_api_base", "");
  if (session && session.session_id && http && typeof http.post === "function" && baseUrl) {
    var response = http.post(baseUrl + "/refund", {
      merchant_id: safeString(config && config.merchant_id, "demo-merchant"),
      order_id: refundRecord.order_id,
      order_no: refundRecord.order_no,
      session_id: refundRecord.session_id
    }, buildProviderHeaders(config || {}));
    var data = responseData(response);
    if (!response.error && Number(response.status) >= 200 && Number(response.status) < 300) {
      refundRecord.refund_id = firstNonEmpty(data.refund_id, data.refundId, data.id);
      refundRecord.remote_response = data;
      writeStoredJSON(storageKey("refund_request", refundRecord.order_no || refundRecord.order_id), refundRecord);
      return {
        success: true,
        message: "Hosted refund request accepted by provider.",
        data: refundRecord
      };
    }
    refundRecord.remote_error = safeString(response && response.error, safeString(response && response.statusText, "remote refund failed"));
  }

  writeStoredJSON(storageKey("refund_request", refundRecord.order_no || refundRecord.order_id), refundRecord);
  return {
    success: true,
    message: "Refund request recorded for manual processing.",
    data: refundRecord
  };
}
