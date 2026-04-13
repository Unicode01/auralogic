import * as Recharts from 'recharts'

import {
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
} from './lazy-recharts'

describe('lazy-recharts', () => {
  it('re-exports the original recharts components', () => {
    expect(Bar).toBe(Recharts.Bar)
    expect(BarChart).toBe(Recharts.BarChart)
    expect(CartesianGrid).toBe(Recharts.CartesianGrid)
    expect(Cell).toBe(Recharts.Cell)
    expect(Legend).toBe(Recharts.Legend)
    expect(Line).toBe(Recharts.Line)
    expect(LineChart).toBe(Recharts.LineChart)
    expect(Pie).toBe(Recharts.Pie)
    expect(PieChart).toBe(Recharts.PieChart)
    expect(ResponsiveContainer).toBe(Recharts.ResponsiveContainer)
    expect(Tooltip).toBe(Recharts.Tooltip)
    expect(XAxis).toBe(Recharts.XAxis)
    expect(YAxis).toBe(Recharts.YAxis)
  })
})
