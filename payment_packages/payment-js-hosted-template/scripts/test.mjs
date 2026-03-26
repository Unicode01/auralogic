import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(scriptDir, "..");
const runtimePath = path.join(packageRoot, "dist", "index.js");

function createRuntime(options = {}) {
  const storageValues = new Map();
  const paymentDataWrites = [];
  const httpCalls = [];
  const httpPostQueue = Array.isArray(options.httpPostResponses) ? options.httpPostResponses.slice() : [];
  const httpGetQueue = Array.isArray(options.httpGetResponses) ? options.httpGetResponses.slice() : [];
  const webhookHeaders = normalizeHeaderMap(options.webhookHeaders || {});
  const webhookQuery = normalizeStringMap(options.webhookQuery || {});
  const webhookBodyText = typeof options.webhookBodyText === "string" ? options.webhookBodyText : "";

  const AuraLogic = {
    storage: {
      get(key) {
        return storageValues.has(key) ? storageValues.get(key) : undefined;
      },
      set(key, value) {
        storageValues.set(String(key), String(value));
        return true;
      },
      delete(key) {
        return storageValues.delete(String(key));
      },
      list() {
        return Array.from(storageValues.keys()).sort();
      },
      clear() {
        storageValues.clear();
        return true;
      }
    },
    order: {
      updatePaymentData(value) {
        paymentDataWrites.push(value);
        return true;
      }
    },
    system: {
      getTimestamp() {
        return 1710000000;
      },
      getWebhookUrl(hook) {
        return `https://example.test/api/payment-methods/9/webhooks/${String(hook || "")}`;
      }
    },
    utils: {
      generateId() {
        return "demo-session-generated";
      },
      md5(value) {
        return `md5-${String(value || "")}`;
      }
    },
    http: {
      post(url, body, headers) {
        httpCalls.push({ method: "POST", url, body, headers });
        return shiftResponse(httpPostQueue, { status: 500, error: "missing mocked POST response" });
      },
      get(url, headers) {
        httpCalls.push({ method: "GET", url, headers });
        return shiftResponse(httpGetQueue, { status: 500, error: "missing mocked GET response" });
      }
    },
    webhook: {
      json() {
        return webhookBodyText ? JSON.parse(webhookBodyText) : {};
      },
      text() {
        return webhookBodyText;
      },
      query(name) {
        return webhookQuery[String(name || "")] || undefined;
      },
      header(name) {
        return webhookHeaders[String(name || "").toLowerCase()] || undefined;
      }
    }
  };

  const context = vm.createContext({
    AuraLogic,
    JSON,
    Date,
    Math,
    Number,
    String,
    Boolean,
    Object,
    Array,
    RegExp,
    parseInt,
    parseFloat,
    isFinite,
    encodeURIComponent,
    decodeURIComponent,
    console
  });
  const source = readFileSync(runtimePath, "utf8");
  vm.runInContext(source, context, { filename: runtimePath });

  return {
    context,
    storageValues,
    paymentDataWrites,
    httpCalls
  };
}

function shiftResponse(queue, fallbackValue) {
  if (queue.length === 0) {
    return fallbackValue;
  }
  const next = queue.shift();
  return typeof next === "function" ? next() : next;
}

function normalizeStringMap(value) {
  const out = {};
  for (const key of Object.keys(value)) {
    out[key] = String(value[key]);
  }
  return out;
}

function normalizeHeaderMap(value) {
  const out = {};
  for (const key of Object.keys(value)) {
    out[key.toLowerCase()] = String(value[key]);
  }
  return out;
}

const sampleOrder = {
  id: 88,
  order_no: "ORD-88",
  total_amount_minor: 2599,
  currency: "USD"
};

const sampleConfig = {
  checkout_title: "Hosted Checkout",
  provider_label: "Demo Hosted Gateway",
  provider_api_base: "https://gateway.example.test/api",
  provider_checkout_base: "https://checkout.example.test/pay",
  merchant_id: "demo-merchant",
  api_key: "demo-api-key",
  success_status: "paid",
  session_ttl_seconds: 900,
  simulate_paid: false,
  webhook_secret: "change-me-demo-secret"
};

test("payment-js hosted template exports expected callbacks", () => {
  const runtime = createRuntime();
  assert.equal(typeof runtime.context.onGeneratePaymentCard, "function");
  assert.equal(typeof runtime.context.onCheckPaymentStatus, "function");
  assert.equal(typeof runtime.context.onRefund, "function");
  assert.equal(typeof runtime.context.onWebhook, "function");
});

test("onGeneratePaymentCard creates a hosted session and writes payment metadata", () => {
  const runtime = createRuntime({
    httpPostResponses: [
      {
        status: 201,
        data: {
          session_id: "sess-hosted-88",
          checkout_url: "https://checkout.example.test/pay/sess-hosted-88",
          provider_reference: "prov-88",
          expires_in: 600
        }
      }
    ]
  });

  const result = runtime.context.onGeneratePaymentCard(sampleOrder, sampleConfig);

  assert.equal(result.title, "Hosted Checkout");
  assert.match(result.html, /checkout\.example\.test\/pay\/sess-hosted-88/);
  assert.equal(result.cache_ttl, 30);
  assert.equal(runtime.paymentDataWrites.length, 1);
  assert.equal(runtime.paymentDataWrites[0].session_id, "sess-hosted-88");
  assert.equal(runtime.httpCalls.length, 1);
  assert.equal(runtime.httpCalls[0].url, "https://gateway.example.test/api/session");
});

test("onGeneratePaymentCard reuses a cached hosted session", () => {
  const runtime = createRuntime({
    httpPostResponses: [
      {
        status: 201,
        data: {
          session_id: "sess-cache-88",
          checkout_url: "https://checkout.example.test/pay/sess-cache-88",
          expires_in: 600
        }
      }
    ]
  });

  runtime.context.onGeneratePaymentCard(sampleOrder, sampleConfig);
  const second = runtime.context.onGeneratePaymentCard(sampleOrder, sampleConfig);

  assert.match(second.html, /sess-cache-88/);
  assert.equal(runtime.httpCalls.length, 1);
});

test("onCheckPaymentStatus polls the hosted provider and confirms payment", () => {
  const runtime = createRuntime({
    httpPostResponses: [
      {
        status: 201,
        data: {
          session_id: "sess-status-88",
          checkout_url: "https://checkout.example.test/pay/sess-status-88",
          expires_in: 600
        }
      }
    ],
    httpGetResponses: [
      {
        status: 200,
        data: {
          status: "paid",
          transaction_id: "TX-HOSTED-88",
          provider_reference: "prov-88"
        }
      }
    ]
  });

  runtime.context.onGeneratePaymentCard(sampleOrder, sampleConfig);
  const result = runtime.context.onCheckPaymentStatus(sampleOrder, sampleConfig);

  assert.equal(result.paid, true);
  assert.equal(result.transaction_id, "TX-HOSTED-88");
  assert.equal(runtime.httpCalls[1].url, "https://gateway.example.test/api/session-status?session_id=sess-status-88");
});

test("onWebhook persists a paid record that polling can later reuse", () => {
  const runtime = createRuntime({
    webhookBodyText: JSON.stringify({
      order_no: "ORD-88",
      session_id: "sess-hook-88",
      status: "paid",
      transaction_id: "TX-WEBHOOK-88"
    })
  });

  const webhookResult = runtime.context.onWebhook("payment.notify", sampleConfig);
  assert.equal(webhookResult.ack_status, 200);
  assert.equal(webhookResult.paid, true);
  assert.equal(webhookResult.order_no, "ORD-88");

  const polled = runtime.context.onCheckPaymentStatus(sampleOrder, sampleConfig);
  assert.equal(polled.paid, true);
  assert.equal(polled.transaction_id, "TX-WEBHOOK-88");
});

test("onRefund calls the remote refund endpoint when a hosted session exists", () => {
  const runtime = createRuntime({
    httpPostResponses: [
      {
        status: 201,
        data: {
          session_id: "sess-refund-88",
          checkout_url: "https://checkout.example.test/pay/sess-refund-88",
          expires_in: 600
        }
      },
      {
        status: 202,
        data: {
          refund_id: "refund-88",
          accepted: true
        }
      }
    ]
  });

  runtime.context.onGeneratePaymentCard(sampleOrder, sampleConfig);
  const result = runtime.context.onRefund(sampleOrder, sampleConfig);

  assert.equal(result.success, true);
  assert.equal(result.data.refund_id, "refund-88");
  assert.equal(runtime.httpCalls[1].url, "https://gateway.example.test/api/refund");
});
