package models

import (
	"time"

	"gorm.io/gorm"
)

// PaymentMethodType 付款方式类型
type PaymentMethodType string

const (
	PaymentMethodTypeBuiltin PaymentMethodType = "builtin" // 内置付款方式
	PaymentMethodTypeCustom  PaymentMethodType = "custom"  // 自定义JS付款方式
)

// PaymentMethod 付款方式模型
type PaymentMethod struct {
	ID           uint              `gorm:"primaryKey" json:"id"`
	Name         string            `gorm:"size:100;not null" json:"name"`                     // 付款方式名称
	Description  string            `gorm:"size:500" json:"description"`                       // 描述
	Type         PaymentMethodType `gorm:"size:20;not null;default:builtin" json:"type"`      // 类型: builtin/custom
	Enabled      bool              `gorm:"default:true" json:"enabled"`                       // 是否启用
	Script       string            `gorm:"type:text" json:"script"`                           // JS脚本代码(custom类型)
	Config       string            `gorm:"type:text" json:"config"`                           // 配置JSON(如API密钥等)
	Icon         string            `gorm:"size:100" json:"icon"`                              // 图标(lucide图标名或URL)
	SortOrder    int               `gorm:"default:0" json:"sort_order"`                       // 排序顺序
	PollInterval int               `gorm:"default:30" json:"poll_interval"`                   // 轮询检查间隔(秒)，默认30秒
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// TableName 指定表名
func (PaymentMethod) TableName() string {
	return "payment_methods"
}

// BeforeCreate 创建前钩子
func (pm *PaymentMethod) BeforeCreate(tx *gorm.DB) error {
	pm.CreatedAt = time.Now()
	pm.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate 更新前钩子
func (pm *PaymentMethod) BeforeUpdate(tx *gorm.DB) error {
	pm.UpdatedAt = time.Now()
	return nil
}

// OrderPaymentMethod 订单选择的付款方式
type OrderPaymentMethod struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	OrderID          uint      `gorm:"not null;uniqueIndex" json:"order_id"`        // 订单ID
	PaymentMethodID  uint      `gorm:"not null;index" json:"payment_method_id"`     // 付款方式ID
	PaymentData      string    `gorm:"type:text" json:"payment_data"`               // 支付相关数据(如交易号等)
	PaymentCardCache string    `gorm:"type:text" json:"payment_card_cache"`         // 付款卡片缓存(JSON格式)
	CacheExpiresAt   *time.Time `gorm:"index" json:"cache_expires_at"`              // 缓存过期时间
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 指定表名
func (OrderPaymentMethod) TableName() string {
	return "order_payment_methods"
}

// PaymentPollingTask 付款轮询任务（用于服务重启后恢复）
type PaymentPollingTask struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null;uniqueIndex" json:"order_id"`
	Data      string    `gorm:"type:text" json:"data"` // JSON格式的任务数据
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (PaymentPollingTask) TableName() string {
	return "payment_polling_tasks"
}

// BuiltinPaymentMethods 内置付款方式列表（全部使用JS脚本）
var BuiltinPaymentMethods = []PaymentMethod{
	{
		Name:        "USDT TRC20",
		Description: "Pay with USDT via TRC20 network, supports auto-confirmation",
		Type:        PaymentMethodTypeCustom,
		Icon:        "Coins",
		SortOrder:   1,
		Enabled:     true,
		Config:      `{"wallet_address":"","trongrid_api_key":"","usdt_contract":"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t","cny_rate":7.2,"amount_tolerance":0.0000005,"auto_confirm":true}`,
		Script: `
/**
 * USDT TRC20 付款方式脚本
 * 支持自动查询 TronGrid API 确认付款
 *
 * 配置说明:
 * - wallet_address: 收款钱包地址 (必填)
 * - trongrid_api_key: TronGrid API Key (推荐，提高查询频率限制)
 * - usdt_contract: USDT合约地址 (默认: TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t)
 * - cny_rate: CNY兑USDT汇率 (默认: 7.2)
 * - amount_tolerance: 金额容差 (默认: 0.0000005 USDT，需小于订单最小差值0.000001)
 * - auto_confirm: 是否自动确认 (默认: true)
 */

/**
 * 生成付款卡片
 */
function onGeneratePaymentCard(order, config) {
    var walletAddress = config.wallet_address || '';
    var cnyRate = parseFloat(config.cny_rate) || 7.2;

    // 计算USDT金额
    var usdtAmount = order.total_amount;
    if (order.currency === 'CNY') {
        usdtAmount = order.total_amount / cnyRate;
    } else if (order.currency === 'USD') {
        usdtAmount = order.total_amount;
    }
    // 保留2位小数，加上基于订单ID的偏移量区分并发订单（USDT支持6位小数）
    // order.id % 10000 产生 0~9999 的偏移值，除以 1000000 得到 0.000000~0.009999 的尾数
    // 最大支持 10000 笔同金额订单同时待支付
    var randomCents = (parseInt(order.id) % 10000) / 1000000;
    usdtAmount = (Math.floor(usdtAmount * 100) / 100) + randomCents;
    usdtAmount = usdtAmount.toFixed(6);

    // 保存订单信息到storage
    var orderKey = 'order_' + order.id;
    AuraLogic.storage.set(orderKey + '_amount', usdtAmount);
    AuraLogic.storage.set(orderKey + '_time', AuraLogic.system.getTimestamp().toString());
    AuraLogic.storage.set(orderKey + '_address', walletAddress);

    // 更新订单付款数据
    AuraLogic.order.updatePaymentData({
        usdt_amount: usdtAmount,
        wallet_address: walletAddress,
        expected_at: new Date().toISOString()
    });

    var autoConfirmTip = config.auto_confirm !== false
        ? '<p class="text-xs text-green-600 dark:text-green-400 mt-2">✓ <span class="lang-zh">系统将自动确认您的付款（约1-5分钟）</span><span class="lang-en">Payment will be auto-confirmed (1-5 min)</span></p>'
        : '<p class="text-xs text-amber-600 dark:text-amber-400 mt-2"><span class="lang-zh">付款完成后请联系客服确认</span><span class="lang-en">Please contact support after payment</span></p>';

    var html = '<div class="space-y-4">' +
        '<div class="bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-950 dark:to-emerald-950 border border-green-200 dark:border-green-800 rounded-lg p-4">' +
            '<div class="flex items-center justify-between mb-3">' +
                '<span class="text-sm font-medium text-green-700 dark:text-green-300">USDT TRC20</span>' +
                '<span class="text-xs bg-green-100 dark:bg-green-900 text-green-600 dark:text-green-400 px-2 py-0.5 rounded"><span class="lang-zh">推荐</span><span class="lang-en">Recommended</span></span>' +
            '</div>' +
            '<div class="text-center py-3">' +
                '<div class="text-3xl font-bold text-green-600 dark:text-green-400">' + usdtAmount + ' USDT</div>' +
                '<div class="text-xs text-muted-foreground mt-1">≈ ' + order.currency + ' ' + order.total_amount.toFixed(2) + '</div>' +
            '</div>' +
        '</div>' +
        '<div class="space-y-2">' +
            '<div class="flex items-center justify-between">' +
                '<span class="text-sm font-medium"><span class="lang-zh">收款地址</span><span class="lang-en">Wallet Address</span></span>' +
                '<button type="button" class="text-xs text-primary hover:underline" onclick="navigator.clipboard.writeText(document.getElementById(\'wallet-addr\').innerText).then(function(){alert(\'Copied!\')})"><span class="lang-zh">点击复制</span><span class="lang-en">Copy</span></button>' +
            '</div>' +
            '<div id="wallet-addr" class="bg-muted p-3 rounded-lg font-mono text-xs break-all select-all cursor-pointer border-2 border-dashed border-muted-foreground/20 hover:border-primary/50 transition-colors" onclick="navigator.clipboard.writeText(this.innerText).then(function(){alert(\'Copied!\')})">' + walletAddress + '</div>' +
        '</div>' +
        '<div class="bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-800 rounded-lg p-3">' +
            '<div class="flex items-start gap-2">' +
                '<svg class="w-4 h-4 text-amber-600 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">' +
                    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>' +
                '</svg>' +
                '<div class="text-xs text-amber-700 dark:text-amber-300">' +
                    '<p class="font-medium mb-1"><span class="lang-zh">转账须知</span><span class="lang-en">Transfer Notice</span></p>' +
                    '<ul class="list-disc list-inside space-y-0.5 text-amber-600 dark:text-amber-400">' +
                        '<li class="lang-zh">请务必使用 <strong>TRC20</strong> 网络转账</li>' +
                        '<li class="lang-en">Please use <strong>TRC20</strong> network</li>' +
                        '<li class="lang-zh">转账金额必须为 <strong>' + usdtAmount + ' USDT</strong></li>' +
                        '<li class="lang-en">Amount must be exactly <strong>' + usdtAmount + ' USDT</strong></li>' +
                        '<li class="lang-zh">金额不符将无法自动确认</li>' +
                        '<li class="lang-en">Incorrect amount will not be auto-confirmed</li>' +
                    '</ul>' +
                '</div>' +
            '</div>' +
        '</div>' +
        '<div class="text-xs text-muted-foreground border-t pt-3">' +
            '<p><span class="lang-zh">订单号</span><span class="lang-en">Order</span>: <code class="bg-muted px-1 rounded">' + order.order_no + '</code></p>' +
            autoConfirmTip +
        '</div>' +
    '</div>';

    return {
        html: html,
        title: 'USDT TRC20 付款',
        description: '使用USDT TRC20网络转账',
        data: {
            usdt_amount: usdtAmount,
            wallet_address: walletAddress
        },
        cache_ttl: 300  // 缓存5分钟，付款信息固定但允许定期刷新
    };
}

/**
 * 检查付款状态
 * 通过 TronGrid API 查询 TRC20 转账记录
 */
function onCheckPaymentStatus(order, config) {
    // 检查是否启用自动确认
    if (config.auto_confirm === false) {
        return { paid: false, message: '需要人工确认付款' };
    }

    var walletAddress = config.wallet_address;
    var apiKey = config.trongrid_api_key || '';
    var usdtContract = config.usdt_contract || 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t';
    var tolerance = parseFloat(config.amount_tolerance) || 0.0000005;

    if (!walletAddress) {
        return { paid: false, message: '未配置收款地址' };
    }

    // 获取订单保存的信息
    var orderKey = 'order_' + order.id;
    var expectedAmount = AuraLogic.storage.get(orderKey + '_amount');
    var orderTime = parseInt(AuraLogic.storage.get(orderKey + '_time') || '0');

    if (!expectedAmount || !orderTime) {
        return { paid: false, message: '订单数据不完整' };
    }

    expectedAmount = parseFloat(expectedAmount);

    // 构建 TronGrid API 请求
    // API: GET /v1/accounts/{address}/transactions/trc20
    var apiUrl = 'https://api.trongrid.io/v1/accounts/' + walletAddress + '/transactions/trc20';
    var params = '?only_confirmed=true&only_to=true&limit=50&contract_address=' + usdtContract;
    params += '&min_timestamp=' + (orderTime * 1000); // TronGrid使用毫秒时间戳

    var headers = {
        'Accept': 'application/json'
    };
    if (apiKey) {
        headers['TRON-PRO-API-KEY'] = apiKey;
    }

    var response = AuraLogic.http.get(apiUrl + params, headers);

    // 检查请求是否成功
    if (response.error) {
        return { paid: false, message: '查询区块链失败: ' + response.error };
    }

    if (response.status !== 200) {
        return { paid: false, message: 'API返回错误: ' + response.status };
    }

    var data = response.data;
    if (!data || !data.success || !data.data) {
        return { paid: false, message: '等待付款...' };
    }

    // 遍历交易记录查找匹配的付款
    var transactions = data.data;
    for (var i = 0; i < transactions.length; i++) {
        var tx = transactions[i];

        // 验证是否是USDT交易
        if (!tx.token_info || tx.token_info.symbol !== 'USDT') {
            continue;
        }

        // 验证收款地址
        if (tx.to !== walletAddress) {
            continue;
        }

        // 计算交易金额 (USDT有6位小数)
        var decimals = parseInt(tx.token_info.decimals) || 6;
        var txAmount = parseFloat(tx.value) / Math.pow(10, decimals);

        // 检查金额是否匹配 (允许容差)
        if (Math.abs(txAmount - expectedAmount) > tolerance) {
            continue;
        }

        // 检查是否已处理过这笔交易
        var txKey = 'tx_' + tx.transaction_id;
        var processedOrderId = AuraLogic.storage.get(txKey);
        if (processedOrderId) {
            // 已处理过，检查是否是当前订单
            if (processedOrderId === order.id.toString()) {
                return {
                    paid: true,
                    transaction_id: tx.transaction_id,
                    message: '付款已确认'
                };
            }
            continue; // 这笔交易已分配给其他订单
        }

        // 标记交易为已处理
        AuraLogic.storage.set(txKey, order.id.toString());

        // 清理订单临时数据
        AuraLogic.storage.delete(orderKey + '_amount');
        AuraLogic.storage.delete(orderKey + '_time');
        AuraLogic.storage.delete(orderKey + '_address');

        // 返回付款成功
        return {
            paid: true,
            transaction_id: tx.transaction_id,
            message: '付款成功',
            data: {
                amount: txAmount,
                from_address: tx.from,
                block_timestamp: tx.block_timestamp,
                confirmed: true
            }
        };
    }

    return { paid: false, message: '等待区块确认...' };
}

/**
 * 退款处理
 * USDT为去中心化加密货币，无法自动退款，需要管理员手动转账
 */
function onRefund(order, config) {
    var orderKey = 'order_' + order.id;
    var savedAmount = AuraLogic.storage.get(orderKey + '_amount') || '';

    return {
        success: true,
        message: 'USDT TRC20退款需手动操作，请将' + (savedAmount || order.total_amount) + ' USDT转回用户地址',
        data: {
            network: 'TRC20',
            amount: savedAmount || order.total_amount.toString(),
            wallet_address: config.wallet_address || ''
        }
    };
}
`,
	},
	{
		Name:        "USDT BEP20 (BSC)",
		Description: "Pay with USDT via BEP20 (BSC) network, supports auto-confirmation",
		Type:        PaymentMethodTypeCustom,
		Icon:        "Coins",
		SortOrder:   2,
		Enabled:     true,
		Config:      `{"wallet_address":"","bscscan_api_key":"","usdt_contract":"0x55d398326f99059fF775485246999027B3197955","cny_rate":7.2,"amount_tolerance":0.0000005,"auto_confirm":true}`,
		Script: `
/**
 * USDT BEP20 (BSC) 付款方式脚本
 * 支持自动查询 BSCScan API 确认付款
 *
 * 配置说明:
 * - wallet_address: 收款钱包地址 (必填)
 * - bscscan_api_key: BSCScan API Key (必填，免费申请: https://bscscan.com/myapikey)
 * - usdt_contract: BSC USDT合约地址 (默认: 0x55d398326f99059fF775485246999027B3197955)
 * - cny_rate: CNY兑USDT汇率 (默认: 7.2)
 * - amount_tolerance: 金额容差 (默认: 0.0000005 USDT，需小于订单最小差值0.000001)
 * - auto_confirm: 是否自动确认 (默认: true)
 */

/**
 * 生成付款卡片
 */
function onGeneratePaymentCard(order, config) {
    var walletAddress = config.wallet_address || '';
    var cnyRate = parseFloat(config.cny_rate) || 7.2;

    // 计算USDT金额
    var usdtAmount = order.total_amount;
    if (order.currency === 'CNY') {
        usdtAmount = order.total_amount / cnyRate;
    } else if (order.currency === 'USD') {
        usdtAmount = order.total_amount;
    }
    // 保留2位小数，加上基于订单ID的偏移量区分并发订单（USDT支持18位小数）
    var randomCents = (parseInt(order.id) % 10000) / 1000000;
    usdtAmount = (Math.floor(usdtAmount * 100) / 100) + randomCents;
    usdtAmount = usdtAmount.toFixed(6);

    // 保存订单信息到storage
    var orderKey = 'order_' + order.id;
    AuraLogic.storage.set(orderKey + '_amount', usdtAmount);
    AuraLogic.storage.set(orderKey + '_time', AuraLogic.system.getTimestamp().toString());
    AuraLogic.storage.set(orderKey + '_address', walletAddress);

    // 更新订单付款数据
    AuraLogic.order.updatePaymentData({
        usdt_amount: usdtAmount,
        wallet_address: walletAddress,
        expected_at: new Date().toISOString()
    });

    var autoConfirmTip = config.auto_confirm !== false
        ? '<p class="text-xs text-green-600 dark:text-green-400 mt-2">✓ <span class="lang-zh">系统将自动确认您的付款（约1-5分钟）</span><span class="lang-en">Payment will be auto-confirmed (1-5 min)</span></p>'
        : '<p class="text-xs text-amber-600 dark:text-amber-400 mt-2"><span class="lang-zh">付款完成后请联系客服确认</span><span class="lang-en">Please contact support after payment</span></p>';

    var html = '<div class="space-y-4">' +
        '<div class="bg-gradient-to-r from-yellow-50 to-amber-50 dark:from-yellow-950 dark:to-amber-950 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">' +
            '<div class="flex items-center justify-between mb-3">' +
                '<span class="text-sm font-medium text-yellow-700 dark:text-yellow-300">USDT BEP20 (BSC)</span>' +
                '<span class="text-xs bg-yellow-100 dark:bg-yellow-900 text-yellow-600 dark:text-yellow-400 px-2 py-0.5 rounded">BNB Smart Chain</span>' +
            '</div>' +
            '<div class="text-center py-3">' +
                '<div class="text-3xl font-bold text-yellow-600 dark:text-yellow-400">' + usdtAmount + ' USDT</div>' +
                '<div class="text-xs text-muted-foreground mt-1">≈ ' + order.currency + ' ' + order.total_amount.toFixed(2) + '</div>' +
            '</div>' +
        '</div>' +
        '<div class="space-y-2">' +
            '<div class="flex items-center justify-between">' +
                '<span class="text-sm font-medium"><span class="lang-zh">收款地址</span><span class="lang-en">Wallet Address</span></span>' +
                '<button type="button" class="text-xs text-primary hover:underline" onclick="navigator.clipboard.writeText(document.getElementById(\'wallet-addr-bep20\').innerText).then(function(){alert(\'Copied!\')})"><span class="lang-zh">点击复制</span><span class="lang-en">Copy</span></button>' +
            '</div>' +
            '<div id="wallet-addr-bep20" class="bg-muted p-3 rounded-lg font-mono text-xs break-all select-all cursor-pointer border-2 border-dashed border-muted-foreground/20 hover:border-primary/50 transition-colors" onclick="navigator.clipboard.writeText(this.innerText).then(function(){alert(\'Copied!\')})">' + walletAddress + '</div>' +
        '</div>' +
        '<div class="bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-800 rounded-lg p-3">' +
            '<div class="flex items-start gap-2">' +
                '<svg class="w-4 h-4 text-amber-600 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">' +
                    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>' +
                '</svg>' +
                '<div class="text-xs text-amber-700 dark:text-amber-300">' +
                    '<p class="font-medium mb-1"><span class="lang-zh">转账须知</span><span class="lang-en">Transfer Notice</span></p>' +
                    '<ul class="list-disc list-inside space-y-0.5 text-amber-600 dark:text-amber-400">' +
                        '<li class="lang-zh">请务必使用 <strong>BEP20 (BSC)</strong> 网络转账</li>' +
                        '<li class="lang-en">Please use <strong>BEP20 (BSC)</strong> network</li>' +
                        '<li class="lang-zh">转账金额必须为 <strong>' + usdtAmount + ' USDT</strong></li>' +
                        '<li class="lang-en">Amount must be exactly <strong>' + usdtAmount + ' USDT</strong></li>' +
                        '<li class="lang-zh">金额不符将无法自动确认</li>' +
                        '<li class="lang-en">Incorrect amount will not be auto-confirmed</li>' +
                    '</ul>' +
                '</div>' +
            '</div>' +
        '</div>' +
        '<div class="text-xs text-muted-foreground border-t pt-3">' +
            '<p><span class="lang-zh">订单号</span><span class="lang-en">Order</span>: <code class="bg-muted px-1 rounded">' + order.order_no + '</code></p>' +
            autoConfirmTip +
        '</div>' +
    '</div>';

    return {
        html: html,
        title: 'USDT BEP20 (BSC) 付款',
        description: '使用USDT BEP20(BSC)网络转账',
        data: {
            usdt_amount: usdtAmount,
            wallet_address: walletAddress
        },
        cache_ttl: 300  // 缓存5分钟，付款信息固定但允许定期刷新
    };
}

/**
 * 检查付款状态
 * 通过 BSCScan API 查询 BEP20 转账记录
 */
function onCheckPaymentStatus(order, config) {
    // 检查是否启用自动确认
    if (config.auto_confirm === false) {
        return { paid: false, message: '需要人工确认付款' };
    }

    var walletAddress = config.wallet_address;
    var apiKey = config.bscscan_api_key || '';
    var usdtContract = config.usdt_contract || '0x55d398326f99059fF775485246999027B3197955';
    var tolerance = parseFloat(config.amount_tolerance) || 0.0000005;

    if (!walletAddress) {
        return { paid: false, message: '未配置收款地址' };
    }

    if (!apiKey) {
        return { paid: false, message: '未配置BSCScan API Key' };
    }

    // 获取订单保存的信息
    var orderKey = 'order_' + order.id;
    var expectedAmount = AuraLogic.storage.get(orderKey + '_amount');
    var orderTime = parseInt(AuraLogic.storage.get(orderKey + '_time') || '0');

    if (!expectedAmount || !orderTime) {
        return { paid: false, message: '订单数据不完整' };
    }

    expectedAmount = parseFloat(expectedAmount);

    // 构建 BSCScan API 请求
    // API: https://api.bscscan.com/api?module=account&action=tokentx
    var apiUrl = 'https://api.bscscan.com/api';
    var params = '?module=account&action=tokentx' +
        '&contractaddress=' + usdtContract +
        '&address=' + walletAddress +
        '&page=1&offset=50&sort=desc' +
        '&startblock=0&endblock=99999999' +
        '&apikey=' + apiKey;

    var headers = {
        'Accept': 'application/json'
    };

    var response = AuraLogic.http.get(apiUrl + params, headers);

    // 检查请求是否成功
    if (response.error) {
        return { paid: false, message: '查询区块链失败: ' + response.error };
    }

    if (response.status !== 200) {
        return { paid: false, message: 'API返回错误: ' + response.status };
    }

    var data = response.data;
    if (!data || data.status !== '1' || !data.result) {
        return { paid: false, message: '等待付款...' };
    }

    // 遍历交易记录查找匹配的付款
    var transactions = data.result;
    for (var i = 0; i < transactions.length; i++) {
        var tx = transactions[i];

        // 验证收款地址 (BSCScan返回小写地址)
        if (tx.to.toLowerCase() !== walletAddress.toLowerCase()) {
            continue;
        }

        // 验证合约地址
        if (tx.contractAddress.toLowerCase() !== usdtContract.toLowerCase()) {
            continue;
        }

        // 验证交易时间 (BSCScan timeStamp是秒级)
        var txTime = parseInt(tx.timeStamp) || 0;
        if (txTime < orderTime) {
            continue;
        }

        // 计算交易金额 (BSC USDT有18位小数)
        var decimals = parseInt(tx.tokenDecimal) || 18;
        // 避免大数精度丢失：先截取有效位数再转换
        var valueStr = tx.value || '0';
        var txAmount;
        if (decimals > 0 && valueStr.length > decimals) {
            var intPart = valueStr.substring(0, valueStr.length - decimals);
            var decPart = valueStr.substring(valueStr.length - decimals);
            txAmount = parseFloat(intPart + '.' + decPart);
        } else if (decimals > 0) {
            var padded = '';
            for (var p = 0; p < decimals - valueStr.length; p++) { padded += '0'; }
            txAmount = parseFloat('0.' + padded + valueStr);
        } else {
            txAmount = parseFloat(valueStr);
        }

        // 检查金额是否匹配 (允许容差)
        if (Math.abs(txAmount - expectedAmount) > tolerance) {
            continue;
        }

        // 检查是否已处理过这笔交易
        var txKey = 'tx_' + tx.hash;
        var processedOrderId = AuraLogic.storage.get(txKey);
        if (processedOrderId) {
            if (processedOrderId === order.id.toString()) {
                return {
                    paid: true,
                    transaction_id: tx.hash,
                    message: '付款已确认'
                };
            }
            continue;
        }

        // 标记交易为已处理
        AuraLogic.storage.set(txKey, order.id.toString());

        // 清理订单临时数据
        AuraLogic.storage.delete(orderKey + '_amount');
        AuraLogic.storage.delete(orderKey + '_time');
        AuraLogic.storage.delete(orderKey + '_address');

        // 返回付款成功
        return {
            paid: true,
            transaction_id: tx.hash,
            message: '付款成功',
            data: {
                amount: txAmount,
                from_address: tx.from,
                block_timestamp: txTime,
                confirmed: true
            }
        };
    }

    return { paid: false, message: '等待区块确认...' };
}

/**
 * 退款处理
 * USDT为去中心化加密货币，无法自动退款，需要管理员手动转账
 */
function onRefund(order, config) {
    var orderKey = 'order_' + order.id;
    var savedAmount = AuraLogic.storage.get(orderKey + '_amount') || '';

    return {
        success: true,
        message: 'USDT BEP20退款需手动操作，请将' + (savedAmount || order.total_amount) + ' USDT转回用户地址',
        data: {
            network: 'BEP20',
            amount: savedAmount || order.total_amount.toString(),
            wallet_address: config.wallet_address || ''
        }
    };
}
`,
	},
}
