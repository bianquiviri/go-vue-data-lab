package generator

// Static pools used to build realistic-looking synthetic records. All strings
// are plain ASCII so generated emails/URLs stay portable.

var firstNames = []string{
	"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda",
	"David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph",
	"Jessica", "Thomas", "Sarah", "Charles", "Karen", "Christopher", "Nancy",
	"Daniel", "Lisa", "Matthew", "Betty", "Anthony", "Margaret", "Mark", "Sandra",
	"Donald", "Ashley", "Steven", "Kimberly", "Paul", "Emily", "Andrew", "Donna",
	"Joshua", "Michelle", "Kenneth", "Carol", "Kevin", "Amanda", "Brian",
	"Dorothy", "George", "Melissa", "Edward", "Deborah", "Ronald", "Stephanie",
	"Timothy", "Rebecca", "Jason", "Sharon", "Jeffrey", "Laura", "Ryan", "Cynthia",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson",
	"Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez",
	"Thompson", "White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis",
	"Robinson", "Walker", "Young", "Allen", "King", "Wright", "Scott", "Torres",
	"Nguyen", "Hill", "Flores", "Green", "Adams", "Nelson", "Baker", "Hall",
	"Rivera", "Campbell", "Mitchell", "Carter", "Roberts", "Gomez", "Phillips",
	"Evans", "Turner", "Diaz", "Parker", "Cruz", "Edwards", "Collins", "Reyes",
}

// location couples a country code with a city/state, keeping customers and
// their shipping addresses coherent.
type location struct {
	CountryCode string
	City        string
	State       string
}

var locations = []location{
	{"US", "New York", "NY"},
	{"US", "Los Angeles", "CA"},
	{"US", "Chicago", "IL"},
	{"US", "Houston", "TX"},
	{"US", "Phoenix", "AZ"},
	{"US", "Seattle", "WA"},
	{"US", "Boston", "MA"},
	{"US", "Denver", "CO"},
	{"CA", "Toronto", "ON"},
	{"CA", "Vancouver", "BC"},
	{"CA", "Montreal", "QC"},
	{"GB", "London", "ENG"},
	{"GB", "Manchester", "ENG"},
	{"GB", "Edinburgh", "SCT"},
	{"DE", "Berlin", "BE"},
	{"DE", "Munich", "BY"},
	{"DE", "Hamburg", "HH"},
	{"FR", "Paris", "IDF"},
	{"FR", "Lyon", "ARA"},
	{"ES", "Madrid", "MD"},
	{"ES", "Barcelona", "CT"},
	{"MX", "Mexico City", "CDMX"},
	{"MX", "Guadalajara", "JAL"},
	{"BR", "Sao Paulo", "SP"},
	{"BR", "Rio de Janeiro", "RJ"},
	{"AU", "Sydney", "NSW"},
	{"AU", "Melbourne", "VIC"},
	{"IN", "Mumbai", "MH"},
	{"IN", "Bangalore", "KA"},
	{"JP", "Tokyo", "13"},
}

var emailDomains = []string{
	"example.com", "mail.example.com", "inbox.example.net", "shop.example.org",
}

var productAdjectives = []string{
	"Compact", "Premium", "Wireless", "Ergonomic", "Portable", "Eco", "Smart",
	"Classic", "Ultra", "Deluxe", "Rugged", "Slim", "Pro", "Essential", "Advanced",
}

// categoryNouns maps a category to the nouns/products that belong to it so that
// generated product names are plausible.
var categoryNouns = map[string][]string{
	"Electronics":    {"Headphones", "Speaker", "Monitor", "Keyboard", "Mouse", "Camera", "Tablet", "Charger"},
	"Home & Kitchen": {"Blender", "Kettle", "Cookware Set", "Food Processor", "Toaster", "Knife Set"},
	"Sports":         {"Yoga Mat", "Dumbbell Set", "Water Bottle", "Tennis Racket", "Bike Helmet", "Backpack"},
	"Toys":           {"Building Blocks", "Puzzle", "Action Figure", "Board Game", "RC Car", "Plush Toy"},
	"Books":          {"Notebook", "Cookbook", "Guide", "Novel", "Journal", "Workbook"},
	"Clothing":       {"T-Shirt", "Hoodie", "Jacket", "Sneakers", "Jeans", "Cap"},
	"Beauty":         {"Moisturizer", "Shampoo", "Perfume", "Lip Balm", "Face Mask", "Serum"},
	"Automotive":     {"Phone Mount", "Dash Camera", "Tire Pump", "Seat Cover", "Jump Starter"},
	"Garden":         {"Pruner", "Watering Can", "Planter", "Garden Hose", "Seed Kit", "Rake"},
	"Grocery":        {"Coffee Beans", "Olive Oil", "Green Tea", "Granola", "Pasta", "Honey"},
}

// productCategories is the ordered list of categories (stable ordering keeps
// generation deterministic across map iterations).
var productCategories = []string{
	"Electronics", "Home & Kitchen", "Sports", "Toys", "Books", "Clothing",
	"Beauty", "Automotive", "Garden", "Grocery",
}

var orderStatuses = []string{
	"pending", "pending", "paid", "paid", "paid", "shipped", "shipped",
	"delivered", "delivered", "cancelled", "refunded",
}

var currencies = []string{
	"USD", "USD", "USD", "USD", "EUR", "GBP", "CAD", "MXN",
}

var streetNames = []string{
	"Main St", "Oak Ave", "Maple Rd", "Cedar Ln", "Pine St", "Elm St", "Park Ave",
	"Lake Dr", "Hill Rd", "Sunset Blvd", "First St", "Second Ave", "Broadway",
	"Market St", "Church St",
}

// productStatusWeighted keeps most products active with a few discontinued.
var productStatuses = []string{
	"active", "active", "active", "active", "active", "active", "active",
	"active", "active", "inactive", "discontinued",
}
