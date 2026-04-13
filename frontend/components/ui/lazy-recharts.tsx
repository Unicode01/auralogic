'use client'

// Re-export the real Recharts components from a single client module so
// component identity remains stable for helpers like `findAllByType(..., Cell)`.
export {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
