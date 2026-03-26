import {
  Activity,
  BarChart3,
  Bell,
  BookOpen,
  Bot,
  Boxes,
  Briefcase,
  Building2,
  Calendar,
  HelpCircle,
  Code,
  Cog,
  CreditCard,
  Database,
  FileText,
  Gauge,
  Globe,
  Key,
  LayoutDashboard,
  LifeBuoy,
  Link as LinkIcon,
  Lock,
  Megaphone,
  MessageSquare,
  Package,
  Puzzle,
  Send,
  Settings,
  Shield,
  ShieldCheck,
  ShoppingBag,
  ShoppingCart,
  Star,
  Tag,
  Truck,
  User,
  Users,
  Wallet,
  Warehouse,
  Wrench,
  Zap,
  type LucideIcon,
} from 'lucide-react'

const pluginMenuIconEntries: Array<[string, LucideIcon]> = [
  ['activity', Activity],
  ['analytics', BarChart3],
  ['bag', ShoppingBag],
  ['barchart3', BarChart3],
  ['bell', Bell],
  ['bookopen', BookOpen],
  ['bot', Bot],
  ['boxes', Boxes],
  ['briefcase', Briefcase],
  ['building2', Building2],
  ['calendar', Calendar],
  ['cart', ShoppingCart],
  ['circlehelp', HelpCircle],
  ['code', Code],
  ['cog', Cog],
  ['creditcard', CreditCard],
  ['dashboard', LayoutDashboard],
  ['database', Database],
  ['filetext', FileText],
  ['gauge', Gauge],
  ['globe', Globe],
  ['helpcircle', HelpCircle],
  ['key', Key],
  ['layoutdashboard', LayoutDashboard],
  ['lifebuoy', LifeBuoy],
  ['link', LinkIcon],
  ['lock', Lock],
  ['megaphone', Megaphone],
  ['messagesquare', MessageSquare],
  ['package', Package],
  ['puzzle', Puzzle],
  ['send', Send],
  ['settings', Settings],
  ['shield', Shield],
  ['shieldcheck', ShieldCheck],
  ['shoppingbag', ShoppingBag],
  ['shoppingcart', ShoppingCart],
  ['star', Star],
  ['support', LifeBuoy],
  ['tag', Tag],
  ['truck', Truck],
  ['user', User],
  ['users', Users],
  ['wallet', Wallet],
  ['warehouse', Warehouse],
  ['wrench', Wrench],
  ['zap', Zap],
]

const pluginMenuIconMap = pluginMenuIconEntries.reduce<Record<string, LucideIcon>>(
  (acc, [name, icon]) => {
    acc[name] = icon
    return acc
  },
  {}
)

function normalizePluginMenuIconName(iconName?: string | null): string {
  return String(iconName || '')
    .trim()
    .replace(/[-_\s]+/g, '')
    .toLowerCase()
}

export function resolvePluginMenuIcon(iconName?: string | null): LucideIcon {
  const normalized = normalizePluginMenuIconName(iconName)
  if (!normalized) return Puzzle
  return pluginMenuIconMap[normalized] || Puzzle
}
