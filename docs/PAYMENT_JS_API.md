# Payment Method JS API Documentation

AuraLogic 付款方式使用 JavaScript 脚本进行扩展，本文档描述了所有可用的 API。

## 回调函数

付款方式脚本必须实现以下回调函数：

### onGeneratePaymentCard(order, config)

生成付款卡片的 HTML 内容，当用户选择付款方式时调用。

**参数：**
- `order` - 订单对象
  - `id` - 订单ID
  - `order_no` - 订单号
  - `status` - 订单状态
  - `total_amount` - 订单总金额
  - `currency` - 货币代码 (CNY, USD, EUR 等)
  - `created_at` - 创建时间 (RFC3339 格式)
- `config` - 付款方式配置对象 (来自配置 JSON)

**返回值：**
```javascript
{
  html: "<div>付款信息 HTML</div>",   // 必需：显示的HTML内容
  title: "付款方式标题",               // 可选：卡片标题
  description: "付款说明",             // 可选：描述文字
  ttl: 300,                           // 可选：HTML内容缓存时间（秒），默认0（不缓存）
  data: { key: "value" }              // 可选：附加数据
}
```

或直接返回 HTML 字符串。

**ttl 参数说明：**
- `ttl` (Time To Live) 用于控制生成的 HTML 内容在前端的缓存时间
- 单位为秒，默认值为 0（不缓存，每次都重新生成）
- 适用场景：
  - 静态付款信息（如固定的银行账号）：可设置较长的 ttl（如 3600 秒）
  - 动态付款信息（如实时汇率、临时二维码）：应设置较短的 ttl 或不设置（默认 0）
- 示例：`ttl: 300` 表示 HTML 内容在 5 分钟内不会重新生成

**示例：**
```javascript
function onGeneratePaymentCard(order, config) {
  const amount = AuraLogic.utils.formatPrice(order.total_amount, order.currency);
  return {
    html: `
      <div class="space-y-4">
        <p>请向以下账户转账：</p>
        <p><strong>银行：</strong>${config.bank_name || '未设置'}</p>
        <p><strong>账号：</strong>${config.account_number || '未设置'}</p>
        <p><strong>金额：</strong>${amount}</p>
        <p><strong>备注：</strong>${order.order_no}</p>
      </div>
    `,
    title: '银行转账'
  };
}
```

### onCheckPaymentStatus(order, config)

检查付款状态，由后台轮询服务调用。

**参数：** 同 `onGeneratePaymentCard`

**返回值：**
```javascript
{
  paid: true,                        // 是否已付款
  transaction_id: "TX123456",        // 可选：交易ID
  message: "付款成功",                // 可选：状态消息
  data: { key: "value" }             // 可选：附加数据
}
```

或直接返回 `true`/`false`。

**示例：**
```javascript
function onCheckPaymentStatus(order, config) {
  // 对于需要人工确认的付款方式
  return {
    paid: false,
    message: '此付款方式需要人工确认'
  };
}
```

### onRefund(order, config)

处理退款请求，当管理员在订单详情页点击退款时调用。

**参数：** 同 `onGeneratePaymentCard`

**返回值：**
```javascript
{
  success: true,                     // 是否退款成功
  transaction_id: "TX123456",        // 可选：退款交易ID
  message: "退款成功",                // 可选：状态消息
  data: { key: "value" }             // 可选：附加数据
}
```

或直接返回 `true`/`false`。

**示例：**
```javascript
function onRefund(order, config) {
  // 对于加密货币等无法自动退款的付款方式
  return {
    success: true,
    message: '请手动将款项退回用户地址',
    data: {
      amount: order.total_amount,
      wallet_address: config.wallet_address
    }
  };
}
```

---

## AuraLogic API

所有 API 都通过 `AuraLogic` 命名空间访问。

### AuraLogic.storage - 本地存储

用于存储付款方式相关的持久化数据，数据保存在 `storage/payments/` 目录下，按付款方式 ID 隔离。

> **说明**: 数据存储在本地 JSON 文件中，服务重启后数据不会丢失。每个付款方式有独立的存储文件。

#### storage.get(key)

获取存储值。

```javascript
const value = AuraLogic.storage.get('last_payment_time');
// 如果不存在返回 undefined
```

#### storage.set(key, value)

设置存储值。

```javascript
const success = AuraLogic.storage.set('counter', '10');
// 返回 true/false
```

> 注意：value 会被转换为字符串存储

#### storage.delete(key)

删除存储值。

```javascript
const success = AuraLogic.storage.delete('temp_data');
// 返回 true/false
```

#### storage.list()

列出当前付款方式的所有存储键。

```javascript
const keys = AuraLogic.storage.list();
// 返回 ["key1", "key2", "key3"]
```

#### storage.clear()

清除当前付款方式的所有存储数据。

```javascript
const success = AuraLogic.storage.clear();
// 返回 true/false
```

> **存储隔离说明**: 每个付款方式的存储是完全隔离的，一个付款方式的脚本无法访问其他付款方式的存储数据。

---

### AuraLogic.order - 订单操作

#### order.get()

获取当前订单信息。

```javascript
const order = AuraLogic.order.get();
// 返回:
// {
//   id: 123,
//   order_no: "ORD-20240101-001",
//   status: "pending_payment",
//   total_amount: 99.99,
//   currency: "CNY",
//   created_at: "2024-01-01T12:00:00Z"
// }
```

#### order.getItems()

获取订单商品列表。

```javascript
const items = AuraLogic.order.getItems();
// 返回:
// [
//   {
//     index: 0,
//     name: "商品名称",
//     sku: "SKU001",
//     quantity: 2,
//     image_url: "https://...",
//     product_type: "physical",
//     attributes: { color: "red" }
//   }
// ]
```

#### order.getUser()

获取订单用户信息。

```javascript
const user = AuraLogic.order.getUser();
// 返回:
// {
//   id: 456,
//   email: "user@example.com",
//   name: "用户名"
// }
```

#### order.updatePaymentData(data)

更新订单付款数据。

```javascript
const success = AuraLogic.order.updatePaymentData({
  payment_address: 'TRc123...',
  expected_amount: '10.5'
});
// 返回 true/false
```

---

### AuraLogic.config - 配置访问

#### config.get(key?, defaultValue?)

获取配置值。

```javascript
// 获取所有配置
const allConfig = AuraLogic.config.get();

// 获取指定配置项
const bankName = AuraLogic.config.get('bank_name');

// 获取配置项，带默认值
const currency = AuraLogic.config.get('currency', 'CNY');
```

---

### AuraLogic.utils - 工具函数

#### utils.formatPrice(amount, currency?)

格式化价格显示。

```javascript
AuraLogic.utils.formatPrice(99.99, 'CNY');  // "¥99.99"
AuraLogic.utils.formatPrice(10.50, 'USD');  // "$10.50"
AuraLogic.utils.formatPrice(100, 'EUR');    // "€100.00"
AuraLogic.utils.formatPrice(1000, 'JPY');   // "¥1000.00"
AuraLogic.utils.formatPrice(50, 'GBP');     // "£50.00"
AuraLogic.utils.formatPrice(100, 'BTC');    // "BTC 100.00"
```

#### utils.formatDate(date, format?)

格式化日期。

```javascript
// 使用默认格式 (2006-01-02 15:04:05)
AuraLogic.utils.formatDate('2024-01-01T12:00:00Z');
// "2024-01-01 12:00:00"

// 使用自定义格式 (Go time format)
AuraLogic.utils.formatDate('2024-01-01T12:00:00Z', '2006/01/02');
// "2024/01/01"

// 支持 Unix 时间戳
AuraLogic.utils.formatDate(1704110400, '2006-01-02');
// "2024-01-01"
```

#### utils.generateId()

生成唯一 ID (UUID v4)。

```javascript
const id = AuraLogic.utils.generateId();
// "f47ac10b-58cc-4372-a567-0e02b2c3d479"
```

#### utils.md5(data)

计算 MD5 哈希值。

```javascript
const hash = AuraLogic.utils.md5('hello world');
// "5eb63bbbe01eeed093cb22bb8f5acdc3"
```

#### utils.base64Encode(data)

Base64 编码。

```javascript
const encoded = AuraLogic.utils.base64Encode('hello');
// "aGVsbG8="
```

#### utils.base64Decode(data)

Base64 解码。

```javascript
const decoded = AuraLogic.utils.base64Decode('aGVsbG8=');
// "hello"
```

#### utils.jsonEncode(data)

JSON 编码。

```javascript
const json = AuraLogic.utils.jsonEncode({ name: 'test', value: 123 });
// '{"name":"test","value":123}'
```

#### utils.jsonDecode(data)

JSON 解码。

```javascript
const obj = AuraLogic.utils.jsonDecode('{"name":"test"}');
// { name: "test" }
```

---

### AuraLogic.system - 系统信息

#### system.getTimestamp()

获取当前 Unix 时间戳（秒）。

```javascript
const timestamp = AuraLogic.system.getTimestamp();
// 1704110400
```

#### system.getPaymentMethodInfo()

获取当前付款方式信息。

```javascript
const info = AuraLogic.system.getPaymentMethodInfo();
// {
//   id: 1,
//   name: "USDT TRC20",
//   description: "使用USDT TRC20付款",
//   type: "custom",
//   icon: "Bitcoin"
// }
```

---

### AuraLogic.http - HTTP 请求

用于调用第三方支付接口或外部 API。

#### http.get(url, headers?)

发送 GET 请求。

```javascript
// 简单 GET 请求
const response = AuraLogic.http.get('https://api.example.com/status');

// 带自定义 headers
const response = AuraLogic.http.get('https://api.example.com/data', {
  'Authorization': 'Bearer token123',
  'Accept': 'application/json'
});
```

#### http.post(url, body?, headers?)

发送 POST 请求。

```javascript
// POST JSON 数据
const response = AuraLogic.http.post('https://api.example.com/order', {
  order_id: '12345',
  amount: 100.00
});

// POST 带自定义 headers
const response = AuraLogic.http.post(
  'https://api.example.com/callback',
  { status: 'paid' },
  {
    'Authorization': 'Bearer token123',
    'Content-Type': 'application/json'
  }
);

// POST 字符串数据
const response = AuraLogic.http.post(
  'https://api.example.com/webhook',
  'raw string data',
  { 'Content-Type': 'text/plain' }
);
```

#### http.request(options)

通用 HTTP 请求方法，支持所有 HTTP 方法。

```javascript
const response = AuraLogic.http.request({
  url: 'https://api.example.com/resource',
  method: 'PUT',  // GET, POST, PUT, DELETE, PATCH 等
  headers: {
    'Authorization': 'Bearer token123',
    'Content-Type': 'application/json'
  },
  body: {
    key: 'value'
  }
});
```

#### 响应格式

所有 HTTP 方法返回相同格式的响应对象：

```javascript
{
  status: 200,                    // HTTP 状态码
  statusText: "200 OK",           // 状态文本
  headers: {                      // 响应头
    "Content-Type": "application/json",
    "X-Request-Id": "abc123"
  },
  body: '{"success":true}',       // 原始响应体（字符串）
  data: { success: true },        // 解析后的 JSON（仅当响应为 JSON 时）
  error: "错误信息"                // 仅在请求失败时存在
}
```

#### 使用示例：调用第三方支付 API

```javascript
function onCheckPaymentStatus(order, config) {
  const apiKey = config.api_key;
  const merchantId = config.merchant_id;

  // 调用第三方支付查询接口
  const response = AuraLogic.http.post(
    'https://payment-api.example.com/query',
    {
      merchant_id: merchantId,
      order_no: order.order_no,
      timestamp: AuraLogic.system.getTimestamp()
    },
    {
      'Authorization': 'Bearer ' + apiKey,
      'Content-Type': 'application/json'
    }
  );

  // 检查请求是否成功
  if (response.error) {
    return {
      paid: false,
      message: '查询失败: ' + response.error
    };
  }

  if (response.status !== 200) {
    return {
      paid: false,
      message: '接口返回错误: ' + response.statusText
    };
  }

  // 解析响应数据
  const data = response.data;
  if (data && data.status === 'paid') {
    return {
      paid: true,
      transaction_id: data.transaction_id,
      message: '付款成功',
      data: {
        paid_amount: data.amount,
        paid_at: data.paid_time
      }
    };
  }

  return {
    paid: false,
    message: data ? data.message : '等待付款'
  };
}
```

#### 注意事项

- **超时限制**: HTTP 请求超时为 30 秒
- **响应大小限制**: 响应体最大 10MB
- **仅支持 HTTP/HTTPS**: URL 必须以 `http://` 或 `https://` 开头
- **JSON 自动处理**: 对象/数组类型的 body 会自动序列化为 JSON
- **默认 User-Agent**: `AuraLogic-PaymentScript/1.0`

---

## 完整示例

### USDT TRC20 付款方式

```javascript
function onGeneratePaymentCard(order, config) {
  const walletAddress = config.wallet_address || 'TRc...';
  const amount = order.total_amount;
  const currency = order.currency;

  // 计算 USDT 金额（示例汇率）
  let usdtAmount = amount;
  if (currency === 'CNY') {
    usdtAmount = (amount / 7.2).toFixed(2);
  } else if (currency !== 'USD') {
    usdtAmount = amount.toFixed(2);
  }

  return {
    html: `
      <div class="space-y-4">
        <div class="p-4 bg-muted rounded-lg">
          <p class="text-sm text-muted-foreground mb-2">钱包地址 (TRC20)</p>
          <code class="text-sm break-all">${walletAddress}</code>
        </div>
        <div class="flex justify-between items-center">
          <span>转账金额</span>
          <span class="text-xl font-bold text-primary">${usdtAmount} USDT</span>
        </div>
        <div class="text-sm text-muted-foreground">
          <p>1. 请使用 TRC20 网络转账</p>
          <p>2. 转账时请在备注中填写订单号: ${order.order_no}</p>
          <p>3. 转账完成后系统将自动确认（约1-5分钟）</p>
        </div>
      </div>
    `,
    title: 'USDT TRC20 付款',
    ttl: 60,  // 钱包地址是固定的，缓存60秒减少重复生成
    data: {
      wallet_address: walletAddress,
      usdt_amount: usdtAmount
    }
  };
}

function onCheckPaymentStatus(order, config) {
  // 加密货币付款通常需要人工确认或接入区块链API
  return {
    paid: false,
    message: '加密货币付款需要人工确认'
  };
}

function onRefund(order, config) {
  // 加密货币无法自动退款，提示管理员手动处理
  return {
    success: true,
    message: '请手动将 USDT 退回用户钱包地址',
    data: {
      amount: order.total_amount,
      wallet_address: config.wallet_address
    }
  };
}
```

### 银行转账付款方式

```javascript
function onGeneratePaymentCard(order, config) {
  const bankName = config.bank_name || '未设置银行';
  const accountName = config.account_name || '未设置户名';
  const accountNumber = config.account_number || '未设置账号';
  const amount = AuraLogic.utils.formatPrice(order.total_amount, order.currency);

  return {
    html: `
      <div class="space-y-3">
        <div class="grid grid-cols-2 gap-2 text-sm">
          <span class="text-muted-foreground">银行名称</span>
          <span class="font-medium">${bankName}</span>
          <span class="text-muted-foreground">账户名</span>
          <span class="font-medium">${accountName}</span>
          <span class="text-muted-foreground">银行账号</span>
          <span class="font-medium font-mono">${accountNumber}</span>
          <span class="text-muted-foreground">转账金额</span>
          <span class="font-bold text-primary">${amount}</span>
          <span class="text-muted-foreground">转账备注</span>
          <span class="font-mono">${order.order_no}</span>
        </div>
        <p class="text-xs text-muted-foreground mt-4">
          请务必在转账备注中填写订单号，以便我们快速确认您的付款。
        </p>
      </div>
    `,
    title: '银行转账',
    ttl: 3600  // 银行账号等信息是静态的，缓存1小时
  };
}

function onCheckPaymentStatus(order, config) {
  return {
    paid: false,
    message: '银行转账需要人工确认'
  };
}

function onRefund(order, config) {
  // 银行转账需要人工退款
  const amount = AuraLogic.utils.formatPrice(order.total_amount, order.currency);
  return {
    success: true,
    message: '请通过银行将 ' + amount + ' 退回用户账户'
  };
}
```

---

## 主题适配

JS 生成的 HTML 会被渲染在支持亮色/暗色主题切换的前端页面中。为了确保付款卡片在两种主题下都能正常显示，请遵循以下指南：

### 使用 Tailwind CSS 主题感知类

推荐使用 Tailwind CSS 的主题感知类，这些类会自动适配亮色和暗色主题：

| 用途 | 推荐类名 | 避免使用 |
|------|---------|---------|
| 背景色 | `bg-muted`, `bg-background` | `bg-gray-100`, `bg-white` |
| 文字色 | `text-muted-foreground`, `text-foreground` | `text-gray-500`, `text-black` |
| 主色调 | `text-primary`, `bg-primary` | 硬编码颜色 |
| 边框色 | `border-border` | `border-gray-200` |
| 代码块 | `bg-muted` | `bg-gray-100` |

### 示例

```javascript
function onGeneratePaymentCard(order, config) {
  return {
    html: `
      <div class="space-y-4">
        <!-- 使用主题感知类 -->
        <div class="p-4 bg-muted rounded-lg">
          <p class="text-sm text-muted-foreground mb-2">钱包地址</p>
          <code class="text-sm break-all">${config.wallet_address}</code>
        </div>
        <div class="flex justify-between items-center">
          <span class="text-foreground">转账金额</span>
          <span class="text-xl font-bold text-primary">${order.total_amount} USDT</span>
        </div>
        <p class="text-sm text-muted-foreground">
          请使用 TRC20 网络转账
        </p>
      </div>
    `
  };
}
```

### 暗色模式特定样式

如果需要为暗色模式设置不同的样式，可以使用 Tailwind 的 `dark:` 前缀：

```html
<div class="bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100">
  内容
</div>
```

### CSS 变量

也可以使用系统提供的 CSS 变量：

```html
<div style="background: hsl(var(--muted)); color: hsl(var(--muted-foreground));">
  内容
</div>
```

可用的 CSS 变量包括：
- `--background`, `--foreground` - 主背景和前景色
- `--muted`, `--muted-foreground` - 柔和背景和文字色
- `--primary`, `--primary-foreground` - 主色调
- `--secondary`, `--secondary-foreground` - 次要色
- `--border` - 边框色

---

## 多语言支持

系统支持中英文双语。付款卡片容器会自动设置 `data-locale` 属性（值为 `zh` 或 `en`），你可以利用 CSS 类来实现多语言内容切换。

### 使用语言切换类

使用 `lang-zh` 和 `lang-en` 类包裹不同语言的内容，系统会根据用户语言偏好自动显示对应内容：

```html
<span class="lang-zh">中文内容</span>
<span class="lang-en">English content</span>
```

对于块级元素，使用 `lang-zh-block` 和 `lang-en-block`：

```html
<div class="lang-zh-block">中文段落内容</div>
<div class="lang-en-block">English paragraph content</div>
```

### 完整示例

```javascript
function onGeneratePaymentCard(order, config) {
  return {
    html: `
      <div class="space-y-4">
        <div class="p-4 bg-muted rounded-lg">
          <p class="text-sm font-medium">
            <span class="lang-zh">收款地址</span>
            <span class="lang-en">Wallet Address</span>
          </p>
          <code class="text-sm break-all">${config.wallet_address}</code>
        </div>
        <div class="flex justify-between items-center">
          <span>
            <span class="lang-zh">转账金额</span>
            <span class="lang-en">Amount</span>
          </span>
          <span class="text-xl font-bold text-primary">${order.total_amount} USDT</span>
        </div>
        <p class="text-sm text-muted-foreground">
          <span class="lang-zh">请使用 TRC20 网络转账</span>
          <span class="lang-en">Please use TRC20 network</span>
        </p>
      </div>
    `
  };
}
```

### 注意事项

- `lang-zh` 和 `lang-en` 默认为 `display: inline`，适用于行内元素
- `lang-zh-block` 和 `lang-en-block` 为 `display: block`，适用于块级元素
- 语言切换由前端 CSS 控制，无需在 JS 脚本中处理语言逻辑

---

## 脚本执行说明

1. **超时限制**: 脚本执行时间限制为 5 秒（付款卡片）或 10 秒（状态检查）
2. **沙箱环境**: 脚本在隔离的 goja VM 中执行，无法访问文件系统或网络（但支持通过 `AuraLogic.http` 发起 HTTP 请求）
3. **错误处理**: 脚本错误会被捕获并显示给管理员，不会影响系统稳定性
4. **数据持久化**: 使用 `AuraLogic.storage` 进行数据持久化，数据按付款方式 ID 隔离

## 配置 JSON 示例

在付款方式设置中，可以配置如下 JSON：

```json
{
  "wallet_address": "TRc1234567890abcdef",
  "bank_name": "中国工商银行",
  "account_name": "张三",
  "account_number": "6222021234567890123",
  "qr_code_url": "https://example.com/qrcode.png"
}
```

配置项可通过 `config` 参数或 `AuraLogic.config.get()` 访问。
