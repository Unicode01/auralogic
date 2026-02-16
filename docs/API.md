# AuraLogic API Documentation

## Overview

Base URL: `http://localhost:8080`

All responses follow a unified format:

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

Error responses:

```json
{
  "code": 40100,
  "message": "Unauthorized"
}
```

## Authentication

### JWT Token

Most endpoints require a JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Token is obtained via the login endpoint and contains `user_id`, `email`, and `role` claims.

### API Key

Admin API (`/api/admin`) supports dual authentication: JWT Token or API Key.

API Key authentication uses request headers:

```
X-API-Key: <api_key>
X-API-Secret: <api_secret>
```

API Key access is controlled by scopes (permissions). Each API key can be granted specific permissions (e.g., `order.view`, `order.edit`).

---

## Permission System

### Roles

| Role | Description |
|------|-------------|
| `user` | Regular user |
| `admin` | Administrator |
| `super_admin` | Super administrator (all permissions except special ones) |

### Permissions

| Permission | Description |
|------------|-------------|
| `order.view` | View orders |
| `order.view_privacy` | View order privacy info (**special**, requires explicit grant even for super admin) |
| `order.edit` | Edit orders |
| `order.delete` | Delete orders |
| `order.status_update` | Update order status |
| `order.assign_tracking` | Assign tracking numbers |
| `product.view` | View products |
| `product.edit` | Edit products |
| `product.delete` | Delete products |
| `serial.view` | View serial numbers |
| `serial.manage` | Manage serial numbers |
| `user.view` | View users |
| `user.edit` | Edit users |
| `admin.create` | Create admins |
| `admin.edit` | Edit admins |
| `admin.delete` | Delete admins |
| `admin.permission` | Manage permissions |
| `system.config` | System configuration |
| `system.logs` | View system logs |
| `api.manage` | Manage API keys |
| `ticket.view` | View tickets |
| `ticket.reply` | Reply to tickets |
| `ticket.status_update` | Update ticket status |
| `knowledge.view` | View knowledge base |
| `knowledge.edit` | Edit knowledge base |
| `announcement.view` | View announcements |
| `announcement.edit` | Edit announcements |

### Middleware Layers

1. **Global**: Recovery, Logger, CORS, SecurityHeaders
2. **APIKeyMiddleware**: Validates API key + secret (external API)
3. **AuthMiddleware**: Validates JWT token
4. **RequireAdmin**: Requires `admin` or `super_admin` role
5. **RequireSuperAdmin**: Requires `super_admin` role
6. **RequirePermission**: Checks specific permission
7. **RequireTicketEnabled**: Checks if ticket system is enabled
8. **RequireSerialEnabled**: Checks if serial verification is enabled

---

## Public Endpoints (No Auth Required)

### Config

#### GET /api/config/public

Get public configuration.

**Response:**

```json
{
  "currency": "CNY",
  "app_name": "AuraLogic",
  "default_theme": "system",
  "customization": {
    "primary_color": "217.2 91% 60%",
    "logo_url": "",
    "favicon_url": ""
  },
  "ticket": {
    "enabled": true,
    "categories": ["订单问题", "支付问题", "售后服务"],
    "attachment": {
      "enable_image": true,
      "enable_voice": true,
      "max_image_size": 5242880,
      "max_voice_size": 10485760,
      "max_voice_duration": 60,
      "allowed_image_types": [".jpg", ".jpeg", ".png", ".gif", ".webp"],
      "retention_days": 30
    }
  }
}
```

#### GET /api/config/page-inject

Get page-specific CSS/JS injection.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `path` | string | Current page path (e.g. `/products`). Falls back to `Referer` header if empty. |

**Response:**

```json
{
  "css": "/* matched CSS rules */",
  "js": "// matched JS rules"
}
```

**Headers:** `Cache-Control: public, max-age=300`

### Serial

> All serial endpoints require the serial verification system to be enabled (`RequireSerialEnabled`).

#### POST /api/serial/verify

Verify a serial number.

**Request:**

```json
{
  "serial_number": "XXXXX-XXXXX"
}
```

#### GET /api/serial/:serial_number

Get serial information by serial number.

### User Auth

#### POST /api/user/auth/login

User login (also used for admin login).

**Request:**

```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response:**

```json
{
  "token": "eyJhbGci...",
  "token_type": "Bearer",
  "user": {
    "user_id": 1,
    "uuid": "abc-123",
    "email": "user@example.com",
    "name": "User",
    "role": "user",
    "avatar": "",
    "locale": "zh",
    "permissions": []
  }
}
```

> Admin/super_admin users will also receive `permissions` array in the response.

#### POST /api/user/auth/register

Register a new user account.

**Request:**

```json
{
  "email": "user@example.com",
  "password": "password123",
  "name": "User Name"
}
```

#### GET /api/user/auth/captcha

Get captcha for login/register forms.

#### GET /api/user/auth/verify-email

Verify user email address.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `token` | string | Email verification token |

#### POST /api/user/auth/resend-verification

Resend email verification link.

**Request:**

```json
{
  "email": "user@example.com"
}

### Products (Public)

#### GET /api/user/products/featured

Get featured products list (no auth required).

#### GET /api/user/products/recommended

Get recommended products list (no auth required).

### Static & Health

#### GET /uploads/*

Static file serving for uploaded images.

#### GET /health

Health check endpoint. Returns `{"status": "ok"}`.

---

## User Endpoints (Auth Required)

All endpoints below require `Authorization: Bearer <token>`.

### Auth

#### POST /api/user/auth/logout

Logout and invalidate current token.

#### GET /api/user/auth/me

Get current user information.

#### POST /api/user/auth/change-password

Change password.

**Request:**

```json
{
  "old_password": "oldpass",
  "new_password": "newpass"
}
```

#### PUT /api/user/auth/preferences

Update user preferences.

**Request:**

```json
{
  "locale": "en",
  "country": "US"
}
```

### Products

#### GET /api/user/products

List products with pagination.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `limit` | int | Items per page |
| `category` | string | Filter by category |
| `search` | string | Search keyword |

#### GET /api/user/products/categories

Get product categories.

#### GET /api/user/products/:id

Get product detail by ID.

#### GET /api/user/products/:id/available-stock

Get available stock for a product.

**Query Parameters:** Attribute key-value pairs for variant filtering.

### Orders

#### POST /api/user/orders

Create a new order.

**Request:**

```json
{
  "items": [
    {
      "sku": "PROD-001",
      "name": "Product Name",
      "quantity": 1,
      "image_url": "https://...",
      "attributes": {
        "color": "red"
      }
    }
  ]
}
```

#### GET /api/user/orders

List user's orders.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `limit` | int | Items per page |
| `status` | string | Filter by status |

#### GET /api/user/orders/:order_no

Get order details by order number.

#### GET /api/user/orders/:order_no/form-token

Get or refresh form token for an order.

#### GET /api/user/orders/:order_no/virtual-products

Get virtual products (card keys) for an order.

#### POST /api/user/orders/:order_no/complete

Mark order as completed (user confirmation).

#### POST /api/user/orders/:order_no/complete

Mark order as completed.

### Payment

#### GET /api/user/payment-methods

List available payment methods.

#### GET /api/user/orders/:order_no/payment-info

Get payment info for an order.

#### GET /api/user/orders/:order_no/payment-card

Get payment card for an order.

#### POST /api/user/orders/:order_no/select-payment

Select payment method for an order.

**Request:**

```json
{
  "payment_method_id": 1
}
```

### Cart

#### GET /api/user/cart

Get shopping cart.

#### GET /api/user/cart/count

Get cart item count.

#### POST /api/user/cart/items

Add item to cart.

**Request:**

```json
{
  "product_id": 1,
  "quantity": 1,
  "attributes": {
    "color": "red"
  }
}
```

#### PUT /api/user/cart/items/:id

Update cart item quantity.

**Request:**

```json
{
  "quantity": 2
}
```

#### DELETE /api/user/cart/items/:id

Remove item from cart.

#### DELETE /api/user/cart

Clear entire cart.

### Tickets

> All ticket endpoints additionally require the ticket system to be enabled (`RequireTicketEnabled`).

#### POST /api/user/tickets

Create a new ticket.

**Request:**

```json
{
  "subject": "Order issue",
  "content": "Description...",
  "category": "订单问题",
  "priority": "normal",
  "order_id": 1
}
```

#### GET /api/user/tickets

List user's tickets.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `limit` | int | Items per page |
| `status` | string | Filter by status |
| `search` | string | Search keyword |

#### GET /api/user/tickets/:id

Get ticket details.

#### GET /api/user/tickets/:id/messages

Get ticket messages.

#### POST /api/user/tickets/:id/messages

Send a message to a ticket.

**Request:**

```json
{
  "content": "Message text",
  "type": "text"
}
```

#### PUT /api/user/tickets/:id/status

Update ticket status.

**Request:**

```json
{
  "status": "closed"
}
```

#### POST /api/user/tickets/:id/share-order

Share an order with support in a ticket.

#### GET /api/user/tickets/:id/shared-orders

Get orders shared in a ticket.

#### DELETE /api/user/tickets/:id/shared-orders/:orderId

Revoke order access from a ticket.

#### POST /api/user/tickets/:id/upload

Upload a file attachment to a ticket.

**Content-Type:** `multipart/form-data`

### Form (Auth Required)

#### GET /api/form/shipping

Get shipping form data.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `token` | string | Form token |

#### POST /api/form/shipping

Submit shipping form.

#### GET /api/form/countries

Get country list for shipping form.

### Promo Codes

#### POST /api/user/promo-codes/validate

Validate a promo code.

**Request:**

```json
{
  "code": "SAVE10",
  "product_ids": [1, 2],
  "amount": 100.00
}
```

### Knowledge Base

#### GET /api/user/knowledge/categories

Get knowledge base category tree.

#### GET /api/user/knowledge/articles

List knowledge base articles.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `limit` | int | Items per page |
| `category_id` | int | Filter by category |
| `search` | string | Search keyword |

#### GET /api/user/knowledge/articles/:id

Get knowledge base article detail.

### Announcements

#### GET /api/user/announcements

List announcements.

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `limit` | int | Items per page |

#### GET /api/user/announcements/unread-mandatory

Get unread mandatory announcements that the user must read.

#### GET /api/user/announcements/:id

Get announcement detail.

#### POST /api/user/announcements/:id/read

Mark announcement as read.

---

## Admin Endpoints

All admin endpoints require authentication (JWT Token or API Key) + appropriate permissions.

API Key authentication is supported on all admin endpoints. Access is controlled by the API key's scopes.

### Orders

#### POST /api/admin/orders/draft

Create a draft order. Requires `order.edit` permission.

**Request Headers (API Key):**

```
X-API-Key: ak_live_xxx
X-API-Secret: sk_live_xxx
```

**Request Body:**

```json
{
  "external_user_id": "test_user_001",
  "user_email": "test@example.com",
  "user_name": "Test User",
  "items": [{
    "sku": "PROD-001",
    "name": "iPhone 15 Pro",
    "quantity": 1,
    "attributes": {"color": "Black", "storage": "256GB"}
  }],
  "external_order_id": "EXT-123456",
  "platform": "test_platform",
  "remark": "Test Order"
}
```

#### POST /api/admin/orders

Create an order for a user. **Permission:** `order.edit`
```

### Dashboard (Super Admin Only)

**Middleware:** `RequireSuperAdmin()`

#### GET /api/admin/dashboard/statistics

Get dashboard statistics.

#### GET /api/admin/dashboard/activities

Get recent activities.

### Order Management

#### GET /api/admin/orders

List all orders. **Permission:** `order.view`

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `limit` | int | Items per page |
| `status` | string | Filter by status |
| `search` | string | Search by order number or email |
| `product_search` | string | Search by product name |
| `user_id` | int | Filter by user ID |
| `country` | string | Filter by country |
| `start_date` | string | Start date filter |
| `end_date` | string | End date filter |

#### GET /api/admin/orders/countries

Get distinct order countries. **Permission:** `order.view`

#### GET /api/admin/orders/:id

Get order details. **Permission:** `order.view`

#### POST /api/admin/orders/:id/assign-shipping

Assign tracking number. **Permission:** `order.assign_tracking`

#### PUT /api/admin/orders/:id/shipping-info

Update shipping info. **Permission:** `order.edit`

#### POST /api/admin/orders/:id/request-resubmit

Request order resubmission. **Permission:** `order.edit`

#### POST /api/admin/orders/:id/complete

Complete order. **Permission:** `order.status_update`

#### POST /api/admin/orders/:id/cancel

Cancel order. **Permission:** `order.status_update`

#### POST /api/admin/orders/:id/mark-paid

Mark order as paid. **Permission:** `order.status_update`

#### POST /api/admin/orders/:id/deliver-virtual

Deliver virtual stock to order. **Permission:** `order.status_update`

#### PUT /api/admin/orders/:id/price

Update order price. **Permission:** `order.edit`

#### DELETE /api/admin/orders/:id

Delete order. **Permission:** `order.delete`

#### POST /api/admin/orders/batch/complete-shipped

Batch complete all shipped orders. **Permission:** `order.status_update`

#### POST /api/admin/orders/batch/update

Batch update orders (status change). **Permission:** `order.status_update`

**Request:**

```json
{
  "order_ids": [1, 2, 3],
  "action": "complete"
}
```

#### GET /api/admin/orders/export

Export orders to Excel. **Permission:** `order.view`

#### POST /api/admin/orders/import

Import orders from Excel. **Permission:** `order.assign_tracking`

**Content-Type:** `multipart/form-data`

#### GET /api/admin/orders/import-template

Download import template. **Permission:** `order.view`

### User Management

#### GET /api/admin/users

List users. **Permission:** `user.view`

#### POST /api/admin/users

Create user. **Permission:** `user.edit`

> Creating admin/super_admin roles requires super admin.

#### GET /api/admin/users/:id

Get user details. **Permission:** `user.view`

#### PUT /api/admin/users/:id

Update user. **Permission:** `user.edit`

> Modifying roles requires super admin.

#### DELETE /api/admin/users/:id

Delete user. **Permission:** `user.edit`

> Cannot delete self. Cannot delete admins (use admin management).

#### GET /api/admin/users/:id/orders

Get user's orders. **Permission:** `user.view`

### Product Management

#### GET /api/admin/products

List products. **Permission:** `product.view`

#### POST /api/admin/products

Create product. **Permission:** `product.edit`

#### GET /api/admin/products/categories

Get product categories. **Permission:** `product.view`

#### GET /api/admin/products/:id

Get product details. **Permission:** `product.view`

#### PUT /api/admin/products/:id

Update product. **Permission:** `product.edit`

#### DELETE /api/admin/products/:id

Delete product. **Permission:** `product.delete`

#### PUT /api/admin/products/:id/status

Update product status. **Permission:** `product.edit`

#### PUT /api/admin/products/:id/stock

Update stock. **Permission:** `product.edit`

#### POST /api/admin/products/:id/toggle-featured

Toggle featured status. **Permission:** `product.edit`

#### PUT /api/admin/products/:id/inventory-mode

Update inventory mode. **Permission:** `product.edit`

### Product Inventory Bindings

#### GET /api/admin/products/:id/inventory-bindings

Get product inventory bindings. **Permission:** `product.view`

#### POST /api/admin/products/:id/inventory-bindings

Create binding. **Permission:** `product.edit`

#### POST /api/admin/products/:id/inventory-bindings/batch

Batch create bindings. **Permission:** `product.edit`

#### PUT /api/admin/products/:id/inventory-bindings/:bindingId

Update binding. **Permission:** `product.edit`

#### DELETE /api/admin/products/:id/inventory-bindings/:bindingId

Delete binding. **Permission:** `product.edit`

#### DELETE /api/admin/products/:id/inventory-bindings

Delete all product bindings. **Permission:** `product.edit`

#### PUT /api/admin/products/:id/inventory-bindings/replace

Replace all product bindings. **Permission:** `product.edit`

### Product Virtual Inventory Bindings

#### GET /api/admin/products/:id/virtual-inventory-bindings

Get virtual inventory bindings. **Permission:** `product.view`

#### POST /api/admin/products/:id/virtual-inventory-bindings

Create virtual inventory binding. **Permission:** `product.edit`

#### PUT /api/admin/products/:id/virtual-inventory-bindings

Save variant bindings. **Permission:** `product.edit`

#### PUT /api/admin/products/:id/virtual-inventory-bindings/:bindingId

Update virtual inventory binding. **Permission:** `product.edit`

#### DELETE /api/admin/products/:id/virtual-inventory-bindings/:bindingId

Delete virtual inventory binding. **Permission:** `product.edit`

### Inventory Management

#### GET /api/admin/inventories

List inventories. **Permission:** `product.view`

#### POST /api/admin/inventories

Create inventory. **Permission:** `product.edit`

#### GET /api/admin/inventories/low-stock

Get low stock list. **Permission:** `product.view`

#### GET /api/admin/inventories/:id

Get inventory details. **Permission:** `product.view`

#### PUT /api/admin/inventories/:id

Update inventory. **Permission:** `product.edit`

#### POST /api/admin/inventories/:id/adjust

Adjust stock. **Permission:** `product.edit`

#### DELETE /api/admin/inventories/:id

Delete inventory. **Permission:** `product.delete`

#### GET /api/admin/inventories/:id/products

Get inventory's product bindings. **Permission:** `product.view`

### Virtual Inventory Management

#### GET /api/admin/virtual-inventories

List virtual inventories. **Permission:** `product.view`

#### POST /api/admin/virtual-inventories

Create virtual inventory. **Permission:** `product.edit`

#### GET /api/admin/virtual-inventories/:id

Get virtual inventory. **Permission:** `product.view`

#### PUT /api/admin/virtual-inventories/:id

Update virtual inventory. **Permission:** `product.edit`

#### DELETE /api/admin/virtual-inventories/:id

Delete virtual inventory. **Permission:** `product.delete`

#### POST /api/admin/virtual-inventories/:id/import

Import stock items. **Permission:** `product.edit`

**Content-Type:** `multipart/form-data` or JSON

#### POST /api/admin/virtual-inventories/:id/stocks

Create stock item manually. **Permission:** `product.edit`

#### GET /api/admin/virtual-inventories/:id/stocks

Get stock list. **Permission:** `product.view`

#### GET /api/admin/virtual-inventories/:id/stats

Get stock statistics. **Permission:** `product.view`

#### DELETE /api/admin/virtual-inventories/:id/stocks/:stock_id

Delete stock item. **Permission:** `product.edit`

#### POST /api/admin/virtual-inventories/:id/stocks/:stock_id/reserve

Reserve stock item. **Permission:** `product.edit`

#### POST /api/admin/virtual-inventories/:id/stocks/:stock_id/release

Release stock item. **Permission:** `product.edit`

#### DELETE /api/admin/virtual-inventories/batch

Batch delete virtual inventories. **Permission:** `product.edit`

#### GET /api/admin/virtual-inventories/:id/products

Get virtual inventory's product bindings. **Permission:** `product.view`

### Virtual Products (Legacy)

#### GET /api/admin/virtual-products/:id/stocks

Get stock list for product. **Permission:** `product.view`

#### GET /api/admin/virtual-products/:id/stats

Get stock stats for product. **Permission:** `product.view`

#### POST /api/admin/virtual-products/:id/import

Import stock for product. **Permission:** `product.edit`

#### DELETE /api/admin/virtual-products/stocks/:id

Delete stock by ID. **Permission:** `product.edit`

#### DELETE /api/admin/virtual-products/batch

Batch delete. **Permission:** `product.edit`

### Serial Number Management

#### GET /api/admin/serials

List serial numbers. **Permission:** `serial.view`

#### GET /api/admin/serials/statistics

Get serial statistics. **Permission:** `serial.view`

#### GET /api/admin/serials/:serial_number

Get serial by number. **Permission:** `serial.view`

#### GET /api/admin/serials/order/:order_id

Get serials by order. **Permission:** `serial.view`

#### GET /api/admin/serials/product/:product_id

Get serials by product. **Permission:** `serial.view`

#### DELETE /api/admin/serials/:id

Delete serial. **Permission:** `serial.manage`

#### POST /api/admin/serials/batch-delete

Batch delete serials. **Permission:** `serial.manage`

### Payment Method Management

#### GET /api/admin/payment-methods

List payment methods. **Permission:** `system.config`

#### POST /api/admin/payment-methods

Create payment method. **Permission:** `system.config`

#### GET /api/admin/payment-methods/:id

Get payment method. **Permission:** `system.config`

#### PUT /api/admin/payment-methods/:id

Update payment method. **Permission:** `system.config`

#### DELETE /api/admin/payment-methods/:id

Delete payment method. **Permission:** `system.config`

#### POST /api/admin/payment-methods/:id/toggle

Toggle enabled status. **Permission:** `system.config`

#### POST /api/admin/payment-methods/reorder

Reorder payment methods. **Permission:** `system.config`

#### POST /api/admin/payment-methods/test-script

Test payment script. **Permission:** `system.config`

#### POST /api/admin/payment-methods/init-builtin

Initialize built-in payment methods. **Permission:** `system.config`

### Ticket Management

#### GET /api/admin/tickets

List tickets. **Permission:** `ticket.view`

#### GET /api/admin/tickets/stats

Get ticket statistics. **Permission:** `ticket.view`

#### GET /api/admin/tickets/:id

Get ticket details. **Permission:** `ticket.view`

#### GET /api/admin/tickets/:id/messages

Get ticket messages. **Permission:** `ticket.view`

#### POST /api/admin/tickets/:id/messages

Send message. **Permission:** `ticket.reply`

#### PUT /api/admin/tickets/:id

Update ticket (status, assignment). **Permission:** `ticket.status_update`

#### GET /api/admin/tickets/:id/shared-orders

Get shared orders in ticket. **Permission:** `ticket.view`

#### GET /api/admin/tickets/:id/shared-orders/:orderId

Get shared order details. **Permission:** `ticket.view`

#### POST /api/admin/tickets/:id/upload

Upload file to ticket. **Permission:** `ticket.reply`

### File Upload

#### POST /api/admin/upload/image

Upload image. **Permission:** `product.edit`

**Content-Type:** `multipart/form-data`

#### POST /api/admin/upload/image/delete

Delete image. **Permission:** `product.edit`

### Log Management

#### GET /api/admin/logs/operations

List operation logs. **Permission:** `system.logs`

#### GET /api/admin/logs/emails

List email logs. **Permission:** `system.logs`

#### GET /api/admin/logs/statistics

Get log statistics. **Permission:** `system.logs`

#### POST /api/admin/logs/emails/retry

Retry failed emails. **Permission:** `system.logs`

#### GET /api/admin/logs/inventories

List inventory logs. **Permission:** `system.logs`

#### GET /api/admin/logs/inventories/statistics

Get inventory log statistics. **Permission:** `system.logs`

### System Settings (Super Admin Only)

**Middleware:** `RequireSuperAdmin()` + `RequirePermission("system.config")`

#### GET /api/admin/settings

Get all system settings.

#### PUT /api/admin/settings

Update system settings.

**Request:** Partial update - only include sections to modify.

```json
{
  "app": {
    "name": "AuraLogic",
    "url": "https://example.com",
    "debug": false,
    "default_theme": "system"
  },
  "smtp": {
    "enabled": true,
    "host": "smtp.gmail.com",
    "port": 587,
    "user": "user@gmail.com",
    "password": "optional-new-password",
    "from_email": "noreply@example.com",
    "from_name": "AuraLogic"
  },
  "security": {
    "password_policy": {
      "min_length": 8,
      "require_uppercase": true,
      "require_lowercase": true,
      "require_number": true,
      "require_special": false
    },
    "login": {
      "allow_password_login": true
    },
    "cors": {
      "allowed_origins": ["http://localhost:3000"],
      "max_age": 86400
    }
  },
  "rate_limit": {
    "enabled": true,
    "api": 10000,
    "user_login": 100,
    "user_request": 600,
    "admin_request": 2000
  },
  "order": {
    "no_prefix": "ORD",
    "auto_cancel_hours": 72,
    "currency": "CNY"
  },
  "ticket": {
    "enabled": true,
    "categories": ["订单问题", "支付问题"],
    "template": "请描述您的问题...",
    "attachment": {
      "enable_image": true,
      "enable_voice": true,
      "max_image_size": 5242880,
      "max_voice_size": 10485760,
      "max_voice_duration": 60,
      "allowed_image_types": [".jpg", ".jpeg", ".png"],
      "retention_days": 30
    }
  },
  "customization": {
    "_submitted": true,
    "primary_color": "217.2 91% 60%",
    "logo_url": "",
    "favicon_url": "",
    "page_rules": [
      {
        "name": "全局样���",
        "pattern": ".*",
        "match_type": "regex",
        "css": "body { font-family: sans-serif; }",
        "js": "",
        "enabled": true
      }
    ]
  }
}
```

#### POST /api/admin/settings/smtp/test

Test SMTP configuration.

**Request:**

```json
{
  "host": "smtp.gmail.com",
  "port": 587,
  "user": "user@gmail.com",
  "password": "password",
  "to_email": "test@example.com"
}
```

#### GET /api/admin/settings/email-templates

List all email templates. **Permission:** `system.config`

#### GET /api/admin/settings/email-templates/:filename

Get email template content. **Permission:** `system.config`

#### PUT /api/admin/settings/email-templates/:filename

Update email template. **Permission:** `system.config`

**Request:**

```json
{
  "content": "<html>...</html>"
}
```

#### GET /api/admin/settings/landing-page

Get landing page HTML. **Permission:** `system.config`

#### PUT /api/admin/settings/landing-page

Update landing page HTML. **Permission:** `system.config`

**Request:**

```json
{
  "html_content": "<html>...</html>"
}
```

#### POST /api/admin/settings/landing-page/reset

Reset landing page to default. **Permission:** `system.config`

### Permission Management (Super Admin Only)

**Middleware:** `RequireSuperAdmin()`

#### GET /api/admin/permissions/all

List all available permissions.

#### GET /api/admin/permissions/users/:id

Get user's permissions.

#### PUT /api/admin/permissions/users/:id

Update user's permissions. **Permission:** `admin.permission`

**Request:**

```json
{
  "permissions": ["order.view", "order.edit", "product.view"]
}
```

### Admin Account Management (Super Admin Only)

**Middleware:** `RequireSuperAdmin()`

#### GET /api/admin/admins

List all admin accounts.

#### GET /api/admin/admins/:id

Get admin details.

#### POST /api/admin/admins

Create admin. **Permission:** `admin.create`

#### PUT /api/admin/admins/:id

Update admin. **Permission:** `admin.edit`

> Cannot modify own role/status.

#### DELETE /api/admin/admins/:id

Delete admin. **Permission:** `admin.delete`

> Cannot delete self.

### API Key Management

#### GET /api/admin/api-keys

List API keys. **Permission:** `api.manage`

#### POST /api/admin/api-keys

Create API key. **Permission:** `api.manage`

#### PUT /api/admin/api-keys/:id

Update API key. **Permission:** `api.manage`

#### DELETE /api/admin/api-keys/:id

Delete API key. **Permission:** `api.manage`

### Analytics (Super Admin Only)

**Middleware:** `RequireSuperAdmin()`

#### GET /api/admin/analytics/users

Get user analytics data.

#### GET /api/admin/analytics/orders

Get order analytics data.

#### GET /api/admin/analytics/revenue

Get revenue analytics data.

#### GET /api/admin/analytics/devices

Get device analytics data.

#### GET /api/admin/analytics/pageviews

Get page view analytics data.

### Promo Code Management

#### GET /api/admin/promo-codes

List promo codes. **Permission:** `product.view`

#### POST /api/admin/promo-codes

Create promo code. **Permission:** `product.edit`

**Request:**

```json
{
  "code": "SAVE10",
  "name": "10% Off",
  "discount_type": "percentage",
  "discount_value": 10,
  "max_discount": 50,
  "min_order_amount": 100,
  "total_quantity": 100,
  "product_ids": [1, 2],
  "status": "active",
  "expires_at": "2025-12-31T23:59:59Z"
}
```

#### GET /api/admin/promo-codes/:id

Get promo code details. **Permission:** `product.view`

#### PUT /api/admin/promo-codes/:id

Update promo code. **Permission:** `product.edit`

#### DELETE /api/admin/promo-codes/:id

Delete promo code. **Permission:** `product.delete`

### Knowledge Base Management

#### GET /api/admin/knowledge/categories

List knowledge base categories. **Permission:** `knowledge.view`

#### POST /api/admin/knowledge/categories

Create category. **Permission:** `knowledge.edit`

#### PUT /api/admin/knowledge/categories/:id

Update category. **Permission:** `knowledge.edit`

#### DELETE /api/admin/knowledge/categories/:id

Delete category. **Permission:** `knowledge.edit`

#### GET /api/admin/knowledge/articles

List articles. **Permission:** `knowledge.view`

#### POST /api/admin/knowledge/articles

Create article. **Permission:** `knowledge.edit`

#### GET /api/admin/knowledge/articles/:id

Get article detail. **Permission:** `knowledge.view`

#### PUT /api/admin/knowledge/articles/:id

Update article. **Permission:** `knowledge.edit`

#### DELETE /api/admin/knowledge/articles/:id

Delete article. **Permission:** `knowledge.edit`

### Announcement Management

#### GET /api/admin/announcements

List announcements. **Permission:** `announcement.view`

#### POST /api/admin/announcements

Create announcement. **Permission:** `announcement.edit`

**Request:**

```json
{
  "title": "System Maintenance",
  "content": "Scheduled maintenance...",
  "is_mandatory": false,
  "require_full_read": false
}
```

#### GET /api/admin/announcements/:id

Get announcement detail. **Permission:** `announcement.view`

#### PUT /api/admin/announcements/:id

Update announcement. **Permission:** `announcement.edit`

#### DELETE /api/admin/announcements/:id

Delete announcement. **Permission:** `announcement.edit`

---

## Endpoint Summary

| Category | Count | Auth |
|----------|-------|------|
| Public | 10 | None |
| User (Auth) | 38 | JWT Token |
| Admin | 150+ | JWT + Role + Permission |
| Super Admin Only | 20 | JWT + Super Admin |
| **Total** | **~218** | |
