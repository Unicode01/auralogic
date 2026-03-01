# 虚拟库存 JS 发货脚本文档

本文档基于当前代码实现，说明 `type=script` 的虚拟库存发货机制。

## 1. 结论先看

- 现在 **JS 库存默认不再创建 `reserved` 占位库存记录**。
- 下单时仅记录订单级绑定：`order.virtual_inventory_bindings`。
- 真正发货时执行脚本，**直接创建 `sold` 记录**到 `virtual_product_stocks`。
- 系统仍兼容旧订单（历史上可能存在 `reserved` 占位记录）。

## 2. 关键数据结构

## 2.1 `order.virtual_inventory_bindings`

- 类型：`map[int]uint`
- 含义：`订单项索引 -> virtual_inventory_id`
- 用途：标记哪些订单项是脚本库存，以及对应哪个库存池。

## 2.2 `virtual_product_stocks`

- 新流程下，脚本库存发货前不写记录。
- 发货成功后直接写入：
  - `status = sold`
  - `content`（脚本返回）
  - `remark`（可选）
  - `order_no / order_id / delivered_at / delivered_by`

## 3. 下单与待发货逻辑

## 3.1 下单分配

- 静态库存：照常从可用池分配并置为 `reserved`。
- 脚本库存：不建占位，仅返回 `scriptInventoryID`，由订单服务写入 `virtual_inventory_bindings`。

## 3.2 如何判断“还有待发货虚拟库存”

系统同时检查两类来源：

- 旧流程：`virtual_product_stocks` 中 `status=reserved` 的记录。
- 新流程：通过 `virtual_inventory_bindings` + `order.items.quantity` 计算应发数量，再减去已写入的 `sold` 数量。

即：  
`待发数量 = 订单项绑定数量合计 - 当前已 sold 数量`

## 3.3 自动发货判断

自动发货判断也是新旧两套一起算：

- 旧流程 `reserved` 数量中，属于 `auto_delivery=true` 商品的数量。
- 新流程 `pending` 数量中，属于 `auto_delivery=true` 商品的数量。

只有二者合并后“全部都可自动发货”才会自动发货。

## 4. 发货执行流程

发货入口：

- 自动：订单支付后触发（满足自动发货条件时）。
- 手动：`POST /api/admin/orders/:id/deliver-virtual`
- 混合订单：分配物流单号时会尝试把剩余虚拟库存一起发掉。

执行时分两段：

## 4.1 旧流程兼容段（有占位记录的历史订单）

- 查 `status=reserved` 且库存类型为 `script` 的记录。
- 按 `virtual_inventory_id` 分组执行脚本。
- 将脚本结果写回占位记录，再由后续流程转 `sold`。

## 4.2 新流程主路径（无占位）

- 从 `virtual_inventory_bindings` 计算每个脚本库存池的待发数量。
- 每个库存池执行一次脚本：`onDeliver(order, config)`，`quantity=该池待发数量`。
- 校验返回条目足够后，直接创建 `sold` 记录。

## 5. 脚本回调规范

必须定义全局函数：

```javascript
function onDeliver(order, config) {
  return {
    success: true,
    items: [
      { content: "KEY-001", remark: "optional" },
      { content: "KEY-002" }
    ],
    message: "optional"
  };
}
```

要求：

- `success` 必须为 `true`。
- `items` 必须是数组，且每项 `content` 非空。
- 生产发货时 `items.length` 不能小于应发数量。

## 6. 脚本可用 API

通过全局对象 `AuraLogic` 使用：

- `AuraLogic.order`
- `AuraLogic.utils`
- `AuraLogic.http`
- `AuraLogic.config`
- `AuraLogic.system`

与后台“脚本 API 参考”一致，`onDeliver(order, config)` 参数如下：

- `order`: `id/order_no/status/total_amount/currency/quantity/created_at`
- `config`: 来自 `script_config` 的 JSON 对象（解析失败则为空对象）

## 7. 网络与安全限制

- 仅允许 `http/https`
- 拒绝本地/内网地址（含 `localhost`、私网 IP 等）
- 最多 5 次重定向（每跳重新校验）
- HTTP 请求超时 30 秒
- 响应体读取上限 10MB
- 脚本 VM 执行超时 10 秒

## 8. 测试接口

- `POST /api/admin/virtual-inventories/test-script`
- 权限：`product.edit`
- `quantity <= 0` 会按 `1` 处理
- 使用模拟订单测试脚本执行与返回结构

## 9. 取消订单与释放说明

- 新流程下脚本库存无 `reserved` 记录，因此通常不存在“脚本占位回滚”。
- 旧流程历史订单若有脚本 `reserved` 记录，释放时会被删除（兼容逻辑仍保留）。
- 静态库存释放逻辑不变：`reserved -> available`。

## 10. 代码定位（便于核对）

- `backend/internal/service/virtual_inventory_service.go`
  - `AllocateStockForProductByAttributes`
  - `AllocateStockFromInventory`
  - `getScriptPendingItems`
  - `executeScriptDelivery`
  - `CanAutoDeliver`
  - `HasPendingVirtualStock`
- `backend/internal/service/order_service.go`
  - 创建订单时写入 `order.VirtualInventoryBindings`
  - `MarkAsPaid` / `DeliverVirtualStock` 发货触发

