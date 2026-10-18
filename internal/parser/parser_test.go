package parser

import (
	"strings"
	"testing"
	"time"
)

func TestParseExamples(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string // fields that must match; "-" means must be absent
	}{
		{"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70", map[Field]string{
			Province: "Алматы", City: "Алматы", District: "Ауэзовский", LivingArea: "-", StreetType: "микрорайон",
			StreetName: "Аксай 3а", House: "59", Apartment: "70", Entrance: "-", Floor: "-", Landmark: "-"}},
		{"66квартал 12дом 69квартира", map[Field]string{
			StreetType: "квартал", StreetName: "66", House: "12", Apartment: "69"}},
		{"Ул .Курильская 9 кв 1", map[Field]string{
			StreetType: "улица", StreetName: "Курильская", House: "9", Apartment: "1"}},
		{"Туран 55/6 магазин береке 4подьезд 9этаж", map[Field]string{
			StreetName: "Туран", House: "55/6", Landmark: "магазин береке", Entrance: "4", Floor: "9", Apartment: "-"}},
		{"Нурсити 2 микрорайон 11дом 5 подьезд 239 кв", map[Field]string{
			StreetType: "микрорайон", StreetName: "Нурсити 2", House: "11", Entrance: "5", Apartment: "239"}},
		// a district is returned as written, without "район"; its city and region come from the directory
		{"Астана қаласы, Сарыарка ауданы, Кенесары 40", map[Field]string{City: "Астана", District: "Сарыарка", StreetName: "Кенесары"}},
		{"г. Астана, Есильский район, ул. Сығанақ 10", map[Field]string{City: "Астана", District: "Есильский"}},
		{"Аастана алматы ауд. Туран пр 47", map[Field]string{City: "Астана", District: "алматы", StreetName: "Туран", House: "47"}},
		{"Алматы, Түрксіб ауданы/Турксибский район, Рыбинский 4", map[Field]string{City: "Алматы", District: "Түрксіб"}},
		{"Турксибский район, Рыбинский переулок, дом 4", map[Field]string{
			District: "Турксибский", City: "Алматы", Province: "Алматы", StreetType: "переулок", StreetName: "Рыбинский", House: "4"}},
		{"Кулманова 133", map[Field]string{StreetName: "Кулманова", House: "133", StreetType: "-"}},
		{"жуалы 19", map[Field]string{StreetName: "жуалы", House: "19"}},
		{"город Актобе/Ақтөбе қаласы, Назарбаева8, кв 12", map[Field]string{
			City: "Актобе", Province: "Актюбинская область", StreetName: "Назарбаева", House: "8", Apartment: "12"}},
		{"Кулманова 133 flat 191", map[Field]string{StreetName: "Кулманова", House: "133", Apartment: "191"}},
		{"ул Абая 10 kv 12", map[Field]string{StreetType: "улица", StreetName: "Абая", House: "10", Apartment: "12"}},
		{"Абай көшесі 5 үй 7 пәтер", map[Field]string{StreetType: "улица", StreetName: "Абай", House: "5", Apartment: "7"}},
		{"г.Алматы, пр. Достык 1/2, кв 5", map[Field]string{
			City: "Алматы", StreetType: "проспект", StreetName: "Достык", House: "1/2", Apartment: "5"}},
		{"Боралдай, ул Ленина 12а", map[Field]string{
			City: "Боралдай", Province: "Алматинская область", District: "Илийский", StreetName: "Ленина", House: "12а"}},
		{"Qazpost Солнечный, Мира 7", map[Field]string{Landmark: "Qazpost Солнечный", StreetName: "Мира", House: "7"}},
		{"​ул. Сатпаева​ 22, кв­ 3", map[Field]string{StreetName: "Сатпаева", House: "22", Apartment: "3"}},
		{"Карагандинская обл, г. Темиртау, ул. Мира 4, кв. 15", map[Field]string{
			Province: "Карагандинская область", City: "Темиртау", StreetName: "Мира", House: "4", Apartment: "15"}},
		// review fixes: admin markers without commas, initials, postcodes, hyphenated microdistricts
		{"г. Алматы ул. Абая 10 кв 5", map[Field]string{City: "Алматы", StreetType: "улица", StreetName: "Абая", House: "10", Apartment: "5"}},
		{"г Алматы Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10"}},
		{"г.Шымкент ул.Байтурсынова 12 кв.4", map[Field]string{City: "Шымкент", StreetName: "Байтурсынова", House: "12", Apartment: "4"}},
		{"с.Иргели ул.Абая 5", map[Field]string{City: "Иргели", StreetName: "Абая", House: "5"}},
		{"Тұран ауданы Байтұрсынов көшесі 12", map[Field]string{District: "Тұран", City: "Шымкент", StreetName: "Байтұрсынов", House: "12"}},
		{"Карагандинская обл г. Темиртау ул. Мира 4", map[Field]string{Province: "Карагандинская область", City: "Темиртау", StreetName: "Мира", House: "4"}},
		{"Алматы, ул. Г. Потанина 3", map[Field]string{City: "Алматы", StreetType: "улица", StreetName: "Г. Потанина", House: "3"}},
		{"ул. Д. Кунаева 12, кв 5", map[Field]string{StreetType: "улица", StreetName: "Д. Кунаева", House: "12", Apartment: "5"}},
		// initials without a street type: a capital letter before a name that is not a place
		{"Алматы, С. Сейфуллина 45", map[Field]string{City: "Алматы", StreetName: "С. Сейфуллина", House: "45"}},
		{"Г. Мурата 5", map[Field]string{StreetName: "Г. Мурата", House: "5", City: "-"}},
		{"Д. Кунаева 12", map[Field]string{StreetName: "Д. Кунаева", House: "12"}},
		{"Г. Шымкнт, Тауке хана 20", map[Field]string{City: "Шымкент", StreetName: "Тауке хана", House: "20"}},
		{"Астана Ш. Коргалжынское 39", map[Field]string{StreetType: "шоссе", StreetName: "Коргалжынское", House: "39"}},
		{"КАРАГАНДЫ К. Мұстафин үй 20", map[Field]string{City: "Караганда", StreetName: "Мұстафин", House: "20"}},
		{"160000, г. Шымкент, ул. Байтурсынова 12", map[Field]string{City: "Шымкент", House: "12", Apartment: "-"}},
		{"050000 Алматы ул Абая 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "Абая", House: "10"}},
		// a quoted ЖК after the street type does not take the type away from the street
		{"Шымкент д 42/3 Темирлановское ш. Ж/К «Комфорт»", map[Field]string{StreetType: "шоссе", StreetName: "Темирлановское", LivingArea: "Комфорт", House: "42/3"}},
		// the new postcode after the old one, as 2GIS writes it
		{"Шымкент, улица Тауке хана, 12, 160011/X09F4F0", map[Field]string{City: "Шымкент", StreetName: "Тауке хана", House: "12"}},
		{"X09F4F0 Шымкент Тауке хана 12 кв 5", map[Field]string{City: "Шымкент", StreetName: "Тауке хана", House: "12", Apartment: "5"}},
		{"Шымкент, Абайский район, Шымкент городская администрация160011/X09F4F0", map[Field]string{City: "Шымкент", District: "Абайский", StreetName: "-", House: "-"}},
		{"Алматы, мкр Самал-2, д. 33, кв. 7", map[Field]string{StreetName: "Самал-2", House: "33", Apartment: "7", District: "Медеуский"}},
		{"Алматы, мкр Самал-2", map[Field]string{StreetName: "Самал-2", House: "-"}},
		// fast typing: typos in markers, no commas
		{"гролд актобе дом5 6 кв 55 подлъезд 5", map[Field]string{
			City: "Актобе", Province: "Актюбинская область", House: "5", Apartment: "55", Entrance: "5"}},
		{"алматы абая 15 кваритра 3", map[Field]string{City: "Алматы", StreetName: "абая", House: "15", Apartment: "3"}},
		{"Астана, проспкт Республики 24, этж 3", map[Field]string{City: "Астана", StreetType: "проспект", StreetName: "Республики", House: "24", Floor: "3", Apartment: "-"}},
		// typos in city names: canonical city + province
		{"г. алматыы, ул. Абая 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "Абая", House: "10"}},
		{"алматыы абая 10 кв 5", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "абая", House: "10", Apartment: "5"}},
		{"город Караганды, ул. Ерубаева 5", map[Field]string{City: "Караганда", Province: "Карагандинская область"}},
		{"г. Шымкнт, Тауке хана 20", map[Field]string{City: "Шымкент", Province: "Шымкент", StreetName: "Тауке хана", House: "20"}},
		// real street names close to marker words must stay names
		{"Райымбек 160", map[Field]string{StreetName: "Райымбек", House: "160"}},
		{"Гоголя 5", map[Field]string{StreetName: "Гоголя", House: "5"}},
		{"Магжана 7", map[Field]string{StreetName: "Магжана", House: "7"}},
		{"Сатпаева 22", map[Field]string{StreetName: "Сатпаева", House: "22", City: "-"}},
		{"Алматы, Сатпаева 22", map[Field]string{City: "Алматы", StreetName: "Сатпаева", House: "22"}},
		{"Алматы, Сатпаева, 22", map[Field]string{City: "Алматы", House: "22"}},
		{"Сатпаева, 22", map[Field]string{StreetName: "Сатпаева", House: "22", City: "-"}},
		{"шымкнет квартира 120 д 48", map[Field]string{City: "Шымкент", Apartment: "120", House: "48"}},
		{"алматыы, абая 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "абая", House: "10"}},
		{"Ауэзова 12", map[Field]string{StreetName: "Ауэзова", House: "12"}},
		{"ул. Сейфуллина 498", map[Field]string{StreetType: "улица", StreetName: "Сейфуллина", House: "498"}},
		// review: blocks, letter after a space, country prefix
		{"Абая 10 корпус 2 кв 5", map[Field]string{StreetName: "Абая", House: "10", Apartment: "5"}},
		{"ул Абая 10 а, кв 5", map[Field]string{StreetName: "Абая", House: "10а", Apartment: "5"}},
		{"ул Абая 10 в подъезде 2", map[Field]string{StreetName: "Абая", House: "10"}},
		{"Казахстан, г. Алматы, ул. Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10", Landmark: "-"}},
		{"РК, Алматы, Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10"}},
		// fast-typing set: Latin cities, duplicates, direction of "кв", ЖК, spaced fractions, regions
		{"almaty, abaya 52 kv 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "abaya", House: "52", Apartment: "10"}},
		{"алматы алматы жандосова 58 кв 4", map[Field]string{City: "Алматы", StreetName: "жандосова", House: "58", Apartment: "4"}},
		{"ул Сыганак 25 кв 300 4подъезд 12этаж", map[Field]string{StreetName: "Сыганак", House: "25", Apartment: "300", Entrance: "4", Floor: "12"}},
		{"Муканова 241 кв 34 5 этаж", map[Field]string{StreetName: "Муканова", House: "241", Apartment: "34", Floor: "5"}},
		{"Астана жк Северное сияние дом 7 кв 88", map[Field]string{StreetType: "жилой комплекс", StreetName: "Северное сияние", House: "7", LivingArea: "-"}},
		{"алматы жк Esentai City аль-фараби 77/7 кв 112", map[Field]string{LivingArea: "Esentai City", StreetName: "аль-фараби", House: "77/7"}},
		{"навои 208 / 2 кв 17", map[Field]string{StreetName: "навои", House: "208/2", Apartment: "17"}},
		{"шымкент микрорайн нурсат 112 кв 45", map[Field]string{StreetType: "микрорайон", StreetName: "нурсат", House: "112"}},
		{"усть-каменогорск пр казахстан 66 кв 20", map[Field]string{StreetType: "проспект", StreetName: "казахстан", House: "66"}},
		{"ВКО, Усть-Каман, ул. Протозанова 83 кв 5", map[Field]string{Province: "Восточно-Казахстанская область", City: "Усть-Каменогорск"}},
		{"Илийский район п Боралдай ул Аэродромная 14", map[Field]string{City: "Боралдай", District: "Илийский", StreetName: "Аэродромная"}},
		{"Астана Алматинский р-н Момышулы 23 кв 56", map[Field]string{District: "Алматинский", StreetName: "Момышулы", House: "23"}},
		{"Алматы мкр Орбита-1 дом 21 кв 30 напротив школы 12", map[Field]string{Landmark: "напротив школы 12", House: "21"}},
		{"Алматы улца Сейфуллина 498 кв 61", map[Field]string{StreetType: "улица", StreetName: "Сейфуллина"}},
		{"Астана р-н Нура ул Мухамедханова 8 кв 3", map[Field]string{City: "Астана", District: "Нура", StreetName: "Мухамедханова"}},
		// generic fixes: folded postfix markers, blocks, letters, Latin types, Kazakh landmarks and villages
		{"Акмолинская область Шортандинский район п Шортанды ул Целинная 8", map[Field]string{
			Province: "Акмолинская область", District: "Шортандинский", City: "Шортанды", StreetName: "Целинная", House: "8"}},
		{"Абая 89а блок Б, кв 211", map[Field]string{StreetName: "Абая", House: "89а", Apartment: "211"}},
		{"ул. Шевченко 162 ж кв 15", map[Field]string{House: "162ж", Apartment: "15"}},
		{"Шевченко 165, Алматы", map[Field]string{City: "Алматы", StreetName: "Шевченко", House: "165"}},
		{"г. Новый Узень, мкрн. Арай, д.2", map[Field]string{City: "Жанаозен", Province: "Мангистауская область", House: "2"}},
		{"Лениногорск, ул. Ленина 5", map[Field]string{City: "Риддер", Province: "Восточно-Казахстанская область"}},
		{"Шевченко 90 кв 5", map[Field]string{StreetName: "Шевченко", House: "90", Apartment: "5", City: "-"}},
		{"кв 5 и кв 6", map[Field]string{Apartment: "5"}},
		{"astana prospekt Tauelsizdik 41 kv 8", map[Field]string{City: "Астана", StreetType: "проспект", StreetName: "Tauelsizdik", House: "41"}},
		{"Казахстан Караганда пр Абдирова 17 кв 3", map[Field]string{City: "Караганда", StreetName: "Абдирова", House: "17"}},
		{"Ақтөбе, 12 ықшам аудан, 25 үй, 40 пәтер", map[Field]string{StreetType: "микрорайон", StreetName: "12", House: "25", Apartment: "40"}},
		{"Ауэзов көшесі 31, мешіттің жанында", map[Field]string{StreetName: "Ауэзов", House: "31", Landmark: "мешіттің жанында"}},
		// a street typed after the numbers, right before a Kazakh landmark: the name is the street
		{"Атырау кв 12 д 7 ул. Сатпаева мешіттің жанында", map[Field]string{
			StreetType: "улица", StreetName: "Сатпаева", House: "7", Apartment: "12", Landmark: "мешіттің жанында"}},
		{"Алматы 5 кв, дом 3, пр. Абая Small дүкенінің жанында", map[Field]string{
			StreetType: "проспект", StreetName: "Абая", House: "3", Landmark: "Small дүкенінің жанында"}},
		{"Астана кв 5 д 12 ул. Кабанбай батыра магнумның жанында", map[Field]string{StreetName: "Кабанбай батыра", Landmark: "магнумның жанында"}},
		// a number before a name typed after it starts a date or an anniversary name, unless a unit word takes it
		{"8 Марта ул 5", map[Field]string{StreetType: "улица", StreetName: "8 Марта", House: "5"}},
		{"Рудный 50 лет Октября улица 7", map[Field]string{StreetName: "50 лет Октября", House: "7"}},
		{"Павлодар кв 3 үй 29 Кривенко көшесі", map[Field]string{StreetName: "Кривенко", House: "29", Apartment: "3"}},
		// a street with its type, then a Kazakh microdistrict with its marker after the name
		{"Павлодар үй 12 Абай көшесі Самал шағын ауданы", map[Field]string{StreetName: "Абай", LivingArea: "Самал", House: "12"}},
		{"Шымкент дом 5 Байтурсынова пр-т Нурсат ықшам ауданы", map[Field]string{
			StreetType: "проспект", StreetName: "Байтурсынова", LivingArea: "Нурсат", House: "5"}},
		// a type after its name types that name
		{"Семей 3 кв үй 12 Абая көшесі Small дүкенінің жанында", map[Field]string{StreetName: "Абая", House: "12", Landmark: "Small дүкенінің жанында"}},
		{"Іле ауданы, Өтеген батыр ауылы, Нұрлы көшесі 17", map[Field]string{City: "Отеген батыр", StreetName: "Нұрлы", House: "17"}},
		{"Астана жк Экспо Бульвар д 3 кв 117", map[Field]string{StreetType: "жилой комплекс", StreetName: "Экспо Бульвар", House: "3"}},
		{"жк Хайвил ул Абая 5", map[Field]string{LivingArea: "Хайвил", StreetType: "улица", StreetName: "Абая", House: "5"}},
		// glued words without spaces
		{"Алматыабая10кв5", map[Field]string{City: "Алматы", StreetName: "абая", House: "10", Apartment: "5"}},
		{"ШымкентТаукеХана20", map[Field]string{City: "Шымкент", StreetName: "ТаукеХана", House: "20"}},
		{"Алматықаласыабая10", map[Field]string{City: "Алматы", StreetName: "абая", House: "10"}},
		{"Караганды23мкр10д5кв", map[Field]string{City: "Караганда", StreetType: "микрорайон", StreetName: "23", House: "10", Apartment: "5"}},
		{"городшымкент байтурсынова 12", map[Field]string{City: "Шымкент", StreetName: "байтурсынова", House: "12"}},
		{"Самалмкр 5 кв 2", map[Field]string{StreetType: "микрорайон", StreetName: "Самал", House: "5"}},
		{"ЖКХайвил ул Абая 5", map[Field]string{LivingArea: "Хайвил", StreetName: "Абая"}},
		{"балхашгордалимжанова49", map[Field]string{City: "Балхаш", StreetName: "алимжанова", House: "49"}},
		{"карагандықаласгоголя7уй", map[Field]string{City: "Караганда", StreetName: "гоголя", House: "7"}},
		{"асстанаакжол50кв9", map[Field]string{City: "Астана", StreetName: "акжол", House: "50", Apartment: "9"}},
		{"алматыул.брусиловского30кв97", map[Field]string{City: "Алматы", StreetName: "брусиловского", House: "30", Apartment: "97"}},
		{"Талдыкоргангород аба78д", map[Field]string{City: "Талдыкорган", StreetName: "аба", House: "78"}},
		{"актоб11мкр170уй", map[Field]string{City: "Актобе", StreetType: "микрорайон", StreetName: "11", House: "170"}},
		{"шымкентгагарина52 квартира59", map[Field]string{City: "Шымкент", StreetName: "гагарина", House: "52", Apartment: "59"}},
		{"алматыбогенбайбатыра41д136пәтер4кіреберіс", map[Field]string{House: "41", Apartment: "136", Entrance: "4"}},
		{"абылайхана д73кв.11под6эт4", map[Field]string{House: "73", Apartment: "11", Entrance: "6", Floor: "4"}},
		{"актобе 8шағынауданүй13кв23", map[Field]string{StreetType: "микрорайон", StreetName: "8", House: "13", Apartment: "23"}},
		// a microdistrict without "мкр", the number after a hyphen stays in the name
		{"Алматы аксай-3, 12, кв 5, у входа во двор", map[Field]string{
			City: "Алматы", StreetName: "аксай-3", House: "12", Apartment: "5", Landmark: "у входа во двор"}},
		{"Караганда восток-5 д 20", map[Field]string{StreetName: "восток-5", House: "20"}},
		{"Тараз жайлау-2", map[Field]string{City: "Тараз"}},
		// cases from the 10k review
		{"микрарайон 8 д 15, 286 кв", map[Field]string{StreetType: "микрорайон", StreetName: "8", House: "15", Apartment: "286"}},
		{"мкр.12 дом 117 эт 5, 140 офис", map[Field]string{StreetType: "микрорайон", StreetName: "12", House: "117", Floor: "5"}},
		{"Ақсай-1 үй 51б 104 пәтер 3 кабат", map[Field]string{StreetName: "Ақсай-1", House: "51б", Apartment: "104", Floor: "3"}},
		{"город Туркестан, микрорайон Жана кала, д.5/7", map[Field]string{City: "Туркестан", StreetType: "микрорайон", StreetName: "Жана кала", House: "5/7"}},
		{"с Отеген батыр ул.садовая д 36", map[Field]string{City: "Отеген батыр", StreetName: "садовая", House: "36"}},
		{"050013 мамыр-7, 53 / 5 УЙ", map[Field]string{StreetName: "мамыр-7", House: "53/5"}},
		{"Республика уй 33 корп 4, 28 патер", map[Field]string{StreetName: "Республика", House: "33", Apartment: "28"}},
		{"Республика Казахстан, Алматы, Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10"}},
		{"астана қаласы, коргалжын тас жолы 5", map[Field]string{StreetType: "шоссе", StreetName: "коргалжын", House: "5"}},
		{"Алматы, трасса Алматы-Бишкек 25", map[Field]string{StreetType: "шоссе", StreetName: "Алматы-Бишкек", House: "25"}},
		{"астана қаласы алматы ауданы абай даңғылы 126б", map[Field]string{District: "алматы", StreetType: "проспект", StreetName: "абай", House: "126б"}},
		{"Астана, мкр. кАбулак, 98", map[Field]string{City: "Астана", StreetName: "кАбулак", House: "98"}},
		{"проспект гагарина,72, кв 45, за рынком", map[Field]string{House: "72", Landmark: "за рынком"}},
		// a living area without "мкр"/"ЖК", examples from the case
		{"Караганда, Пришахтинск, ул. Ленина 5, кв 3", map[Field]string{City: "Караганда", LivingArea: "Пришахтинск", StreetName: "Ленина", House: "5", Apartment: "3"}},
		{"Астана, Томирис, Сыганак 10", map[Field]string{City: "Астана", LivingArea: "Томирис", StreetName: "Сыганак", House: "10"}},
		{"Астана, ЖК Томирис, Сыганак 10, кв 5", map[Field]string{LivingArea: "Томирис", StreetName: "Сыганак", House: "10", Apartment: "5"}},
		{"Алматы, Самал-2, ул. Жолдасбекова 97", map[Field]string{LivingArea: "Самал-2", StreetName: "Жолдасбекова", District: "Медеуский"}},
		// a known living area gives its city, the case lists "Пришахтинск" as a living area
		{"Пришахтинск, ул. Ленина 5", map[Field]string{City: "Караганда", Province: "Карагандинская область", LivingArea: "Пришахтинск", StreetName: "Ленина", House: "5"}},
		{"Карагандинская область, Пришахтинск, 21 мкр, д 5, кв 7", map[Field]string{
			Province: "Карагандинская область", City: "Караганда", LivingArea: "Пришахтинск", StreetType: "микрорайон", StreetName: "21", House: "5", Apartment: "7"}},
		{"Карагандинская обл, Майкудук, 15 мкр 3дом", map[Field]string{City: "Караганда", LivingArea: "Майкудук", StreetName: "15", House: "3"}},
		// without a street the area is the street name
		{"Караганда, Майкудук, д.30", map[Field]string{City: "Караганда", StreetName: "Майкудук", House: "30", LivingArea: "-"}},
		// an area of another city stays a plain living area, the written city wins
		{"Темиртау, Пришахтинск, ул. Ленина 5", map[Field]string{City: "Темиртау"}},
		{"Алматы, Сатпаева, 22", map[Field]string{City: "Алматы", StreetName: "Сатпаева", House: "22", LivingArea: "-"}},
		// words that only look glued must stay whole
		{"ул. Алматинская 3", map[Field]string{StreetName: "Алматинская", House: "3"}},
		{"ул. Таразская 5", map[Field]string{StreetName: "Таразская", House: "5"}},
		{"ул. Городская 7", map[Field]string{StreetName: "Городская", House: "7"}},
		{"Райымбека 10", map[Field]string{StreetName: "Райымбека", House: "10"}},
		{"Сатпаевская 3", map[Field]string{StreetName: "Сатпаевская", House: "3"}},
		{"Алматы Жолдасбекова 97", map[Field]string{City: "Алматы", StreetName: "Жолдасбекова", House: "97"}},
		{"Алматы мкр Самал-2 ул Жолдасбекова 97 кв 14", map[Field]string{LivingArea: "Самал-2", StreetType: "улица", StreetName: "Жолдасбекова", House: "97"}},
		// a ЖК name ends before the type word of the street that follows
		{"Almaty жк Нурлы Тау ул. Толе би д22 под 4 квартира 92", map[Field]string{LivingArea: "Нурлы Тау", StreetType: "улица", StreetName: "Толе би", House: "22"}},
		{"астана жк green quarter ул. жанибек хандар 21", map[Field]string{LivingArea: "green quarter", StreetName: "жанибек хандар", House: "21"}},
		{"г. Шымкент жк Мирас ул.джангельдина д.30 кв 117", map[Field]string{LivingArea: "Мирас", StreetName: "джангельдина", House: "30"}},
		{"г алматы жк. нурлы тау просп. абылай хана д55", map[Field]string{LivingArea: "нурлы тау", StreetType: "проспект", StreetName: "абылай хана", House: "55"}},
		// old and Latin names; a city of two words without "г."; a street named like a town stays a street
		{"aktyubinsk, ул. Маресьева 12", map[Field]string{City: "Актобе", Province: "Актюбинская область", StreetName: "Маресьева"}},
		{"Ермак ул. Ленина 3 кв 7", map[Field]string{City: "Аксу", Province: "Павлодарская область", StreetName: "Ленина"}},
		{"boralday Аймауытова 15", map[Field]string{City: "Боралдай", StreetName: "Аймауытова", House: "15"}},
		{"Отеген батыр ул. Сейфуллина 8", map[Field]string{City: "Отеген батыр", StreetName: "Сейфуллина", House: "8"}},
		{"новый узень Сатпаева 5 кв 3", map[Field]string{City: "Жанаозен", StreetName: "Сатпаева", House: "5"}},
		{"Өтеген батыр көшесі 14", map[Field]string{City: "-", StreetType: "улица", StreetName: "Өтеген батыр", House: "14"}},
		{"Ермекво 12 кв 5", map[Field]string{City: "-", StreetName: "Ермекво", House: "12"}},
		// Kazakh type words and four-letter typos of short markers
		{"Алматы, Абая 150 кеңсе 12", map[Field]string{House: "150", Apartment: "12"}},
		{"Астана, Хайвил ТҮК 3 пәтер 15", map[Field]string{StreetType: "жилой комплекс", StreetName: "Хайвил", House: "3", Apartment: "15"}},
		{"Шымкент, Ботаника тұйығы 6", map[Field]string{StreetType: "переулок", StreetName: "Ботаника", House: "6"}},
		{"Тараз бульв. Толе би 12", map[Field]string{StreetType: "бульвар", StreetName: "Толе би", House: "12"}},
		{"Абая 5 кв 7 этпж 3", map[Field]string{House: "5", Apartment: "7", Floor: "3"}},
		{"Тимирязева 42 кв 15 эьаж 4", map[Field]string{StreetName: "Тимирязева", House: "42", Apartment: "15", Floor: "4"}},
		{"Абая 5 офси 12", map[Field]string{House: "5", Apartment: "12"}},
		// a house after a comma; a block takes the number before it when none follows
		{"Шолохова, 21 кв 40", map[Field]string{StreetName: "Шолохова", House: "21", Apartment: "40"}},
		{"Сейфуллина 31 дом 2 блок кв 9", map[Field]string{StreetName: "Сейфуллина", House: "31", Apartment: "9"}},
		{"Абая 12 корпус", map[Field]string{StreetName: "Абая", House: "12"}},
		// a glued "д" after a fraction is the house marker, as after a whole number
		{"шымкенттауке хана12/3д45кв", map[Field]string{StreetName: "тауке хана", House: "12/3", Apartment: "45"}},
		{"Тауке хана д 12/3д", map[Field]string{House: "12/3д"}},
		// the name of a microdistrict or ЖК before its marker, the street after it
		{"Караганда, Жастар мкр Ерубаева 12 кв 4", map[Field]string{LivingArea: "Жастар", StreetName: "Ерубаева", House: "12", Apartment: "4"}},
		{"Шымкент, Нурсат шағын ауданы Байтурсынова 7", map[Field]string{LivingArea: "Нурсат", StreetName: "Байтурсынова", House: "7"}},
		{"Астана Хайвил жк Сыганак 18 кв 3", map[Field]string{LivingArea: "Хайвил", StreetName: "Сыганак", House: "18"}},
		{"Алматы Esentai City ТҮК Розыбакиева 40", map[Field]string{LivingArea: "Esentai City", StreetName: "Розыбакиева", House: "40"}},
		{"Талдыкорган \"Жастар\" мкр Шевченко 4", map[Field]string{LivingArea: "Жастар", StreetName: "Шевченко", House: "4"}},
		{"Тараз 7 шағын ауданы Толе би көшесі 15", map[Field]string{LivingArea: "7", StreetType: "улица", StreetName: "Толе би", House: "15"}},
		{"Павлодар 3 мкр. Камзина переулок 12", map[Field]string{LivingArea: "3", StreetType: "переулок", StreetName: "Камзина", House: "12"}},
		// the city written twice or with a typo before the area; a town name before көшесі is the street
		{"Костанай Костанай 5 мкр Абая 3", map[Field]string{LivingArea: "5", StreetName: "Абая", House: "3"}},
		{"шымкнет «Мирас» жк Тауке хана 12", map[Field]string{City: "Шымкент", LivingArea: "Мирас", StreetName: "Тауке хана"}},
		{"Сәтбаев көшесі 12 Алматы", map[Field]string{City: "Алматы", StreetType: "улица", StreetName: "Сәтбаев", House: "12"}},
		{"мкр Жулдыз Сатпаева ул. д. 5", map[Field]string{City: "-", LivingArea: "Жулдыз", StreetType: "улица", StreetName: "Сатпаева", House: "5"}},
		{"Сатпаева ул 12 кв 3", map[Field]string{City: "-", StreetName: "Сатпаева", House: "12", Apartment: "3"}},
		{"Almaty ZhK Koktem Garden ul. Timiryazeva 40", map[Field]string{LivingArea: "Koktem Garden", StreetName: "Timiryazeva", House: "40"}},
		// a number before the marker after a street is its house
		{"ул Абая 10 мкр Самал 5", map[Field]string{StreetName: "Абая", House: "10", LivingArea: "Самал"}},
		// a microdistrict or ЖК and a street read from the same words
		{"Тараз мкр Карасу Толе би улица 50", map[Field]string{LivingArea: "Карасу", StreetType: "улица", StreetName: "Толе би", House: "50"}},
		{"Кокшетау мкр Васильковский Ауельбекова ул 9", map[Field]string{LivingArea: "Васильковский", StreetName: "Ауельбекова", House: "9"}},
		{"Шымкент ЖК «Нұрлы жол» Тауке хана даңғылы 12", map[Field]string{LivingArea: "Нұрлы жол", StreetType: "проспект", StreetName: "Тауке хана"}},
		{"ЖК Grand Park Абылай хана ул 5", map[Field]string{LivingArea: "Grand Park", StreetName: "Абылай хана", House: "5"}},
		{"Шымкент жк Мирас Сайрам көш. д 4", map[Field]string{LivingArea: "Мирас", StreetType: "улица", StreetName: "Сайрам", House: "4"}},
		{"Усть-Каменогорск ул Протозанова Ахмирово мкр д 5", map[Field]string{StreetName: "Протозанова", LivingArea: "Ахмирово", House: "5"}},
		// an area name close to a town before its type and a street type is the area, not Сарань
		{"Сайран мкрн Ул. Ходжанова 43 кв208", map[Field]string{City: "-", LivingArea: "Сайран", StreetType: "улица", StreetName: "Ходжанова", House: "43", Apartment: "208"}},
		{"Актау, д 12, ул Бостандык 9 мкр", map[Field]string{StreetName: "Бостандык", LivingArea: "9", House: "12"}},
		// a quoted area name ends at its closing quote
		{"Тараз жк \"Алтын Орда\" Абая 7", map[Field]string{LivingArea: "Алтын Орда", StreetName: "Абая", House: "7"}},
		{"Кызылорда мкр «Шугыла» Жахаева 15", map[Field]string{LivingArea: "Шугыла", StreetName: "Жахаева", House: "15"}},
		// a type word inside a ЖК name, unlike a type of the street before the ЖК
		{"Астана, «Изумрудный Квартал» ЖК, Кабанбай батыра пр., 11", map[Field]string{LivingArea: "Изумрудный Квартал", StreetName: "Кабанбай батыра", StreetType: "проспект"}},
		{"Алматы Парк Бульвар тұрғын үй кешені Сатпаева 30", map[Field]string{LivingArea: "Парк Бульвар", StreetName: "Сатпаева", House: "30"}},
		{"Шымкент жк Самал Бульвар Тауке хана ул 8", map[Field]string{LivingArea: "Самал Бульвар", StreetType: "улица", StreetName: "Тауке хана", House: "8"}},
		{"Астана Абая пр. ЖК Хайвил 5", map[Field]string{LivingArea: "Хайвил", StreetType: "проспект", StreetName: "Абая", House: "5"}},
		{"Алматы Баганашыл мкр. Сейфуллина 44", map[Field]string{LivingArea: "Баганашыл", StreetName: "Сейфуллина", House: "44"}},
		// a large city mistyped in four letters, but not a word of a street name or a line naming its city
		{"Смей, ул. Абая 12", map[Field]string{City: "Семей", Province: "область Абай", StreetName: "Абая", House: "12"}},
		{"тарз ул Толе би 15", map[Field]string{City: "Тараз", StreetName: "Толе би", House: "15"}},
		{"гор. Семй, ул. Абая 3", map[Field]string{City: "Семей", StreetName: "Абая"}},
		{"дом 5, ул. Акын Сара", map[Field]string{City: "-", StreetName: "Акын Сара", House: "5"}},
		{"Esim khana 7, Taraz", map[Field]string{City: "Тараз", StreetName: "Esim khana", House: "7"}},
		{"Лисаков, 3 мкр, 12", map[Field]string{City: "Лисаковск", StreetName: "3", House: "12"}},
		// a city glued to a street the map knows there: a short city, a type word that starts the street name,
		// a rest that starts like an adjective, a close spelling of the city
		{"Аксупролетарская 11", map[Field]string{City: "Аксу", StreetName: "пролетарская", House: "11"}},
		{"Актобепрохорова 14 кв 3", map[Field]string{City: "Актобе", StreetName: "прохорова", House: "14", Apartment: "3"}},
		{"Алматыевропейская 9", map[Field]string{City: "Алматы", StreetName: "европейская", House: "9"}},
		{"алматжандосова 21", map[Field]string{City: "Алматы", StreetName: "жандосова", House: "21"}},
		{"Таразская 5", map[Field]string{City: "-", StreetName: "Таразская"}},
		{"оралхана бокея 12", map[Field]string{City: "-", StreetName: "оралхана бокея"}},
		// a typo as close to a regional centre as to a smaller town is the centre; old names of towns
		{"акту 12 мкр, 7", map[Field]string{City: "Актау", Province: "Мангистауская область", StreetName: "12", House: "7"}},
		{"с. Георгиевка, ул. Абая 3", map[Field]string{City: "Георгиевка"}}, // not Кордай: villages so called live in several regions
		{"Восточно-Казахстанская область, с. Георгиевка, ул. Абая 3", map[Field]string{City: "Георгиевка", Province: "Восточно-Казахстанская область"}},
		{"Капчагай, ул. Абая 5", map[Field]string{City: "Конаев"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				got, ok := o.Values[f]
				if want == "-" {
					if ok {
						t.Errorf("%s = %q, want empty", f, got)
					}
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}
}

func TestSourcesAndSpans(t *testing.T) {
	raw := "город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70"
	o := Parse(raw)
	if o.SourceOf(District) != SourceDirectory || o.SourceOf(StreetName) != SourceString || o.SourceOf(Entrance) != SourceAbsent {
		t.Fatalf("sources = %v", o.Sources)
	}
	u := []rune(raw) // raw is BMP-only, so UTF-16 offsets equal rune offsets
	found := false
	for i, s := range o.Spans {
		if i > 0 && s.Start < o.Spans[i-1].End {
			t.Fatalf("overlapping spans: %v", o.Spans)
		}
		if s.Field == StreetName && string(u[s.Start:s.End]) == "Аксай 3а" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no street_name span for «Аксай 3а»: %v", o.Spans)
	}
	if o.Status() != "confident" {
		t.Fatalf("status = %s (%.2f)", o.Status(), o.Confidence)
	}
}

// Repeated markers used to loop forever in units().
func TestRepeatedMarkersTerminate(t *testing.T) {
	for _, raw := range []string{"кв 5 кв 6", "ул Абая 10, оф 5, кв 3", "дом 1 дом 2", "Абая 10 2 этаж 3 эт", "ул Абая 10 кв 5 кв.5"} {
		done := make(chan Output, 1)
		go func() { done <- Parse(raw) }()
		select {
		case o := <-done:
			if raw == "кв 5 кв 6" && o.Values[Apartment] != "5" {
				t.Errorf("%q: apartment = %q", raw, o.Values[Apartment])
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%q: Parse did not return", raw)
		}
	}
}

func TestSuggest(t *testing.T) {
	for raw, want := range map[string]string{
		"астана кенесары 40 кв 12":             "Астана, улица Кенесары, д. 40, кв. 12",
		"алматыы толеби 286 kv 41":             "Алматы, улица Толе би, д. 286, кв. 41",
		"Караганды бухаржырау 50":              "Карагандинская область, Караганда, проспект Бухар жырау, д. 50",
		"Темиртау ул. Мира 4":                  "Карагандинская область, Темиртау, улица Мира, д. 4",
		"Алматы, мкр Самал-2, Жолдасбекова 97": "Алматы, Самал-2, улица Жолдасбекова, д. 97",
	} {
		if got := Parse(raw).Suggest(); got != want {
			t.Errorf("Suggest(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestGarbageIsLowConfidence(t *testing.T) {
	for _, raw := range []string{"", "   ", "???", "привет как дела"} {
		o := Parse(raw)
		if o.Confidence >= 0.5 {
			t.Errorf("%q: confidence %.2f", raw, o.Confidence)
		}
	}
}

// A record with no parsed field gets confidence 0, as the output contract says.
func TestNothingParsedIsZeroConfidence(t *testing.T) {
	for _, raw := range []string{"", "   ", "???", "привет"} {
		if o := Parse(raw); len(o.Values) != 0 || o.Confidence != 0 {
			t.Errorf("%q: values %v, confidence %.2f", raw, o.Values, o.Confidence)
		}
	}
}

// Several stages are quadratic in tokens: a megabyte in one record must not stall a batch.
func TestLongInputIsBounded(t *testing.T) {
	for _, raw := range []string{strings.Repeat(",", 1<<20), strings.Repeat("1 кв ", 200000), "г " + strings.Repeat("а ", 500000),
		strings.Repeat("а", 1<<20), strings.Repeat("аБ", 1<<19)} {
		done := make(chan struct{})
		start := time.Now()
		go func() { Parse(raw); close(done) }()
		select {
		case <-done:
			if d := time.Since(start); d > 100*time.Millisecond {
				t.Errorf("%.12q…: %v", raw, d)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%.12q…: Parse did not return", raw)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		Parse("город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70")
	}
}
