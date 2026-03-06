'use client'

import { useState, useRef, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  getVirtualInventory,
  updateVirtualInventory,
  getVirtualInventoryStocks,
  importVirtualInventoryStock,
  deleteVirtualInventoryStock,
  createVirtualInventoryStockManually,
  reserveVirtualInventoryStock,
  releaseVirtualInventoryStock,
  testDeliveryScript
} from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { ArrowLeft, Save, Plus, Trash2, RefreshCw, Database, FileText, Upload, Loader2, Lock, Unlock, Code2, Play, BookOpen } from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import dynamic from 'next/dynamic'
import { useTheme } from '@/contexts/theme-context'
import { ConfigEditor } from '@/components/admin/config-editor'

const CodeMirror = dynamic(() => import('@uiw/react-codemirror'), { ssr: false })
const loadJsLang = () => import('@codemirror/lang-javascript').then(m => m.javascript())

// Example delivery scripts
const SCRIPT_EXAMPLE_BASIC = `// Generate random activation codes
function onDeliver(order, config) {
  var items = [];
  var chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  for (var i = 0; i < order.quantity; i++) {
    var code = "";
    for (var s = 0; s < 4; s++) {
      if (s > 0) code += "-";
      for (var j = 0; j < 4; j++) {
        code += chars.charAt(Math.floor(Math.random() * chars.length));
      }
    }
    items.push({
      content: code,
      remark: "Order " + order.order_no + " #" + (i + 1)
    });
  }
  return { success: true, items: items };
}`

const SCRIPT_EXAMPLE_HTTP = `// Call external API to get delivery content
// Set api_url and api_key in Script Config below
function onDeliver(order, config) {
  var resp = AuraLogic.http.post(config.api_url, {
    order_no: order.order_no,
    quantity: order.quantity
  }, {
    "Authorization": "Bearer " + config.api_key
  });
  if (resp.error || resp.status !== 200) {
    return { success: false, message: resp.error || "API error: " + resp.status };
  }
  var items = [];
  var codes = resp.data.codes || [];
  for (var i = 0; i < codes.length; i++) {
    items.push({ content: codes[i], remark: "" });
  }
  return { success: true, items: items };
}`

const SCRIPT_EXAMPLE_ORDER = `// Generate delivery content based on order & user info
function onDeliver(order, config) {
  var user = AuraLogic.order.getUser();
  var items = [];
  var prefix = (config.prefix || "VIP");
  var ts = AuraLogic.system.getTimestamp();
  for (var i = 0; i < order.quantity; i++) {
    var id = AuraLogic.utils.generateId();
    var content = prefix + "-" + id;
    var remark = "";
    if (user) {
      remark = user.name + " (" + user.email + ")";
    }
    items.push({ content: content, remark: remark });
  }
  return { success: true, items: items };
}`

const SCRIPT_EXAMPLE_EMBY_CONFIG = `{
  "server_url": "https://emby.example.com/emby",
  "api_key": "YOUR_EMBY_ADMIN_API_KEY",
  "username_prefix": "al",
  "username": "",
  "default_password": "",
  "password_length": 12,
  "auto_suffix_on_conflict": true
}`

const SCRIPT_EXAMPLE_EMBY_REGISTER = `// Emby register-only example
// - Always creates account(s)
// - If username already exists: auto suffix (_2, _3...) when auto_suffix_on_conflict=true
// Config fields are provided by script_config JSON.

function _trimSlash(s) {
  if (!s) return "";
  while (s.length > 0 && s.charAt(s.length - 1) === "/") {
    s = s.slice(0, -1);
  }
  return s;
}

function _sanitizeUsername(s) {
  s = String(s || "").toLowerCase();
  s = s.replace(/[^a-z0-9._-]/g, "_");
  s = s.replace(/_+/g, "_");
  if (s.length > 30) s = s.slice(0, 30);
  if (!s) s = "user";
  return s;
}

function _randomPassword(len) {
  var chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%";
  var out = "";
  for (var i = 0; i < len; i++) {
    out += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return out;
}

function _buildUrl(base, path, query) {
  base = _trimSlash(base);
  path = path || "";
  if (path && path.charAt(0) !== "/") {
    path = "/" + path;
  }
  var url = base + path;
  var first = true;
  query = query || {};
  for (var k in query) {
    if (!Object.prototype.hasOwnProperty.call(query, k)) continue;
    var v = query[k];
    if (v === undefined || v === null || v === "") continue;
    url += (first ? "?" : "&") + encodeURIComponent(k) + "=" + encodeURIComponent(String(v));
    first = false;
  }
  return url;
}

function _parseResponseData(resp) {
  if (resp && resp.data) return resp.data;
  if (resp && resp.body) {
    var decoded = AuraLogic.utils.jsonDecode(resp.body);
    if (decoded) return decoded;
  }
  return null;
}

function _request(baseUrl, apiKey, method, path, body, extraHeaders, extraQuery) {
  var query = {};
  if (extraQuery) {
    for (var qk in extraQuery) {
      if (!Object.prototype.hasOwnProperty.call(extraQuery, qk)) continue;
      query[qk] = extraQuery[qk];
    }
  }
  query.api_key = apiKey;
  var headers = extraHeaders || {};
  var url = _buildUrl(baseUrl, path, query);
  var resp = AuraLogic.http.request(method, url, body, headers);
  if (!resp) {
    throw new Error("Empty HTTP response");
  }
  if (resp.error) {
    throw new Error(resp.error);
  }
  if ((resp.status || 0) >= 400) {
    throw new Error("HTTP " + resp.status + ": " + (resp.body || "request failed"));
  }
  return {
    raw: resp,
    data: _parseResponseData(resp)
  };
}

function _resolveUsername(order, user, config) {
  if (config.username) {
    return _sanitizeUsername(config.username);
  }
  var prefix = _sanitizeUsername(config.username_prefix || "al");
  if (user && user.email) {
    var email = String(user.email);
    var local = email.split("@")[0] || "user";
    return _sanitizeUsername(prefix + "_" + local);
  }
  if (user && user.id) {
    return _sanitizeUsername(prefix + "_" + user.id);
  }
  return _sanitizeUsername(prefix + "_" + String(order.order_no || "order"));
}

function _findUserByName(users, username) {
  var target = String(username || "").toLowerCase();
  for (var i = 0; i < users.length; i++) {
    var u = users[i] || {};
    var name = String(u.Name || u.name || "").toLowerCase();
    if (name === target) return u;
  }
  return null;
}

function _collectTakenNames(users) {
  var taken = {};
  for (var i = 0; i < users.length; i++) {
    var u = users[i] || {};
    var name = String(u.Name || u.name || "").toLowerCase();
    if (name) taken[name] = true;
  }
  return taken;
}

function _withSuffix(base, suffix) {
  var suffixText = "_" + String(suffix);
  var maxBaseLen = 30 - suffixText.length;
  if (maxBaseLen < 1) maxBaseLen = 1;
  var b = String(base || "user");
  if (b.length > maxBaseLen) b = b.slice(0, maxBaseLen);
  return _sanitizeUsername(b + suffixText);
}

function _nextAvailableUsername(seed, taken, autoSuffix) {
  var base = _sanitizeUsername(seed);
  if (!taken[base.toLowerCase()]) return base;
  if (!autoSuffix) return "";

  for (var i = 2; i <= 9999; i++) {
    var candidate = _withSuffix(base, i);
    if (!taken[candidate.toLowerCase()]) return candidate;
  }
  return "";
}

function _fetchUsers(baseUrl, apiKey) {
  // Prefer /Users/Query (stable shape: { Items: [...] }), fallback to /Users.
  try {
    var q = _request(baseUrl, apiKey, "GET", "/Users/Query", null, null, null);
    var qData = q.data || {};
    if (qData.Items && qData.Items.length !== undefined) return qData.Items;
    if (qData.length !== undefined) return qData;
  } catch (e) {}

  var raw = _request(baseUrl, apiKey, "GET", "/Users", null, null, null);
  var data = raw.data || [];
  if (data.Items && data.Items.length !== undefined) return data.Items;
  if (data.length !== undefined) return data;
  return [];
}

function _createUser(baseUrl, apiKey, username) {
  // Emby API docs: POST /Users/New with JSON body { Name }
  var created = _request(
    baseUrl,
    apiKey,
    "POST",
    "/Users/New",
    { Name: username },
    { "Content-Type": "application/json" },
    null
  );
  if (created.data && (created.data.Id || created.data.id)) {
    return created.data;
  }

  // Some versions may return empty body; re-query by username.
  var users = _fetchUsers(baseUrl, apiKey);
  return _findUserByName(users, username);
}

function _setPassword(baseUrl, apiKey, userId, password) {
  if (!password) return;
  _request(
    baseUrl,
    apiKey,
    "POST",
    "/Users/" + userId + "/Password",
    {
      Id: String(userId),
      NewPw: password,
      ResetPassword: false
    },
    { "Content-Type": "application/json" },
    null
  );
}

function onDeliver(order, config) {
  try {
    var baseUrl = _trimSlash(config.server_url || "");
    var apiKey = String(config.api_key || "");
    if (!baseUrl) {
      return { success: false, message: "config.server_url is required" };
    }
    if (!apiKey) {
      return { success: false, message: "config.api_key is required" };
    }

    var user = AuraLogic.order.getUser();
    var baseUsername = _resolveUsername(order, user, config);
    var count = order.quantity || 1;
    var autoSuffix = config.auto_suffix_on_conflict !== false;
    var users = _fetchUsers(baseUrl, apiKey);
    var taken = _collectTakenNames(users);
    var items = [];

    for (var i = 0; i < count; i++) {
      var seed = baseUsername;
      if (count > 1) {
        seed = _withSuffix(baseUsername, i + 1);
      }
      var username = _nextAvailableUsername(seed, taken, autoSuffix);
      if (!username) {
        return { success: false, message: "Username already exists and auto suffix is disabled: " + seed };
      }

      var embyUser = _createUser(baseUrl, apiKey, username) || {};
      var createdUserId = embyUser.Id || embyUser.id;
      if (!createdUserId) {
        return { success: false, message: "Emby create user succeeded but user id is missing" };
      }

      var password = String(config.default_password || "");
      if (!password) {
        var pwLen = parseInt(config.password_length || 12, 10);
        if (!pwLen || pwLen < 8) pwLen = 12;
        password = _randomPassword(pwLen);
      }
      _setPassword(baseUrl, apiKey, createdUserId, password);

      var lines = [
        "Emby Server: " + baseUrl,
        "Username: " + username,
        "Password: " + password,
        "Action: created",
        "Order No: " + String(order.order_no || "")
      ];
      items.push({
        content: lines.join("\\n"),
        remark: "Emby account created (" + (i + 1) + "/" + count + ")"
      });

      taken[username.toLowerCase()] = true;
    }

    return { success: true, items: items };
  } catch (e) {
    return { success: false, message: String(e && e.message ? e.message : e) };
  }
}`

export default function VirtualInventoryEditPage() {
  const params = useParams()
  const router = useRouter()
  const toast = useToast()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const inventoryId = Number(params.id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminVirtualInventory)
  const { resolvedTheme } = useTheme()
  const cmTheme = resolvedTheme === 'dark' ? 'dark' as const : 'light' as const

  const [page, setPage] = useState(1)
  const [limit] = useState(20)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [jsExtensions, setJsExtensions] = useState<any[]>([])
  const [testQuantity, setTestQuantity] = useState(1)
  const [testResult, setTestResult] = useState<any>(null)
  const configFlushRef = useRef<(() => string | null) | null>(null)

  useEffect(() => {
    loadJsLang().then(ext => setJsExtensions([ext]))
  }, [])

  const [editForm, setEditForm] = useState({
    name: '',
    sku: '',
    type: 'static' as string,
    script: '',
    script_config: '',
    description: '',
    total_limit: 0,
    is_active: true,
    notes: ''
  })
  const [isFormLoaded, setIsFormLoaded] = useState(false)

  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [importType, setImportType] = useState<'file' | 'text'>('text')
  const [textContent, setTextContent] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)

  const [manualDialogOpen, setManualDialogOpen] = useState(false)
  const [manualContent, setManualContent] = useState('')
  const [manualRemark, setManualRemark] = useState('')

  const { data: inventoryData, isLoading: inventoryLoading, refetch: refetchInventory } = useQuery({
    queryKey: ['virtualInventory', inventoryId],
    queryFn: () => getVirtualInventory(inventoryId),
    enabled: !!inventoryId,
  })

  if (inventoryData?.data && !isFormLoaded) {
    const inv = inventoryData.data
    setEditForm({
      name: inv.name || '',
      sku: inv.sku || '',
      type: inv.type || 'static',
      script: inv.script || '',
      script_config: inv.script_config || '',
      description: inv.description || '',
      total_limit: inv.total_limit || 0,
      is_active: inv.is_active ?? true,
      notes: inv.notes || ''
    })
    setIsFormLoaded(true)
  }

  const { data: stocksData, isLoading: stocksLoading, refetch: refetchStocks } = useQuery({
    queryKey: ['virtualInventoryStocks', inventoryId, page, limit, statusFilter],
    queryFn: () => getVirtualInventoryStocks(inventoryId, {
      page,
      limit,
      status: statusFilter === 'all' ? undefined : statusFilter
    }),
    enabled: !!inventoryId,
  })

  const updateMutation = useMutation({
    mutationFn: (data: typeof editForm) => updateVirtualInventory(inventoryId, data),
    onSuccess: () => {
      toast.success(t.admin.saveSuccess)
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.saveFailed}: ${error.message}`)
    },
  })

  const importMutation = useMutation({
    mutationFn: (data: { import_type: 'file' | 'text'; file?: File; content?: string }) =>
      importVirtualInventoryStock(inventoryId, data),
    onSuccess: (response: any) => {
      toast.success(t.admin.importSuccessCount.replace('{count}', String(response?.data?.count || 0)))
      setImportDialogOpen(false)
      setTextContent('')
      setSelectedFile(null)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.importFailedMsg}: ${error.message}`)
    },
  })

  const manualCreateMutation = useMutation({
    mutationFn: (data: { content: string; remark?: string }) =>
      createVirtualInventoryStockManually(inventoryId, data),
    onSuccess: () => {
      toast.success(t.admin.addSuccess)
      setManualDialogOpen(false)
      setManualContent('')
      setManualRemark('')
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.addFailed}: ${error.message}`)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (stockId: number) => deleteVirtualInventoryStock(inventoryId, stockId),
    onSuccess: () => {
      toast.success(t.admin.deleteSuccessMsg)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.deleteFailedMsg}: ${error.message}`)
    },
  })

  const reserveMutation = useMutation({
    mutationFn: (stockId: number) => reserveVirtualInventoryStock(inventoryId, stockId),
    onSuccess: () => {
      toast.success(t.admin.reserveSuccess)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.reserveFailed}: ${error.message}`)
    },
  })

  const releaseMutation = useMutation({
    mutationFn: (stockId: number) => releaseVirtualInventoryStock(inventoryId, stockId),
    onSuccess: () => {
      toast.success(t.admin.releaseSuccess)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.releaseFailed}: ${error.message}`)
    },
  })

  const testMutation = useMutation({
    mutationFn: ({ script, config, quantity }: { script: string; config: Record<string, any>; quantity: number }) =>
      testDeliveryScript(script, config, quantity),
    onSuccess: (data: any) => {
      setTestResult(data?.data)
      toast.success(t.admin.scriptTestSuccess)
    },
    onError: (error: Error) => {
      setTestResult({ error: error.message })
      toast.error(`${t.admin.scriptTestFailed}: ${error.message}`)
    },
  })

  const inventory = inventoryData?.data
  const stocks = stocksData?.data?.items || []
  const total = stocksData?.data?.pagination?.total || 0
  const totalPages = Math.ceil(total / limit)

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      const validTypes = ['.xlsx', '.xls', '.csv', '.txt']
      const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()
      if (!validTypes.includes(ext)) {
        toast.error(t.admin.onlySupportedFormatsError)
        return
      }
      setSelectedFile(file)
    }
  }

  const handleImport = () => {
    if (importType === 'text') {
      if (!textContent.trim()) {
        toast.error(t.admin.pleaseInputContent)
        return
      }
      importMutation.mutate({ import_type: 'text', content: textContent })
    } else {
      if (!selectedFile) {
        toast.error(t.admin.pleaseSelectFile)
        return
      }
      importMutation.mutate({ import_type: 'file', file: selectedFile })
    }
  }

  const handleManualCreate = () => {
    if (!manualContent.trim()) {
      toast.error(t.admin.pleaseInputCardKey)
      return
    }
    manualCreateMutation.mutate({ content: manualContent, remark: manualRemark })
  }

  const handleSave = () => {
    if (!editForm.name.trim()) {
      toast.error(t.admin.pleaseInputInventoryName)
      return
    }
    if (editForm.type === 'script' && !editForm.script.trim()) {
      toast.error(t.admin.scriptPlaceholder)
      return
    }
    // Flush any pending config editor changes
    const flushed = configFlushRef.current?.()
    const formToSave = flushed ? { ...editForm, script_config: flushed } : editForm
    updateMutation.mutate(formToSave)
  }

  const handleTest = () => {
    if (!editForm.script.trim()) {
      toast.error(t.admin.scriptPlaceholder)
      return
    }
    // Flush any pending config editor changes
    const flushed = configFlushRef.current?.()
    const configStr = flushed || editForm.script_config
    let config: Record<string, any> = {}
    try {
      config = JSON.parse(configStr || '{}')
    } catch {
      // use empty config
    }
    testMutation.mutate({ script: editForm.script, config, quantity: testQuantity })
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'available':
        return <Badge variant="default">{t.admin.statusAvailable}</Badge>
      case 'reserved':
        return <Badge variant="secondary">{t.admin.statusReserved}</Badge>
      case 'sold':
        return <Badge variant="outline">{t.admin.statusSold}</Badge>
      case 'invalid':
        return <Badge variant="destructive">{t.admin.statusInvalid}</Badge>
      default:
        return <Badge variant="outline">{status}</Badge>
    }
  }

  if (inventoryLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4" />
          <p className="text-muted-foreground">{t.common.loading}</p>
        </div>
      </div>
    )
  }

  if (!inventory) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t.admin.virtualNotExist}</p>
        <Button asChild className="mt-4">
          <Link href="/admin/inventories?tab=virtual">{t.admin.backToList}</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="sm" asChild>
            <Link href="/admin/inventories?tab=virtual">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.common.back}</span>
            </Link>
          </Button>
          <div>
            <h1 className="text-lg md:text-xl font-bold flex items-center gap-2">
              {inventory.name}
              {inventory.type === 'script' && (
                <Badge variant="outline" className="text-xs text-purple-600 dark:text-purple-400 border-purple-300 dark:border-purple-700">
                  <Code2 className="h-3 w-3 mr-1" />
                  {t.admin.scriptTypeTag}
                </Badge>
              )}
            </h1>
            <p className="text-muted-foreground">{t.admin.virtualInventoryEditSubtitle}</p>
          </div>
        </div>
        <div className="flex gap-2">
          {inventory.type !== 'script' && (
            <>
              <Button variant="outline" onClick={() => setManualDialogOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                {t.admin.addCardKeyBtn}
              </Button>
              <Button variant="outline" onClick={() => setImportDialogOpen(true)}>
                <Upload className="mr-2 h-4 w-4" />
                {t.admin.batchImportBtn}
              </Button>
            </>
          )}
        </div>
      </div>

      {editForm.type === 'script' ? (
        <div className={`grid gap-4 ${inventory.total_limit > 0 ? 'grid-cols-2 md:grid-cols-3' : 'grid-cols-2'}`}>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                {inventory.total_limit > 0 ? inventory.total_limit : t.admin.scriptUnlimited}
              </div>
              <p className="text-sm text-muted-foreground">{t.admin.scriptDeliveryLimit}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-gray-500 dark:text-gray-400">{inventory.sold || 0}</div>
              <p className="text-sm text-muted-foreground">{t.admin.statusSold}</p>
            </CardContent>
          </Card>
          {inventory.total_limit > 0 && (
            <Card>
              <CardContent className="pt-6">
                <div className={`text-2xl font-bold ${
                  (inventory.total_limit - (inventory.sold || 0)) <= 0
                    ? 'text-red-600 dark:text-red-400'
                    : 'text-green-600 dark:text-green-400'
                }`}>
                  {Math.max(0, inventory.total_limit - (inventory.sold || 0))}
                </div>
                <p className="text-sm text-muted-foreground">{t.admin.scriptDeliveryRemaining}</p>
              </CardContent>
            </Card>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold">{inventory.total || 0}</div>
              <p className="text-sm text-muted-foreground">{t.admin.totalInventory}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-green-600 dark:text-green-400">{inventory.available || 0}</div>
              <p className="text-sm text-muted-foreground">{t.admin.statusAvailable}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">{inventory.reserved || 0}</div>
              <p className="text-sm text-muted-foreground">{t.admin.statusReserved}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-gray-500 dark:text-gray-400">{inventory.sold || 0}</div>
              <p className="text-sm text-muted-foreground">{t.admin.statusSold}</p>
            </CardContent>
          </Card>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            {t.admin.basicInfo}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className={`grid gap-4 ${editForm.type === 'script' ? 'grid-cols-3' : 'grid-cols-2'}`}>
            <div className="space-y-2">
              <Label htmlFor="name">{t.admin.inventoryNameRequired}</Label>
              <Input
                id="name"
                value={editForm.name}
                onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sku">SKU</Label>
              <Input
                id="sku"
                value={editForm.sku}
                onChange={(e) => setEditForm({ ...editForm, sku: e.target.value })}
              />
            </div>
            {editForm.type === 'script' && (
              <div className="space-y-2">
                <Label htmlFor="total_limit">{t.admin.scriptDeliveryLimit}</Label>
                <Input
                  id="total_limit"
                  type="number"
                  min={0}
                  placeholder="0"
                  value={editForm.total_limit}
                  onChange={(e) => setEditForm({ ...editForm, total_limit: Math.max(0, parseInt(e.target.value) || 0) })}
                />
                <p className="text-xs text-muted-foreground">{t.admin.scriptDeliveryLimitDesc}</p>
              </div>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="description">{t.admin.descriptionLabel}</Label>
            <Textarea
              id="description"
              value={editForm.description}
              onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
              rows={2}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="notes">{t.admin.notesLabel}</Label>
            <Textarea
              id="notes"
              value={editForm.notes}
              onChange={(e) => setEditForm({ ...editForm, notes: e.target.value })}
              rows={2}
            />
          </div>
          <div className="flex items-center space-x-2">
            <Switch
              id="is_active"
              checked={editForm.is_active}
              onCheckedChange={(checked) => setEditForm({ ...editForm, is_active: checked })}
            />
            <Label htmlFor="is_active">{t.admin.activeStatusLabel}</Label>
          </div>
          <div className="flex justify-end">
            <Button onClick={handleSave} disabled={updateMutation.isPending}>
              <Save className="mr-2 h-4 w-4" />
              {updateMutation.isPending ? t.admin.savingText : t.common.save}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Script editing section (only for script type) */}
      {editForm.type === 'script' && (
        <>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Code2 className="h-5 w-5 text-purple-500" />
              {t.admin.scriptLabel}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>{t.admin.scriptLabel}</Label>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm">
                      <BookOpen className="h-3.5 w-3.5 mr-1.5" />
                      {t.admin.scriptExamples}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => {
                      setEditForm({ ...editForm, script: SCRIPT_EXAMPLE_BASIC })
                      toast.success(t.admin.scriptExampleInserted)
                    }}>
                      {t.admin.scriptExampleBasic}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => {
                      setEditForm({ ...editForm, script: SCRIPT_EXAMPLE_HTTP })
                      toast.success(t.admin.scriptExampleInserted)
                    }}>
                      {t.admin.scriptExampleHttp}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => {
                      setEditForm({ ...editForm, script: SCRIPT_EXAMPLE_ORDER })
                      toast.success(t.admin.scriptExampleInserted)
                    }}>
                      {t.admin.scriptExampleOrder}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => {
                      setEditForm({ ...editForm, script: SCRIPT_EXAMPLE_EMBY_REGISTER, script_config: SCRIPT_EXAMPLE_EMBY_CONFIG })
                      toast.success(t.admin.scriptExampleInserted)
                    }}>
                      {t.admin.scriptExampleEmby}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <div className="rounded-md border overflow-hidden">
                <CodeMirror
                  value={editForm.script}
                  extensions={jsExtensions}
                  onChange={(v: string) => setEditForm({ ...editForm, script: v })}
                  height="300px"
                  theme={cmTheme}
                  placeholder={t.admin.scriptPlaceholder}
                  className="text-sm"
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label>{t.admin.scriptConfigLabel}</Label>
              <p className="text-xs text-muted-foreground">{t.admin.scriptConfigDesc}</p>
              <ConfigEditor
                value={editForm.script_config}
                onChange={(v) => setEditForm({ ...editForm, script_config: v })}
                flushRef={configFlushRef}
                labels={{
                  configJson: t.admin.scriptConfigJsonLabel,
                  configFields: t.admin.scriptConfigFieldsLabel,
                  jsonEditor: t.admin.scriptConfigJsonEditor,
                  visualEditor: t.admin.scriptConfigVisualEditor,
                  invalidJson: t.admin.scriptConfigJsonLabel,
                  noFields: t.admin.scriptConfigNoFields,
                  addField: t.admin.scriptConfigAddField,
                }}
                cmTheme={cmTheme}
              />
            </div>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Label>{t.admin.scriptTestQuantity}</Label>
                <Input
                  type="number"
                  min={1}
                  max={10}
                  value={testQuantity}
                  onChange={(e) => setTestQuantity(Math.max(1, Math.min(10, parseInt(e.target.value) || 1)))}
                  className="w-20"
                />
                <Button variant="outline" onClick={handleTest} disabled={testMutation.isPending}>
                  <Play className="h-4 w-4 mr-2" />
                  {testMutation.isPending ? t.admin.scriptTesting : t.admin.scriptTestBtn}
                </Button>
              </div>
              <Button onClick={handleSave} disabled={updateMutation.isPending}>
                <Save className="mr-2 h-4 w-4" />
                {updateMutation.isPending ? t.admin.savingText : t.common.save}
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Test Result */}
        {testResult && (
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">{t.admin.scriptTestResult}</CardTitle>
            </CardHeader>
            <CardContent>
              {testResult.error ? (
                <div className="text-sm text-destructive bg-destructive/10 p-3 rounded-md font-mono">
                  {testResult.error}
                </div>
              ) : testResult.items && testResult.items.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-sm text-muted-foreground">
                    {t.admin.scriptTestItems}: {testResult.items.length}
                  </p>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>#</TableHead>
                        <TableHead>{t.admin.scriptTestContent}</TableHead>
                        <TableHead>{t.admin.scriptTestRemark}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {testResult.items.map((item: any, i: number) => (
                        <TableRow key={i}>
                          <TableCell className="font-mono text-muted-foreground">{i + 1}</TableCell>
                          <TableCell className="font-mono max-w-md break-all">{item.content}</TableCell>
                          <TableCell className="text-sm text-muted-foreground">{item.remark || '-'}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">{t.admin.scriptTestNoItems}</p>
              )}
              {testResult.message && (
                <p className="text-sm mt-2 text-muted-foreground">{testResult.message}</p>
              )}
            </CardContent>
          </Card>
        )}

        {/* API Reference */}
        <Card className="bg-muted/50">
          <CardHeader>
            <CardTitle className="text-sm">{t.admin.scriptApiRef}</CardTitle>
            <CardDescription className="text-xs">
              {t.admin.scriptApiRefDesc}
            </CardDescription>
          </CardHeader>
          <CardContent className="text-xs space-y-3">
            <div>
              <p className="font-semibold mb-1">{t.admin.scriptRequiredCallback}</p>
              <p><code>onDeliver(order, config)</code> - {t.admin.scriptCallbackDesc}</p>
              <p className="text-muted-foreground ml-4">
                {t.admin.scriptReturns}
                <code>{`{success: true, items: [{content: "...", remark: "..."}]}`}</code>
              </p>
            </div>
            <div>
              <p className="font-semibold mb-1">AuraLogic.order <span className="font-normal text-muted-foreground">({t.admin.scriptOrderApi})</span></p>
              <p><code>get()</code> - {t.admin.scriptGetOrder}</p>
              <p><code>getItems()</code> - {t.admin.scriptGetOrderItems}</p>
              <p><code>getUser()</code> - {t.admin.scriptGetUser}</p>
            </div>
            <div>
              <p className="font-semibold mb-1">AuraLogic.utils <span className="font-normal text-muted-foreground">({t.admin.scriptUtilsApi})</span></p>
              <p><code>generateId()</code> / <code>jsonEncode(obj)</code> / <code>jsonDecode(str)</code> / <code>formatDate()</code></p>
            </div>
            <div>
              <p className="font-semibold mb-1">AuraLogic.http <span className="font-normal text-muted-foreground">({t.admin.scriptHttpApi})</span></p>
              <p><code>get(url, headers?)</code> / <code>post(url, body, headers?)</code></p>
            </div>
          </CardContent>
        </Card>
        </>
      )}

      {editForm.type !== 'script' && (
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <FileText className="h-5 w-5" />
                {t.admin.stockItemList}
              </CardTitle>
              <CardDescription>
                {t.admin.totalRecordsCount.replace('{count}', String(total))}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.admin.allStatusFilter}</SelectItem>
                  <SelectItem value="available">{t.admin.statusAvailable}</SelectItem>
                  <SelectItem value="reserved">{t.admin.statusReserved}</SelectItem>
                  <SelectItem value="sold">{t.admin.statusSold}</SelectItem>
                  <SelectItem value="invalid">{t.admin.statusInvalid}</SelectItem>
                </SelectContent>
              </Select>
              <Button variant="outline" onClick={() => refetchStocks()}>
                <RefreshCw className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {stocksLoading ? (
            <div className="text-center py-8">{t.common.loading}</div>
          ) : stocks.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>{t.admin.noStockItems}</p>
              <p className="text-sm mt-2">{t.admin.noStockItemsHint}</p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>{t.admin.contentColumn}</TableHead>
                    <TableHead>{t.admin.remarkColumn}</TableHead>
                    <TableHead>{t.admin.statusColumn}</TableHead>
                    <TableHead>{t.admin.orderNoColumn}</TableHead>
                    <TableHead>{t.admin.batchNoColumn}</TableHead>
                    <TableHead>{t.admin.createdAtColumn}</TableHead>
                    <TableHead>{t.admin.operationsColumn}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stocks.map((stock: any) => (
                    <TableRow key={stock.id}>
                      <TableCell className="font-mono">{stock.id}</TableCell>
                      <TableCell className="font-mono max-w-xs truncate" title={stock.content}>
                        {stock.content}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground max-w-xs truncate">
                        {stock.remark || '-'}
                      </TableCell>
                      <TableCell>{getStatusBadge(stock.status)}</TableCell>
                      <TableCell className="text-sm">
                        {stock.order_no || '-'}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {stock.batch_no || '-'}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {new Date(stock.created_at).toLocaleString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          {stock.status === 'available' && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => reserveMutation.mutate(stock.id)}
                                disabled={reserveMutation.isPending}
                                title={t.admin.reserve}
                              >
                                <Lock className="h-3 w-3" />
                              </Button>
                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button size="sm" variant="destructive" title={t.common.delete}>
                                    <Trash2 className="h-3 w-3" />
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>{t.admin.confirmDeleteTitle}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      {t.admin.confirmDeleteStockItem}
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                                    <AlertDialogAction onClick={() => deleteMutation.mutate(stock.id)}>
                                      {t.common.delete}
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            </>
                          )}
                          {stock.status === 'reserved' && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => releaseMutation.mutate(stock.id)}
                                disabled={releaseMutation.isPending}
                                title={t.admin.release}
                              >
                                <Unlock className="h-3 w-3" />
                              </Button>
                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button size="sm" variant="destructive" title={t.common.delete}>
                                    <Trash2 className="h-3 w-3" />
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>{t.admin.confirmDeleteTitle}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      {t.admin.confirmDeleteStockItem}
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                                    <AlertDialogAction onClick={() => deleteMutation.mutate(stock.id)}>
                                      {t.common.delete}
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            </>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {total > 0 && (
                <div className="flex items-center justify-between mt-4">
                  <div className="text-sm text-muted-foreground">
                    {t.admin.paginationInfo
                      .replace('{total}', String(total))
                      .replace('{page}', String(page))
                      .replace('{totalPages}', String(totalPages))}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(1)}
                      disabled={page <= 1}
                    >
                      {t.admin.firstPage}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(p => Math.max(1, p - 1))}
                      disabled={page <= 1}
                    >
                      {t.admin.prevPageBtn}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(p => p + 1)}
                      disabled={page >= totalPages}
                    >
                      {t.admin.nextPageBtn}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(totalPages)}
                      disabled={page >= totalPages}
                    >
                      {t.admin.lastPage}
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
      )}

      <Dialog open={importDialogOpen} onOpenChange={setImportDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t.admin.batchImportTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.batchImportDesc}
            </DialogDescription>
          </DialogHeader>

          <Tabs value={importType} onValueChange={(v) => setImportType(v as 'file' | 'text')}>
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="text">
                <FileText className="w-4 h-4 mr-2" />
                {t.admin.textInputTab}
              </TabsTrigger>
              <TabsTrigger value="file">
                <Upload className="w-4 h-4 mr-2" />
                {t.admin.fileUploadTab}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="text" className="space-y-4">
              <div>
                <Textarea
                  placeholder={t.admin.textInputPlaceholderEdit}
                  value={textContent}
                  onChange={(e) => setTextContent(e.target.value)}
                  rows={10}
                />
                <p className="text-sm text-muted-foreground mt-2">
                  {t.admin.textInputExampleLabel}<br />
                  ABCD-1234-EFGH<br />
                  WXYZ-5678-IJKL,VIP
                </p>
              </div>
            </TabsContent>

            <TabsContent value="file" className="space-y-4">
              <div
                className="border-2 border-dashed rounded-lg p-8 text-center cursor-pointer hover:border-primary"
                onClick={() => fileInputRef.current?.click()}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".xlsx,.xls,.csv,.txt"
                  onChange={handleFileChange}
                  className="hidden"
                />
                {selectedFile ? (
                  <div className="flex items-center justify-center gap-2">
                    <FileText className="w-6 h-6 text-primary" />
                    <span>{selectedFile.name}</span>
                  </div>
                ) : (
                  <>
                    <Upload className="w-8 h-8 mx-auto mb-2 text-muted-foreground" />
                    <p className="text-muted-foreground">{t.admin.clickSelectFile}</p>
                    <p className="text-sm text-muted-foreground">{t.admin.supportedFormatsText}</p>
                  </>
                )}
              </div>
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button variant="outline" onClick={() => setImportDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleImport} disabled={importMutation.isPending}>
              {importMutation.isPending ? (
                <>
                  <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                  {t.admin.importingText}
                </>
              ) : (
                t.admin.confirmImportBtn
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={manualDialogOpen} onOpenChange={setManualDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t.admin.addCardKeyDialogTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.addCardKeyDialogDesc}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="content">{t.admin.cardKeyContentLabel}</Label>
              <Input
                id="content"
                placeholder={t.admin.cardKeyContentInputPlaceholder}
                value={manualContent}
                onChange={(e) => setManualContent(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="remark">{t.admin.remarkOptionalLabel}</Label>
              <Input
                id="remark"
                placeholder={t.admin.remarkInputPlaceholder}
                value={manualRemark}
                onChange={(e) => setManualRemark(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setManualDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleManualCreate} disabled={manualCreateMutation.isPending}>
              {manualCreateMutation.isPending ? t.admin.addingText : t.admin.addBtn}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
