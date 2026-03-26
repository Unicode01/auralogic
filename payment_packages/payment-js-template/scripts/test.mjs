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
      getWebhookUrl(hook) {
        return `https://example.test/api/payment-methods/7/webhooks/${String(hook || "")}`;
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
    console
  });
  const source = readFileSync(runtimePath, "utf8");
  vm.runInContext(source, context, { filename: runtimePath });

  return {
    context,
    AuraLogic,
    storageValues,
    paymentDataWrites
  };
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
  id: 42,
  order_no: "ORD-42",
  total_amount_minor: 1999,
  currency: "USD"
};

const sampleConfig = {
  checkout_title: "Manual Review Checkout",
  provider_label: "Demo Gateway",
  payment_instructions: "Wait for upstream confirmation.",
  callback_mode: "mark_paid",
  success_status: "paid",
  simulate_paid: false,
  webhook_token: "change-me-demo-token"
};

test("payment-js template exports expected callbacks", () => {
  const runtime = createRuntime();
  assert.equal(typeof runtime.context.onGeneratePaymentCard, "function");
  assert.equal(typeof runtime.context.onCheckPaymentStatus, "function");
  assert.equal(typeof runtime.context.onRefund, "function");
  assert.equal(typeof runtime.context.onWebhook, "function");
});

test("onGeneratePaymentCard writes payment metadata and renders callback url", () => {
  const runtime = createRuntime();
  const result = runtime.context.onGeneratePaymentCard(sampleOrder, sampleConfig);

  assert.equal(result.title, "Manual Review Checkout");
  assert.match(result.html, /ORD-42/);
  assert.match(result.html, /payment\.notify/);
  assert.equal(result.cache_ttl, 60);
  assert.equal(runtime.paymentDataWrites.length, 1);
  assert.equal(runtime.paymentDataWrites[0].provider_label, "Demo Gateway");
});

test("onWebhook records payment and onCheckPaymentStatus resolves it", () => {
  const runtime = createRuntime({
    webhookBodyText: JSON.stringify({
      order_no: "ORD-42",
      status: "paid",
      transaction_id: "TX-42"
    }),
    webhookHeaders: {
      "X-Payment-Transaction-Id": "TX-42"
    }
  });

  const webhookResult = runtime.context.onWebhook("payment.notify", sampleConfig);
  assert.equal(webhookResult.ack_status, 200);
  assert.equal(webhookResult.paid, true);
  assert.equal(webhookResult.order_no, "ORD-42");
  assert.equal(webhookResult.transaction_id, "TX-42");

  const statusResult = runtime.context.onCheckPaymentStatus(sampleOrder, sampleConfig);
  assert.equal(statusResult.paid, true);
  assert.equal(statusResult.transaction_id, "TX-42");
});

test("queue_polling mode defers final confirmation to polling", () => {
  const runtime = createRuntime({
    webhookBodyText: JSON.stringify({
      order_id: 42,
      order_no: "ORD-42",
      status: "paid",
      transaction_id: "TX-QUEUE-42"
    })
  });

  const result = runtime.context.onWebhook("payment.notify", {
    callback_mode: "queue_polling",
    success_status: "paid"
  });

  assert.equal(result.paid, false);
  assert.equal(result.queue_polling, true);

  const polled = runtime.context.onCheckPaymentStatus(sampleOrder, sampleConfig);
  assert.equal(polled.paid, true);
  assert.equal(polled.transaction_id, "TX-QUEUE-42");
});

test("onRefund stores a refund request marker", () => {
  const runtime = createRuntime();
  const result = runtime.context.onRefund(sampleOrder, sampleConfig);

  assert.equal(result.success, true);
  assert.equal(result.data.order_no, "ORD-42");
  assert.ok(runtime.storageValues.has("refund_request:ORD-42"));
});
