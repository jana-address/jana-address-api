package parser

import "strings"

// Suggestion is the address written back in a clean form for the operator:
// "Астана, проспект Мәңгілік Ел, д. 55, кв. 12".
// The street name in the output fields stays verbatim; only the suggestion
// uses the directory spelling of a well-known street.

type knownStreet struct{ name, typ string }

// knownStreets holds main streets of the largest cities until the full OSM directory is loaded.
var knownStreets = map[string][]knownStreet{}

func init() {
	add := func(city, typ string, names ...string) {
		for _, n := range names {
			knownStreets[city] = append(knownStreets[city], knownStreet{n, typ})
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
	return strings.NewReplacer(" ", "", "-", "", ".", "").Replace(fold(s))
}

// matchStreet finds a single well-known street within a small typo distance of the written name.
func matchStreet(city, name string) (knownStreet, bool) {
	w := []rune(squash(name))
	limit := fuzzyLimit(len(w))
	var best knownStreet
	bestD, ambiguous := limit+1, false
	for _, s := range knownStreets[city] {
		d := osaDistance(w, []rune(squash(s.name)))
		switch {
		case d < bestD:
			best, bestD, ambiguous = s, d, false
		case d == bestD && s.name != best.name:
			ambiguous = true
		}
	}
	return best, bestD <= limit && !ambiguous
}

func title(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// Suggest renders the parsed fields as a clean address line.
func (o Output) Suggest() string {
	v := o.Values
	var parts []string
	if p := v[Province]; p != "" && p != v[City] {
		parts = append(parts, p)
	}
	if c := v[City]; c != "" {
		parts = append(parts, c)
	}
	if d := v[District]; d != "" && o.Sources[District] == SourceString {
		if strings.HasSuffix(d, "ий") {
			parts = append(parts, d+" район")
		} else {
			parts = append(parts, "район "+d)
		}
	}
	if la := v[LivingArea]; la != "" {
		parts = append(parts, la)
	}
	if name := v[StreetName]; name != "" {
		typ := v[StreetType]
		if s, ok := matchStreet(v[City], name); ok {
			name = s.name
			if typ == "" {
				typ = s.typ
			}
		} else {
			name = title(name)
		}
		if typ != "" {
			name = typ + " " + name
		}
		parts = append(parts, name)
	}
	for _, f := range []struct {
		f      Field
		prefix string
	}{{House, "д. "}, {Apartment, "кв. "}, {Entrance, "подъезд "}, {Floor, "этаж "}} {
		if x := v[f.f]; x != "" {
			parts = append(parts, f.prefix+x)
		}
	}
	if l := v[Landmark]; l != "" {
		parts = append(parts, "("+l+")")
	}
	return strings.Join(parts, ", ")
}
