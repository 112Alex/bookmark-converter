package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

// Структуры для парсинга JSON файла закладок Chrome
type Bookmarks struct {
	Roots    Roots  `json:"roots"`
	Version  int    `json:"version"`
	Checksum string `json:"checksum"`
}

type Roots struct {
	BookmarkBar BookmarkFolder `json:"bookmark_bar"`
	Other       BookmarkFolder `json:"other"`
	Synced      BookmarkFolder `json:"synced"`
}

type BookmarkFolder struct {
	Children     []BookmarkItem `json:"children"`
	DateAdded    string         `json:"date_added"`
	DateModified string         `json:"date_modified,omitempty"`
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
}

type BookmarkItem struct {
	DateAdded string         `json:"date_added"`
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	URL       string         `json:"url,omitempty"`
	Children  []BookmarkItem `json:"children,omitempty"`
}

// getBookmarksFilePath возвращает путь к файлу закладок Chrome
func getBookmarksFilePath() (string, error) {
	// Путь к файлу закладок Chrome в Windows
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return "", fmt.Errorf("не удалось получить переменную окружения USERPROFILE")
	}

	bookmarksPath := filepath.Join(userProfile, "AppData", "Local", "Google", "Chrome", "User Data", "Default", "Bookmarks")

	// Проверяем существование файла
	if _, err := os.Stat(bookmarksPath); os.IsNotExist(err) {
		return "", fmt.Errorf("файл закладок не найден по пути: %s", bookmarksPath)
	}

	return bookmarksPath, nil
}

// parseBookmarks парсит файл закладок Chrome
func parseBookmarks(filePath string) (*Bookmarks, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла закладок: %w", err)
	}

	var bookmarks Bookmarks
	err = json.Unmarshal(data, &bookmarks)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	return &bookmarks, nil
}

// initDB инициализирует базу данных SQLite
func initDB(dbPath string) (*sql.DB, error) {
	// Удаляем существующий файл базы данных, если он существует
	if _, err := os.Stat(dbPath); err == nil {
		err = os.Remove(dbPath)
		if err != nil {
			return nil, fmt.Errorf("ошибка удаления существующей базы данных: %w", err)
		}
	}

	// Открываем соединение с базой данных
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия базы данных: %w", err)
	}

	// Создаем таблицу для закладок с двумя колонками
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS bookmarks (
		name TEXT NOT NULL,
		url TEXT NOT NULL PRIMARY KEY
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка создания таблицы: %w", err)
	}

	return db, nil
}

// extractBookmarks рекурсивно извлекает закладки из структуры
func extractBookmarks(items []BookmarkItem) []BookmarkItem {
	var result []BookmarkItem

	for _, item := range items {
		if item.Type == "url" {
			result = append(result, item)
		} else if item.Type == "folder" && len(item.Children) > 0 {
			// Рекурсивно обрабатываем вложенные папки
			childBookmarks := extractBookmarks(item.Children)
			result = append(result, childBookmarks...)
		}
	}

	return result
}

// saveBookmarksToDB сохраняет закладки в базу данных
func saveBookmarksToDB(db *sql.DB, bookmarks []BookmarkItem) error {
	// Подготавливаем SQL запрос для вставки
	stmt, err := db.Prepare("INSERT OR REPLACE INTO bookmarks(name, url) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("ошибка подготовки SQL запроса: %w", err)
	}
	defer stmt.Close()

	// Вставляем каждую закладку в базу данных
	for _, bookmark := range bookmarks {
		_, err = stmt.Exec(bookmark.Name, bookmark.URL)
		if err != nil {
			return fmt.Errorf("ошибка вставки закладки: %w", err)
		}
	}

	return nil
}

// GetAllBookmarks получает все закладки из базы данных
func GetAllBookmarks(db *sql.DB) ([]BookmarkRecord, error) {
	rows, err := db.Query("SELECT name, url FROM bookmarks")
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer rows.Close()

	var bookmarks []BookmarkRecord
	for rows.Next() {
		var bookmark BookmarkRecord

		err := rows.Scan(&bookmark.Name, &bookmark.URL)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}

		bookmarks = append(bookmarks, bookmark)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по строкам: %w", err)
	}

	return bookmarks, nil
}

// BookmarkRecord представляет запись о закладке в базе данных
type BookmarkRecord struct {
	Name string
	URL  string
}

// PrintBookmarks выводит список закладок в консоль
func PrintBookmarks(db *sql.DB) error {
	bookmarks, err := GetAllBookmarks(db)
	if err != nil {
		return fmt.Errorf("ошибка получения закладок: %w", err)
	}

	fmt.Println("Список закладок:")
	fmt.Println("-----------------------------------------------------------------------")
	fmt.Printf("| %-30s | %-40s |\n", "Название", "URL")
	fmt.Println("-----------------------------------------------------------------------")

	for _, bookmark := range bookmarks {
		// Обрезаем длинные названия и URL для красивого вывода
		name := bookmark.Name
		if len(name) > 27 {
			name = name[:24] + "..."
		}

		url := bookmark.URL
		if len(url) > 37 {
			url = url[:34] + "..."
		}

		fmt.Printf("| %-30s | %-40s |\n", name, url)
	}

	fmt.Println("-----------------------------------------------------------------------")
	fmt.Printf("Всего закладок: %d\n", len(bookmarks))

	return nil
}

func main() {
	// Получаем путь к файлу закладок
	bookmarksPath, err := getBookmarksFilePath()
	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}

	fmt.Printf("Найден файл закладок: %s\n", bookmarksPath)

	// Парсим файл закладок
	bookmarks, err := parseBookmarks(bookmarksPath)
	if err != nil {
		log.Fatalf("Ошибка при парсинге закладок: %v", err)
	}

	// Инициализируем базу данных
	dbPath := "bookmarks.db"
	db, err := initDB(dbPath)
	if err != nil {
		log.Fatalf("Ошибка инициализации базы данных: %v", err)
	}
	defer db.Close()

	// Извлекаем все закладки из всех разделов
	var allBookmarks []BookmarkItem
	allBookmarks = append(allBookmarks, extractBookmarks(bookmarks.Roots.BookmarkBar.Children)...)
	allBookmarks = append(allBookmarks, extractBookmarks(bookmarks.Roots.Other.Children)...)
	allBookmarks = append(allBookmarks, extractBookmarks(bookmarks.Roots.Synced.Children)...)

	// Сохраняем закладки в базу данных
	err = saveBookmarksToDB(db, allBookmarks)
	if err != nil {
		log.Fatalf("Ошибка при сохранении закладок в базу данных: %v", err)
	}

	fmt.Printf("Успешно сохранено %d закладок в базу данных %s\n", len(allBookmarks), dbPath)

	// Выводим список закладок в консоль
	err = PrintBookmarks(db)
	if err != nil {
		log.Printf("Ошибка при выводе закладок: %v", err)
	}
}
