package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed templates/*.html
var tplFS embed.FS

//go:embed static
var staticFS embed.FS

// ---------- Модель данных страницы ----------

type Link struct {
	Title string
	URL   string
}

type Stat struct {
	Value string
	Label string
}

type Step struct {
	Title string
	Text  string
}

type Tab struct {
	ID       string
	Title    string
	Heading  string
	Subtitle string
	Points   []string
	LinkURL  string
}

type Contact struct {
	Label string
	Email string
}

type Page struct {
	Title      string
	Lang       string
	Langs      []string
	Nav        []Link
	LoginURL   string
	HeroTitle  string
	HeroText   string
	Stats      []Stat
	Partners   []string
	StepsTitle string
	Steps      []Step
	AppTitle   string
	AppText    string
	HelpTitle  string
	Tabs       []Tab
	Contacts   []Contact
	Address    string
	Socials    []Link
	Legal      []Link
	Copyright  string
}

func newPage() Page {
	return Page{
		Title:    "Jańa Post",
		Lang:     "Русский",
		Langs:    []string{"Қазақша", "Русский", "English", "中文"},
		LoginURL: "https://janapost.kz/web/lite/#/pages/login/login",
		Nav: []Link{
			{"Клиенту", "/ru/home"},
			{"Бизнесу", "/ru/v2/business"},
			{"Партнерам", "/ru/partners"},
			{"О нас", "/ru/v2/about"},
			{"Карьера", "/ru/v2/careers"},
			{"Уроки", "/ru/v2/lessons"},
			{"Тарифы", "/ru/home/tariffs"},
		},
		HeroTitle: "Мы доставляем возможности",
		HeroText:  "Принимаем и доставляем все отправления с маркетплейсов",
		Stats: []Stat{
			{"10+ лет", "Опыт на рынке"},
			{"1000+ филиалов", "По Центральной Азии"},
			{"3+ млн", "Пользователей в приложении"},
			{"5 стран", "Международная доставка"},
		},
		// Названия партнёров на странице подгружаются картинками,
		// поэтому здесь заглушки — замените на свои логотипы.
		Partners:   []string{"Partner 1", "Partner 2", "Partner 3", "Partner 4", "Partner 5"},
		StepsTitle: "Инструкция по оформлению заказа",
		Steps: []Step{
			{"Скачать приложение", "Скачайте Jańa Post в App Store или Play Market"},
			{"Оформить заказ", "Оформите заказ с маркетплейсов за несколько минут"},
			{"Отслеживать заказ", "Мы отслеживаем все заказы, вы всегда знаете статус"},
			{"Получить заказ", "Заберите в отделении или дождитесь курьера"},
		},
		AppTitle:  "Мобильное приложение Jańa Post",
		AppText:   "Доступ к статусу заказа и информации о доставке через мобильное приложение",
		HelpTitle: "Как Jańa Post может вам помочь?",
		Tabs: []Tab{
			{
				ID: "business", Title: "Бизнесу",
				Heading:  "Доставка для бизнеса",
				Subtitle: "Доставка, способствующая развитию вашего бизнеса",
				Points: []string{
					"Доставка по Казахстану",
					"Выгодные тарифы",
					"Стабильная доступность",
					"Сеть складов и пунктов выдачи",
				},
				LinkURL: "/ru/v2/business",
			},
			{
				// Текст вкладки «Партнёрам» на странице не отдаётся в HTML — это заглушка.
				ID: "partners", Title: "Партнёрам",
				Heading:  "Партнёрская программа",
				Subtitle: "Развивайте сеть пунктов выдачи вместе с нами",
				Points: []string{
					"Прозрачные условия сотрудничества",
					"Поддержка на каждом этапе",
					"Рост за счёт потока клиентов",
				},
				LinkURL: "/ru/partners",
			},
		},
		Contacts: []Contact{
			{"Общие вопросы", "feedback@janapost.kz"},
			{"Жалобы и обратная связь", "info@janapost.kz"},
			{"Партнёрство", "commercial@janapost.kz"},
		},
		Address: "Қалқаман-2 шағын ауданы, Қарақұлов көш., 40 үй, 4 этаж. 050006",
		Socials: []Link{
			{"Instagram", "https://www.instagram.com/janapost.kazakhstan/"},
			{"YouTube", "https://youtube.com/@janapost.kazakhstan"},
			{"Telegram", "https://t.me/janapostt"},
			{"LinkedIn", "https://www.linkedin.com/company/jana-post-llp/"},
			{"TikTok", "https://www.tiktok.com/@janapost.app"},
			{"Facebook", "https://www.facebook.com/profile.php?id=61551861065461"},
		},
		Legal: []Link{
			{"Договор", "/ru/home/public-document/2"},
			{"Публичный договор", "/ru/home/public-document/1"},
			{"Запрещенные товары", "/ru/prohibited"},
		},
		Copyright: "© 2023–2026 Jańa Post. БИН 230340044188. Все права защищены.",
	}
}

// ---------- Обработчики ----------

func main() {
	tpl := template.Must(template.New("").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}).ParseFS(tplFS, "templates/*.html"))

	mux := http.NewServeMux()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	home := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/ru/home" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tpl.ExecuteTemplate(w, "index.html", newPage()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	mux.HandleFunc("/", home)
	mux.HandleFunc("/ru/home", home)

	// Поле «Поиск» в hero: отслеживание по трек-номеру (демо-заглушка).
	mux.HandleFunc("/api/track", func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("q"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Введите трек-номер"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"code":   code,
			"status": "Заказ не найден в демо-режиме. Подключите реальный API.",
		})
	})

	addr := ":" + envOr("PORT", "8080")
	log.Println("Jańa Post на http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
