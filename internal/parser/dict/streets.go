package dict

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// KnownStreet is a well-known street in the directory spelling.
type KnownStreet struct{ Name, Type string }

// knownStreets holds main streets of the largest cities until the full OSM directory is loaded.
var knownStreets = map[string][]KnownStreet{}

func init() {
	add := func(city, typ string, names ...string) {
		for _, n := range names {
			knownStreets[city] = append(knownStreets[city], KnownStreet{n, typ})
		}
	}
	add("Астана", "проспект", "Мәңгілік Ел", "Қабанбай батыр", "Тұран", "Республики", "Абая", "Сарыарка", "Улы Дала",
		"Богенбай батыра", "Бауыржан Момышулы", "Жеңіс", "Кошкарбаева", "Шәкәрім Құдайбердіұлы")
	add("Астана", "улица", "Кенесары", "Сығанақ", "Кунаева", "Достык", "Сауран", "Бокейхана", "Иманова", "Бейбитшилик",
		"Ауэзова", "Сейфуллина", "Желтоксан", "Пушкина", "Петрова", "Жубанова", "Орынбор", "Акмешит")
	add("Алматы", "проспект", "Абая", "Аль-Фараби", "Достык", "Райымбека", "Назарбаева", "Жибек Жолы", "Сейфуллина", "Гагарина", "Рыскулова", "Суюнбая")
	add("Алматы", "улица", "Толе би", "Сатпаева", "Жандосова", "Розыбакиева", "Тимирязева", "Жолдасбекова", "Байтурсынова",
		"Шевченко", "Гоголя", "Курмангазы", "Кабанбай батыра", "Богенбай батыра", "Муканова", "Навои", "Утепова",
		"Байзакова", "Ауэзова", "Манаса", "Желтоксан", "Фурманова", "Тулебаева", "Панфилова", "Торайгырова", "Жамбыла")
	add("Шымкент", "проспект", "Тауке хана", "Кунаева", "Республики", "Байдибек би", "Абая")
	add("Шымкент", "улица", "Байтурсынова", "Толе би", "Казыбек би", "Мадели кожа", "Жибек жолы")
	add("Караганда", "проспект", "Бухар жырау", "Нуркена Абдирова", "Строителей", "Республики")
	add("Караганда", "улица", "Ерубаева", "Гоголя", "Алиханова", "Ермекова", "Мустафина")
	add("Актобе", "проспект", "Абилкайыр хана", "Санкибай батыра", "Алии Молдагуловой", "Маресьева")
	add("Актобе", "улица", "Бр. Жубановых", "Есет батыра", "Маметовой")
}

func squash(s string) string {
	return strings.NewReplacer(" ", "", "-", "", ".", "").Replace(text.Fold(s))
}

// MatchStreet finds a single well-known street within a small typo distance of the written name.
func MatchStreet(city, name string) (KnownStreet, bool) {
	w := []rune(squash(name))
	limit := fuzzyLimit(len(w))
	var best KnownStreet
	bestD, ambiguous := limit+1, false
	for _, s := range knownStreets[city] {
		d := text.Distance(w, []rune(squash(s.Name)))
		switch {
		case d < bestD:
			best, bestD, ambiguous = s, d, false
		case d == bestD && s.Name != best.Name:
			ambiguous = true
		}
	}
	return best, bestD <= limit && !ambiguous
}
