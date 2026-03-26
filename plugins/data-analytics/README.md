# 数据分析插件示例

这是一个独立于后端主模块的 gRPC 插件示例，用于演示 AuraLogic 的插件协议接入方式。

## 功能

- `HealthCheck` 健康检查
- `Execute` 同步执行
- `ExecuteStream` 流式执行
- `hook.execute` 业务钩子示例（订单前后钩子 + 前端插槽扩展）

## 运行

在项目根目录执行：

```bash
cd plugins/data-analytics

# 如需重新生成 pb 代码
powershell -ExecutionPolicy Bypass -File .\proto\generate.ps1

go run .
```

默认监听 `:50051`。

## 在 AuraLogic 后台注册插件

```json
{
  "name": "data-analytics",
  "display_name": "数据分析插件",
  "type": "data_analysis",
  "address": "localhost:50051",
  "config": "{\"hooks\":[\"order.create.before\",\"order.create.after\",\"frontend.slot.render\"]}",
  "enabled": true
}
```

## 调用示例

```bash
# 测试健康状态
curl -X POST http://localhost:8080/api/admin/plugins/1/test

# 执行同步动作
curl -X POST http://localhost:8080/api/admin/plugins/1/execute \
  -H "Content-Type: application/json" \
  -d '{"action":"analyze_orders","params":{"period":"7d"}}'

# 执行业务 Hook
curl -X POST http://localhost:8080/api/admin/plugins/1/execute \
  -H "Content-Type: application/json" \
  -d '{"action":"hook.execute","params":{"hook":"order.create.before","payload":"{\"promo_code\":\"BLOCKME\"}"}}'
```

## 前端插槽验证

```bash
curl "http://localhost:8080/api/config/plugin-extensions?path=/cart&slot=user.cart.top"
```
