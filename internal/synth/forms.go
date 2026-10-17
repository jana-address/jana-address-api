package synth

// form is a written variant; k marks it: 'k' Kazakh, 'l' Latin, 't' typo.
type form struct {
	s string
	k byte
}

// markers are the ways a number is labeled. Prefix forms end with a space,
// a dot or nothing (glued); postfix forms start with a space or nothing.
type markers struct {
	pre, post         []string
	kzPre, kzPost     []string
	latPre, latPost   []string
	typoPre, typoPost []string
	careful           []string
}

var aptM = markers{
	pre:     list("кв ", "кв ", "кв ", "кв.", "кв. ", "кв", "квартира ", "кв-ра "),
	post:    list(" кв", "кв", " кв."),
	kzPre:   list("пәтер ", "пәтер ", "патер "),
	kzPost:  list(" пәтер", " патер"),
	latPre:  list("kv ", "kv ", "kv.", "kv", "flat ", "apt "),
	latPost: list(" kv"),
	typoPre: list("кваритра ", "квартра ", "кварира ", "квю "),
	careful: list("кв. ", "кв. ", "кв.", "квартира "),
}

var entM = markers{
	pre:      list("подъезд ", "под ", "под ", "под.", "под. ", "подъезд"),
	post:     list(" подъезд", "подъезд", " под", " под."),
	kzPre:    list("кіреберіс ", "кіреберіс "),
	kzPost:   list(" кіреберіс"),
	latPre:   list("pod ", "pod.", "podezd "),
	latPost:  list(" pod", " podezd"),
	typoPre:  list("подьезд ", "подлъезд ", "подезд ", "пожъезд "),
	typoPost: list(" подьезд", "подьезд"),
	careful:  list("подъезд ", "под. ", "подъезд "),
}

var floorM = markers{
	pre:      list("этаж ", "эт ", "эт ", "эт.", "эт. ", "этаж"),
	post:     list(" этаж", " эт", "эт", " эт."),
	kzPre:    list("қабат ", "кабат "),
	kzPost:   list(" қабат", " кабат"),
	latPre:   list("etazh ", "et ", "et.", "floor "),
	latPost:  list(" etazh", " et"),
	typoPre:  list("этж ", "етаж ", "эьаж "),
	typoPost: list(" этж", "этж"),
	careful:  list("этаж ", "эт. "),
}

var officeM = markers{
	pre:     list("оф ", "оф.", "оф. ", "офис ", "офис "),
	post:    list(" офис"),
	kzPre:   list("кеңсе "),
	latPre:  list("of. ", "office "),
	typoPre: list("офси "),
	careful: list("оф. ", "офис "),
}

var blockM = markers{
	pre:     list("корп ", "корп. ", "корпус ", "блок ", "блок "),
	post:    list(" корпус", " блок"),
	kzPre:   list("блок "),
	kzPost:  list(" блок"),
	latPre:  list("korp ", "blok "),
	typoPre: list("корпс "),
	careful: list("корп. ", "корпус ", "блок "),
}

var houseM = markers{
	pre:      list("д ", "д ", "д.", "д. ", "дом ", "д"),
	post:     list(" дом", "дом"),
	kzPre:    list("үй ", "уй "),
	kzPost:   list(" үй", " уй", "үй"),
	latPre:   list("d ", "d.", "dom "),
	latPost:  list(" dom"),
	typoPre:  list("дм ", "домм "),
	typoPost: list(" дои"),
	careful:  list("д. ", "дом ", "д."),
}

// typeForms are the ways a street type word is written.
type typeForms struct {
	pre, post, kzPost, latPre, typoPre, careful []string
}

var typeWords = map[string]typeForms{
	"улица": {
		pre:     list("ул ", "ул ", "ул. ", "ул. ", "ул.", "улица "),
		post:    list(" ул", " улица", " ул."),
		kzPost:  list(" көшесі", " көшесі", " кошеси", " көш."),
		latPre:  list("ul ", "ul. ", "ul.", "ulitsa "),
		typoPre: list("улца ", "уица ", "улиц "),
		careful: list("ул. ", "ул. ", "ул.", "улица "),
	},
	"проспект": {
		pre:     list("пр ", "пр. ", "пр.", "пр-т ", "пр-кт ", "просп. ", "проспект "),
		post:    list(" пр", " проспект", " пр-т"),
		kzPost:  list(" даңғылы", " даңғылы", " дангылы", " даң."),
		latPre:  list("pr ", "pr. ", "prospekt ", "pr-t "),
		typoPre: list("проспкт ", "прспект ", "проспек "),
		careful: list("пр. ", "пр-т ", "проспект "),
	},
	mkrType: {
		pre:     list("мкр ", "мкр ", "мкр. ", "мкр.", "мкрн ", "мкрн. ", "микрорайон "),
		post:    list(" мкр", " мкр.", " микрорайон", " мкрн"),
		kzPost:  list(" шағын ауданы", " шағын аудан", " шагын ауданы", " ықшам ауданы"),
		latPre:  list("mkr ", "mkr. ", "mkrn ", "mikrorayon "),
		typoPre: list("микрорайн ", "микраройон ", "микрарайон ", "мкп "),
		careful: list("мкр. ", "мкр. ", "мкрн. ", "микрорайон "),
	},
	"переулок": {
		pre:     list("пер ", "пер. ", "пер.", "переулок "),
		post:    list(" пер", " переулок", " пер."),
		kzPost:  list(" тұйығы", " тұйық", " туйыгы"),
		latPre:  list("per. ", "pereulok "),
		typoPre: list("переулк ", "перулок "),
		careful: list("пер. ", "переулок "),
	},
	"бульвар": {
		pre:     list("б-р ", "бул. ", "бульв. ", "бульвар "),
		post:    list(" бульвар", " б-р"),
		kzPost:  list(" бульвары", " желекжолы"),
		latPre:  list("bulvar ", "b-r "),
		typoPre: list("бульар ", "бульавр "),
		careful: list("б-р ", "бульвар "),
	},
	"шоссе": {
		pre:     list("ш. ", "шоссе "),
		post:    list(" шоссе", " ш.", " шоссе"),
		kzPost:  list(" тас жолы", " тасжолы"),
		latPre:  list("shosse "),
		typoPre: list("шосе "),
		careful: list("шоссе "),
	},
	zhkType: {
		pre:     list("ЖК ", "ЖК ", "жк ", "ЖК. ", "Ж/К ", "ж.к. ", "жилой комплекс "),
		post:    list(" ЖК", " жк"),
		kzPost:  list(" тұрғын үй кешені", " ТҮК"),
		latPre:  list("zhk ", "ZhK ", "JK "),
		typoPre: list("жилой комлекс ", "жилой копмлекс "),
		careful: list("ЖК ", "ЖК ", "жилой комплекс "),
	},
}

var abbrevs = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range list("ул", "пр", "пр-т", "пр-кт", "просп", "мкр", "мкрн", "пер", "б-р", "бул", "бульв", "ш",
		"жк", "ж/к", "ж.к", "түк", "кв", "кв-ра", "д", "под", "эт", "этж", "оф", "корп", "г", "гор", "көш", "даң",
		"ul", "pr", "pr-t", "mkr", "mkrn", "per", "b-r", "kv", "d", "pod", "et", "of", "korp", "zhk", "jk",
		"рк", "kz") {
		m[s] = true
	}
	return m
}()
