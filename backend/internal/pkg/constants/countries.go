package constants

// Country 国家Info
type Country struct {
	Code   string `json:"code"`    // 国家代码 (ISO 3166-1 alpha-2)
	NameZH string `json:"name_zh"` // 中文名称
	NameEN string `json:"name_en"` // 英文名称
}

// Countries 全球国家/地区列表（ISO 3166-1 标准，按常用程度和地区排序）
var Countries = []Country{
	// 常用国家/地区（置顶）
	{Code: "CN", NameZH: "中国", NameEN: "China"},
	{Code: "HK", NameZH: "中国香港", NameEN: "Hong Kong SAR"},
	{Code: "MO", NameZH: "中国澳门", NameEN: "Macao SAR"},
	{Code: "TW", NameZH: "中国台湾", NameEN: "Taiwan"},
	{Code: "US", NameZH: "美国", NameEN: "United States"},
	{Code: "CA", NameZH: "加拿大", NameEN: "Canada"},
	{Code: "GB", NameZH: "英国", NameEN: "United Kingdom"},
	{Code: "JP", NameZH: "日本", NameEN: "Japan"},
	{Code: "KR", NameZH: "韩国", NameEN: "South Korea"},
	{Code: "SG", NameZH: "新加坡", NameEN: "Singapore"},
	{Code: "AU", NameZH: "澳大利亚", NameEN: "Australia"},

	// 亚洲其他国家
	{Code: "AF", NameZH: "阿富汗", NameEN: "Afghanistan"},
	{Code: "BD", NameZH: "孟加拉国", NameEN: "Bangladesh"},
	{Code: "BN", NameZH: "文莱", NameEN: "Brunei"},
	{Code: "BT", NameZH: "不丹", NameEN: "Bhutan"},
	{Code: "KH", NameZH: "柬埔寨", NameEN: "Cambodia"},
	{Code: "ID", NameZH: "印度尼西亚", NameEN: "Indonesia"},
	{Code: "IN", NameZH: "印度", NameEN: "India"},
	{Code: "IQ", NameZH: "伊拉克", NameEN: "Iraq"},
	{Code: "IR", NameZH: "伊朗", NameEN: "Iran"},
	{Code: "IL", NameZH: "以色列", NameEN: "Israel"},
	{Code: "JO", NameZH: "约旦", NameEN: "Jordan"},
	{Code: "KZ", NameZH: "哈萨克斯坦", NameEN: "Kazakhstan"},
	{Code: "KW", NameZH: "科威特", NameEN: "Kuwait"},
	{Code: "KG", NameZH: "吉尔吉斯斯坦", NameEN: "Kyrgyzstan"},
	{Code: "LA", NameZH: "老挝", NameEN: "Laos"},
	{Code: "LB", NameZH: "黎巴嫩", NameEN: "Lebanon"},
	{Code: "MY", NameZH: "马来西亚", NameEN: "Malaysia"},
	{Code: "MV", NameZH: "马尔代夫", NameEN: "Maldives"},
	{Code: "MN", NameZH: "蒙古", NameEN: "Mongolia"},
	{Code: "MM", NameZH: "缅甸", NameEN: "Myanmar"},
	{Code: "NP", NameZH: "尼泊尔", NameEN: "Nepal"},
	{Code: "OM", NameZH: "阿曼", NameEN: "Oman"},
	{Code: "PK", NameZH: "巴基斯坦", NameEN: "Pakistan"},
	{Code: "PS", NameZH: "巴勒斯坦", NameEN: "Palestine"},
	{Code: "PH", NameZH: "菲律宾", NameEN: "Philippines"},
	{Code: "QA", NameZH: "卡塔尔", NameEN: "Qatar"},
	{Code: "SA", NameZH: "沙特阿拉伯", NameEN: "Saudi Arabia"},
	{Code: "LK", NameZH: "斯里兰卡", NameEN: "Sri Lanka"},
	{Code: "SY", NameZH: "叙利亚", NameEN: "Syria"},
	{Code: "TJ", NameZH: "塔吉克斯坦", NameEN: "Tajikistan"},
	{Code: "TH", NameZH: "泰国", NameEN: "Thailand"},
	{Code: "TL", NameZH: "东帝汶", NameEN: "Timor-Leste"},
	{Code: "TR", NameZH: "土耳其", NameEN: "Turkey"},
	{Code: "TM", NameZH: "土库曼斯坦", NameEN: "Turkmenistan"},
	{Code: "AE", NameZH: "阿联酋", NameEN: "United Arab Emirates"},
	{Code: "UZ", NameZH: "乌兹别克斯坦", NameEN: "Uzbekistan"},
	{Code: "VN", NameZH: "越南", NameEN: "Vietnam"},
	{Code: "YE", NameZH: "也门", NameEN: "Yemen"},

	// 欧洲
	{Code: "AL", NameZH: "阿尔巴尼亚", NameEN: "Albania"},
	{Code: "AD", NameZH: "安道尔", NameEN: "Andorra"},
	{Code: "AM", NameZH: "亚美尼亚", NameEN: "Armenia"},
	{Code: "AT", NameZH: "奥地利", NameEN: "Austria"},
	{Code: "AZ", NameZH: "阿塞拜疆", NameEN: "Azerbaijan"},
	{Code: "BY", NameZH: "白俄罗斯", NameEN: "Belarus"},
	{Code: "BE", NameZH: "比利时", NameEN: "Belgium"},
	{Code: "BA", NameZH: "波黑", NameEN: "Bosnia and Herzegovina"},
	{Code: "BG", NameZH: "保加利亚", NameEN: "Bulgaria"},
	{Code: "HR", NameZH: "克罗地亚", NameEN: "Croatia"},
	{Code: "CY", NameZH: "塞浦路斯", NameEN: "Cyprus"},
	{Code: "CZ", NameZH: "捷克", NameEN: "Czech Republic"},
	{Code: "DK", NameZH: "丹麦", NameEN: "Denmark"},
	{Code: "EE", NameZH: "爱沙尼亚", NameEN: "Estonia"},
	{Code: "FI", NameZH: "芬兰", NameEN: "Finland"},
	{Code: "FR", NameZH: "法国", NameEN: "France"},
	{Code: "GE", NameZH: "格鲁吉亚", NameEN: "Georgia"},
	{Code: "DE", NameZH: "德国", NameEN: "Germany"},
	{Code: "GR", NameZH: "希腊", NameEN: "Greece"},
	{Code: "HU", NameZH: "匈牙利", NameEN: "Hungary"},
	{Code: "IS", NameZH: "冰岛", NameEN: "Iceland"},
	{Code: "IE", NameZH: "爱尔兰", NameEN: "Ireland"},
	{Code: "IT", NameZH: "意大利", NameEN: "Italy"},
	{Code: "LV", NameZH: "拉脱维亚", NameEN: "Latvia"},
	{Code: "LI", NameZH: "列支敦士登", NameEN: "Liechtenstein"},
	{Code: "LT", NameZH: "立陶宛", NameEN: "Lithuania"},
	{Code: "LU", NameZH: "卢森堡", NameEN: "Luxembourg"},
	{Code: "MT", NameZH: "马耳他", NameEN: "Malta"},
	{Code: "MD", NameZH: "摩尔多瓦", NameEN: "Moldova"},
	{Code: "MC", NameZH: "摩纳哥", NameEN: "Monaco"},
	{Code: "ME", NameZH: "黑山", NameEN: "Montenegro"},
	{Code: "NL", NameZH: "荷兰", NameEN: "Netherlands"},
	{Code: "MK", NameZH: "北马其顿", NameEN: "North Macedonia"},
	{Code: "NO", NameZH: "挪威", NameEN: "Norway"},
	{Code: "PL", NameZH: "波兰", NameEN: "Poland"},
	{Code: "PT", NameZH: "葡萄牙", NameEN: "Portugal"},
	{Code: "RO", NameZH: "罗马尼亚", NameEN: "Romania"},
	{Code: "RU", NameZH: "俄罗斯", NameEN: "Russia"},
	{Code: "SM", NameZH: "圣马力诺", NameEN: "San Marino"},
	{Code: "RS", NameZH: "塞尔维亚", NameEN: "Serbia"},
	{Code: "SK", NameZH: "斯洛伐克", NameEN: "Slovakia"},
	{Code: "SI", NameZH: "斯洛文尼亚", NameEN: "Slovenia"},
	{Code: "ES", NameZH: "西班牙", NameEN: "Spain"},
	{Code: "SE", NameZH: "瑞典", NameEN: "Sweden"},
	{Code: "CH", NameZH: "瑞士", NameEN: "Switzerland"},
	{Code: "UA", NameZH: "乌克兰", NameEN: "Ukraine"},
	{Code: "VA", NameZH: "梵蒂冈", NameEN: "Vatican City"},

	// 北美洲
	{Code: "AG", NameZH: "安提瓜和巴布达", NameEN: "Antigua and Barbuda"},
	{Code: "BS", NameZH: "巴哈马", NameEN: "Bahamas"},
	{Code: "BB", NameZH: "巴巴多斯", NameEN: "Barbados"},
	{Code: "BZ", NameZH: "伯利兹", NameEN: "Belize"},
	{Code: "CR", NameZH: "哥斯达黎加", NameEN: "Costa Rica"},
	{Code: "CU", NameZH: "古巴", NameEN: "Cuba"},
	{Code: "DM", NameZH: "多米尼克", NameEN: "Dominica"},
	{Code: "DO", NameZH: "多米尼加", NameEN: "Dominican Republic"},
	{Code: "SV", NameZH: "萨尔瓦多", NameEN: "El Salvador"},
	{Code: "GD", NameZH: "格林纳达", NameEN: "Grenada"},
	{Code: "GT", NameZH: "危地马拉", NameEN: "Guatemala"},
	{Code: "HT", NameZH: "海地", NameEN: "Haiti"},
	{Code: "HN", NameZH: "洪都拉斯", NameEN: "Honduras"},
	{Code: "JM", NameZH: "牙买加", NameEN: "Jamaica"},
	{Code: "MX", NameZH: "墨西哥", NameEN: "Mexico"},
	{Code: "NI", NameZH: "尼加拉瓜", NameEN: "Nicaragua"},
	{Code: "PA", NameZH: "巴拿马", NameEN: "Panama"},
	{Code: "KN", NameZH: "圣基茨和尼维斯", NameEN: "Saint Kitts and Nevis"},
	{Code: "LC", NameZH: "圣卢西亚", NameEN: "Saint Lucia"},
	{Code: "VC", NameZH: "圣文森特和格林纳丁斯", NameEN: "Saint Vincent and the Grenadines"},
	{Code: "TT", NameZH: "特立尼达和多巴哥", NameEN: "Trinidad and Tobago"},

	// 南美洲
	{Code: "AR", NameZH: "阿根廷", NameEN: "Argentina"},
	{Code: "BO", NameZH: "玻利维亚", NameEN: "Bolivia"},
	{Code: "BR", NameZH: "巴西", NameEN: "Brazil"},
	{Code: "CL", NameZH: "智利", NameEN: "Chile"},
	{Code: "CO", NameZH: "哥伦比亚", NameEN: "Colombia"},
	{Code: "EC", NameZH: "厄瓜多尔", NameEN: "Ecuador"},
	{Code: "GY", NameZH: "圭亚那", NameEN: "Guyana"},
	{Code: "PY", NameZH: "巴拉圭", NameEN: "Paraguay"},
	{Code: "PE", NameZH: "秘鲁", NameEN: "Peru"},
	{Code: "SR", NameZH: "苏里南", NameEN: "Suriname"},
	{Code: "UY", NameZH: "乌拉圭", NameEN: "Uruguay"},
	{Code: "VE", NameZH: "委内瑞拉", NameEN: "Venezuela"},

	// 大洋洲
	{Code: "FJ", NameZH: "斐济", NameEN: "Fiji"},
	{Code: "KI", NameZH: "基里巴斯", NameEN: "Kiribati"},
	{Code: "MH", NameZH: "马绍尔群岛", NameEN: "Marshall Islands"},
	{Code: "FM", NameZH: "密克罗尼西亚", NameEN: "Micronesia"},
	{Code: "NR", NameZH: "瑙鲁", NameEN: "Nauru"},
	{Code: "NZ", NameZH: "新西兰", NameEN: "New Zealand"},
	{Code: "PW", NameZH: "帕劳", NameEN: "Palau"},
	{Code: "PG", NameZH: "巴布亚新几内亚", NameEN: "Papua New Guinea"},
	{Code: "WS", NameZH: "萨摩亚", NameEN: "Samoa"},
	{Code: "SB", NameZH: "所罗门群岛", NameEN: "Solomon Islands"},
	{Code: "TO", NameZH: "汤加", NameEN: "Tonga"},
	{Code: "TV", NameZH: "图瓦卢", NameEN: "Tuvalu"},
	{Code: "VU", NameZH: "瓦努阿图", NameEN: "Vanuatu"},

	// 非洲
	{Code: "DZ", NameZH: "阿尔及利亚", NameEN: "Algeria"},
	{Code: "AO", NameZH: "安哥拉", NameEN: "Angola"},
	{Code: "BJ", NameZH: "贝宁", NameEN: "Benin"},
	{Code: "BW", NameZH: "博茨瓦纳", NameEN: "Botswana"},
	{Code: "BF", NameZH: "布基纳法索", NameEN: "Burkina Faso"},
	{Code: "BI", NameZH: "布隆迪", NameEN: "Burundi"},
	{Code: "CM", NameZH: "喀麦隆", NameEN: "Cameroon"},
	{Code: "CV", NameZH: "佛得角", NameEN: "Cape Verde"},
	{Code: "CF", NameZH: "中非", NameEN: "Central African Republic"},
	{Code: "TD", NameZH: "乍得", NameEN: "Chad"},
	{Code: "KM", NameZH: "科摩罗", NameEN: "Comoros"},
	{Code: "CG", NameZH: "刚果（布）", NameEN: "Congo"},
	{Code: "CD", NameZH: "刚果（金）", NameEN: "Congo (DRC)"},
	{Code: "CI", NameZH: "科特迪瓦", NameEN: "Côte d'Ivoire"},
	{Code: "DJ", NameZH: "吉布提", NameEN: "Djibouti"},
	{Code: "EG", NameZH: "埃及", NameEN: "Egypt"},
	{Code: "GQ", NameZH: "赤道几内亚", NameEN: "Equatorial Guinea"},
	{Code: "ER", NameZH: "厄立特里亚", NameEN: "Eritrea"},
	{Code: "ET", NameZH: "埃塞俄比亚", NameEN: "Ethiopia"},
	{Code: "GA", NameZH: "加蓬", NameEN: "Gabon"},
	{Code: "GM", NameZH: "冈比亚", NameEN: "Gambia"},
	{Code: "GH", NameZH: "加纳", NameEN: "Ghana"},
	{Code: "GN", NameZH: "几内亚", NameEN: "Guinea"},
	{Code: "GW", NameZH: "几内亚比绍", NameEN: "Guinea-Bissau"},
	{Code: "KE", NameZH: "肯尼亚", NameEN: "Kenya"},
	{Code: "LS", NameZH: "莱索托", NameEN: "Lesotho"},
	{Code: "LR", NameZH: "利比里亚", NameEN: "Liberia"},
	{Code: "LY", NameZH: "利比亚", NameEN: "Libya"},
	{Code: "MG", NameZH: "马达加斯加", NameEN: "Madagascar"},
	{Code: "MW", NameZH: "马拉维", NameEN: "Malawi"},
	{Code: "ML", NameZH: "马里", NameEN: "Mali"},
	{Code: "MR", NameZH: "毛里塔尼亚", NameEN: "Mauritania"},
	{Code: "MU", NameZH: "毛里求斯", NameEN: "Mauritius"},
	{Code: "MA", NameZH: "摩洛哥", NameEN: "Morocco"},
	{Code: "MZ", NameZH: "莫桑比克", NameEN: "Mozambique"},
	{Code: "NA", NameZH: "纳米比亚", NameEN: "Namibia"},
	{Code: "NE", NameZH: "尼日尔", NameEN: "Niger"},
	{Code: "NG", NameZH: "尼日利亚", NameEN: "Nigeria"},
	{Code: "RW", NameZH: "卢旺达", NameEN: "Rwanda"},
	{Code: "ST", NameZH: "圣多美和普林西比", NameEN: "São Tomé and Príncipe"},
	{Code: "SN", NameZH: "塞内加尔", NameEN: "Senegal"},
	{Code: "SC", NameZH: "塞舌尔", NameEN: "Seychelles"},
	{Code: "SL", NameZH: "塞拉利昂", NameEN: "Sierra Leone"},
	{Code: "SO", NameZH: "索马里", NameEN: "Somalia"},
	{Code: "ZA", NameZH: "南非", NameEN: "South Africa"},
	{Code: "SS", NameZH: "南苏丹", NameEN: "South Sudan"},
	{Code: "SD", NameZH: "苏丹", NameEN: "Sudan"},
	{Code: "SZ", NameZH: "斯威士兰", NameEN: "Eswatini"},
	{Code: "TZ", NameZH: "坦桑尼亚", NameEN: "Tanzania"},
	{Code: "TG", NameZH: "多哥", NameEN: "Togo"},
	{Code: "TN", NameZH: "突尼斯", NameEN: "Tunisia"},
	{Code: "UG", NameZH: "乌干达", NameEN: "Uganda"},
	{Code: "ZM", NameZH: "赞比亚", NameEN: "Zambia"},
	{Code: "ZW", NameZH: "津巴布韦", NameEN: "Zimbabwe"},
}

// GetCountryByCode 根据代码get国家Info
func GetCountryByCode(code string) *Country {
	for _, c := range Countries {
		if c.Code == code {
			return &c
		}
	}
	return nil
}

// GetCountryNameZH 根据代码get中文名称
func GetCountryNameZH(code string) string {
	country := GetCountryByCode(code)
	if country != nil {
		return country.NameZH
	}
	return code
}

// GetCountryNameEN 根据代码get英文名称
func GetCountryNameEN(code string) string {
	country := GetCountryByCode(code)
	if country != nil {
		return country.NameEN
	}
	return code
}
