// Global phone country code map (extracted from shipping-form for reuse)
export const phoneCodeMap: { [key: string]: string } = {
  // Asia
  'CN': '+86', 'HK': '+852', 'MO': '+853', 'TW': '+886',
  'JP': '+81', 'KR': '+82', 'SG': '+65', 'MY': '+60',
  'TH': '+66', 'VN': '+84', 'ID': '+62', 'PH': '+63',
  'IN': '+91', 'PK': '+92', 'BD': '+880', 'LK': '+94',
  'MM': '+95', 'KH': '+855', 'LA': '+856', 'BN': '+673',
  'MV': '+960', 'NP': '+977', 'BT': '+975', 'MN': '+976',
  'AF': '+93', 'IQ': '+964', 'IR': '+98', 'IL': '+972',
  'JO': '+962', 'KW': '+965', 'SA': '+966', 'AE': '+971',
  'QA': '+974', 'OM': '+968', 'YE': '+967', 'SY': '+963',
  'LB': '+961', 'PS': '+970', 'TR': '+90', 'KZ': '+7',
  'UZ': '+998', 'TM': '+993', 'KG': '+996', 'TJ': '+992',
  'AM': '+374', 'AZ': '+994', 'GE': '+995', 'TL': '+670',

  // Europe
  'GB': '+44', 'FR': '+33', 'DE': '+49', 'IT': '+39',
  'ES': '+34', 'PT': '+351', 'NL': '+31', 'BE': '+32',
  'CH': '+41', 'AT': '+43', 'SE': '+46', 'NO': '+47',
  'DK': '+45', 'FI': '+358', 'IS': '+354', 'IE': '+353',
  'PL': '+48', 'CZ': '+420', 'SK': '+421', 'HU': '+36',
  'RO': '+40', 'BG': '+359', 'GR': '+30', 'HR': '+385',
  'SI': '+386', 'RS': '+381', 'BA': '+387', 'ME': '+382',
  'MK': '+389', 'AL': '+355', 'UA': '+380', 'BY': '+375',
  'MD': '+373', 'RU': '+7', 'EE': '+372', 'LV': '+371',
  'LT': '+370', 'CY': '+357', 'MT': '+356', 'LU': '+352',
  'MC': '+377', 'AD': '+376', 'SM': '+378', 'VA': '+379',
  'LI': '+423',

  // North America
  'US': '+1', 'CA': '+1', 'MX': '+52',
  'GT': '+502', 'BZ': '+501', 'SV': '+503', 'HN': '+504',
  'NI': '+505', 'CR': '+506', 'PA': '+507', 'CU': '+53',
  'JM': '+1', 'HT': '+509', 'DO': '+1', 'BS': '+1',
  'BB': '+1', 'TT': '+1', 'AG': '+1', 'DM': '+1',
  'GD': '+1', 'KN': '+1', 'LC': '+1', 'VC': '+1',

  // South America
  'BR': '+55', 'AR': '+54', 'CL': '+56', 'CO': '+57',
  'PE': '+51', 'VE': '+58', 'EC': '+593', 'BO': '+591',
  'PY': '+595', 'UY': '+598', 'GY': '+592', 'SR': '+597',

  // Oceania
  'AU': '+61', 'NZ': '+64', 'FJ': '+679', 'PG': '+675',
  'SB': '+677', 'VU': '+678', 'NC': '+687', 'PF': '+689',
  'WS': '+685', 'TO': '+676', 'KI': '+686', 'FM': '+691',
  'MH': '+692', 'PW': '+680', 'NR': '+674', 'TV': '+688',

  // Africa
  'EG': '+20', 'ZA': '+27', 'NG': '+234', 'KE': '+254',
  'ET': '+251', 'TZ': '+255', 'UG': '+256', 'DZ': '+213',
  'MA': '+212', 'TN': '+216', 'LY': '+218', 'SD': '+249',
  'SS': '+211', 'GH': '+233', 'CI': '+225', 'SN': '+221',
  'CM': '+237', 'AO': '+244', 'MZ': '+258', 'MG': '+261',
  'ZW': '+263', 'ZM': '+260', 'MW': '+265', 'BW': '+267',
  'NA': '+264', 'LS': '+266', 'SZ': '+268', 'MU': '+230',
  'SC': '+248', 'RW': '+250', 'BI': '+257', 'DJ': '+253',
  'ER': '+291', 'SO': '+252', 'GA': '+241', 'CG': '+242',
  'CD': '+243', 'CF': '+236', 'TD': '+235', 'NE': '+227',
  'ML': '+223', 'BF': '+226', 'SL': '+232', 'LR': '+231',
  'GM': '+220', 'GN': '+224', 'GW': '+245', 'MR': '+222',
  'BJ': '+229', 'TG': '+228', 'GQ': '+240', 'CV': '+238',
  'ST': '+239', 'KM': '+269',
}

// Deduplicated list of phone codes sorted for dropdown use
export const phoneCodeList: { code: string; label: string }[] = (() => {
  const codeToCountries = new Map<string, string[]>()
  for (const [country, code] of Object.entries(phoneCodeMap)) {
    const existing = codeToCountries.get(code)
    if (existing) {
      existing.push(country)
    } else {
      codeToCountries.set(code, [country])
    }
  }
  const list: { code: string; label: string }[] = []
  for (const [code, countries] of codeToCountries) {
    const label = countries.length <= 3
      ? `${countries.join('/')} ${code}`
      : `${countries.slice(0, 2).join('/')}+${countries.length - 2} ${code}`
    list.push({ code, label })
  }
  list.sort((a, b) => {
    const na = parseInt(a.code.replace('+', ''))
    const nb = parseInt(b.code.replace('+', ''))
    return na - nb
  })
  return list
})()
