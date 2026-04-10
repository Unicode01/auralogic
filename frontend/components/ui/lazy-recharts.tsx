'use client'

import dynamic from 'next/dynamic'
import type { ComponentProps, ComponentType } from 'react'

type RechartsModule = typeof import('recharts')

type RechartsComponentKey =
  | 'Bar'
  | 'BarChart'
  | 'CartesianGrid'
  | 'Cell'
  | 'Legend'
  | 'Line'
  | 'LineChart'
  | 'Pie'
  | 'PieChart'
  | 'ResponsiveContainer'
  | 'Tooltip'
  | 'XAxis'
  | 'YAxis'

type RechartsComponentProps<Key extends RechartsComponentKey> = ComponentProps<
  RechartsModule[Key]
>

type LazyChartComponent<Key extends RechartsComponentKey> = ComponentType<
  RechartsComponentProps<Key>
>

function lazyRechartsComponent<Key extends RechartsComponentKey>(exportName: Key) {
  return dynamic<RechartsComponentProps<Key>>(
    () =>
      import('recharts').then(
        (mod) => mod[exportName] as unknown as LazyChartComponent<Key>
      ),
    { ssr: false }
  )
}

export const Bar = lazyRechartsComponent('Bar')
export const BarChart = lazyRechartsComponent('BarChart')
export const CartesianGrid = lazyRechartsComponent('CartesianGrid')
export const Cell = lazyRechartsComponent('Cell')
export const Legend = lazyRechartsComponent('Legend')
export const Line = lazyRechartsComponent('Line')
export const LineChart = lazyRechartsComponent('LineChart')
export const Pie = lazyRechartsComponent('Pie')
export const PieChart = lazyRechartsComponent('PieChart')
export const ResponsiveContainer = lazyRechartsComponent('ResponsiveContainer')
export const Tooltip = lazyRechartsComponent('Tooltip')
export const XAxis = lazyRechartsComponent('XAxis')
export const YAxis = lazyRechartsComponent('YAxis')
