package e2e_test

type CompanyInfo struct {
	Ticker   string
	Name     string
	CIK      string
	Industry string
}

var FiscalYears = []string{"2020", "2021", "2022", "2023", "2024", "2025"}

// CompanyUniverse contains 100 S&P 500 companies for batch processing
var CompanyUniverse = []CompanyInfo{
	// Technology (20 companies)
	{"AAPL", "Apple Inc.", "0000320193", "Technology"},
	{"MSFT", "Microsoft Corporation", "0000789019", "Technology"},
	{"GOOGL", "Alphabet Inc.", "0001652044", "Technology"},
	{"AMZN", "Amazon.com Inc.", "0001018724", "Technology"},
	{"NVDA", "NVIDIA Corporation", "0001045810", "Technology"},
	{"META", "Meta Platforms Inc.", "0001326801", "Technology"},
	{"TSLA", "Tesla Inc.", "0001318605", "Technology"},
	{"AVGO", "Broadcom Inc.", "0001730168", "Technology"},
	{"ORCL", "Oracle Corporation", "0001341439", "Technology"},
	{"CRM", "Salesforce Inc.", "0001108524", "Technology"},
	{"AMD", "Advanced Micro Devices Inc.", "0000002488", "Technology"},
	{"ADBE", "Adobe Inc.", "0000796343", "Technology"},
	{"INTC", "Intel Corporation", "0000050863", "Technology"},
	{"CSCO", "Cisco Systems Inc.", "0000858877", "Technology"},
	{"IBM", "International Business Machines", "0000051143", "Technology"},
	{"QCOM", "QUALCOMM Inc.", "0000804328", "Technology"},
	{"TXN", "Texas Instruments Inc.", "0000097476", "Technology"},
	{"NOW", "ServiceNow Inc.", "0001373715", "Technology"},
	{"INTU", "Intuit Inc.", "0000896878", "Technology"},
	{"AMAT", "Applied Materials Inc.", "0000006951", "Technology"},

	// Healthcare (15 companies)
	{"JNJ", "Johnson & Johnson", "0000200406", "Healthcare"},
	{"UNH", "UnitedHealth Group Inc.", "0000731766", "Healthcare"},
	{"LLY", "Eli Lilly and Company", "0000059478", "Healthcare"},
	{"PFE", "Pfizer Inc.", "0000078003", "Healthcare"},
	{"MRK", "Merck & Co. Inc.", "0000310158", "Healthcare"},
	{"ABBV", "AbbVie Inc.", "0001551152", "Healthcare"},
	{"TMO", "Thermo Fisher Scientific", "0000097745", "Healthcare"},
	{"ABT", "Abbott Laboratories", "0000001800", "Healthcare"},
	{"DHR", "Danaher Corporation", "0000313616", "Healthcare"},
	{"BMY", "Bristol-Myers Squibb", "0000014272", "Healthcare"},
	{"AMGN", "Amgen Inc.", "0000318154", "Healthcare"},
	{"MDT", "Medtronic plc", "0001613103", "Healthcare"},
	{"GILD", "Gilead Sciences Inc.", "0000882095", "Healthcare"},
	{"CVS", "CVS Health Corporation", "0000064803", "Healthcare"},
	{"ISRG", "Intuitive Surgical Inc.", "0001035267", "Healthcare"},

	// Financial Services (15 companies)
	{"JPM", "JPMorgan Chase & Co.", "0000019617", "Financial"},
	{"V", "Visa Inc.", "0001403161", "Financial"},
	{"MA", "Mastercard Inc.", "0001141391", "Financial"},
	{"BAC", "Bank of America Corp", "0000070858", "Financial"},
	{"WFC", "Wells Fargo & Company", "0000072971", "Financial"},
	{"GS", "The Goldman Sachs Group", "0000886982", "Financial"},
	{"MS", "Morgan Stanley", "0000895421", "Financial"},
	{"BLK", "BlackRock Inc.", "0001364742", "Financial"},
	{"C", "Citigroup Inc.", "0000831001", "Financial"},
	{"AXP", "American Express Company", "0000004962", "Financial"},
	{"SCHW", "Charles Schwab Corp", "0000316709", "Financial"},
	{"SPGI", "S&P Global Inc.", "0000064040", "Financial"},
	{"CME", "CME Group Inc.", "0001156375", "Financial"},
	{"ICE", "Intercontinental Exchange", "0001571949", "Financial"},
	{"MCO", "Moody's Corporation", "0001059556", "Financial"},

	// Consumer (15 companies)
	{"WMT", "Walmart Inc.", "0000104169", "Consumer"},
	{"PG", "Procter & Gamble Company", "0000080424", "Consumer"},
	{"KO", "The Coca-Cola Company", "0000021344", "Consumer"},
	{"PEP", "PepsiCo Inc.", "0000077476", "Consumer"},
	{"COST", "Costco Wholesale Corp", "0000909832", "Consumer"},
	{"HD", "The Home Depot Inc.", "0000354950", "Consumer"},
	{"MCD", "McDonald's Corporation", "0000063908", "Consumer"},
	{"NKE", "NIKE Inc.", "0000320187", "Consumer"},
	{"SBUX", "Starbucks Corporation", "0000829224", "Consumer"},
	{"TGT", "Target Corporation", "0000027419", "Consumer"},
	{"LOW", "Lowe's Companies Inc.", "0000060667", "Consumer"},
	{"DIS", "The Walt Disney Company", "0001744489", "Consumer"},
	{"NFLX", "Netflix Inc.", "0001065280", "Consumer"},
	{"BKNG", "Booking Holdings Inc.", "0001075531", "Consumer"},
	{"CMG", "Chipotle Mexican Grill", "0001058090", "Consumer"},

	// Energy (10 companies)
	{"XOM", "ExxonMobil Corporation", "0000034088", "Energy"},
	{"CVX", "Chevron Corporation", "0000093410", "Energy"},
	{"COP", "ConocoPhillips", "0001163165", "Energy"},
	{"SLB", "Schlumberger Limited", "0000087347", "Energy"},
	{"EOG", "EOG Resources Inc.", "0000821189", "Energy"},
	{"PXD", "Pioneer Natural Resources", "0001038357", "Energy"},
	{"MPC", "Marathon Petroleum Corp", "0001510295", "Energy"},
	{"PSX", "Phillips 66", "0001534701", "Energy"},
	{"VLO", "Valero Energy Corporation", "0001035002", "Energy"},
	{"OXY", "Occidental Petroleum", "0000797468", "Energy"},

	// Industrials (10 companies)
	{"CAT", "Caterpillar Inc.", "0000018230", "Industrial"},
	{"RTX", "RTX Corporation", "0000101829", "Industrial"},
	{"HON", "Honeywell International", "0000773840", "Industrial"},
	{"UNP", "Union Pacific Corporation", "0000100885", "Industrial"},
	{"BA", "The Boeing Company", "0000012927", "Industrial"},
	{"GE", "General Electric Company", "0000040545", "Industrial"},
	{"LMT", "Lockheed Martin Corp", "0000936468", "Industrial"},
	{"DE", "Deere & Company", "0000315189", "Industrial"},
	{"UPS", "United Parcel Service", "0001090727", "Industrial"},
	{"MMM", "3M Company", "0000066740", "Industrial"},

	// Utilities (5 companies)
	{"NEE", "NextEra Energy Inc.", "0000753308", "Utilities"},
	{"DUK", "Duke Energy Corporation", "0001326160", "Utilities"},
	{"SO", "The Southern Company", "0000092122", "Utilities"},
	{"D", "Dominion Energy Inc.", "0000715957", "Utilities"},
	{"AEP", "American Electric Power", "0000004904", "Utilities"},

	// Real Estate (5 companies)
	{"AMT", "American Tower Corporation", "0001053507", "REIT"},
	{"PLD", "Prologis Inc.", "0001045609", "REIT"},
	{"CCI", "Crown Castle Inc.", "0001051470", "REIT"},
	{"EQIX", "Equinix Inc.", "0001101239", "REIT"},
	{"SPG", "Simon Property Group", "0001063761", "REIT"},

	// Materials (3 companies)
	{"LIN", "Linde plc", "0001707925", "Materials"},
	{"APD", "Air Products and Chemicals", "0000002969", "Materials"},
	{"SHW", "The Sherwin-Williams Company", "0000089800", "Materials"},

	// Communications (2 companies)
	{"T", "AT&T Inc.", "0000732717", "Communications"},
	{"VZ", "Verizon Communications Inc.", "0000732712", "Communications"},
}
