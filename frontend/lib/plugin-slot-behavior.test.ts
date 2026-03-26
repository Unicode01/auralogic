import {
  coercePluginSlotAnimation,
  resolvePluginGlobalSlotAnimationsEnabled,
  resolvePluginGlobalSlotLoadingEnabled,
  resolvePluginPlatformEnabled,
  resolvePluginPageSlotAnimation,
  resolvePluginSlotAnimationDefault,
} from '@/lib/plugin-slot-behavior'

describe('plugin slot behavior helpers', () => {
  it('coerces animation flags from common scalar values', () => {
    expect(coercePluginSlotAnimation(true, false)).toBe(true)
    expect(coercePluginSlotAnimation(false, true)).toBe(false)
    expect(coercePluginSlotAnimation('off', true)).toBe(false)
    expect(coercePluginSlotAnimation('yes', false)).toBe(true)
    expect(coercePluginSlotAnimation(0, true)).toBe(false)
    expect(coercePluginSlotAnimation(1, false)).toBe(true)
    expect(coercePluginSlotAnimation(undefined, false)).toBe(false)
  })

  it('resolves page-level slot animation overrides', () => {
    expect(
      resolvePluginPageSlotAnimation(
        {
          slot_animate: false,
        },
        'top',
        true
      )
    ).toBe(false)

    expect(
      resolvePluginPageSlotAnimation(
        {
          top_slot_animate: false,
          bottom_slot_animate: true,
        },
        'top',
        true
      )
    ).toBe(false)

    expect(
      resolvePluginPageSlotAnimation(
        {
          top_slot_animate: false,
          bottom_slot_animate: true,
        },
        'bottom',
        false
      )
    ).toBe(true)
  })

  it('supports nested slot animation config and slot-specific priority', () => {
    expect(
      resolvePluginPageSlotAnimation(
        {
          slot_animate: true,
          slots: {
            animate: false,
            top: {
              animate: true,
            },
          },
        },
        'top',
        false
      )
    ).toBe(true)

    expect(
      resolvePluginPageSlotAnimation(
        {
          slot_animate: true,
          slots: {
            animate: false,
          },
        },
        'bottom',
        true
      )
    ).toBe(false)
  })

  it('resolves the global slot animation setting from public config', () => {
    expect(
      resolvePluginGlobalSlotAnimationsEnabled(
        {
          plugin: {
            frontend: {
              slot_animations_enabled: false,
            },
          },
        },
        true
      )
    ).toBe(false)

    expect(resolvePluginGlobalSlotAnimationsEnabled({}, true)).toBe(true)
  })

  it('disables global slot loading when slot animations are disabled', () => {
    expect(
      resolvePluginGlobalSlotLoadingEnabled(
        {
          plugin: {
            frontend: {
              slot_animations_enabled: false,
            },
          },
        },
        true
      )
    ).toBe(false)

    expect(resolvePluginGlobalSlotLoadingEnabled({}, true)).toBe(true)
  })

  it('resolves the global plugin platform enabled flag from public config', () => {
    expect(
      resolvePluginPlatformEnabled(
        {
          plugin: {
            enabled: false,
          },
        },
        true
      )
    ).toBe(false)

    expect(resolvePluginPlatformEnabled({}, true)).toBe(true)
  })

  it('lets explicit local animation settings override the global default', () => {
    expect(resolvePluginSlotAnimationDefault(undefined, false)).toBe(false)
    expect(resolvePluginSlotAnimationDefault(true, false)).toBe(true)
    expect(resolvePluginSlotAnimationDefault(false, true)).toBe(false)
  })
})
