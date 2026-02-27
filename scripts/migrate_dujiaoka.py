#!/usr/bin/env python3
"""
独角数卡 (Dujiaoka) -> AuraLogic 数据迁移脚本

从独角数卡的 MySQL 数据库读取商品、分类、卡密等数据，
通过 AuraLogic API 导入到目标系统中。

依赖安装: pip install pymysql requests rich
"""

import json
import sys
import time
import argparse
import logging
from dataclasses import dataclass

import pymysql
import requests
from rich.console import Console
from rich.table import Table
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn
from rich.logging import RichHandler

# ============================================================
# 配置
# ============================================================

@dataclass
class DujiaokaConfig:
    """独角数卡 MySQL 数据库配置"""
    host: str = "127.0.0.1"
    port: int = 3306
    user: str = "root"
    password: str = ""
    database: str = "dujiaoka"
    charset: str = "utf8mb4"


@dataclass
class AuraLogicConfig:
    """AuraLogic API 配置"""
    base_url: str = "http://localhost:8080"
    api_key: str = ""
    api_secret: str = ""
    timeout: int = 30
    retry_count: int = 3
    retry_delay: float = 1.0


@dataclass
class MigrationOptions:
    """迁移选项"""
    migrate_products: bool = True
    migrate_carmis: bool = True       # 卡密 -> 虚拟库存
    migrate_coupons: bool = True      # 优惠码 -> 促销码
    dry_run: bool = False             # 仅预览，不实际写入
    batch_size: int = 50              # 卡密批量导入大小
    skip_disabled: bool = False       # 跳过已禁用的商品/分类
    default_product_status: str = "draft"  # 导入后的商品状态


# ============================================================
# 日志 & 控制台
# ============================================================

console = Console()
logging.basicConfig(
    level=logging.INFO,
    format="%(message)s",
    handlers=[RichHandler(console=console, rich_tracebacks=True)],
)
log = logging.getLogger("migrate")


# ============================================================
# AuraLogic API 客户端
# ============================================================

class AuraLogicClient:
    """AuraLogic API 客户端，使用 API Key + Secret 认证"""

    def __init__(self, config: AuraLogicConfig):
        self.config = config
        self.base_url = config.base_url.rstrip("/")
        self.session = requests.Session()
        self.session.headers.update({
            "X-API-Key": config.api_key,
            "X-API-Secret": config.api_secret,
            "Content-Type": "application/json",
        })
        self.session.timeout = config.timeout

    def _request(self, method: str, path: str, headers_override: dict = None, **kwargs) -> dict:
        url = f"{self.base_url}/api/admin{path}"
        last_err = None
        # 临时移除 headers 前先备份
        saved_headers = {}
        if headers_override:
            for k, v in headers_override.items():
                if v is None:
                    saved_headers[k] = self.session.headers.pop(k, None)
                else:
                    kwargs.setdefault("headers", {})
                    kwargs["headers"][k] = v
        try:
            for attempt in range(1, self.config.retry_count + 1):
                try:
                    resp = self.session.request(method, url, **kwargs)
                    if resp.status_code == 429:
                        wait = float(resp.headers.get("Retry-After", self.config.retry_delay * attempt))
                        log.warning(f"Rate limited, waiting {wait}s (attempt {attempt})")
                        time.sleep(wait)
                        continue
                    resp.raise_for_status()
                    return resp.json() if resp.text else {}
                except requests.exceptions.RequestException as e:
                    last_err = e
                    if attempt < self.config.retry_count:
                        time.sleep(self.config.retry_delay * attempt)
                        log.warning(f"Request failed, retrying ({attempt}/{self.config.retry_count}): {e}")
            raise RuntimeError(f"API request failed after {self.config.retry_count} attempts: {last_err}")
        finally:
            # 恢复被移除的 session headers
            for k, v in saved_headers.items():
                if v is not None:
                    self.session.headers[k] = v

    # -- 商品 --
    def create_product(self, data: dict) -> dict:
        return self._request("POST", "/products", json=data)

    def list_products(self, page: int = 1, page_size: int = 100) -> dict:
        return self._request("GET", "/products", params={"page": page, "page_size": page_size})

    # -- 虚拟库存 --
    def create_virtual_inventory(self, data: dict) -> dict:
        return self._request("POST", "/virtual-inventories", json=data)

    def import_virtual_stock(self, inventory_id: int, items: list[str]) -> dict:
        """通过 text 模式批量导入卡密，每行一条"""
        content = "\n".join(items)
        # 导入接口是 multipart form，不是 JSON
        return self._request(
            "POST",
            f"/virtual-inventories/{inventory_id}/import",
            data={"import_type": "text", "content": content},
            headers_override={"Content-Type": None},  # 让 requests 自动设置
        )

    def create_virtual_stock_item(self, inventory_id: int, content: str, remark: str = "") -> dict:
        """单条创建库存项（用于循环卡密等需要备注的场景）"""
        data = {"content": content}
        if remark:
            data["remark"] = remark
        return self._request("POST", f"/virtual-inventories/{inventory_id}/stocks", json=data)

    def reserve_stock_item(self, inventory_id: int, stock_id: int, remark: str = "") -> dict:
        """将库存项标记为已预留"""
        data = {}
        if remark:
            data["remark"] = remark
        return self._request("POST", f"/virtual-inventories/{inventory_id}/stocks/{stock_id}/reserve", json=data)

    # -- 商品-虚拟库存绑定 --
    def bind_virtual_inventory(self, product_id: int, virtual_inventory_id: int) -> dict:
        return self._request("POST", f"/products/{product_id}/virtual-inventory-bindings", json={
            "virtual_inventory_id": virtual_inventory_id,
        })

    # -- 优惠码 -> 促销码 --
    def create_promo_code(self, data: dict) -> dict:
        return self._request("POST", "/promo-codes", json=data)

    def test_connection(self) -> bool:
        try:
            self._request("GET", "/products", params={"page": 1, "page_size": 1})
            return True
        except Exception as e:
            log.error(f"API connection test failed: {e}")
            return False


# ============================================================
# 独角数卡数据库读取
# ============================================================

class DujiaokaReader:
    """从独角数卡 MySQL 数据库读取数据"""

    def __init__(self, config: DujiaokaConfig):
        self.config = config
        self.conn = None

    def connect(self):
        self.conn = pymysql.connect(
            host=self.config.host,
            port=self.config.port,
            user=self.config.user,
            password=self.config.password,
            database=self.config.database,
            charset=self.config.charset,
            cursorclass=pymysql.cursors.DictCursor,
        )
        log.info(f"Connected to dujiaoka database at {self.config.host}:{self.config.port}")

    def close(self):
        if self.conn:
            self.conn.close()

    def _query(self, sql: str, params=None) -> list[dict]:
        with self.conn.cursor() as cur:
            cur.execute(sql, params)
            return cur.fetchall()

    def get_categories(self, skip_disabled: bool = False) -> list[dict]:
        sql = "SELECT * FROM goods_group WHERE deleted_at IS NULL"
        if skip_disabled:
            sql += " AND is_open = 1"
        sql += " ORDER BY ord DESC, id ASC"
        return self._query(sql)

    def get_goods(self, skip_disabled: bool = False) -> list[dict]:
        sql = """
            SELECT g.*, gg.gp_name AS category_name,
                   IFNULL(gg.is_open, 1) AS category_is_open
            FROM goods g
            LEFT JOIN goods_group gg ON g.group_id = gg.id
            WHERE g.deleted_at IS NULL
        """
        if skip_disabled:
            sql += " AND g.is_open = 1"
        sql += " ORDER BY g.ord DESC, g.id ASC"
        return self._query(sql)

    def get_carmis_by_goods(self, goods_id: int, only_unsold: bool = True) -> list[dict]:
        sql = "SELECT * FROM carmis WHERE goods_id = %s AND deleted_at IS NULL"
        if only_unsold:
            sql += " AND status = 1"
        sql += " ORDER BY id ASC"
        return self._query(sql, (goods_id,))

    def get_carmis_count(self, goods_id: int, only_unsold: bool = True) -> int:
        sql = "SELECT COUNT(*) AS cnt FROM carmis WHERE goods_id = %s AND deleted_at IS NULL"
        if only_unsold:
            sql += " AND status = 1"
        rows = self._query(sql, (goods_id,))
        return rows[0]["cnt"] if rows else 0

    def get_coupons(self) -> list[dict]:
        sql = "SELECT * FROM coupons WHERE deleted_at IS NULL ORDER BY id ASC"
        return self._query(sql)

    def get_coupon_goods(self, coupon_id: int) -> list[int]:
        sql = "SELECT goods_id FROM coupons_goods WHERE coupons_id = %s"
        rows = self._query(sql, (coupon_id,))
        return [r["goods_id"] for r in rows]

    def get_summary(self) -> dict:
        cats = self._query("SELECT COUNT(*) AS c FROM goods_group WHERE deleted_at IS NULL")
        goods = self._query("SELECT COUNT(*) AS c FROM goods WHERE deleted_at IS NULL")
        carmis = self._query("SELECT COUNT(*) AS c FROM carmis WHERE deleted_at IS NULL AND status = 1")
        coupons = self._query("SELECT COUNT(*) AS c FROM coupons WHERE deleted_at IS NULL")
        return {
            "categories": cats[0]["c"],
            "goods": goods[0]["c"],
            "unsold_carmis": carmis[0]["c"],
            "coupons": coupons[0]["c"],
        }


# ============================================================
# 迁移引擎
# ============================================================

class MigrationEngine:
    """执行从独角数卡到 AuraLogic 的数据迁移"""

    def __init__(
        self,
        reader: DujiaokaReader,
        client: AuraLogicClient,
        options: MigrationOptions,
    ):
        self.reader = reader
        self.client = client
        self.opts = options
        # 映射表: dujiaoka goods_id -> auralogic product_id
        self.product_map: dict[int, int] = {}
        # 映射表: dujiaoka goods_id -> auralogic virtual_inventory_id
        self.vinv_map: dict[int, int] = {}
        # 统计
        self.stats = {
            "products_created": 0,
            "products_skipped": 0,
            "products_failed": 0,
            "vinv_created": 0,
            "carmis_imported": 0,
            "bindings_created": 0,
            "coupons_created": 0,
            "coupons_failed": 0,
        }

    def _build_product_payload(self, goods: dict) -> dict:
        """将独角数卡商品转换为 AuraLogic 产品格式"""
        is_virtual = goods["type"] == 1  # 1=自动发货(虚拟), 2=人工处理(实体)

        # is_open=0 或所属分类 is_open=0 → 强制 inactive
        if goods.get("is_open", 1) == 0 or goods.get("category_is_open", 1) == 0:
            status = "inactive"
        else:
            status = self.opts.default_product_status

        payload = {
            "sku": f"djk-{goods['id']}",
            "name": goods["gd_name"],
            "product_type": "virtual" if is_virtual else "physical",
            "short_description": goods.get("gd_description", ""),
            "description": goods.get("description", ""),
            "category": goods.get("category_name", "未分类"),
            "tags": [t.strip() for t in (goods.get("gd_keywords") or "").split(",") if t.strip()],
            "price": float(goods.get("actual_price", 0)),
            "original_price": float(goods.get("retail_price", 0)),
            "stock": int(goods.get("in_stock", 0)),
            "sort_order": int(goods.get("ord", 1)),
            "status": status,
            "auto_delivery": is_virtual,
        }

        if goods.get("buy_limit_num", 0) > 0:
            payload["max_purchase_limit"] = goods["buy_limit_num"]

        # 商品图片
        if goods.get("picture"):
            try:
                pics = json.loads(goods["picture"]) if isinstance(goods["picture"], str) else goods["picture"]
                if isinstance(pics, list):
                    payload["images"] = [
                        {"url": p, "is_primary": i == 0} for i, p in enumerate(pics) if p
                    ]
                elif isinstance(pics, str) and pics:
                    payload["images"] = [{"url": pics, "is_primary": True}]
            except (json.JSONDecodeError, TypeError):
                if goods["picture"]:
                    payload["images"] = [{"url": goods["picture"], "is_primary": True}]

        # 构建备注：购买提示 + 批发价配置 + 自定义输入框 + API Hook
        remark_parts = []
        if goods.get("buy_prompt"):
            remark_parts.append(f"[购买提示] {goods['buy_prompt']}")
        if goods.get("wholesale_price_cnf"):
            remark_parts.append(f"[批发价配置] {goods['wholesale_price_cnf']}")
        if goods.get("other_ipu_cnf"):
            remark_parts.append(f"[自定义输入框] {goods['other_ipu_cnf']}")
        if goods.get("api_hook"):
            remark_parts.append(f"[API Hook] {goods['api_hook']}")
        if remark_parts:
            payload["remark"] = "\n".join(remark_parts)

        return payload

    def migrate_products(self):
        """迁移商品数据"""
        goods_list = self.reader.get_goods(skip_disabled=self.opts.skip_disabled)
        if not goods_list:
            log.info("No goods found to migrate")
            return

        log.info(f"Found {len(goods_list)} goods to migrate")

        with Progress(
            SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
            BarColumn(), TextColumn("{task.completed}/{task.total}"),
            console=console,
        ) as progress:
            task = progress.add_task("Migrating products...", total=len(goods_list))

            for goods in goods_list:
                gid = goods["id"]
                name = goods["gd_name"]
                progress.update(task, description=f"Product: {name[:30]}")

                try:
                    payload = self._build_product_payload(goods)

                    if self.opts.dry_run:
                        log.info(f"[DRY RUN] Would create product: {name} (djk-{gid})")
                        self.product_map[gid] = -1
                        self.stats["products_created"] += 1
                    else:
                        result = self.client.create_product(payload)
                        pid = result.get("id") or result.get("data", {}).get("id")
                        if pid:
                            self.product_map[gid] = pid
                            self.stats["products_created"] += 1
                            log.info(f"Created product: {name} -> id={pid}")
                        else:
                            log.warning(f"Product created but no ID returned: {name}")
                            self.stats["products_failed"] += 1
                except Exception as e:
                    log.error(f"Failed to create product '{name}' (djk-{gid}): {e}")
                    self.stats["products_failed"] += 1

                progress.advance(task)

    def migrate_carmis(self):
        """迁移卡密数据 -> 虚拟库存"""
        if not self.product_map:
            log.warning("No products mapped, skipping carmis migration")
            return

        goods_with_carmis = [
            (gid, pid) for gid, pid in self.product_map.items()
            if self.reader.get_carmis_count(gid) > 0
        ]

        if not goods_with_carmis:
            log.info("No carmis found to migrate")
            return

        log.info(f"Found {len(goods_with_carmis)} products with carmis to migrate")

        with Progress(
            SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
            BarColumn(), TextColumn("{task.completed}/{task.total}"),
            console=console,
        ) as progress:
            task = progress.add_task("Migrating carmis...", total=len(goods_with_carmis))

            for gid, pid in goods_with_carmis:
                progress.update(task, description=f"Carmis for djk-{gid}")
                try:
                    self._migrate_carmis_for_goods(gid, pid)
                except Exception as e:
                    log.error(f"Failed to migrate carmis for djk-{gid}: {e}")
                progress.advance(task)

    def _migrate_carmis_for_goods(self, goods_id: int, product_id: int):
        """为单个商品迁移卡密"""
        carmis = self.reader.get_carmis_by_goods(goods_id)
        if not carmis:
            return

        # 1. 创建虚拟库存
        vinv_data = {
            "name": f"djk-{goods_id} 卡密库存",
            "sku": f"djk-vinv-{goods_id}",
            "description": f"从独角数卡商品 #{goods_id} 迁移的卡密",
            "is_active": True,
        }

        if self.opts.dry_run:
            log.info(f"[DRY RUN] Would create virtual inventory for djk-{goods_id} ({len(carmis)} items)")
            self.vinv_map[goods_id] = -1
            self.stats["vinv_created"] += 1
            self.stats["carmis_imported"] += len(carmis)
            return

        result = self.client.create_virtual_inventory(vinv_data)
        vinv_id = result.get("id") or result.get("data", {}).get("id")
        if not vinv_id:
            log.error(f"Virtual inventory created but no ID returned for djk-{goods_id}")
            return

        self.vinv_map[goods_id] = vinv_id
        self.stats["vinv_created"] += 1
        log.info(f"Created virtual inventory: id={vinv_id} for djk-{goods_id}")

        # 2. 导入卡密，区分普通卡密和循环卡密
        normal_items = [c["carmi"] for c in carmis if c.get("carmi") and not c.get("is_loop")]
        loop_items = [c for c in carmis if c.get("carmi") and c.get("is_loop")]

        # 2a. 普通卡密批量导入
        for i in range(0, len(normal_items), self.opts.batch_size):
            batch = normal_items[i:i + self.opts.batch_size]
            try:
                self.client.import_virtual_stock(vinv_id, batch)
                self.stats["carmis_imported"] += len(batch)
            except Exception as e:
                log.error(f"Failed to import batch {i // self.opts.batch_size + 1} for vinv {vinv_id}: {e}")

        # 2b. 循环卡密逐条导入，创建后标记为已预留
        for c in loop_items:
            try:
                result = self.client.create_virtual_stock_item(
                    vinv_id, c["carmi"], remark="[循环卡密] 可重复使用"
                )
                stock_id = result.get("stock", {}).get("id")
                if stock_id:
                    self.client.reserve_stock_item(vinv_id, stock_id, remark="循环卡密-已预留")
                self.stats["carmis_imported"] += 1
            except Exception as e:
                log.error(f"Failed to import loop carmi for vinv {vinv_id}: {e}")

        if loop_items:
            log.info(f"Imported {len(loop_items)} loop carmis with remark for vinv {vinv_id}")

        # 3. 绑定到商品
        if product_id > 0:
            try:
                self.client.bind_virtual_inventory(product_id, vinv_id)
                self.stats["bindings_created"] += 1
                log.info(f"Bound virtual inventory {vinv_id} to product {product_id}")
            except Exception as e:
                log.error(f"Failed to bind vinv {vinv_id} to product {product_id}: {e}")

    def migrate_coupons(self):
        """迁移优惠码 -> AuraLogic 促销码"""
        coupons = self.reader.get_coupons()
        if not coupons:
            log.info("No coupons found to migrate")
            return

        log.info(f"Found {len(coupons)} coupons to migrate")

        for coupon in coupons:
            cid = coupon["id"]
            code = coupon["coupon"]

            try:
                # 查找关联商品
                djk_goods_ids = self.reader.get_coupon_goods(cid)
                al_product_ids = [
                    self.product_map[gid]
                    for gid in djk_goods_ids
                    if gid in self.product_map and self.product_map[gid] > 0
                ]

                payload = {
                    "code": code,
                    "name": f"djk-coupon-{cid}",
                    "description": f"从独角数卡迁移的优惠码 (原ID: {cid})",
                    "discount_type": "fixed",
                    "discount_value": float(coupon.get("discount", 0)),
                    "total_quantity": int(coupon.get("ret", 0)),
                    "status": "active" if coupon.get("is_open", 1) == 1 else "inactive",
                }

                # 关联商品
                if al_product_ids:
                    payload["product_ids"] = al_product_ids
                    payload["product_scope"] = "specific"
                else:
                    payload["product_scope"] = "all"

                if self.opts.dry_run:
                    log.info(f"[DRY RUN] Would create promo code: {code} (discount={payload['discount_value']})")
                    self.stats["coupons_created"] += 1
                else:
                    self.client.create_promo_code(payload)
                    self.stats["coupons_created"] += 1
                    log.info(f"Created promo code: {code}")

            except Exception as e:
                log.error(f"Failed to create promo code '{code}' (djk-{cid}): {e}")
                self.stats["coupons_failed"] += 1

    def run(self):
        """执行完整迁移流程"""
        console.rule("[bold blue]开始迁移 独角数卡 -> AuraLogic")

        if self.opts.dry_run:
            console.print("[yellow]** DRY RUN 模式 - 不会实际写入数据 **[/yellow]\n")

        # 显示源数据概览
        summary = self.reader.get_summary()
        table = Table(title="独角数卡数据概览")
        table.add_column("数据类型", style="cyan")
        table.add_column("数量", style="green", justify="right")
        table.add_row("商品分类", str(summary["categories"]))
        table.add_row("商品", str(summary["goods"]))
        table.add_row("未售卡密", str(summary["unsold_carmis"]))
        table.add_row("优惠码", str(summary["coupons"]))
        console.print(table)
        console.print()

        # Step 1: 迁移商品
        if self.opts.migrate_products:
            console.rule("[bold]Step 1: 迁移商品")
            self.migrate_products()
            console.print()

        # Step 2: 迁移卡密
        if self.opts.migrate_carmis:
            console.rule("[bold]Step 2: 迁移卡密 -> 虚拟库存")
            self.migrate_carmis()
            console.print()

        # Step 3: 迁移优惠码
        if self.opts.migrate_coupons:
            console.rule("[bold]Step 3: 迁移优惠码 -> 促销码")
            self.migrate_coupons()
            console.print()

        # 打印结果
        self.print_summary()

    def print_summary(self):
        """打印迁移结果统计"""
        console.rule("[bold green]迁移完成")
        table = Table(title="迁移结果统计")
        table.add_column("指标", style="cyan")
        table.add_column("数量", style="green", justify="right")
        table.add_row("商品创建成功", str(self.stats["products_created"]))
        table.add_row("商品跳过", str(self.stats["products_skipped"]))
        table.add_row("商品创建失败", str(self.stats["products_failed"]))
        table.add_row("虚拟库存创建", str(self.stats["vinv_created"]))
        table.add_row("卡密导入", str(self.stats["carmis_imported"]))
        table.add_row("库存绑定", str(self.stats["bindings_created"]))
        table.add_row("优惠码创建成功", str(self.stats["coupons_created"]))
        table.add_row("优惠码创建失败", str(self.stats["coupons_failed"]))
        console.print(table)

        # 导出映射表供后续使用
        mapping = {
            "product_map": {str(k): v for k, v in self.product_map.items()},
            "virtual_inventory_map": {str(k): v for k, v in self.vinv_map.items()},
        }
        mapping_file = "migration_mapping.json"
        with open(mapping_file, "w", encoding="utf-8") as f:
            json.dump(mapping, f, indent=2, ensure_ascii=False)
        console.print(f"\nID 映射表已保存到: [bold]{mapping_file}[/bold]")


# ============================================================
# CLI 入口
# ============================================================

def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="独角数卡 -> AuraLogic 数据迁移工具",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 预览模式 (不写入数据)
  python migrate_dujiaoka.py --dry-run \\
    --db-host 127.0.0.1 --db-password mypass --db-name dujiaoka \\
    --api-url http://localhost:8080 --api-key ak_live_xxx --api-secret sk_live_xxx

  # 正式迁移
  python migrate_dujiaoka.py \\
    --db-host 127.0.0.1 --db-password mypass --db-name dujiaoka \\
    --api-url http://localhost:8080 --api-key ak_live_xxx --api-secret sk_live_xxx

  # 仅迁移商品 (不迁移卡密)
  python migrate_dujiaoka.py --no-carmis \\
    --db-host 127.0.0.1 --db-password mypass --db-name dujiaoka \\
    --api-url http://localhost:8080 --api-key ak_live_xxx --api-secret sk_live_xxx
        """,
    )

    # 独角数卡数据库参数
    db = p.add_argument_group("独角数卡 MySQL 数据库")
    db.add_argument("--db-host", default="127.0.0.1", help="MySQL host (default: 127.0.0.1)")
    db.add_argument("--db-port", type=int, default=3306, help="MySQL port (default: 3306)")
    db.add_argument("--db-user", default="root", help="MySQL user (default: root)")
    db.add_argument("--db-password", default="", help="MySQL password")
    db.add_argument("--db-name", default="dujiaoka", help="Database name (default: dujiaoka)")

    # AuraLogic API 参数
    api = p.add_argument_group("AuraLogic API")
    api.add_argument("--api-url", required=True, help="AuraLogic API base URL")
    api.add_argument("--api-key", required=True, help="API Key (ak_live_xxx)")
    api.add_argument("--api-secret", required=True, help="API Secret (sk_live_xxx)")

    # 迁移选项
    opts = p.add_argument_group("迁移选项")
    opts.add_argument("--dry-run", action="store_true", help="预览模式，不实际写入")
    opts.add_argument("--no-products", action="store_true", help="跳过商品迁移")
    opts.add_argument("--no-carmis", action="store_true", help="跳过卡密迁移")
    opts.add_argument("--no-coupons", action="store_true", help="跳过优惠码迁移")
    opts.add_argument("--skip-disabled", action="store_true", help="跳过已禁用的商品和分类")
    opts.add_argument("--batch-size", type=int, default=50, help="卡密批量导入大小 (default: 50)")
    opts.add_argument("--product-status", default="draft",
                       choices=["draft", "active", "inactive"],
                       help="导入商品的初始状态 (default: draft)")

    return p


def main():
    parser = build_parser()
    args = parser.parse_args()

    # 构建配置
    db_config = DujiaokaConfig(
        host=args.db_host,
        port=args.db_port,
        user=args.db_user,
        password=args.db_password,
        database=args.db_name,
    )

    api_config = AuraLogicConfig(
        base_url=args.api_url,
        api_key=args.api_key,
        api_secret=args.api_secret,
    )

    migration_opts = MigrationOptions(
        migrate_products=not args.no_products,
        migrate_carmis=not args.no_carmis,
        migrate_coupons=not args.no_coupons,
        dry_run=args.dry_run,
        batch_size=args.batch_size,
        skip_disabled=args.skip_disabled,
        default_product_status=args.product_status,
    )

    # 连接数据库
    reader = DujiaokaReader(db_config)
    try:
        reader.connect()
    except Exception as e:
        console.print(f"[red]无法连接独角数卡数据库: {e}[/red]")
        sys.exit(1)

    # 测试 API 连接
    client = AuraLogicClient(api_config)
    if not migration_opts.dry_run:
        console.print("Testing AuraLogic API connection...")
        if not client.test_connection():
            console.print("[red]无法连接 AuraLogic API，请检查 URL 和密钥[/red]")
            reader.close()
            sys.exit(1)
        console.print("[green]API 连接成功[/green]\n")

    # 执行迁移
    try:
        engine = MigrationEngine(reader, client, migration_opts)
        engine.run()
    except KeyboardInterrupt:
        console.print("\n[yellow]迁移被用户中断[/yellow]")
        sys.exit(130)
    except Exception as e:
        console.print(f"\n[red]迁移过程中发生错误: {e}[/red]")
        log.exception("Migration error")
        sys.exit(1)
    finally:
        reader.close()


if __name__ == "__main__":
    main()
